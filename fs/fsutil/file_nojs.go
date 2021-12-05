//go:build !js
// +build !js

package fsutil

import (
	"io"
	"os"
)

// OpenFile opens a file and returns a stream for its data.
//
// The path parts should be separated with slash '/' on any environments.
//
// Note that this doesn't work on mobiles.
func OpenFile(path string) (io.ReadSeekCloser, error) {
	return os.Open(path)
}
