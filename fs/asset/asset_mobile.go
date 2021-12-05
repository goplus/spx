//go:build android || ios
// +build android ios

package asset

import (
	"io"

	"github.com/goplus/spx/fs"
	"golang.org/x/mobile/asset"
)

// Open opens a local filesystem object.
func Open(base string) (fs.Dir, error) {
	if base != "assets" {
		panic("FATAL: asset must be in `assets` directory")
	}
	return &FS{base: ""}, nil
}

func openAsset(path string) (io.ReadSeekCloser, error) {
	return asset.Open(path)
}
