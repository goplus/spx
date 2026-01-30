package ispx

import (
	"errors"
	"io/fs"
	"path"

	xgoparser "github.com/goplus/xgo/parser"
)

// xgoParserFS wraps a [fs.FS] to implement [xgoparser.FileSystem].
type xgoParserFS struct {
	fsys fs.FS
}

// newXGoParserFS creates a new XGo parser file system.
func newXGoParserFS(fsys fs.FS) xgoparser.FileSystem {
	return &xgoParserFS{fsys: fsys}
}

// ReadDir implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return fs.ReadDir(pfs.fsys, dirname)
}

// ReadFile implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadFile(filename string) ([]byte, error) {
	return fs.ReadFile(pfs.fsys, filename)
}

// Join implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Join(elem ...string) string {
	return path.Join(elem...)
}

// Base implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Base(filename string) string {
	return path.Base(filename)
}

// Abs implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Abs(path string) (string, error) {
	return "", errors.ErrUnsupported
}
