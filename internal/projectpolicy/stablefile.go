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

package projectpolicy

import (
	"fmt"
	"io"
	"os"
)

type stableFileAccess struct {
	lstat func(string) (os.FileInfo, error)
	open  func(string) (*os.File, error)
}

func readStableRegularFile(name string) ([]byte, os.FileInfo, bool, error) {
	return readStableFile(name, stableFileAccess{lstat: os.Lstat, open: os.Open})
}

func readStableRegularRootFile(root *os.Root, name string) ([]byte, os.FileInfo, bool, error) {
	return readStableFile(name, stableFileAccess{lstat: root.Lstat, open: root.Open})
}

func readStableFile(name string, access stableFileAccess) ([]byte, os.FileInfo, bool, error) {
	before, err := access.lstat(name)
	if os.IsNotExist(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, false, fmt.Errorf("must be a regular non-symlink file")
	}
	if before.Size() < 0 || before.Size() > maxPortableConfigBytes {
		return nil, nil, false, fmt.Errorf("exceeds %d bytes", maxPortableConfigBytes)
	}

	file, err := access.open(name)
	if err != nil {
		return nil, nil, false, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, false, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, nil, false, fmt.Errorf("changed while opening")
	}
	if opened.Size() < 0 || opened.Size() > maxPortableConfigBytes {
		file.Close()
		return nil, nil, false, fmt.Errorf("exceeds %d bytes", maxPortableConfigBytes)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxPortableConfigBytes+1))
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	afterPath, lstatErr := access.lstat(name)
	if readErr != nil {
		return nil, nil, false, readErr
	}
	if statErr != nil {
		return nil, nil, false, statErr
	}
	if closeErr != nil {
		return nil, nil, false, closeErr
	}
	if int64(len(data)) > maxPortableConfigBytes {
		return nil, nil, false, fmt.Errorf("exceeds %d bytes", maxPortableConfigBytes)
	}
	if lstatErr != nil || afterPath.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
		int64(len(data)) != opened.Size() || !stableMetadata(opened, afterOpened) ||
		!stableMetadata(afterOpened, afterPath) {
		return nil, nil, false, fmt.Errorf("changed while reading")
	}
	return data, afterOpened, true, nil
}

func stableMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
