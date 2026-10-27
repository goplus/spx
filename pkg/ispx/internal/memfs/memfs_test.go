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

package memfs

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"testing"
	"testing/fstest"
	"time"
)

func TestNew(t *testing.T) {
	files := map[string][]byte{
		"file.txt":         []byte("hello"),
		"subdir/file.txt":  []byte("world"),
		"subdir/sub/a.txt": []byte("a"),
	}
	fsys := New(files)
	if fsys == nil || fsys.files == nil {
		t.Fatal("expected non-nil FS and files map")
	}
	if err := fstest.TestFS(fsys, "file.txt", "subdir/file.txt", "subdir/sub/a.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestFileAndDirectoryMetadata(t *testing.T) {
	fsys := New(map[string][]byte{
		"file.txt":        []byte("hello"),
		"subdir/file.txt": []byte("world"),
		"a/b/c/file.txt":  []byte("nested"),
	})
	for _, tt := range []struct {
		name, content string
		mode          fs.FileMode
	}{
		{"file.txt", "hello", fileMode},
		{"subdir/file.txt", "world", fileMode},
		{".", "", dirMode},
		{"subdir", "", dirMode},
		{"a/b", "", dirMode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fsys.Stat(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			assertFileInfo(t, info, path.Base(tt.name), int64(len(tt.content)), tt.mode)
			file := openTestFile(t, fsys, tt.name)
			info, err = file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			assertFileInfo(t, info, path.Base(tt.name), int64(len(tt.content)), tt.mode)
			if !tt.mode.IsDir() {
				data, err := io.ReadAll(file)
				if err != nil || string(data) != tt.content {
					t.Fatalf("content = %q, %v; want %q", data, err, tt.content)
				}
			}
		})
	}
}

func TestFilesystemPathErrors(t *testing.T) {
	fsys := New(nil)
	for op, call := range map[string]func(string) error{
		"open":    func(name string) error { _, err := fsys.Open(name); return err },
		"readdir": func(name string) error { _, err := fsys.ReadDir(name); return err },
		"stat":    func(name string) error { _, err := fsys.Stat(name); return err },
	} {
		for _, tt := range []struct {
			name string
			err  error
		}{
			{"nonexistent.txt", fs.ErrNotExist},
			{"../invalid", fs.ErrInvalid},
			{"", fs.ErrInvalid},
			{"/absolute", fs.ErrInvalid},
		} {
			t.Run(op+"/"+tt.name, func(t *testing.T) {
				assertPathError(t, call(tt.name), op, tt.name, tt.err)
			})
		}
	}
}

func TestFSReadDirEntries(t *testing.T) {
	fsys := New(map[string][]byte{
		"a.txt": []byte("a"), "b.txt": []byte("bb"),
		"subdir/c.txt": []byte("ccc"), "subdir/d.txt": []byte("dddd"),
		"subdir/sub/e.txt": []byte("eeeee"),
	})
	for _, tt := range []struct {
		name  string
		names []string
		sizes []int64
	}{
		{".", []string{"a.txt", "b.txt", "subdir"}, []int64{1, 2, 0}},
		{"subdir", []string{"c.txt", "d.txt", "sub"}, []int64{3, 4, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := fsys.ReadDir(tt.name)
			if err != nil || len(entries) != len(tt.names) {
				t.Fatalf("ReadDir = %v, %v; want %d entries", entries, err, len(tt.names))
			}
			for i, entry := range entries {
				mode := fileMode
				if i == len(entries)-1 {
					mode = dirMode
				}
				if entry.Name() != tt.names[i] || entry.IsDir() != mode.IsDir() || entry.Type() != mode.Type() {
					t.Errorf("entry %d = (%q, directory=%v, %v), want (%q, %v)", i, entry.Name(), entry.IsDir(), entry.Type(), tt.names[i], mode.Type())
				}
				info, err := entry.Info()
				if err != nil {
					t.Fatal(err)
				}
				assertFileInfo(t, info, tt.names[i], tt.sizes[i], mode)
			}
		})
	}
}

func TestReadDirIgnoresEmptyNamesAndPrefersDirectoryEntries(t *testing.T) {
	fsys := New(map[string][]byte{
		"": nil, "/ignored": nil, "shared": []byte("file"), "shared/child": nil,
	})
	entries, err := fsys.ReadDir(".")
	if err != nil || len(entries) != 1 || entries[0].Name() != "shared" || !entries[0].IsDir() {
		t.Fatalf("directory entries = %v, %v; want only shared directory", entries, err)
	}
}

func TestFSHasDir(t *testing.T) {
	for _, tt := range []struct {
		name, file string
		want       bool
	}{
		{".", "", true},
		{"subdir", "subdir/file.txt", true},
		{"a", "a/b/c/file.txt", true},
		{"a/b", "a/b/c/file.txt", true},
		{"a/b/c", "a/b/c/file.txt", true},
		{"nonexistent", "subdir/file.txt", false},
		{"file.txt", "file.txt", false},
		{"subdir", "subdir-other/file.txt", false},
	} {
		t.Run(tt.name+"/"+tt.file, func(t *testing.T) {
			files := map[string][]byte{}
			if tt.file != "" {
				files[tt.file] = []byte("hello")
			}
			if got := New(files).hasDir(tt.name); got != tt.want {
				t.Fatalf("hasDir(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFileRead(t *testing.T) {
	fsys := New(map[string][]byte{"file.txt": []byte("hello world"), "empty.txt": nil})
	t.Run("ReadAll", func(t *testing.T) {
		data, err := io.ReadAll(openTestFile(t, fsys, "file.txt"))
		if err != nil || string(data) != "hello world" {
			t.Fatalf("ReadAll = %q, %v", data, err)
		}
	})
	t.Run("ReadPartialAndEOF", func(t *testing.T) {
		file := openTestFile(t, fsys, "file.txt")
		for _, tt := range []struct {
			size int
			data string
			err  error
		}{
			{0, "", nil}, {5, "hello", nil}, {5, " worl", nil}, {5, "d", nil}, {1, "", io.EOF},
		} {
			buffer := make([]byte, tt.size)
			n, err := file.Read(buffer)
			if n != len(tt.data) || string(buffer[:n]) != tt.data || err != tt.err {
				t.Fatalf("Read(%d) = (%q, %d, %v), want (%q, %v)", tt.size, buffer[:n], n, err, tt.data, tt.err)
			}
		}
	})
	t.Run("ReadEOFAfterReadAll", func(t *testing.T) {
		file := openTestFile(t, fsys, "file.txt")
		if _, err := io.ReadAll(file); err != nil {
			t.Fatal(err)
		}
		if n, err := file.Read(make([]byte, 1)); n != 0 || err != io.EOF {
			t.Fatalf("read after EOF = (%d, %v)", n, err)
		}
	})
	t.Run("EmptyFile", func(t *testing.T) {
		if n, err := openTestFile(t, fsys, "empty.txt").Read(make([]byte, 1)); n != 0 || err != io.EOF {
			t.Fatalf("empty file read = (%d, %v)", n, err)
		}
	})
}

func TestDirRead(t *testing.T) {
	file := openTestFile(t, New(map[string][]byte{"subdir/file.txt": []byte("hello")}), "subdir")
	n, err := file.Read(make([]byte, 1))
	if n != 0 {
		t.Fatalf("directory read = %d bytes, want 0", n)
	}
	assertPathError(t, err, "read", "subdir", fs.ErrInvalid)
}

func TestDirReadDir(t *testing.T) {
	type read struct {
		count int
		names []string
		err   error
	}
	for _, tt := range []struct {
		name  string
		reads []read
	}{
		{"ReadAll", []read{{-1, []string{"a.txt", "b.txt", "c.txt"}, nil}}},
		{"ReadPartial", []read{{2, []string{"a.txt", "b.txt"}, nil}, {2, []string{"c.txt"}, nil}, {2, nil, io.EOF}}},
		{"ReadRemaining", []read{{1, []string{"a.txt"}, nil}, {0, []string{"b.txt", "c.txt"}, nil}, {-1, nil, nil}, {1, nil, io.EOF}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fsys := New(map[string][]byte{"subdir/a.txt": []byte("a"), "subdir/b.txt": []byte("bb"), "subdir/c.txt": []byte("ccc")})
			file := openTestFile(t, fsys, "subdir").(fs.ReadDirFile)
			for _, read := range tt.reads {
				entries, err := file.ReadDir(read.count)
				names := make([]string, len(entries))
				for i, entry := range entries {
					names[i] = entry.Name()
				}
				if err != read.err || !slices.Equal(names, read.names) {
					t.Fatalf("ReadDir(%d) = (%v, %v), want (%v, %v)", read.count, names, err, read.names, read.err)
				}
			}
		})
	}
}

func TestDirReadDirReportsRemovedDirectoryAndCanRetry(t *testing.T) {
	files := map[string][]byte{"subdir/file.txt": []byte("content")}
	directory := openTestFile(t, New(files), "subdir").(fs.ReadDirFile)
	delete(files, "subdir/file.txt")
	entries, err := directory.ReadDir(1)
	if entries != nil {
		t.Fatalf("removed directory returned entries: %v", entries)
	}
	assertPathError(t, err, "readdir", "subdir", fs.ErrNotExist)
	files["subdir/restored.txt"] = []byte("restored")
	entries, err = directory.ReadDir(1)
	if err != nil || len(entries) != 1 || entries[0].Name() != "restored.txt" {
		t.Fatalf("retry after failed listing = %v, %v", entries, err)
	}
}

func openTestFile(t *testing.T, fsys *FS, name string) fs.File {
	t.Helper()
	file, err := fsys.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("Close(%q): %v", name, err)
		}
	})
	return file
}

func assertFileInfo(t *testing.T, info fs.FileInfo, name string, size int64, mode fs.FileMode) {
	t.Helper()
	if info.Name() != name || info.Size() != size || info.Mode() != mode || info.IsDir() != mode.IsDir() {
		t.Errorf("metadata = (%q, %d, %v, directory=%v), want (%q, %d, %v)", info.Name(), info.Size(), info.Mode(), info.IsDir(), name, size, mode)
	}
	if !info.ModTime().Equal(time.Unix(0, 0)) || info.Sys() != nil {
		t.Errorf("metadata time/sys = (%v, %v), want Unix epoch and nil", info.ModTime(), info.Sys())
	}
}

func assertPathError(t *testing.T, err error, op, name string, want error) {
	t.Helper()
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != op || pathErr.Path != name || pathErr.Err != want || !errors.Is(err, want) {
		t.Fatalf("error = %v, want PathError{%q, %q, %v}", err, op, name, want)
	}
}
