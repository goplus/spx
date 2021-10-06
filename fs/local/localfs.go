/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
