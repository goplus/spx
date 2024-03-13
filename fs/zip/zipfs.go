//go:build !js
// +build !js

/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"io"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/goplus/spx/fs"
	"github.com/pkg/errors"
)

// -------------------------------------------------------------------------------------

// A FS represents a zip filesystem.
type FS zip.ReadCloser

// Open opens a zip filesystem object.
func Open(file string) (fs.Dir, error) {
	zipf, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	return (*FS)(zipf), nil
}

// Open opens a zipped file object.
func (zipf *FS) Open(name string) (io.ReadCloser, error) {
	for _, f := range zipf.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, errors.Wrapf(syscall.ENOENT, "`%s` not found in zipfile", name)
}

// Close closes the filesystem object.
func (zipf *FS) Close() error {
	return ((*zip.ReadCloser)(zipf)).Close()
}

// OpenHttp opens hzip:<domain>/<path>
// OpenHttp("open.qiniu.us/weather/res.zip")
func OpenHttp(url string) (fs.Dir, error) {
	return openHttpWith(url, "http://")
}

// OpenHttps opens hzips:<domain>/<path>
// OpenHttps("open.qiniu.us/weather/res.zip")
func OpenHttps(url string) (fs.Dir, error) {
	return openHttpWith(url, "https://")
}

func openHttpWith(url string, schema string) (dir fs.Dir, err error) {
	local := spxBaseDir + url
	dir, err = Open(local)
	if err == nil {
		return
	}

	remote := schema + url
	resp, err := http.Get(remote)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = saveTo(local, resp)
	if err != nil {
		return
	}
	return Open(local)
}

func saveTo(local string, resp *http.Response) (err error) {
	dir := path.Dir(local)
	os.MkdirAll(dir, 0777)

	f, err := os.Create(local)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return
}

var (
	spxBaseDir = os.Getenv("HOME") + "/.spx/"
)

func init() {
	fs.RegisterSchema("zip", Open)
	fs.RegisterSchema("hzip", OpenHttp)
	fs.RegisterSchema("hzips", OpenHttps)
}

// -------------------------------------------------------------------------------------
