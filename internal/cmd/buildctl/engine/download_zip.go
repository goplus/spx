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
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
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
	return shared.ExtractZip(srcZip, dstDir)
}
