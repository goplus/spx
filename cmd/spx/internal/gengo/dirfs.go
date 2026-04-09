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

package gengo

import (
	"io/fs"
	"os"
	"path/filepath"
)

// DirFS adapts a directory to parser.FileSystem.
type DirFS struct {
	root string
}

// NewDirFS creates a DirFS.
func NewDirFS(root string) *DirFS {
	return &DirFS{root: root}
}

// ReadDir reads a directory.
func (d *DirFS) ReadDir(dirname string) ([]fs.DirEntry, error) {
	fullPath := filepath.Join(d.root, dirname)
	return os.ReadDir(fullPath)
}

// ReadFile reads a file.
func (d *DirFS) ReadFile(filename string) ([]byte, error) {
	fullPath := filepath.Join(d.root, filename)
	return os.ReadFile(fullPath)
}

// Join joins path elements.
func (d *DirFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Base returns the last path element.
func (d *DirFS) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs returns an absolute path.
func (d *DirFS) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	fullPath := filepath.Join(d.root, path)
	return filepath.Abs(fullPath)
}
