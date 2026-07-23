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

package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/base/fileutil"
)

var errFileFound = errors.New("file found")

// CopyDir2 copies a directory from the local filesystem.
func CopyDir2(src string, dst string) error {
	return fileutil.CopyDir(src, dst)
}

// CheckFileExist reports whether dir contains a file with ext.
func CheckFileExist(dir, ext string, recursive bool) bool {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
				return errFileFound
			}
			return nil
		})

		if errors.Is(err, errFileFound) {
			return true
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
				return true
			}
		}
	}

	return false
}

// IsFileExist reports whether path exists.
func IsFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CopyFile copies a file.
func CopyFile(src, dst string) error {
	return fileutil.CopyFile(src, dst)
}

// CopyDir copies a directory from fsys into dstDir.
func CopyDir(fsys fs.FS, srcDir, dstDir string, isOverride bool) error {
	subfs, err := fs.Sub(fsys, srcDir)
	if err != nil {
		return fmt.Errorf("create sub fs for %s: %w", srcDir, err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dstDir, err)
	}
	return fs.WalkDir(subfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk directory %s: %w", srcDir, err)
		}

		dstPath := filepath.Join(dstDir, path)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		} else {
			if strings.HasSuffix(dstPath, "go.mod.txt") {
				i := strings.LastIndex(dstPath, "go.mod.txt")
				dstPath = dstPath[:i] + "go.mod"
			}

			if !isOverride {
				if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
					return nil
				}
			}

			srcFile, err := subfs.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			return err
		}
	})
}
