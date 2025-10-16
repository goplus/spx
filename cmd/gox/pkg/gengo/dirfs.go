package gengo

import (
	"io/fs"
	"os"
	"path/filepath"
)

// DirFS implements fsx.FileSystem interface for a directory on the filesystem
// This is needed for xgobuild.BuildFSDir which requires:
//
//	type FileSystem interface {
//		ReadDir(dirname string) ([]fs.DirEntry, error)
//		ReadFile(filename string) ([]byte, error)
//		Join(elem ...string) string
//		Base(filename string) string
//		Abs(path string) (string, error)
//	}
type DirFS struct {
	root string
}

// NewDirFS creates a new DirFS rooted at the specified directory
func NewDirFS(root string) *DirFS {
	return &DirFS{root: root}
}

// ReadDir reads the named directory
func (d *DirFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	fullPath := filepath.Join(d.root, dirname)
	return os.ReadDir(fullPath)
}

// ReadFile reads the named file
func (d *DirFS) ReadFile(filename string) ([]byte, error) {
	fullPath := filepath.Join(d.root, filename)
	return os.ReadFile(fullPath)
}

// Join joins path elements
func (d *DirFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Base returns the last element of path
func (d *DirFS) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs returns an absolute representation of path
func (d *DirFS) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	fullPath := filepath.Join(d.root, path)
	return filepath.Abs(fullPath)
}
