package fsutil

import (
	"io"

	"golang.org/x/mobile/asset"
)

func OpenFile(path string) (io.ReadSeekCloser, error) {
	return asset.Open(path)
}
