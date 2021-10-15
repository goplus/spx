package local

import (
	"io"

	"github.com/goplus/spx/fs"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// -------------------------------------------------------------------------------------

type FS struct {
	base string
}

// Open opens a local filesystem object.
func Open(base string) (fs.Dir, error) {
	return &FS{base: base + "/"}, nil
}

// Open opens a zipped file object.
func (p *FS) Open(name string) (io.ReadCloser, error) {
	f, err := ebitenutil.OpenFile(p.base + name)
	return f, err
}

// Close closes the filesystem object.
func (f *FS) Close() error {
	return nil
}

/*
// OpenLocal opens local:<app>:<path>
func OpenLocal(base string) (fs.Dir, error) {
	pos := strings.Index(base, ":")
	if pos < 0 {
		return nil, errors.New("Use local:<app>:<path> please")
	}
	app, base := base[:pos], base[pos+1:]
	_, err := os.Lstat(base)
	if err != nil {
		base = spxBaseDir + app + "/" + base
		_, err = os.Lstat(base)
		if err != nil {
			return nil, err
		}
	}
	return Open(base)
}

var (
	spxBaseDir = os.Getenv("HOME") + "/.spx/"
)
*/

func init() {
	fs.RegisterSchema("", Open)
	// fs.RegisterSchema("local", OpenLocal)
}

// -------------------------------------------------------------------------------------
