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
	"os"
	"slices"
	"strings"
	"time"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/base/fileutil"
	"github.com/goplus/spx/v4/internal/launchpack/assets"
)

type archiveEntry struct {
	relPath string
	zipName string
	info    os.FileInfo
}

func PackProject(projectDir, dstZipPath string) error {
	projectRoot, err := openPackRoot(projectDir)
	if err != nil {
		return err
	}
	defer projectRoot.Close()
	if err := validatePackDestination(dstZipPath); err != nil {
		return err
	}
	entries, err := collectProjectEntries(projectDir, dstZipPath)
	if err != nil {
		return err
	}
	if err := assets.ValidateLegacy(projectRoot); err != nil {
		return fmt.Errorf("pack: validate asset indexes: %w", err)
	}

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
	if err := closePackOutput(zipWriter, file, writeZipEntries(zipWriter, projectRoot, entries)); err != nil {
		return err
	}
	if err := publishPackOutput(tempName, dstZipPath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func writeZipEntries(zipWriter *zip.Writer, root *os.Root, entries []archiveEntry) error {
	seenNames := make(map[string]struct{}, len(entries))
	slices.SortFunc(entries, func(a, b archiveEntry) int {
		return strings.Compare(a.zipName, b.zipName)
	})
	for _, entry := range entries {
		relPath := entry.relPath
		info := entry.info
		current, err := root.Lstat(relPath)
		if err != nil {
			return fmt.Errorf("inspect project entry %s before packing: %w", relPath, err)
		}
		if current.IsDir() != info.IsDir() {
			return fmt.Errorf("project entry %s changed type after collection", relPath)
		}
		if (!current.IsDir() && !current.Mode().IsRegular()) || !os.SameFile(info, current) {
			return fmt.Errorf("project entry %s changed after collection", relPath)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Modified = time.Unix(0, 0)

		header.Name = entry.zipName
		if !validZipEntryName(header.Name) {
			return fmt.Errorf("project entry %s has unsafe zip name %q", relPath, header.Name)
		}
		if _, exists := seenNames[header.Name]; exists {
			return fmt.Errorf("project entry %s duplicates zip name %q", relPath, header.Name)
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

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		reader := fileutil.StableReader{Expected: info, Lstat: root.Lstat, Open: root.Open}
		if _, err := reader.Copy(relPath, writer); err != nil {
			switch err {
			case fileutil.ErrNotRegular, fileutil.ErrChangedOpening:
				return fmt.Errorf("project entry %s changed while opening", relPath)
			case fileutil.ErrChangedReading:
				return fmt.Errorf("project entry %s changed while packing", relPath)
			default:
				return err
			}
		}
	}
	return nil
}

func validZipEntryName(name string) bool {
	if name == "" || spxfs.IsAbsResourcePath(name) {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
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
