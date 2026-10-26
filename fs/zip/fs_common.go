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

package zip

import (
	"archive/zip"
	"fmt"
	"io"
	"syscall"

	"github.com/goplus/spx/v4/fs"
)

// Open opens a zipped file object.
func (zipf *FS) Open(name string) (io.ReadCloser, error) {
	for _, f := range zipf.File {
		if f.Name == name {
			return openZipEntry(f)
		}
	}
	return nil, fmt.Errorf("`%s` not found in zipfile: %w", name, syscall.ENOENT)
}

// newZipReader validates archive bounds before parsing and checking its entries.
func newZipReader(source io.ReaderAt, size int64) (*zip.Reader, error) {
	if err := preflightZipArchive(source, size); err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(source, size)
	if err != nil {
		return nil, err
	}
	if err := validateZipReader(reader); err != nil {
		return nil, err
	}
	return reader, nil
}

func init() {
	fs.RegisterSchema("zip", Open)
	fs.RegisterSchema("hzip", OpenHttp)
	fs.RegisterSchema("hzips", OpenHttps)
}
