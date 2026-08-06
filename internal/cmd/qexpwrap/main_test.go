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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
