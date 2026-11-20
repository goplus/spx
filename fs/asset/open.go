//go:build !android && !ios
// +build !android,!ios

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
	"fmt"
	"io"
	"os"
	"path/filepath"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/base/fileutil"
)

// Open selects one local asset directory. A generated project directory can
// use its parent's assets when it has no local copy.
func Open(base string) (spxfs.Dir, error) {
	_, base = fileutil.AssetRoots(base)
	return &FS{base: base + "/"}, nil
}

func (f *FS) Open(name string) (io.ReadCloser, error) {
	file, err := f.resolvePhysicalPath(name)
	if err != nil {
		return nil, err
	}
	return os.Open(file)
}

func (f *FS) ReadDir(name string) ([]spxfs.DirEntry, error) {
	dir, err := f.resolvePhysicalPath(name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	return spxfs.DirEntriesFromFS(entries), nil
}

// ValidateResourcePath checks the physical location, including symlink targets.
func (f *FS) ValidateResourcePath(name string) error {
	_, err := f.resolvePhysicalPath(name)
	return err
}

func (f *FS) resolvePhysicalPath(name string) (string, error) {
	localName, err := localResourcePath(name)
	if err != nil {
		return "", err
	}
	root, err := fileutil.CanonicalPath(f.base)
	if err != nil {
		return "", err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(root, localName))
	if err != nil {
		return "", err
	}
	if !fileutil.IsWithinRoot(target, root) {
		return "", fmt.Errorf("resource %q is outside the asset directory", name)
	}
	return target, nil
}
