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

package engine

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

var engineDownloadHTTPClient = &http.Client{Timeout: 30 * time.Minute}

type downloadHTTPStatusError struct {
	url        string
	statusCode int
	status     string
}

func (err *downloadHTTPStatusError) Error() string {
	return fmt.Sprintf("download %s failed: %s", err.url, err.status)
}

func fetchURLToFile(url, dst string) error {
	return fetchURLToFileWithLimit(url, dst, runtimebundle.MaxArchiveBytes)
}

func fetchURLToFileWithLimit(url, dst string, maxBytes int64) (err error) {
	if maxBytes <= 0 {
		return fmt.Errorf("invalid download size limit %d", maxBytes)
	}
	fmt.Fprintf(os.Stdout, "Downloading %s -> %s\n", url, dst)

	resp, err := engineDownloadHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &downloadHTTPStatusError{
			url:        url,
			statusCode: resp.StatusCode,
			status:     resp.Status,
		}
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

	body := io.LimitReader(resp.Body, downloadLimitWithOverflow(maxBytes))
	var downloaded int64
	if resp.ContentLength <= 0 {
		downloaded, err = io.Copy(file, body)
	} else {
		lastReport := time.Now().Add(-time.Second)
		buffer := make([]byte, 128*1024)

		for {
			n, readErr := body.Read(buffer)
			if n > 0 {
				if _, err := file.Write(buffer[:n]); err != nil {
					return err
				}
				downloaded += int64(n)
				if downloaded > maxBytes {
					return fmt.Errorf("%w: download %s exceeds limit %d", runtimebundle.ErrArchiveLimit, url, maxBytes)
				}
				if time.Since(lastReport) >= 500*time.Millisecond || downloaded == resp.ContentLength {
					fmt.Fprintf(os.Stdout, "  %.1f%% (%s/%s)\r", float64(downloaded)*100/float64(resp.ContentLength), formatDownloadSize(downloaded), formatDownloadSize(resp.ContentLength))
					lastReport = time.Now()
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
	}
	if err != nil {
		return err
	}
	if downloaded > maxBytes {
		return fmt.Errorf("%w: download %s exceeds limit %d", runtimebundle.ErrArchiveLimit, url, maxBytes)
	}
	if resp.ContentLength > 0 {
		fmt.Fprintf(os.Stdout, "  100.0%% (%s/%s)\n", formatDownloadSize(resp.ContentLength), formatDownloadSize(resp.ContentLength))
	}

	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	return replaceDownloadedFile(tmpPath, dst)
}

func downloadLimitWithOverflow(maxBytes int64) int64 {
	if maxBytes == math.MaxInt64 {
		return maxBytes
	}
	return maxBytes + 1
}

func formatDownloadSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.1fGB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.1fMB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1fKB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%dB", size)
	}
}
