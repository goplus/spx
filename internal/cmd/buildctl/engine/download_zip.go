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
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type binaryInstall struct {
	assetName string
	dst       string
}

func downloadBinaryFromZip(env engineDownloadEnv, zipName, assetName, dst string) error {
	return downloadBinariesFromZip(env, zipName, []binaryInstall{{assetName, dst}})
}

func downloadBinariesFromZip(env engineDownloadEnv, zipName string, installs []binaryInstall) error {
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := fetchEngineAsset(env, zipName, env.urlPrefix+zipName, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "engine-zip-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}
	for _, install := range installs {
		info, err := os.Stat(filepath.Join(extractDir, install.assetName))
		if err != nil {
			return fmt.Errorf("engine archive %s is missing %s: %w", zipName, install.assetName, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("engine archive entry %s is not a regular file", install.assetName)
		}
	}
	for _, install := range installs {
		if err := copyEngineAssetAtomically(filepath.Join(extractDir, install.assetName), install.dst); err != nil {
			return err
		}
	}
	return nil
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
	if !strings.HasSuffix(basePrefix, string(filepath.Separator)) {
		basePrefix += string(filepath.Separator)
	}
	targetPrefix := targetPath
	if !strings.HasSuffix(targetPrefix, string(filepath.Separator)) {
		targetPrefix += string(filepath.Separator)
	}
	if targetPath != cleanBase && !strings.HasPrefix(targetPrefix, basePrefix) {
		return "", fmt.Errorf("illegal path in archive entry: %s", name)
	}
	return targetPath, nil
}

func extractZipFile(file *zip.File, dst string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, reader)
	return err
}
