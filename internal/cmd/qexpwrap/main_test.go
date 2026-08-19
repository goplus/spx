/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/gcexportdata"
)

func TestExpandDirectCallsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "direct_calls.txt")
	contents := "\r\n" + `
# Package functions.
*

List.* # Small container type.
Value.*
Game.Timer`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	args := []string{"-outdir", "out", "-directcalls-file", path, "example.com/pkg"}
	got, err := expandDirectCallsFile(args)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-outdir", "out", "-directcalls", "*,List.*,Value.*,Game.Timer", "example.com/pkg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
}

func TestExpandDirectCallsFileEqualsForm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "direct_calls.txt")
	if err := os.WriteFile(path, []byte("List.*\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := expandDirectCallsFile([]string{"-directcalls-file=" + path, "example.com/pkg"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-directcalls=List.*", "example.com/pkg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
}

func TestExpandDirectCallsFileWithoutFlag(t *testing.T) {
	tests := [][]string{
		{"-outdir", "out", "example.com/pkg"},
		{"--", "-directcalls-file", "not-a-flag.txt"},
	}
	for _, args := range tests {
		got, err := expandDirectCallsFile(args)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, args) {
			t.Fatalf("args = %q; want %q", got, args)
		}
	}
}

func TestExpandDirectCallsFileRejectsInvalidConfiguration(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(empty, []byte("# No selectors.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing path", args: []string{"-directcalls-file"}, want: "requires a path"},
		{name: "empty equals path", args: []string{"-directcalls-file="}, want: "requires a path"},
		{name: "missing file", args: []string{"-directcalls-file", empty + ".missing"}, want: empty + ".missing"},
		{name: "empty file", args: []string{"-directcalls-file", empty}, want: "contains no selectors"},
		{name: "combined flags", args: []string{"-directcalls", "all", "-directcalls-file", empty}, want: "cannot be combined"},
		{name: "combined equals flags", args: []string{"-directcalls=all", "-directcalls-file=" + empty}, want: "cannot be combined"},
		{name: "duplicate file flag", args: []string{"-directcalls-file", empty, "-directcalls-file", empty}, want: "may only be specified once"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := expandDirectCallsFile(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want substring %q", err, test.want)
			}
		})
	}
}

func TestCanonicalizeTypesDataRemovesMachineSpecificPaths(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "checkout-a")
	rootB := filepath.Join(t.TempDir(), "checkout-b")
	dataA := exportTestTypes(t, filepath.Join(rootA, "source.go"))
	dataB := exportTestTypes(t, filepath.Join(rootB, "source.go"))

	canonicalA, err := canonicalizeTypesData(dataA, "example.com/pkg", sourceRoots{moduleRoot: rootA})
	if err != nil {
		t.Fatal(err)
	}
	canonicalB, err := canonicalizeTypesData(dataB, "example.com/pkg", sourceRoots{moduleRoot: rootB})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalA, canonicalB) {
		t.Fatal("canonical type data differs across checkout paths")
	}

	fset := token.NewFileSet()
	pkg, err := gcexportdata.Read(bytes.NewReader(canonicalA), fset, make(map[string]*types.Package), "example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	got := fset.Position(pkg.Scope().Lookup("Number").Pos()).Filename
	if want := "$MODULE/source.go"; got != want {
		t.Fatalf("source filename = %q; want %q", got, want)
	}
}

func TestCanonicalSourcePath(t *testing.T) {
	base := t.TempDir()
	roots := sourceRoots{
		moduleRoot:  filepath.Join(base, "work", "spx"),
		goRoot:      filepath.Join(base, "go"),
		moduleCache: filepath.Join(base, "module-cache"),
	}
	tests := map[string]string{
		filepath.Join(roots.moduleRoot, "game.go"):                        "$MODULE/game.go",
		filepath.Join(roots.goRoot, "src", "sync", "mutex.go"):            "$GOROOT/src/sync/mutex.go",
		filepath.Join(roots.moduleCache, "example.com", "mod@v1", "a.go"): "$GOMODCACHE/example.com/mod@v1/a.go",
		filepath.Join(base, "unrecognized", "source.go"):                  "$ABS/source.go",
		filepath.Join("relative", "generated", "source.go"):               "relative/generated/source.go",
	}
	for filename, want := range tests {
		if got := canonicalSourcePath(filename, roots); got != want {
			t.Errorf("canonicalSourcePath(%q) = %q; want %q", filename, got, want)
		}
	}
}

func exportTestTypes(t *testing.T, filename string) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file := fset.AddFile(filename, -1, len("package pkg\n"))
	file.SetLinesForContent([]byte("package pkg\n"))
	pkg := types.NewPackage("example.com/pkg", "pkg")
	pkg.Scope().Insert(types.NewTypeName(file.Pos(0), pkg, "Number", types.Typ[types.Int]))
	pkg.MarkComplete()

	var buf bytes.Buffer
	if err := gcexportdata.Write(&buf, fset, pkg); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
