package memfs

import (
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestNew(t *testing.T) {
	files := map[string][]byte{
		"file.txt":         []byte("hello"),
		"subdir/file.txt":  []byte("world"),
		"subdir/sub/a.txt": []byte("a"),
	}
	fsys := New(files)
	if fsys == nil {
		t.Fatal("expected non-nil FS")
	}
	if fsys.files == nil {
		t.Fatal("expected non-nil files map")
	}
	if err := fstest.TestFS(fsys, "file.txt", "subdir/file.txt", "subdir/sub/a.txt"); err != nil {
		t.Errorf("fstest.TestFS failed: %v", err)
	}
}

func TestFSOpen(t *testing.T) {
	t.Run("File", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		f, err := fsys.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("got %q, want %q", string(data), "hello")
		}
	})

	t.Run("Directory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("world"),
		})
		f, err := fsys.Open("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("RootDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		f, err := fsys.Open(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("NotExist", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.Open("nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrNotExist {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrNotExist)
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.Open("../invalid")
		if err == nil {
			t.Fatal("expected error for invalid path")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrInvalid {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrInvalid)
		}
	})
}

func TestFSReadDir(t *testing.T) {
	t.Run("RootDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"a.txt":            []byte("a"),
			"b.txt":            []byte("bb"),
			"subdir/c.txt":     []byte("ccc"),
			"subdir/d.txt":     []byte("dddd"),
			"subdir/sub/e.txt": []byte("eeeee"),
		})
		entries, err := fsys.ReadDir(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}

		expected := []struct {
			name  string
			isDir bool
		}{
			{"a.txt", false},
			{"b.txt", false},
			{"subdir", true},
		}
		for i, e := range expected {
			if entries[i].Name() != e.name {
				t.Errorf("entry %d: got name %q, want %q", i, entries[i].Name(), e.name)
			}
			if entries[i].IsDir() != e.isDir {
				t.Errorf("entry %d: got isDir %v, want %v", i, entries[i].IsDir(), e.isDir)
			}
		}
	})

	t.Run("Subdirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/c.txt":     []byte("ccc"),
			"subdir/d.txt":     []byte("dddd"),
			"subdir/sub/e.txt": []byte("eeeee"),
		})
		entries, err := fsys.ReadDir("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}

		expected := []struct {
			name  string
			isDir bool
		}{
			{"c.txt", false},
			{"d.txt", false},
			{"sub", true},
		}
		for i, e := range expected {
			if entries[i].Name() != e.name {
				t.Errorf("entry %d: got name %q, want %q", i, entries[i].Name(), e.name)
			}
			if entries[i].IsDir() != e.isDir {
				t.Errorf("entry %d: got isDir %v, want %v", i, entries[i].IsDir(), e.isDir)
			}
		}
	})

	t.Run("NotExist", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.ReadDir("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent directory")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrNotExist {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrNotExist)
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.ReadDir("../invalid")
		if err == nil {
			t.Fatal("expected error for invalid path")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrInvalid {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrInvalid)
		}
	})
}

func TestFSStat(t *testing.T) {
	t.Run("File", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		fi, err := fsys.Stat("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fi.Name() != "file.txt" {
			t.Errorf("got name %q, want %q", fi.Name(), "file.txt")
		}
		if fi.Size() != 5 {
			t.Errorf("got size %d, want 5", fi.Size())
		}
		if fi.IsDir() {
			t.Error("expected file, not directory")
		}
		if fi.Mode() != fileMode {
			t.Errorf("got mode %v, want %v", fi.Mode(), fileMode)
		}
	})

	t.Run("FileInSubdirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("world"),
		})
		fi, err := fsys.Stat("subdir/file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fi.Name() != "file.txt" {
			t.Errorf("got name %q, want %q", fi.Name(), "file.txt")
		}
		if fi.Size() != 5 {
			t.Errorf("got size %d, want 5", fi.Size())
		}
	})

	t.Run("Directory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("world"),
		})
		fi, err := fsys.Stat("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fi.Name() != "subdir" {
			t.Errorf("got name %q, want %q", fi.Name(), "subdir")
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
		if fi.Mode() != dirMode {
			t.Errorf("got mode %v, want %v", fi.Mode(), dirMode)
		}
	})

	t.Run("RootDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		fi, err := fsys.Stat(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fi.Name() != "." {
			t.Errorf("got name %q, want %q", fi.Name(), ".")
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("NotExist", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.Stat("nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrNotExist {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrNotExist)
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		_, err := fsys.Stat("../invalid")
		if err == nil {
			t.Fatal("expected error for invalid path")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected *fs.PathError, got %T", err)
		}
		if pathErr.Err != fs.ErrInvalid {
			t.Errorf("got %v, want %v", pathErr.Err, fs.ErrInvalid)
		}
	})
}

