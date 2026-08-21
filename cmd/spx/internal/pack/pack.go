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

package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type dirInfo struct {
	path string
	info os.FileInfo
	// zipPath overrides the zip entry path for a legacy external asset.
	zipPath  string
	root     *os.Root
	rootPath string
}

func PackProject(baseFolder string, dstZipPath string) error {
	projectRoot, err := openPackRoot(baseFolder)
	if err != nil {
		return err
	}
	defer projectRoot.Close()
	if err := validatePackDestination(dstZipPath); err != nil {
		return err
	}
	paths, err := collectProjectPaths(baseFolder, dstZipPath, projectRoot)
	if err != nil {
		return err
	}
	extAssetDir, err := validateLegacyPackInputs(baseFolder)
	if err != nil {
		return err
	}
	existingZipPaths := make(map[string]struct{}, len(paths))
	for _, dirInfo := range paths {
		existingZipPaths[zipEntryName(baseFolder, dirInfo)] = struct{}{}
	}
	extraPaths, err := collectExternalAssetPathsWithConfig(baseFolder, existingZipPaths, &extAssetDir)
	if err != nil {
		return err
	}
	defer closePackRoots(extraPaths)
	paths = append(paths, extraPaths...)

	tempName, file, err := createPackOutput(dstZipPath)
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()

	zipWriter := zip.NewWriter(file)
	if err := closePackOutput(zipWriter, file, packZip(zipWriter, baseFolder, paths)); err != nil {
		return err
	}
	if err := publishPackOutput(tempName, dstZipPath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func packZip(zipWriter *zip.Writer, baseFolder string, paths []dirInfo) error {
	baseRootPath := filepath.Clean(baseFolder)
	var defaultRoot *os.Root
	for i := range paths {
		if paths[i].root != nil {
			continue
		}
		if defaultRoot == nil {
			var err error
			defaultRoot, err = openPackRoot(baseRootPath)
			if err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(baseRootPath, filepath.Clean(paths[i].path))
		if err != nil {
			defaultRoot.Close()
			return fmt.Errorf("project entry %s: resolve path relative to base folder: %w", paths[i].path, err)
		}
		if rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			defaultRoot.Close()
			return fmt.Errorf("project entry %s is outside base folder", paths[i].path)
		}
		paths[i].root = defaultRoot
		paths[i].rootPath = rel
	}
	if defaultRoot != nil {
		defer defaultRoot.Close()
	}
	baseFolder = strings.ReplaceAll(baseFolder, "\\", "/")
	seenNames := make(map[string]struct{}, len(paths))
	slices.SortFunc(paths, func(a, b dirInfo) int {
		nameA := zipEntryName(baseFolder, a)
		nameB := zipEntryName(baseFolder, b)
		if nameA < nameB {
			return -1
		} else if nameA > nameB {
			return 1
		}
		return 0
	})
	for _, dirInfo := range paths {
		filePath := dirInfo.path
		info := dirInfo.info
		current, err := dirInfo.root.Lstat(dirInfo.rootPath)
		if err != nil {
			return fmt.Errorf("inspect project entry %s before packing: %w", filePath, err)
		}
		if current.Mode()&os.ModeSymlink != 0 || (!current.IsDir() && !current.Mode().IsRegular()) || !os.SameFile(info, current) {
			return fmt.Errorf("project entry %s changed after collection", filePath)
		}
		if current.IsDir() != info.IsDir() {
			return fmt.Errorf("project entry %s changed type after collection", filePath)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Modified = time.Unix(0, 0)

		header.Name = zipEntryName(baseFolder, dirInfo)
		if header.Name == "" {
			continue
		}
		if !validZipEntryName(header.Name) {
			return fmt.Errorf("project entry %s has unsafe zip name %q", filePath, header.Name)
		}
		if _, exists := seenNames[header.Name]; exists {
			return fmt.Errorf("project entry %s duplicates zip name %q", filePath, header.Name)
		}
		seenNames[header.Name] = struct{}{}
		if info.IsDir() {
			header.Name += "/"
			_, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}
			continue
		}

		fileToZip, err := dirInfo.root.Open(dirInfo.rootPath)
		if err != nil {
			return err
		}
		opened, err := fileToZip.Stat()
		if err != nil {
			fileToZip.Close()
			return fmt.Errorf("stat opened project entry %s: %w", filePath, err)
		}
		current, err = dirInfo.root.Lstat(dirInfo.rootPath)
		if err != nil || !opened.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, opened) || !os.SameFile(opened, current) {
			fileToZip.Close()
			return fmt.Errorf("project entry %s changed while opening", filePath)
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			fileToZip.Close()
			return err
		}
		copied, copyErr := io.Copy(writer, fileToZip)
		afterOpened, statErr := fileToZip.Stat()
		afterPath, lstatErr := dirInfo.root.Lstat(dirInfo.rootPath)
		closeErr := fileToZip.Close()
		if copyErr != nil {
			return copyErr
		}
		if statErr != nil || lstatErr != nil || afterPath.Mode()&os.ModeSymlink != 0 ||
			!os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
			copied != opened.Size() || !sameStableFileMetadata(opened, afterOpened) {
			return fmt.Errorf("project entry %s changed while packing", filePath)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func validZipEntryName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func sameStableFileMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}

func openPackRoot(path string) (*os.Root, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("pack root %q must be a real directory", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil {
		root.Close()
		return nil, err
	}
	if !opened.IsDir() || !os.SameFile(before, opened) || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		root.Close()
		return nil, fmt.Errorf("pack root %q changed while it was opened", path)
	}
	return root, nil
}

func closePackRoots(paths []dirInfo) {
	closed := make(map[*os.Root]struct{})
	for _, dirInfo := range paths {
		if dirInfo.root == nil {
			continue
		}
		if _, ok := closed[dirInfo.root]; ok {
			continue
		}
		closed[dirInfo.root] = struct{}{}
		_ = dirInfo.root.Close()
	}
}

func zipEntryName(baseFolder string, dirInfo dirInfo) string {
	if dirInfo.zipPath != "" {
		return strings.TrimPrefix(normalizeZipPath(dirInfo.zipPath), "/")
	}
	baseFolder = normalizeZipPath(baseFolder)
	name := strings.TrimPrefix(normalizeZipPath(dirInfo.path), baseFolder)
	return strings.TrimPrefix(name, "/")
}
