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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// ZIP's portable DOS timestamp range starts in 1980. Using that epoch and
// canonical regular/executable modes keeps archives independent of workspace
// mtimes and the process umask.
var normalizedZipModTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// WriteNamedZip writes files to dst using the provided zip entry names.
func WriteNamedZip(dst string, namedFiles map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	return writeZipFile(dst, func(writer *zip.Writer) error {
		names := slices.Sorted(maps.Keys(namedFiles))

		for _, name := range names {
			if err := addFileToZip(writer, namedFiles[name], name); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeZipFile(dst string, writeEntries func(*zip.Writer) error) (err error) {
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
	if err := writeEntries(writer); err != nil {
		return err
	}
	return writer.Close()
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
	header.Modified = normalizedZipModTime.UTC()
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
