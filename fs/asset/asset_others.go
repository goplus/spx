//go:build !android && !ios
// +build !android,!ios

package asset

import (
	"io"

	"github.com/goplus/spx/fs"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Open opens a local filesystem object.
func Open(base string) (fs.Dir, error) {
	return &FS{base: base + "/"}, nil
}

func openAsset(path string) (io.ReadSeekCloser, error) {
	return ebitenutil.OpenFile(path)
}
