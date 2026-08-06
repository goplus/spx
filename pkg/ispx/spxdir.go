/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ispx

import (
	"io"
	"io/fs"
	"path"

	spxfs "github.com/goplus/spx/v3/fs"
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
	return d.fsys.Open(d.resolve(file))
}

// Close implements [spxfs.Dir].
func (d *spxDir) Close() error {
	return nil
}

// ReadDir implements [spxfs.ReadDirer]. The web interpreter only receives
// source and JSON files, but their paths are sufficient to discover resource
// collection directories such as assets/fonts/*.
func (d *spxDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	entries, err := fs.ReadDir(d.fsys, d.resolve(name))
	if err != nil {
		return nil, err
	}
	return spxfs.DirEntriesFromFS(entries), nil
}

// GetPath implements [spxfs.GdDir]. Binary assets are intentionally omitted
// from the interpreter filesystem and are loaded by Godot from this path.
func (d *spxDir) GetPath() string {
	return d.prefix
}

func (d *spxDir) resolve(name string) string {
	if d.prefix == "" {
		return name
	}
	return path.Join(d.prefix, name)
}
