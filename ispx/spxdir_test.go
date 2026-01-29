package ispx

import (
	"io"
	"testing"
	"testing/fstest"
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
