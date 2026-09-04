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

package shared

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/goplus/spx/v3/internal/base/fileutil"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

var fileDownloadHTTPClient = &http.Client{Timeout: 30 * time.Minute}

func copyDir(src, dst string) error {
	return fileutil.CopyDir(src, dst)
}

func zipDirectory(srcDir, dstZip string) (err error) {
	return fileutil.ZipDirectory(srcDir, dstZip)
}

func extractZip(srcZip, dstDir string) error {
	return extractZipWithOptions(srcZip, dstDir, ZipExtractOptions{})
}

func extractZipWithLimits(srcZip, dstDir string, limits runtimebundle.Limits) error {
	return extractZipWithOptions(srcZip, dstDir, ZipExtractOptions{Limits: limits})
}

func extractZipWithOptions(srcZip, dstDir string, options ZipExtractOptions) error {
	_, err := runtimebundle.ExtractZip(srcZip, dstDir, runtimebundle.VerifyOptions{
		Limits:                     options.Limits,
		MaterializeSymlinksAsFiles: options.MaterializeSymlinksAsFiles,
	})
	return err
}

func fetchURLToFile(url, dst string) error {
	return fetchURLToFileWithLimit(url, dst, runtimebundle.MaxArchiveBytes)
}

func fetchURLToFileWithLimit(url, dst string, maxBytes int64) (err error) {
	if maxBytes <= 0 {
		return fmt.Errorf("invalid download size limit %d", maxBytes)
	}
	resp, err := fileDownloadHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: %s", url, resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("%w: download %s declares %d bytes, limit %d", runtimebundle.ErrArchiveLimit, url, resp.ContentLength, maxBytes)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer func() {
		if file != nil {
			_ = file.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	downloaded, err := io.Copy(file, io.LimitReader(resp.Body, downloadLimitWithOverflow(maxBytes)))
	if downloaded > maxBytes {
		return fmt.Errorf("%w: download %s exceeds limit %d", runtimebundle.ErrArchiveLimit, url, maxBytes)
	}
	if resp.ContentLength > 0 && downloaded != resp.ContentLength {
		return fmt.Errorf("download %s ended after %d bytes, want %d: %w", url, downloaded, resp.ContentLength, io.ErrUnexpectedEOF)
	}
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	return os.Rename(tmpPath, dst)
}

func downloadLimitWithOverflow(maxBytes int64) int64 {
	if maxBytes == math.MaxInt64 {
		return maxBytes
	}
	return maxBytes + 1
}
