//go:build windows

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

	"golang.org/x/sys/windows"
)

const (
	lockFileFailImmediately = 0x00000001
	lockFileExclusiveLock   = 0x00000002
)

type windowsLockFile struct {
	file       *os.File
	overlapped windows.Overlapped
	locked     bool
}

func openPlatformLockFileImpl(root *os.Root, name string) (rawPlatformLockFile, error) {
	for attempt := 0; attempt < 16; attempt++ {
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			return &windowsLockFile{file: file}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		file, err = root.OpenFile(name, os.O_RDWR, 0)
		if err == nil {
			return &windowsLockFile{file: file}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("runtimebundle: lock sidecar changed repeatedly while opening: %s", name)
}

func (f *windowsLockFile) tryLock(mode lockMode) (bool, error) {
	flags := uint32(lockFileFailImmediately)
	if mode == lockExclusive {
		flags |= lockFileExclusiveLock
	}
	err := windows.LockFileEx(windows.Handle(f.file.Fd()), flags, 0, 1, 0, &f.overlapped)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return false, nil
		}
		return false, err
	}
	f.locked = true
	return true, nil
}

func (f *windowsLockFile) unlock() error {
	if !f.locked {
		return nil
	}
	f.locked = false
	return windows.UnlockFileEx(windows.Handle(f.file.Fd()), 0, 1, 0, &f.overlapped)
}

func (f *windowsLockFile) close() error { return f.file.Close() }

func (f *windowsLockFile) stat() (os.FileInfo, error) { return f.file.Stat() }

func (f *windowsLockFile) protect() error { return nil }

func validateLockParentSecurity(path string) error { return verifyPrivateDACL(path) }
