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
	"runtime"
	"slices"
)

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

// ReplaceRegularFile rejects an existing non-regular or symlink destination,
// renames src to dst, then sets mode except on Windows.
// It does not validate src or pin either path against concurrent changes.
func ReplaceRegularFile(src, dst string, mode os.FileMode) error {
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("destination %q is not a regular non-symlink file", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Chmod(dst, mode)
	}
	return nil
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

// ZipDirectory writes the files in srcDir to dstZip using slash-separated relative paths.
func ZipDirectory(srcDir, dstZip string) error {
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
	slices.Sort(files)

	return writeZipFile(dstZip, func(writer *zip.Writer) error {
		for _, path := range files {
			rel, err := filepath.Rel(srcDir, path)
			if err != nil {
				return err
			}
			if err := addFileToZip(writer, path, filepath.ToSlash(rel)); err != nil {
				return err
			}
		}
		return nil
	})
}

func mkdirAllWithMode(path string, mode fs.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
