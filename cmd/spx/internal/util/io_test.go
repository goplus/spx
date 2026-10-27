package util

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

type failingCopyFS struct{ fs.FS }

func TestCopyDir(t *testing.T) {
	for _, tt := range []struct {
		name, source, destination, content, existing, want string
		override                                           bool
	}{
		{"preserve existing", "project.godot", "project.godot", "template", "existing", "existing", false},
		{"overwrite existing", "project.godot", "project.godot", "template", "existing", "template", true},
		{"dotfile", ".gitignore", ".gitignore", "/.godot\n", "", "/.godot\n", true},
		{"module template", "nested/go.mod.txt", "nested/go.mod", "module example", "", "module example", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			writeIOTestFile(t, filepath.Join(source, "template", tt.source), tt.content)
			output := filepath.Join(destination, tt.destination)
			if tt.existing != "" {
				writeIOTestFile(t, output, tt.existing)
			}
			if err := CopyDir(os.DirFS(source), "template", destination, tt.override); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(output)
			if err != nil || string(got) != tt.want {
				t.Fatalf("copied content = %q, %v; want %q", got, err, tt.want)
			}
			if tt.source != tt.destination && IsFileExist(filepath.Join(destination, tt.source)) {
				t.Fatal("module template was copied without renaming")
			}
		})
	}
}

func TestCheckFileExist(t *testing.T) {
	root := t.TempDir()
	writeIOTestFile(t, filepath.Join(root, "main.spx"), "source")
	writeIOTestFile(t, filepath.Join(root, "nested", "data.json"), "{}")
	if err := os.Mkdir(filepath.Join(root, "directory.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, dir, ext  string
		recursive, want bool
	}{
		{"root file", root, "spx", false, true},
		{"dotted extension", root, ".spx", true, true},
		{"nested excluded", root, "json", false, false},
		{"nested included", root, "json", true, true},
		{"directory is not a file", root, "txt", false, false},
		{"no recursive match", root, "txt", true, false},
		{"missing directory", filepath.Join(root, "missing"), "spx", false, false},
		{"missing recursive directory", filepath.Join(root, "missing"), "spx", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckFileExist(tt.dir, tt.ext, tt.recursive); got != tt.want {
				t.Fatalf("CheckFileExist = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalCopiesAndExistence(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	writeIOTestFile(t, filepath.Join(source, "nested", "data.txt"), "payload")
	for _, tt := range []struct {
		name string
		copy func() error
		path string
	}{
		{"file", func() error {
			return CopyFile(filepath.Join(source, "nested", "data.txt"), filepath.Join(destination, "single.txt"))
		}, "single.txt"},
		{"directory", func() error { return CopyDir2(source, filepath.Join(destination, "all")) }, "all/nested/data.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.copy(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(destination, tt.path)
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "payload" || !IsFileExist(path) {
				t.Fatalf("copy = %q, %v; want existing payload", data, err)
			}
		})
	}
	if IsFileExist(filepath.Join(destination, "missing")) {
		t.Fatal("missing file reported as existing")
	}
	if err := CopyFile(filepath.Join(source, "missing"), filepath.Join(destination, "bad")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source error = %v", err)
	}
	if err := CopyDir2(filepath.Join(source, "missing"), filepath.Join(destination, "bad-dir")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing directory error = %v", err)
	}
}

func TestCopyDirReportsFilesystemErrors(t *testing.T) {
	for _, tt := range []struct {
		name, source, conflict, want string
		files                        fstest.MapFS
		denyOpen                     bool
	}{
		{"invalid source", "../outside", "", "create sub fs", nil, false},
		{"missing source", "missing", "", "walk directory", nil, false},
		{"destination is file", ".", ".", "create directory", nil, false},
		{"nested directory is file", ".", "nested", "", fstest.MapFS{"nested/file": {Data: []byte("data")}}, false},
		{"destination file is directory", ".", "file/child", "", fstest.MapFS{"file": {Data: []byte("data")}}, false},
		{"source open denied", ".", "", "permission denied", fstest.MapFS{"blocked.txt": {Data: []byte("data")}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "output")
			if tt.conflict != "" {
				writeIOTestFile(t, filepath.Join(destination, tt.conflict), "conflict")
			}
			var source fs.FS = tt.files
			if tt.denyOpen {
				source = failingCopyFS{source}
			}
			err := CopyDir(source, tt.source, destination, true)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CopyDir error = %v, want %q", err, tt.want)
			}
		})
	}
}

func (f failingCopyFS) Open(name string) (fs.File, error) {
	if strings.HasSuffix(name, "blocked.txt") {
		return nil, fs.ErrPermission
	}
	return f.FS.Open(name)
}

func writeIOTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
