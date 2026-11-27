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

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
)

const maxEngineArchiveEntrySize int64 = 2 << 30

type binaryInstall struct {
	assetName string
	dst       string
}

func downloadBinariesFromZip(env engineDownloadEnv, zipName string, installs ...binaryInstall) error {
	return withExtractedEngineArchive(env, zipName, func(extractDir string) error {
		return installEngineBinaries(extractDir, "engine archive "+zipName, installs)
	})
}

func installEngineBinaries(extractDir, description string, installs []binaryInstall) error {
	for _, install := range installs {
		info, err := os.Stat(filepath.Join(extractDir, install.assetName))
		if err != nil {
			return fmt.Errorf("%s is missing %s: %w", description, install.assetName, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("engine archive entry %s is not a regular file", install.assetName)
		}
		// A blocked later destination must not leave an earlier binary updated.
		if dst, err := os.Lstat(install.dst); err == nil {
			if dst.IsDir() {
				return fmt.Errorf("engine asset destination %q is a directory", install.dst)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, install := range installs {
		if err := copyEngineAssetAtomically(filepath.Join(extractDir, install.assetName), install.dst, nil); err != nil {
			return err
		}
	}
	return nil
}

// Each download owns both the staged archive and its extracted contents.
func withExtractedEngineArchive(env engineDownloadEnv, zipName string, install func(string) error) error {
	if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(env.cacheDir, "engine-zip-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	archive := filepath.Join(work, "download")
	if err := fetchEngineAsset(env, zipName, archive); err != nil {
		return err
	}
	contents := filepath.Join(work, "contents")
	if err := extractZip(archive, contents); err != nil {
		return err
	}
	return install(contents)
}

func extractZip(srcZip, dstDir string) error {
	return shared.ExtractZipWithOptions(srcZip, dstDir, shared.ZipExtractOptions{
		Limits: shared.ZipLimits{
			MaxEntrySize: maxEngineArchiveEntrySize,
		},
	})
}
