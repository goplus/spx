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
	"io"
	"reflect"
	"testing"
	"testing/fstest"

	spxfs "github.com/goplus/spx/v3/fs"
)

func TestNewSpxDir(t *testing.T) {
	fsys := fstest.MapFS{}
	dir := newSpxDir(fsys, "")
	if dir == nil {
		t.Fatal("expected non-nil dir")
	}
}

func TestSpxDirOpen(t *testing.T) {
	fsys := fstest.MapFS{
		"file.txt":        &fstest.MapFile{Data: []byte("hello")},
		"subdir/file.txt": &fstest.MapFile{Data: []byte("world")},
	}

	t.Run("WithoutPrefix", func(t *testing.T) {
		dir := newSpxDir(fsys, "")
		rc, err := dir.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("got %q, want %q", string(data), "hello")
		}
	})

	t.Run("WithPrefix", func(t *testing.T) {
		dir := newSpxDir(fsys, "subdir")
		rc, err := dir.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "world" {
			t.Errorf("got %q, want %q", string(data), "world")
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		dir := newSpxDir(fsys, "")
		_, err := dir.Open("nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func TestSpxDirClose(t *testing.T) {
	fsys := fstest.MapFS{}
	dir := newSpxDir(fsys, "")
	spxDir := dir.(*spxDir)
	if err := spxDir.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
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
}
