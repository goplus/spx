package util

import (
	"golang.org/x/mobile/asset"
)

func OpenFile(path string) (ReadSeekCloser, error) {
	return asset.Open(path)
}