func TestFSHasDir(t *testing.T) {
	t.Run("RootDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{})
		if !fsys.hasDir(".") {
			t.Error("expected root directory to exist")
		}
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("hello"),
		})
		if !fsys.hasDir("subdir") {
			t.Error("expected subdir to exist")
		}
	})

	t.Run("NestedDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"a/b/c/file.txt": []byte("hello"),
		})
		if !fsys.hasDir("a") {
			t.Error("expected a to exist")
		}
		if !fsys.hasDir("a/b") {
			t.Error("expected a/b to exist")
		}
		if !fsys.hasDir("a/b/c") {
			t.Error("expected a/b/c to exist")
		}
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("hello"),
		})
		if fsys.hasDir("nonexistent") {
			t.Error("expected nonexistent directory to not exist")
		}
	})

	t.Run("FileNotDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		if fsys.hasDir("file.txt") {
			t.Error("expected file to not be a directory")
		}
	})

	t.Run("SimilarPrefix", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir-other/file.txt": []byte("hello"),
		})
		if fsys.hasDir("subdir") {
			t.Error("expected subdir to not exist when only subdir-other exists")
		}
	})
}

func TestFileStat(t *testing.T) {
	fsys := New(map[string][]byte{
		"file.txt": []byte("hello"),
	})
	f, err := fsys.Open("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("failed to stat: %v", err)
	}
	if fi.Name() != "file.txt" {
		t.Errorf("got name %q, want %q", fi.Name(), "file.txt")
	}
	if fi.Size() != 5 {
		t.Errorf("got size %d, want 5", fi.Size())
	}
	if fi.IsDir() {
		t.Error("expected file, not directory")
	}
}

func TestFileRead(t *testing.T) {
	t.Run("ReadAll", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello world"),
		})
		f, err := fsys.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("got %q, want %q", string(data), "hello world")
		}
	})

	t.Run("ReadPartial", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello world"),
		})
		f, err := fsys.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		buf := make([]byte, 5)
		n, err := f.Read(buf)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if n != 5 {
			t.Errorf("got %d bytes, want 5", n)
		}
		if string(buf) != "hello" {
			t.Errorf("got %q, want %q", string(buf), "hello")
		}

		n, err = f.Read(buf)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if n != 5 {
			t.Errorf("got %d bytes, want 5", n)
		}
		if string(buf) != " worl" {
			t.Errorf("got %q, want %q", string(buf), " worl")
		}
	})

	t.Run("ReadEOF", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello world"),
		})
		f, err := fsys.Open("file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		_, err = io.ReadAll(f)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		buf := make([]byte, 1)
		n, err := f.Read(buf)
		if n != 0 {
			t.Errorf("got %d bytes, want 0", n)
		}
		if err != io.EOF {
			t.Errorf("got %v, want io.EOF", err)
		}
	})
}

func TestFileClose(t *testing.T) {
	fsys := New(map[string][]byte{
		"file.txt": []byte("hello"),
	})
	f, err := fsys.Open("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDirStat(t *testing.T) {
	t.Run("Subdirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("hello"),
		})
		f, err := fsys.Open("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}
		if fi.Name() != "subdir" {
			t.Errorf("got name %q, want %q", fi.Name(), "subdir")
		}
		if fi.Size() != 0 {
			t.Errorf("got size %d, want 0", fi.Size())
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
		if fi.Mode() != dirMode {
			t.Errorf("got mode %v, want %v", fi.Mode(), dirMode)
		}
	})

	t.Run("RootDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		f, err := fsys.Open(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}
		if fi.Name() != "." {
			t.Errorf("got name %q, want %q", fi.Name(), ".")
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("NestedDirectory", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"a/b/c/file.txt": []byte("hello"),
		})
		f, err := fsys.Open("a/b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}
		if fi.Name() != "b" {
			t.Errorf("got name %q, want %q", fi.Name(), "b")
		}
		if !fi.IsDir() {
			t.Error("expected directory")
		}
	})
}

