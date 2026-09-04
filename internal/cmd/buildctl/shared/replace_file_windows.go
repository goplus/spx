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

package shared

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

const (
	replaceFileWriteThrough = 1
	windowsLongPathLimit    = 248
)

func windowsPath(path string) (string, error) {
	if path == "" || hasWindowsNamespace(path) {
		return path, nil
	}
	full, err := windows.FullPath(path)
	if err != nil {
		return "", err
	}
	if len(full) < windowsLongPathLimit {
		return path, nil
	}
	if len(full) >= 2 && os.IsPathSeparator(full[0]) && os.IsPathSeparator(full[1]) {
		return `\\?\UNC\` + full[2:], nil
	}
	return `\\?\` + full, nil
}

func hasWindowsNamespace(path string) bool {
	if strings.HasPrefix(path, `\??\`) {
		return true
	}
	return len(path) >= 4 &&
		os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) &&
		(path[2] == '?' || path[2] == '.') && os.IsPathSeparator(path[3])
}

func windowsPathPtr(path string) (*uint16, error) {
	path, err := windowsPath(path)
	if err != nil {
		return nil, err
	}
	return windows.UTF16PtrFromString(path)
}

func replaceExistingFile(src, dst string) error {
	dstName, err := windowsPathPtr(dst)
	if err != nil {
		return err
	}
	srcName, err := windowsPathPtr(src)
	if err != nil {
		return err
	}
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(dstName)), uintptr(unsafe.Pointer(srcName)), 0, replaceFileWriteThrough, 0, 0,
	)
	if result != 0 {
		return nil
	}
	return callErr
}

func moveFile(src, dst string) error {
	srcName, err := windowsPathPtr(src)
	if err != nil {
		return err
	}
	dstName, err := windowsPathPtr(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(srcName, dstName, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func replaceFile(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		if err := replaceExistingFile(src, dst); err == nil {
			return nil
		} else if err != windows.ERROR_FILE_NOT_FOUND && err != windows.ERROR_PATH_NOT_FOUND {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return moveFile(src, dst)
}
