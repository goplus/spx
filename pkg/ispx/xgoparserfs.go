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
	"errors"
	"io/fs"
	"path"

	xgoparser "github.com/goplus/xgo/parser"
)

// xgoParserFS wraps a [fs.FS] to implement [xgoparser.FileSystem].
type xgoParserFS struct {
	fsys fs.FS
}

// newXGoParserFS creates a new XGo parser file system.
func newXGoParserFS(fsys fs.FS) xgoparser.FileSystem {
	return &xgoParserFS{fsys: fsys}
}

// ReadDir implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return fs.ReadDir(pfs.fsys, dirname)
}

// ReadFile implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) ReadFile(filename string) ([]byte, error) {
	return fs.ReadFile(pfs.fsys, filename)
}

// Join implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Join(elem ...string) string {
	return path.Join(elem...)
}

// Base implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Base(filename string) string {
	return path.Base(filename)
}

// Abs implements [xgoparser.FileSystem].
func (pfs *xgoParserFS) Abs(path string) (string, error) {
	return "", errors.ErrUnsupported
}
