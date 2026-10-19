/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package fs

import (
	"errors"
	"io"
	iofs "io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSplitSchema(t *testing.T) {
	tests := []struct {
		path   string
		schema string
		file   string
	}{
		{"", "", ""},
		{"assets", "", "assets"},
		{"assets/project.zip", "", "assets/project.zip"},
		{"zip:project.zip", "zip", "project.zip"},
		{"zip://project.zip", "zip", "project.zip"},
		{"zip:///absolute/project.zip", "zip", "/absolute/project.zip"},
		{"zip:", "zip", ""},
		{"hzips://example.test/project.zip?v=2", "hzips", "example.test/project.zip?v=2"},
		{"assets/project:one.zip", "", "assets/project:one.zip"},
		{`assets\project:one.zip`, "", `assets\project:one.zip`},
		{"project one:assets", "", "project one:assets"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			schema, file := SplitSchema(tt.path)
			if schema != tt.schema || file != tt.file {
				t.Fatalf("SplitSchema(%q) = (%q, %q), want (%q, %q)", tt.path, schema, file, tt.schema, tt.file)
			}
		})
	}
}

func TestRegisterSchemaAndOpen(t *testing.T) {
	original := openSchemas
	openSchemas = make(map[string]OpenFunc)
	t.Cleanup(func() { openSchemas = original })

	for _, path := range []string{"unregistered://project", "assets"} {
		dir, err := Open(path)
		schema, _ := SplitSchema(path)
		if dir != nil || err == nil || err.Error() != "fs.Open: unsupported schema - "+schema {
			t.Fatalf("Open(%q) = (%v, %v), want unsupported schema error", path, dir, err)
		}
	}

	wantDir := &schemaTestDir{}
	var opened []string
	open := func(file string) (Dir, error) {
		opened = append(opened, file)
		return wantDir, nil
	}
	RegisterSchema("", open)
	RegisterSchema("test", open)
	for _, path := range []string{"assets", "test:project", "test://other/project"} {
		dir, err := Open(path)
		if err != nil || dir != wantDir {
			t.Fatalf("Open(%q) = (%v, %v), want registered directory", path, dir, err)
		}
	}
	if want := []string{"assets", "project", "other/project"}; !reflect.DeepEqual(opened, want) {
		t.Fatalf("opener paths = %q, want %q", opened, want)
	}

	wantErr := errors.New("opener failed")
	RegisterSchema("test", func(file string) (Dir, error) {
		if file != "replacement" {
			t.Fatalf("replacement opener path = %q, want replacement", file)
		}
		return nil, wantErr
	})
	if dir, err := Open("test://replacement"); dir != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Open after replacement = (%v, %v), want opener error", dir, err)
	}
}

type schemaTestDir struct{}

func (*schemaTestDir) Open(name string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(name)), nil
}

func (*schemaTestDir) Close() error { return nil }

func TestDirEntriesFromFS(t *testing.T) {
	fixture := fstest.MapFS{
		"a.txt":  &fstest.MapFile{},
		"folder": &fstest.MapFile{Mode: iofs.ModeDir | 0o755},
		"z.txt":  &fstest.MapFile{},
	}
	entries, err := iofs.ReadDir(fixture, ".")
	if err != nil {
		t.Fatal(err)
	}
	// Use a non-alphabetical order to check that conversion preserves input order.
	entries = []iofs.DirEntry{entries[2], entries[1], entries[0]}
	want := []DirEntry{{Name: "z.txt"}, {Name: "folder", IsDir: true}, {Name: "a.txt"}}
	if got := DirEntriesFromFS(entries); !reflect.DeepEqual(got, want) {
		t.Fatalf("DirEntriesFromFS = %#v, want %#v", got, want)
	}
	for _, empty := range [][]iofs.DirEntry{nil, {}} {
		if got := DirEntriesFromFS(empty); len(got) != 0 {
			t.Fatalf("DirEntriesFromFS(%v) = %#v, want no entries", empty, got)
		}
	}
}
