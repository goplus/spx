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

package interpsession

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
)

var scaffoldTempSequence atomic.Uint64

func ensureDirectoryNoSymlink(name, path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, mode); err != nil {
			return fmt.Errorf("interpsession: create %s %q: %w", name, path, err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("interpsession: stat %s %q: %w", name, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("interpsession: %s %q must not be a symlink", name, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("interpsession: %s %q is not a directory", name, path)
	}
	return nil
}

func openPinnedRoot(name, path string) (*os.Root, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("interpsession: lstat %s %q: %w", name, path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("interpsession: %s %q is not a real directory", name, path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("interpsession: open %s root %q: %w", name, path, err)
	}
	after, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("interpsession: stat opened %s root %q: %w", name, path, err)
	}
	if !after.IsDir() || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("interpsession: %s %q changed while it was opened", name, path)
	}
	if err := verifyPinnedRootPath(name, path, root); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func verifyPinnedRootPath(name, path string, root *os.Root) error {
	opened, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("interpsession: stat pinned %s %q: %w", name, path, err)
	}
	current, err := os.Lstat(path)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("interpsession: %s %q changed after it was opened", name, path)
	}
	return nil
}

func ensurePinnedSubdirectory(parent *os.Root, name string, mode os.FileMode) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		if err := parent.Mkdir(name, mode); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("interpsession: create session directory %q: %w", name, err)
		}
		before, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("interpsession: inspect session directory %q: %w", name, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("interpsession: session directory %q is not a real directory", name)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("interpsession: open session directory %q: %w", name, err)
	}
	after, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	if !after.IsDir() || !os.SameFile(before, after) {
		root.Close()
		return nil, fmt.Errorf("interpsession: session directory %q changed while it was opened", name)
	}
	if err := verifyPinnedSubdirectory(parent, name, root); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func verifyPinnedSubdirectory(parent *os.Root, name string, root *os.Root) error {
	opened, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("interpsession: stat pinned session directory %q: %w", name, err)
	}
	current, err := parent.Lstat(name)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("interpsession: session directory %q changed after it was opened", name)
	}
	return nil
}

func replaceRootFile(root *os.Root, name string, input io.Reader, mode os.FileMode) (err error) {
	if name == "" || name == "." || filepath.Base(name) != name {
		return fmt.Errorf("interpsession: invalid scaffold file name %q", name)
	}
	if info, statErr := root.Lstat(name); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("interpsession: scaffold target %q is not a regular non-symlink file", name)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("interpsession: inspect scaffold target %q: %w", name, statErr)
	}

	var tempName string
	var output *os.File
	for range 100 {
		tempName = fmt.Sprintf(".%s.tmp-%d-%d", name, os.Getpid(), scaffoldTempSequence.Add(1))
		output, err = root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("interpsession: create temporary scaffold %q: %w", name, err)
		}
	}
	if output == nil {
		return fmt.Errorf("interpsession: could not allocate temporary scaffold for %q", name)
	}
	defer func() {
		_ = root.Remove(tempName)
	}()
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpsession: write temporary scaffold %q: %w", name, err)
	}
	if err := output.Chmod(mode); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpsession: chmod temporary scaffold %q: %w", name, err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return fmt.Errorf("interpsession: sync temporary scaffold %q: %w", name, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("interpsession: close temporary scaffold %q: %w", name, err)
	}
	if runtime.GOOS == "windows" {
		// Windows rename does not consistently replace an existing regular file.
		// Root.Remove removes the directory entry itself and never follows it.
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("interpsession: replace existing scaffold %q: %w", name, err)
		}
	}
	if err := root.Rename(tempName, name); err != nil {
		return fmt.Errorf("interpsession: publish scaffold %q: %w", name, err)
	}
	return nil
}

func validateRegularFile(name, path string) error {
	file, _, err := openPinnedRegularFile(name, path)
	if err != nil {
		return err
	}
	return file.Close()
}

func openPinnedRegularFile(name, path string) (*os.File, os.FileInfo, error) {
	if err := validateAbsoluteCleanPath(name, path); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("interpsession: lstat %s %q: %w", name, path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("interpsession: %s %q is not a regular non-symlink file", name, path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("interpsession: open %s %q: %w", name, path, err)
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("interpsession: stat opened %s %q: %w", name, path, err)
	}
	after, err := os.Lstat(path)
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		file.Close()
		return nil, nil, fmt.Errorf("interpsession: %s %q changed while it was opened", name, path)
	}
	return file, opened, nil
}
