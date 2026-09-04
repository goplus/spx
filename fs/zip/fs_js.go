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

package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"syscall"

	"github.com/goplus/spx/v3/fs"
)

// A FS represents a zip filesystem.
type FS struct {
	*zip.Reader
}

// Open opens a zip filesystem object.
func Open(file string) (fs.Dir, error) {
	return OpenHttp(file)
}

// Open opens a zipped file object.
func (zipf *FS) Open(name string) (io.ReadCloser, error) {
	for _, f := range zipf.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("`%s` not found in zipfile: %w", name, syscall.ENOENT)
}

// Close closes the filesystem object.
func (zipf *FS) Close() error {
	return nil
}

// OpenHttp opens hzip:<domain>/<path>
// OpenHttp("open.qiniu.us/weather/res.zip")
func OpenHttp(remotePath string) (fs.Dir, error) {
	return openHttpWith(remotePath, "http://")
}

// OpenHttps opens hzips:<domain>/<path>
// OpenHttps("open.qiniu.us/weather/res.zip")
func OpenHttps(remotePath string) (fs.Dir, error) {
	return openHttpWith(remotePath, "https://")
}

func openHttpWith(remotePath, scheme string) (fs.Dir, error) {
	remote, _, err := parseRemoteURL(remotePath, scheme)
	if err != nil {
		return nil, err
	}
	resp, err := getRemote(remote, scheme)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readRemoteBody(resp)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(body)
	zipf, err := zip.NewReader(r, int64(r.Len()))
	if err != nil {
		return nil, err
	}
	return &FS{zipf}, nil
}

func init() {
	fs.RegisterSchema("zip", Open)
	fs.RegisterSchema("hzip", OpenHttp)
	fs.RegisterSchema("hzips", OpenHttps)
}
