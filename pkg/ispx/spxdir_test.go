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

package ispx

import (
	"errors"
	"io"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	spxfs "github.com/goplus/spx/v3/fs"
)

func TestSpxDirOpen(t *testing.T) {
	fsys := fstest.MapFS{
		"file.txt":        {Data: []byte("hello")},
		"subdir/file.txt": {Data: []byte("world")},
	}
	for _, tt := range []struct{ name, prefix, content string }{
		{"WithoutPrefix", "", "hello"},
		{"WithPrefix", "subdir", "world"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := newSpxDir(fsys, tt.prefix)
			if dir == nil {
				t.Fatal("expected non-nil dir")
			}
			if path := dir.(spxfs.GdDir).GetPath(); path != tt.prefix {
				t.Fatalf("GetPath = %q, want %q", path, tt.prefix)
			}
			reader, err := dir.Open("file.txt")
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil || string(data) != tt.content {
				t.Fatalf("file = %q, read=%v close=%v; want %q", data, readErr, closeErr, tt.content)
			}
			if _, err := dir.Open("nonexistent.txt"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing file error = %v", err)
			}
			if err := dir.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestSpxDirResourceCapabilities(t *testing.T) {
	fsys := fstest.MapFS{
		"assets/fonts/Pixel/index.json": &fstest.MapFile{Data: []byte(`{}`)},
		"assets/fonts/Serif/index.json": &fstest.MapFile{Data: []byte(`{}`)},
		"assets/index.json":             &fstest.MapFile{Data: []byte(`{}`)},
	}
	dir := newSpxDir(fsys, "assets")

	gdDir, ok := dir.(spxfs.GdDir)
	if !ok {
		t.Fatalf("%T does not implement GdDir", dir)
	}
	if got := gdDir.GetPath(); got != "assets" {
		t.Fatalf("GetPath() = %q, want assets", got)
	}
	reader, ok := dir.(spxfs.ReadDirer)
	if !ok {
		t.Fatalf("%T does not implement ReadDirer", dir)
	}
	entries, err := reader.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	want := []spxfs.DirEntry{{Name: "Pixel", IsDir: true}, {Name: "Serif", IsDir: true}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	entries, err = reader.ReadDir(".")
	want = []spxfs.DirEntry{{Name: "fonts", IsDir: true}, {Name: "index.json"}}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatalf("resource root entries = %#v, %v; want %#v", entries, err, want)
	}
	if _, err := reader.ReadDir("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing resource directory error = %v", err)
	}
}

func TestXGoParserFileSystem(t *testing.T) {
	parser := newXGoParserFS(fstest.MapFS{
		"sources/main.spx": {Data: []byte("onStart => {}")},
	})
	if parser == nil {
		t.Fatal("expected non-nil parser filesystem")
	}
	data, err := parser.ReadFile("sources/main.spx")
	if err != nil || string(data) != "onStart => {}" {
		t.Fatalf("parser source = %q, %v", data, err)
	}
	entries, err := parser.ReadDir("sources")
	if err != nil || len(entries) != 1 || entries[0].Name() != "main.spx" || entries[0].IsDir() {
		t.Fatalf("parser directory = %v, %v", entries, err)
	}
	if _, err := parser.ReadFile("missing.spx"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source error = %v", err)
	}
	if _, err := parser.ReadDir("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source directory error = %v", err)
	}
	if joined := parser.Join("sources", "nested", "..", "main.spx"); joined != "sources/main.spx" {
		t.Fatalf("joined parser path = %q", joined)
	}
	for _, tt := range []struct{ name, base string }{
		{"sources/main.spx", "main.spx"}, {"sources/", "sources"}, {"", "."},
	} {
		if base := parser.Base(tt.name); base != tt.base {
			t.Errorf("Base(%q) = %q, want %q", tt.name, base, tt.base)
		}
	}
	if absolute, err := parser.Abs("sources/main.spx"); absolute != "" || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("absolute parser path = %q, %v; want unsupported", absolute, err)
	}
}
