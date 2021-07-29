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

package zip

import (
	"archive/zip"
	"io"
	"syscall"

	"github.com/goplus/spx/fs"
	"github.com/pkg/errors"
)

// -------------------------------------------------------------------------------------

// A FS represents a zip filesystem.
type FS zip.ReadCloser

// Open opens a zip filesystem object.
func Open(file string) (fs.Dir, error) {
	zipf, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	return (*FS)(zipf), nil
}

// Open opens a zipped file object.
func (zipf *FS) Open(name string) (io.ReadCloser, error) {
	for _, f := range zipf.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, errors.Wrapf(syscall.ENOENT, "`%s` not found in zipfile", name)
}

// Close closes the filesystem object.
func (zipf *FS) Close() error {
	return ((*zip.ReadCloser)(zipf)).Close()
}

func init() {
	fs.RegisterSchema("zip", Open)
}

// -------------------------------------------------------------------------------------
