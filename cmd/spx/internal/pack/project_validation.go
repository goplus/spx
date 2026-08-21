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

package pack

import (
	"fmt"
	"os"
	"path/filepath"
)

type assetProjectConfig struct {
	ExtAsset string `json:"extasset"`
}

func validateLegacyPackInputs(baseFolder string) (string, error) {
	rootInfo, err := os.Lstat(baseFolder)
	if err != nil {
		return "", fmt.Errorf("pack: inspect project directory %q: %w", baseFolder, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("pack: project directory %q must be a real directory", baseFolder)
	}

	extAssetDir, err := validateLegacyProjectConfig(baseFolder)
	if err != nil {
		return "", err
	}

	assetRoot := filepath.Join(baseFolder, packDirName)
	assetInfo, err := os.Lstat(assetRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("projectassets: PackDir %q is missing", packDirName)
		}
		return "", fmt.Errorf("projectassets: inspect PackDir %q: %w", packDirName, err)
	}
	if assetInfo.Mode()&os.ModeSymlink != 0 || !assetInfo.IsDir() {
		return "", fmt.Errorf("projectassets: PackDir %q must be a real directory", packDirName)
	}

	hasIndex := false
	for _, name := range []string{sourceIndexName, packedIndexName} {
		indexPath := filepath.Join(assetRoot, name)
		info, statErr := os.Lstat(indexPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return "", fmt.Errorf("projectassets: inspect %q: %w", indexPath, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("projectassets: asset index %q must be a regular non-symlink file", indexPath)
		}
		hasIndex = true
	}
	if !hasIndex {
		return "", fmt.Errorf("projectassets: PackDir %q contains neither %q nor %q", packDirName, sourceIndexName, packedIndexName)
	}

	return extAssetDir, nil
}

func validateLegacyProjectConfig(baseFolder string) (string, error) {
	configPath := filepath.Join(baseFolder, projectConfigName)
	info, err := os.Lstat(configPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("projectpolicy: inspect project config %q: %w", configPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("projectpolicy: project config %q must be a regular non-symlink file", configPath)
	}
	extAssetDir, err := readExtAssetDir(baseFolder)
	if err != nil {
		return "", fmt.Errorf("projectpolicy: parse project config %q: %w", configPath, err)
	}
	return extAssetDir, nil
}

func readExtAssetDir(baseFolder string) (string, error) {
	configPath := filepath.Join(baseFolder, projectConfigName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	var conf assetProjectConfig
	if err := readJSONFile(configPath, &conf); err != nil {
		return "", fmt.Errorf("parse %s: %w", configPath, err)
	}
	return conf.ExtAsset, nil
}
