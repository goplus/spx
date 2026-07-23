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
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/base/fileutil"
	"github.com/goplus/spx/v3/internal/release"
)

var fileDownloadHTTPClient = &http.Client{Timeout: 30 * time.Minute}

func ensureGoPath() (string, error) {
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		return goPath, nil
	}

	output, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "", err
	}
	goPath := strings.TrimSpace(string(output))
	if goPath == "" {
		return "", fmt.Errorf("missing GOPATH")
	}
	return goPath, nil
}

func defaultRuntimeVersion() (string, error) {
	version := release.DefaultReleaseMeta().Runtime.Version
	if version == "" {
		return "", fmt.Errorf("release: Runtime.Version is empty")
	}
	return version, nil
}

func copyFile(src, dst string) (err error) {
	return fileutil.CopyFile(src, dst)
}

func copyDir(src, dst string) error {
	return fileutil.CopyDir(src, dst)
}

func writeNamedZip(dst string, namedFiles map[string]string) (err error) {
	return fileutil.WriteNamedZip(dst, namedFiles)
}

func zipDirectory(srcDir, dstZip string) (err error) {
	return fileutil.ZipDirectory(srcDir, dstZip)
}

func extractZip(srcZip, dstDir string) error {
	reader, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath, err := resolveZipExtractPath(dstDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func resolveZipExtractPath(dstDir, name string) (string, error) {
	cleanBase := filepath.Clean(dstDir)
	targetPath := filepath.Clean(filepath.Join(cleanBase, name))
	basePrefix := cleanBase
	if !strings.HasSuffix(basePrefix, string(os.PathSeparator)) {
		basePrefix += string(os.PathSeparator)
	}
	targetPrefix := targetPath
	if !strings.HasSuffix(targetPrefix, string(os.PathSeparator)) {
		targetPrefix += string(os.PathSeparator)
	}
	if targetPath != cleanBase && !strings.HasPrefix(targetPrefix, basePrefix) {
		return "", fmt.Errorf("illegal path in archive entry: %s", name)
	}
	return targetPath, nil
}

func extractZipFile(file *zip.File, dst string) (err error) {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := output.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(output, reader)
	return err
}

func fetchURLToFile(url, dst string) (err error) {
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

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	return os.Rename(tmpPath, dst)
}
