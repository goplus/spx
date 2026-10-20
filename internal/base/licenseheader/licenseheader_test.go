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

package licenseheader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddToGoSource(t *testing.T) {
	const tags = "//go:build js\n// +build js\n"
	for _, tc := range []struct{ name, source, want string }{
		{"plain", "package demo\n", Text + "package demo\n"},
		{"build tags", tags + "/* generated */\npackage demo\n", tags + "\n" + Text + "/* generated */\npackage demo\n"},
		{"existing blank line", tags + "\npackage demo\n", tags + "\n" + Text + "package demo\n"},
		{"existing header", Text + "package demo\n", Text + "package demo\n"},
		{"UTF-8 BOM", "\xef\xbb\xbfpackage demo\n", "\xef\xbb\xbf" + Text + "package demo\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AddToGoSource([]byte(tc.source))
			if string(got) != tc.want {
				t.Fatalf("source = %q, want %q", got, tc.want)
			}
			if again := AddToGoSource(got); string(again) != tc.want {
				t.Fatal("adding header twice changed the source")
			}
		})
	}
}

func TestAddToGoFileIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.go")
	if err := AddToGoFile(path); !os.IsNotExist(err) {
		t.Fatalf("missing source error = %v", err)
	}
	if err := os.WriteFile(path, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddToGoFile(path); err != nil {
		t.Fatal(err)
	}
	want := Text + "package demo\n"
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("file = %q, %v; want header and source", got, err)
	}
	stamp := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddToGoFile(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("already licensed source was rewritten")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("second pass changed file: %q, %v", got, err)
	}
}

func TestHasOnlyInspectsHeaderWindow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int
		want   bool
	}{
		{"inside", 1024 - len(Phrase), true},
		{"crosses boundary", 1025 - len(Phrase), false},
		{"after window", 1024, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Repeat(" ", tc.offset) + Phrase + "package demo\n"
			if got := Has([]byte(source)); got != tc.want {
				t.Fatalf("Has = %v, want %v", got, tc.want)
			}
		})
	}
}
