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
	"runtime"
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

func TestCanonicalizeOutputDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "example.com", "pkg")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, "pkg.types")
	if err := os.WriteFile(filename, exportTestTypes(t, filepath.Join(root, "source.go")), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "pkg.go")
	if err := os.WriteFile(source, []byte("package pkg\n"), 0600); err != nil {
		t.Fatal(err)
	}
	untouched := filepath.Join(dir, "asset.txt")
	if err := os.WriteFile(untouched, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := canonicalizeTypesInDir(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	pkg, err := gcexportdata.Read(bytes.NewReader(data), fset, make(map[string]*types.Package), "example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if got := fset.Position(pkg.Scope().Lookup("Number").Pos()).Filename; got != "$ABS/source.go" {
		t.Errorf("canonical path=%q", got)
	}
	if err := addHeadersInDir(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(first, []byte("Licensed under the Apache License")) {
		t.Fatal("missing license header")
	}
	if err := addHeadersInDir(root); err != nil {
		t.Fatal(err)
	}
	if second, err := os.ReadFile(source); err != nil || !bytes.Equal(first, second) {
		t.Fatal("header addition is not idempotent")
	}
	if got, err := os.ReadFile(untouched); err != nil || string(got) != "unchanged" {
		t.Fatal("non-Go file changed")
	}
	for _, call := range []func(string) error{canonicalizeTypesInDir, addHeadersInDir} {
		if err := call(filepath.Join(root, "missing")); err == nil {
			t.Fatal("missing root accepted")
		}
	}
	if err := canonicalizeTypesInDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := canonicalizeTypesFile(filepath.Join(root, "missing.types"), "pkg", sourceRoots{}); err == nil {
		t.Fatal("missing types accepted")
	}
	if err := os.WriteFile(filename, []byte("invalid export data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := canonicalizeTypesInDir(root); err == nil || !strings.Contains(err.Error(), "canonicalize") {
		t.Fatalf("invalid export error=%v", err)
	}
}

func TestQexpMainWithLocalGoFixture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixture")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nif [ \"$1\" = env ]; then printf '%s' \"$QEXP_ENV\"; exit 0; fi\nprintf '%s\\n' \"$@\" > \"$QEXP_ARGS\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("QEXP_ARGS", record)
	t.Setenv("QEXP_ENV", `{"GOMOD":"/module/go.mod","GOROOT":"/sdk","GOMODCACHE":"/cache"}`)
	roots, err := loadSourceRoots()
	if err != nil || roots.moduleRoot != "/module" || roots.goRoot != "/sdk" || roots.moduleCache != "/cache" {
		t.Fatalf("roots=%+v,%v", roots, err)
	}
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	for _, args := range [][]string{{"qexpwrap", "example.com/pkg"}, {"qexpwrap", "-outdir", t.TempDir(), "example.com/pkg"}} {
		os.Args = args
		main()
		got, err := os.ReadFile(record)
		if err != nil || string(got) != "tool\nqexp\n"+strings.Join(args[1:], "\n")+"\n" {
			t.Fatalf("delegated args=%q,%v", got, err)
		}
	}
	t.Setenv("QEXP_ENV", "invalid")
	if _, err := loadSourceRoots(); err == nil || !strings.Contains(err.Error(), "decode Go source roots") {
		t.Fatalf("decode error=%v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := loadSourceRoots(); err == nil || !strings.Contains(err.Error(), "load Go source roots") {
		t.Fatalf("command error=%v", err)
	}
}

func TestFlagValue(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		ok   bool
	}{{nil, "", false}, {[]string{"-outdir"}, "", false}, {[]string{"-outdir="}, "", false}, {[]string{"-other", "x", "-outdir=out"}, "out", true}, {[]string{"-outdir", "out"}, "out", true}} {
		got, ok := flagValue(tc.args, "-outdir")
		if got != tc.want || ok != tc.ok {
			t.Errorf("flags%q=%q,%v", tc.args, got, ok)
		}
	}
}
