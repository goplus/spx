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

package projectassets

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

const maxConfigBytes = int64(16 << 20)

func openProjectRoot(name string) (*os.Root, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("projectassets: inspect ProjectDir: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("projectassets: ProjectDir %q is not a real directory", name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("projectassets: open ProjectDir: %w", err)
	}
	probe, err := root.Open(".")
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("projectassets: pin ProjectDir: %w", err)
	}
	opened, statErr := probe.Stat()
	closeErr := probe.Close()
	if statErr != nil || closeErr != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		root.Close()
		return nil, fmt.Errorf("projectassets: ProjectDir %q changed while opening", name)
	}
	return root, nil
}

func readPinnedFile(root *os.Root, name string) ([]byte, bool, error) {
	before, err := inspectPath(root, name, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false, fmt.Errorf("projectassets: open %q: %w", name, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before[len(before)-1], opened) {
		file.Close()
		return nil, false, fmt.Errorf("projectassets: %q changed while opening", name)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	after, inspectErr := inspectPath(root, name, false)
	if readErr != nil {
		return nil, false, fmt.Errorf("projectassets: read %q: %w", name, readErr)
	}
	if statErr != nil {
		return nil, false, fmt.Errorf("projectassets: stat %q: %w", name, statErr)
	}
	if closeErr != nil {
		return nil, false, fmt.Errorf("projectassets: close %q: %w", name, closeErr)
	}
	if int64(len(data)) > maxConfigBytes {
		return nil, false, fmt.Errorf("projectassets: %q exceeds %d bytes", name, maxConfigBytes)
	}
	if inspectErr != nil || int64(len(data)) != opened.Size() || !samePathSnapshot(before, after) || !sameStableFile(opened, afterOpened) || !os.SameFile(opened, afterOpened) {
		return nil, false, fmt.Errorf("projectassets: %q changed while reading", name)
	}
	return data, true, nil
}

func readPinnedDir(root *os.Root, name string) ([]os.DirEntry, bool, error) {
	before, err := inspectPath(root, name, true)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	directory, err := root.Open(name)
	if err != nil {
		return nil, false, fmt.Errorf("projectassets: open directory %q: %w", name, err)
	}
	opened, statErr := directory.Stat()
	if statErr != nil || !opened.IsDir() || !os.SameFile(before[len(before)-1], opened) {
		directory.Close()
		return nil, false, fmt.Errorf("projectassets: directory %q changed while opening", name)
	}
	entries, readErr := directory.ReadDir(-1)
	afterOpened, statErr := directory.Stat()
	closeErr := directory.Close()
	after, inspectErr := inspectPath(root, name, true)
	if readErr != nil {
		return nil, false, fmt.Errorf("projectassets: read directory %q: %w", name, readErr)
	}
	if statErr != nil {
		return nil, false, fmt.Errorf("projectassets: stat directory %q: %w", name, statErr)
	}
	if closeErr != nil {
		return nil, false, fmt.Errorf("projectassets: close directory %q: %w", name, closeErr)
	}
	if inspectErr != nil || !samePathSnapshot(before, after) || !sameStableFile(opened, afterOpened) || !os.SameFile(opened, afterOpened) {
		return nil, false, fmt.Errorf("projectassets: directory %q changed while reading", name)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, true, nil
}

func pinRegularFile(root *os.Root, name string) error {
	before, err := inspectPath(root, name, false)
	if err != nil {
		return err
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	afterOpened, secondStatErr := file.Stat()
	closeErr := file.Close()
	after, inspectErr := inspectPath(root, name, false)
	if statErr != nil {
		return statErr
	}
	if secondStatErr != nil {
		return secondStatErr
	}
	if closeErr != nil {
		return closeErr
	}
	if inspectErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before[len(before)-1], opened) || !os.SameFile(opened, afterOpened) || !sameStableFile(opened, afterOpened) || !samePathSnapshot(before, after) {
		return fmt.Errorf("projectassets: %q is not a stable regular non-symlink file", name)
	}
	return nil
}

func inspectPath(root *os.Root, name string, finalDirectory bool) ([]os.FileInfo, error) {
	if name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00") {
		return nil, fmt.Errorf("projectassets: unsafe project path %q", name)
	}
	parts := strings.Split(name, "/")
	infos := make([]os.FileInfo, 0, len(parts))
	for i := range parts {
		candidate := path.Join(parts[:i+1]...)
		info, err := root.Lstat(candidate)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("projectassets: %q is not a regular non-symlink path (symlink at %q)", name, candidate)
		}
		last := i == len(parts)-1
		if !last || finalDirectory {
			if !info.IsDir() {
				return nil, fmt.Errorf("projectassets: %q contains a non-directory component %q", name, candidate)
			}
		} else if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("projectassets: %q is not a regular non-symlink file", name)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func samePathSnapshot(before, after []os.FileInfo) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if !os.SameFile(before[i], after[i]) || !sameStableFile(before[i], after[i]) {
			return false
		}
	}
	return true
}

func sameStableFile(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
