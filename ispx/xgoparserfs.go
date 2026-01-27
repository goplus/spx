package ispx

import (
	"errors"
	"io/fs"
	"path"

	xgoparser "github.com/goplus/xgo/parser"
)

// xgoParserFS wraps a [fs.ReadDirFS] to implement [xgoparser.FileSystem].
type xgoParserFS struct {
	rdfs fs.ReadDirFS
}

// newXGoParserFS creates a new XGo parser file system.
func newXGoParserFS(rdfs fs.ReadDirFS) xgoparser.FileSystem {
	return &xgoParserFS{rdfs: rdfs}
}

// ReadDir implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return fs.ReadDir(pfs.rdfs, dirname)
}

// ReadFile implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadFile(filename string) ([]byte, error) {
	return fs.ReadFile(pfs.rdfs, filename)
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
