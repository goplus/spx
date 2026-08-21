//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package runtimebundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type posixLockFile struct{ file *os.File }

func openPlatformLockFileImpl(root *os.Root, name string) (rawPlatformLockFile, error) {
	for attempt := 0; attempt < 16; attempt++ {
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			return &posixLockFile{file: file}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		directory, err := root.Open(".")
		if err != nil {
			return nil, err
		}
		fd, openErr := unix.Openat(int(directory.Fd()), name, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		closeErr := directory.Close()
		if openErr != nil {
			if errors.Is(openErr, os.ErrNotExist) {
				continue
			}
			if errors.Is(openErr, unix.ELOOP) {
				return nil, fmt.Errorf("%w: lock sidecar is a symlink: %s", ErrUnsafeArchive, name)
			}
			return nil, openErr
		}
		if closeErr != nil {
			_ = unix.Close(fd)
			return nil, closeErr
		}
		file = os.NewFile(uintptr(fd), filepath.Join(root.Name(), name))
		if file == nil {
			_ = unix.Close(fd)
			return nil, os.ErrInvalid
		}
		return &posixLockFile{file: file}, nil
	}
	return nil, fmt.Errorf("runtimebundle: lock sidecar changed repeatedly while opening: %s", name)
}

func (f *posixLockFile) tryLock(mode lockMode) (bool, error) {
	flags := unix.LOCK_NB
	if mode == lockExclusive {
		flags |= unix.LOCK_EX
	} else {
		flags |= unix.LOCK_SH
	}
	if err := unix.Flock(int(f.file.Fd()), flags); err != nil {
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *posixLockFile) unlock() error {
	return unix.Flock(int(f.file.Fd()), unix.LOCK_UN)
}

func (f *posixLockFile) close() error { return f.file.Close() }

func (f *posixLockFile) stat() (os.FileInfo, error) { return f.file.Stat() }

func (f *posixLockFile) protect() error { return f.file.Chmod(0o600) }

func validateLockParentSecurity(string) error { return nil }
