//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

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

	"golang.org/x/sys/unix"
)

type safeDir struct {
	file *os.File
}

func openSafeRoot(name string, observed os.FileInfo) (*safeDir, error) {
	name = filepath.Clean(name)
	if !filepath.IsAbs(name) {
		return nil, fmt.Errorf("%w: root %q is not absolute", ErrInvalidPath, name)
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	fd, err := unix.Open(string(os.PathSeparator), flags, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(name, string(os.PathSeparator)), string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		nextFD, openErr := unix.Openat(fd, component, flags, 0)
		_ = unix.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("%w: no-follow open root %q: %w", ErrUnsafeFile, name, openErr)
		}
		fd = nextFD
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("projectbundle: wrap root descriptor %q", name)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.IsDir() || observed == nil || !os.SameFile(observed, info) {
		file.Close()
		return nil, fmt.Errorf("%w: root %q changed after it was observed", ErrUnsafeFile, name)
	}
	return &safeDir{file: file}, nil
}

func (d *safeDir) OpenFile(name string) (*os.File, error) {
	file, err := d.open(name, false)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("%w: %q has mode %s", ErrUnsafeFile, name, info.Mode())
	}
	return file, nil
}

func (d *safeDir) OpenDir(name string) (*safeDir, error) {
	file, err := d.open(name, true)
	if err != nil {
		return nil, err
	}
	return &safeDir{file: file}, nil
}

func (d *safeDir) open(name string, directory bool) (*os.File, error) {
	parts := strings.Split(name, string(os.PathSeparator))
	parentFD := int(d.file.Fd())
	ownedParent := false
	defer func() {
		if ownedParent {
			_ = unix.Close(parentFD)
		}
	}()

	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		last := i == len(parts)-1
		if !last || directory {
			flags |= unix.O_DIRECTORY
		}
		fd, err := unix.Openat(parentFD, part, flags, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: no-follow open %q: %w", ErrUnsafeFile, name, err)
		}
		if !last {
			if ownedParent {
				_ = unix.Close(parentFD)
			}
			parentFD = fd
			ownedParent = true
			continue
		}
		file := os.NewFile(uintptr(fd), name)
		if file == nil {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("projectbundle: wrap descriptor %q", name)
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, err
		}
		if directory && !info.IsDir() {
			file.Close()
			return nil, fmt.Errorf("%w: %q is not a directory", ErrUnsafeFile, name)
		}
		return file, nil
	}
	return nil, fmt.Errorf("%w: empty descriptor path", ErrInvalidPath)
}

func (d *safeDir) ReadDir() ([]fs.DirEntry, error) {
	return d.file.ReadDir(-1)
}

func (d *safeDir) Close() error {
	return d.file.Close()
}
