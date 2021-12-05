package asset

import (
	"io"

	"github.com/goplus/spx/fs"
)

// -------------------------------------------------------------------------------------

type FS struct {
	base string
}

// Open opens a zipped file object.
func (p *FS) Open(name string) (io.ReadCloser, error) {
	f, err := openAsset(p.base + name)
	return f, err
}

// Close closes the filesystem object.
func (f *FS) Close() error {
	return nil
}

func init() {
	fs.RegisterSchema("", Open)
}

// -------------------------------------------------------------------------------------
