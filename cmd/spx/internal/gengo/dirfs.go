package gengo

import (
	"io/fs"
	"os"
	"path/filepath"
)

// DirFS adapts a directory to parser.FileSystem.
type DirFS struct {
	root string
}

// NewDirFS creates a DirFS.
func NewDirFS(root string) *DirFS {
	return &DirFS{root: root}
}

// ReadDir reads a directory.
func (d *DirFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	fullPath := filepath.Join(d.root, dirname)
	return os.ReadDir(fullPath)
}

// ReadFile reads a file.
func (d *DirFS) ReadFile(filename string) ([]byte, error) {
	fullPath := filepath.Join(d.root, filename)
	return os.ReadFile(fullPath)
}

// Join joins path elements.
func (d *DirFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Base returns the last path element.
func (d *DirFS) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs returns an absolute path.
func (d *DirFS) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	fullPath := filepath.Join(d.root, path)
	return filepath.Abs(fullPath)
}
