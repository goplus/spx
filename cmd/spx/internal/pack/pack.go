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
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/goplus/spx/v3/cmd/spx/internal/util"
	"github.com/goplus/spx/v3/internal/projectassets"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

type DirInfos struct {
	path string
	info os.FileInfo
}

func PackProject(baseFolder string, dstZipPath string) error {
	if err := projectpolicy.ValidateConfig(baseFolder); err != nil {
		return err
	}
	if _, err := projectassets.Resolve(projectassets.Config{
		ProjectDir: baseFolder,
		PackDir:    "assets",
		PackIndex:  "index.json",
	}); err != nil {
		return err
	}
	paths := []DirInfos{}
	if util.IsFileExist(dstZipPath) {
		if err := os.Remove(dstZipPath); err != nil {
			return err
		}
	}
	skipDirs := map[string]struct{}{
		".git": {}, "project": {},
	}

	file, err := os.Create(dstZipPath)
	if err != nil {
		return err
	}
	zipWriter := zip.NewWriter(file)
	closeZip := func(err error) error {
		if closeErr := zipWriter.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		return err
	}

	err = filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("project entry %s must not be a symlink", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("project entry %s must be a regular file or directory", path)
		}
		rel, err := filepath.Rel(baseFolder, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasSuffix(path, ".import") {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 1 || (len(parts) == 2 && info.IsDir()) {
			if _, ok := skipDirs[info.Name()]; ok {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		paths = append(paths, DirInfos{path: path, info: info})
		return nil
	})
	if err != nil {
		return closeZip(err)
	}

	return closeZip(PackZip(zipWriter, baseFolder, paths))
}

func PackZip(zipWriter *zip.Writer, baseFolder string, paths []DirInfos) error {
	baseFolder = strings.ReplaceAll(baseFolder, "\\", "/")
	slices.SortFunc(paths, func(a, b DirInfos) int {
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
		current, err := os.Lstat(filePath)
		if err != nil {
			return fmt.Errorf("inspect project entry %s before packing: %w", filePath, err)
		}
		if current.Mode()&os.ModeSymlink != 0 || (!current.IsDir() && !current.Mode().IsRegular()) || !os.SameFile(info, current) {
			return fmt.Errorf("project entry %s changed after collection", filePath)
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
		if info.IsDir() {
			header.Name += "/"
			_, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}
			continue
		}

		fileToZip, err := os.Open(filePath)
		if err != nil {
			return err
		}
		opened, err := fileToZip.Stat()
		if err != nil {
			fileToZip.Close()
			return fmt.Errorf("stat opened project entry %s: %w", filePath, err)
		}
		current, err = os.Lstat(filePath)
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
		afterPath, lstatErr := os.Lstat(filePath)
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

func sameStableFileMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}

func PackDirFiles(zipName string, targetDir string, directories, files []string) error {
	zipFile, err := os.Create(zipName)
	if err != nil {
		return err
	}
	zipWriter := zip.NewWriter(zipFile)
	closeZip := func(err error) error {
		if closeErr := zipWriter.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := zipFile.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		return err
	}

	paths := []DirInfos{}
	for _, dir := range directories {
		paths, err = addDirToZip(path.Join(targetDir, dir), paths)
		if err != nil {
			return closeZip(err)
		}
	}

	for _, file := range files {
		paths, err = addFileToZip(path.Join(targetDir, file), paths)
		if err != nil {
			return closeZip(err)
		}
	}

	return closeZip(PackZip(zipWriter, targetDir, paths))
}

func zipEntryName(baseFolder string, dirInfo DirInfos) string {
	baseFolder = normalizeZipPath(baseFolder)
	name := strings.TrimPrefix(normalizeZipPath(dirInfo.path), baseFolder)
	return strings.TrimPrefix(name, "/")
}

func normalizeZipPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func addDirToZip(dirPath string, paths []DirInfos) ([]DirInfos, error) {
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, DirInfos{path: path, info: info})
		return nil
	})
	return paths, err
}

func addFileToZip(path string, paths []DirInfos) ([]DirInfos, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	paths = append(paths, DirInfos{path: path, info: info})
	return paths, nil
}
