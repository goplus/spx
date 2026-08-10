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

package fileutil

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ZIP's portable DOS timestamp range starts in 1980. Using that epoch and
// canonical regular/executable modes keeps archives independent of workspace
// mtimes and the process umask.
var normalizedZipModTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// CopyFile streams a file from src to dst, creating dst's parent directory and preserving file mode.
func CopyFile(src, dst string) (err error) {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := input.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("fileutil.CopyFile: %s is a directory", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := output.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if err := output.Chmod(info.Mode()); err != nil {
		return err
	}

	_, err = io.Copy(output, input)
	return err
}

// CopyDir recursively copies a local directory from src to dst.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return mkdirAllWithMode(target, info.Mode())
		}
		return CopyFile(path, target)
	})
}

// WriteNamedZip writes files to dst using the provided zip entry names.
func WriteNamedZip(dst string, namedFiles map[string]string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	writer := zip.NewWriter(file)

	names := make([]string, 0, len(namedFiles))
	for name := range namedFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		src := namedFiles[name]
		if err := addFileToZip(writer, src, name); err != nil {
			return err
		}
	}
	return writer.Close()
}

// ZipDirectory writes the files in srcDir to dstZip using slash-separated relative paths.
func ZipDirectory(srcDir, dstZip string) (err error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", srcDir)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", srcDir)
	}
	if err := os.MkdirAll(filepath.Dir(dstZip), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dstZip); err != nil {
		return err
	}

	var files []string
	if err := filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	file, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	writer := zip.NewWriter(file)

	for _, path := range files {
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if err := addFileToZip(writer, path, filepath.ToSlash(rel)); err != nil {
			return err
		}
	}
	return writer.Close()
}

func mkdirAllWithMode(path string, mode fs.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func addFileToZip(writer *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("fileutil.addFileToZip: %s is a directory", src)
	}
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetModTime(normalizedZipModTime)
	header.SetMode(normalizedZipMode(info.Mode()))

	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, input); err != nil {
		_ = input.Close()
		return err
	}
	return input.Close()
}

func normalizedZipMode(mode fs.FileMode) fs.FileMode {
	if mode.Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}
