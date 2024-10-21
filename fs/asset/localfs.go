/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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

package asset

import (
	"io"

	"github.com/goplus/spx/fs"
)

// -------------------------------------------------------------------------------------

type FS struct {
	base string
}

// Open opens a zipped file object.
func (p *FS) Open(name string) (io.ReadCloser, error) {
	f, err := openAsset(p.base + name)
	return f, err
}

// Close closes the filesystem object.
func (f *FS) Close() error {
	return nil
}

func (f *FS) GetPath() string {
	return f.base
}

func init() {
	fs.RegisterSchema("", Open)
}

// -------------------------------------------------------------------------------------