func TestDirRead(t *testing.T) {
	fsys := New(map[string][]byte{
		"subdir/file.txt": []byte("hello"),
	})
	f, err := fsys.Open("subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 1)
	_, err = f.Read(buf)
	if err == nil {
		t.Fatal("expected error when reading directory")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("expected *fs.PathError, got %T", err)
	}
	if pathErr.Err != fs.ErrInvalid {
		t.Errorf("got %v, want %v", pathErr.Err, fs.ErrInvalid)
	}
}

func TestDirClose(t *testing.T) {
	fsys := New(map[string][]byte{
		"subdir/file.txt": []byte("hello"),
	})
	f, err := fsys.Open("subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDirReadDir(t *testing.T) {
	t.Run("ReadAll", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/a.txt": []byte("a"),
			"subdir/b.txt": []byte("bb"),
			"subdir/c.txt": []byte("ccc"),
		})
		f, err := fsys.Open("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		dirFile := f.(fs.ReadDirFile)
		entries, err := dirFile.ReadDir(-1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}
	})

	t.Run("ReadPartial", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/a.txt": []byte("a"),
			"subdir/b.txt": []byte("bb"),
			"subdir/c.txt": []byte("ccc"),
		})
		f, err := fsys.Open("subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer f.Close()

		dirFile := f.(fs.ReadDirFile)
		entries, err := dirFile.ReadDir(2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}

		entries, err = dirFile.ReadDir(2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}

		entries, err = dirFile.ReadDir(2)
		if err != io.EOF {
			t.Fatalf("got %v, want io.EOF", err)
		}
		if len(entries) != 0 {
			t.Fatalf("got %d entries, want 0", len(entries))
		}
	})
}

func TestFileInfo(t *testing.T) {
	fsys := New(map[string][]byte{
		"file.txt": []byte("hello"),
	})
	fi, err := fsys.Stat("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fi.Name() != "file.txt" {
		t.Errorf("got name %q, want %q", fi.Name(), "file.txt")
	}
	if fi.Size() != 5 {
		t.Errorf("got size %d, want 5", fi.Size())
	}
	if fi.Mode() != fileMode {
		t.Errorf("got mode %v, want %v", fi.Mode(), fileMode)
	}
	if fi.ModTime().IsZero() {
		t.Error("expected non-zero mod time")
	}
	if fi.IsDir() {
		t.Error("expected file, not directory")
	}
	if fi.Sys() != nil {
		t.Errorf("got sys %v, want nil", fi.Sys())
	}
}

func TestDirEntry(t *testing.T) {
	t.Run("FileEntry", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"file.txt": []byte("hello"),
		})
		entries, err := fsys.ReadDir(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var fileEntry fs.DirEntry
		for _, e := range entries {
			if e.Name() == "file.txt" {
				fileEntry = e
				break
			}
		}
		if fileEntry == nil {
			t.Fatal("file.txt entry not found")
		}

		if fileEntry.IsDir() {
			t.Error("expected file, not directory")
		}
		if fileEntry.Type() != fileMode.Type() {
			t.Errorf("got type %v, want %v", fileEntry.Type(), fileMode.Type())
		}

		fi, err := fileEntry.Info()
		if err != nil {
			t.Fatalf("failed to get info: %v", err)
		}
		if fi.Name() != "file.txt" {
			t.Errorf("got name %q, want %q", fi.Name(), "file.txt")
		}
		if fi.Size() != 5 {
			t.Errorf("got size %d, want 5", fi.Size())
		}
	})

	t.Run("DirEntry", func(t *testing.T) {
		fsys := New(map[string][]byte{
			"subdir/file.txt": []byte("world"),
		})
		entries, err := fsys.ReadDir(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var dirEntry fs.DirEntry
		for _, e := range entries {
			if e.Name() == "subdir" {
				dirEntry = e
				break
			}
		}
		if dirEntry == nil {
			t.Fatal("subdir entry not found")
		}

		if !dirEntry.IsDir() {
			t.Error("expected directory")
		}
		if dirEntry.Type() != dirMode.Type() {
			t.Errorf("got type %v, want %v", dirEntry.Type(), dirMode.Type())
		}
	})
}
