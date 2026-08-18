//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

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

package projectbundle

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// safeDir uses os.Root on platforms without openat(2). os.Root provides
// root-confined opens; explicit Lstat/open/fstat identity checks reject links
// and replacements observed around an open.
type safeDir struct {
	root *os.Root
}

func openSafeRoot(name string, observed os.FileInfo) (*safeDir, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() || observed == nil || !os.SameFile(observed, before) {
		return nil, fmt.Errorf("%w: root %q is not a real directory", ErrUnsafeFile, name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	probe, err := root.Open(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	after, statErr := probe.Stat()
	closeErr := probe.Close()
	if statErr != nil {
		root.Close()
		return nil, statErr
	}
	if closeErr != nil {
		root.Close()
		return nil, closeErr
	}
	if !after.IsDir() || !os.SameFile(observed, after) || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("%w: root %q changed while it was opened", ErrUnsafeFile, name)
	}
	return &safeDir{root: root}, nil
}

func (d *safeDir) OpenFile(name string) (*os.File, error) {
	before, err := d.validateComponents(name, false)
	if err != nil {
		return nil, err
	}
	file, err := d.root.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, fmt.Errorf("%w: file %q changed while it was opened", ErrUnsafeFile, name)
	}
	return file, nil
}

func (d *safeDir) OpenDir(name string) (*safeDir, error) {
	before, err := d.validateComponents(name, true)
	if err != nil {
		return nil, err
	}
	root, err := d.root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	probe, err := root.Open(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	after, statErr := probe.Stat()
	closeErr := probe.Close()
	if statErr != nil {
		root.Close()
		return nil, statErr
	}
	if closeErr != nil {
		root.Close()
		return nil, closeErr
	}
	if !after.IsDir() || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("%w: directory %q changed while it was opened", ErrUnsafeFile, name)
	}
	return &safeDir{root: root}, nil
}

func (d *safeDir) validateComponents(name string, finalDirectory bool) (os.FileInfo, error) {
	parts := strings.Split(filepath.Clean(name), string(filepath.Separator))
	for i := range parts {
		candidate := filepath.Join(parts[:i+1]...)
		info, err := d.root.Lstat(candidate)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %q contains a symbolic link", ErrUnsafeFile, name)
		}
		last := i == len(parts)-1
		if !last || finalDirectory {
			if !info.IsDir() {
				return nil, fmt.Errorf("%w: %q contains a non-directory component", ErrUnsafeFile, name)
			}
		} else if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %q is not a regular file", ErrUnsafeFile, name)
		}
		if last {
			return info, nil
		}
	}
	return nil, fmt.Errorf("%w: empty descriptor path", ErrInvalidPath)
}

func (d *safeDir) ReadDir() ([]fs.DirEntry, error) {
	file, err := d.root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := file.ReadDir(-1)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	return entries, closeErr
}

func (d *safeDir) Close() error {
	return d.root.Close()
}
