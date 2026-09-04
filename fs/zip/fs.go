//go:build !js
// +build !js

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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"

	"github.com/goplus/spx/v3/fs"
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
	if err := validateZipReader(&zipf.Reader); err != nil {
		_ = zipf.Close()
		return nil, err
	}
	return (*FS)(zipf), nil
}

// Open opens a zipped file object.
func (zipf *FS) Open(name string) (io.ReadCloser, error) {
	for _, f := range zipf.File {
		if f.Name == name {
			return openZipEntry(f)
		}
	}
	return nil, fmt.Errorf("`%s` not found in zipfile: %w", name, syscall.ENOENT)
}

// Close closes the filesystem object.
func (zipf *FS) Close() error {
	return ((*zip.ReadCloser)(zipf)).Close()
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
	remote, cacheKey, err := parseRemoteURL(remotePath, scheme)
	if err != nil {
		return nil, err
	}
	cachePath, err := remoteCachePath(spxBaseDir, scheme, cacheKey)
	if err != nil {
		return nil, err
	}
	if dir, ok := openCached(cachePath); ok {
		return dir, nil
	}

	resp, err := getRemote(remote, scheme)
	if err != nil {
		return nil, fmt.Errorf("zip: download %s: %w", remote, err)
	}
	defer resp.Body.Close()

	if err := saveTo(cachePath, resp); err != nil {
		return nil, err
	}
	return Open(cachePath)
}

func saveTo(cachePath string, resp *http.Response) (err error) {
	if cachePath == "" || !filepath.IsAbs(cachePath) {
		return fmt.Errorf("zip: cache path must be absolute: %q", cachePath)
	}
	if err := checkRemoteHTTPResponse(resp, "remote archive"); err != nil {
		return err
	}
	if resp.Body == nil {
		return fmt.Errorf("zip: HTTP response has no body")
	}
	cacheDir := filepath.Dir(cachePath)
	if err := ensureCacheDirectories(cacheDir); err != nil {
		return err
	}
	f, err := os.CreateTemp(cacheDir, ".zip-cache-*")
	if err != nil {
		return fmt.Errorf("zip: create cache temporary file: %w", err)
	}
	tmpPath := f.Name()
	committed := false
	fileClosed := false
	defer func() {
		if !fileClosed {
			if closeErr := f.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("zip: close cache temporary file: %w", closeErr)
			}
		}
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	limit := remoteMaxBytes()
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	if copyErr != nil {
		return fmt.Errorf("zip: download remote archive: %w", copyErr)
	}
	if n > limit {
		return fmt.Errorf("%w: response exceeds limit %d", ErrRemoteSizeLimit, limit)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("zip: sync cache temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("zip: close cache temporary file: %w", err)
	}
	fileClosed = true

	archive, err := Open(tmpPath)
	if err != nil {
		return fmt.Errorf("zip: downloaded response is not a valid ZIP: %w", err)
	}
	if closeErr := archive.Close(); closeErr != nil {
		return fmt.Errorf("zip: close downloaded ZIP: %w", closeErr)
	}
	if err := publishCacheFile(tmpPath, cachePath); err != nil {
		return err
	}
	committed = true
	return nil
}

func openCached(cachePath string) (fs.Dir, bool) {
	info, err := os.Lstat(cachePath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false
	}
	dir, err := Open(cachePath)
	if err != nil {
		return nil, false
	}
	return dir, true
}

func remoteCachePath(base, scheme, cacheKey string) (string, error) {
	if base == "" || !filepath.IsAbs(base) {
		return "", fmt.Errorf("zip: cache directory must be an absolute clean path: %q", base)
	}
	base = filepath.Clean(base)
	if len(cacheKey) != 64 {
		return "", fmt.Errorf("zip: invalid remote cache key")
	}
	var protocol string
	switch scheme {
	case "http://":
		protocol = "http"
	case "https://":
		protocol = "https"
	default:
		return "", fmt.Errorf("zip: unsupported remote transport %q", scheme)
	}
	return filepath.Join(base, protocol, cacheKey+".zip"), nil
}

func ensureCacheDirectories(protocolDir string) error {
	base := filepath.Dir(protocolDir)
	for _, dir := range []string{base, protocolDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("zip: create cache directory %s: %w", dir, err)
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("zip: inspect cache directory %s: %w", dir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("zip: cache directory is not a real directory: %s", dir)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("zip: protect cache directory %s: %w", dir, err)
		}
	}
	return nil
}

func publishCacheFile(tmpPath, local string) error {
	renameErr := os.Rename(tmpPath, local)
	if renameErr == nil {
		return nil
	} else if !isWindowsRenameConflict(renameErr) {
		return fmt.Errorf("zip: publish cache archive: %w", renameErr)
	}

	// Windows needs an explicit remove before replacing the target.
	info, statErr := os.Lstat(local)
	if statErr != nil {
		return fmt.Errorf("zip: publish cache archive: %w", renameErr)
	}
	if info.Mode()&os.ModeSymlink == 0 && !info.Mode().IsRegular() {
		return fmt.Errorf("zip: cache target is not a regular file: %s", local)
	}
	if removeErr := os.Remove(local); removeErr != nil {
		return fmt.Errorf("zip: replace cache archive: %w", removeErr)
	}
	if renameErr := os.Rename(tmpPath, local); renameErr != nil {
		return fmt.Errorf("zip: publish cache archive: %w", renameErr)
	}
	return nil
}

func isWindowsRenameConflict(err error) bool {
	// os.Rename reports an existence error on Windows; on other platforms a
	// failed rename should be returned unchanged rather than deleting a target.
	return os.IsExist(err)
}

func defaultSPXBaseDir() string {
	if home := os.Getenv("HOME"); home != "" && filepath.IsAbs(home) {
		return filepath.Join(home, ".spx")
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" && filepath.IsAbs(cache) {
		return filepath.Join(cache, "spx")
	}
	return filepath.Join(os.TempDir(), "spx")
}

var spxBaseDir = defaultSPXBaseDir()

func init() {
	fs.RegisterSchema("zip", Open)
	fs.RegisterSchema("hzip", OpenHttp)
	fs.RegisterSchema("hzips", OpenHttps)
}
