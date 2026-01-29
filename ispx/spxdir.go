package ispx

import (
	"io"
	"io/fs"
	"path"

	spxfs "github.com/goplus/spx/v2/fs"
)

// spxDir wraps a [fs.FS] to implement [spxfs.Dir].
type spxDir struct {
	fsys   fs.FS
	prefix string
}

// newSpxDir creates a new spx directory.
func newSpxDir(fsys fs.FS, prefix string) spxfs.Dir {
	return &spxDir{fsys: fsys, prefix: prefix}
}

// Open implements [spxfs.Dir].
func (d *spxDir) Open(file string) (io.ReadCloser, error) {
	name := file
	if d.prefix != "" {
		name = path.Join(d.prefix, file)
	}
	return d.fsys.Open(name)
}

// Close implements [spxfs.Dir].
func (d *spxDir) Close() error {
	return nil
}
