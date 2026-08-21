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

package command

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func outputPathsEqual(left, right string) bool {
	return strings.EqualFold(left, right)
}

func launcherOutputPathIsReparse(path string) (bool, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attrs, err := windows.GetFileAttributes(name)
	if err != nil {
		return false, err
	}
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

func launcherOutputAliasAllowed(existing, resolved string) bool {
	return false
}

// launchpack validates the PE header before this size check.
func launcherStageIsExecutable(info os.FileInfo) bool {
	return info.Size() > 0
}

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

const replaceFileWriteThrough = 1

func replaceExistingLauncherOutput(stage, final string) error {
	finalName, err := windows.UTF16PtrFromString(final)
	if err != nil {
		return err
	}
	stageName, err := windows.UTF16PtrFromString(stage)
	if err != nil {
		return err
	}
	// ReplaceFileW performs a single replacement while retaining the old file
	// until the operation commits; no delete/rename empty window is exposed.
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(finalName)), uintptr(unsafe.Pointer(stageName)), 0, replaceFileWriteThrough, 0, 0,
	)
	if result != 0 {
		return nil
	}
	if callErr != nil {
		return callErr
	}
	return windows.GetLastError()
}

func moveLauncherOutput(stage, final string) error {
	stageName, err := windows.UTF16PtrFromString(stage)
	if err != nil {
		return err
	}
	finalName, err := windows.UTF16PtrFromString(final)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(stageName, finalName, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func commitLauncherOutputPlatform(stage, final string) error {
	if _, err := os.Lstat(final); err == nil {
		if err := replaceExistingLauncherOutput(stage, final); err == nil {
			return nil
		} else if err != windows.ERROR_FILE_NOT_FOUND && err != windows.ERROR_PATH_NOT_FOUND {
			return err
		}
	}
	// MOVEFILE_REPLACE_EXISTING handles a destination which appears after the
	// check above, while WRITE_THROUGH asks Windows to flush the move metadata.
	return moveLauncherOutput(stage, final)
}
