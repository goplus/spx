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
