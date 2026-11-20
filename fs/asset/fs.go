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

package asset

import (
	iofs "io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/fs"
)

type FS struct {
	fs.NativeResourcePaths
	base string
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

// localResourcePath translates a logical child path for the host asset reader.
func localResourcePath(name string) (string, error) {
	clean := path.Clean(name)
	local, err := filepath.Localize(clean)
	if err != nil || fs.IsAbsResourcePath(name) || fs.IsAbsResourcePath(clean) || strings.IndexByte(name, 0) >= 0 {
		return "", &iofs.PathError{Op: "resolve", Path: name, Err: iofs.ErrInvalid}
	}
	return local, nil
}
