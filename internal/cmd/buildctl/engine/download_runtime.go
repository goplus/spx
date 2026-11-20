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
	"slices"

	"github.com/goplus/spx/v4/internal/release"
)

func downloadRuntimePack(env engineDownloadEnv) error {
	return withExtractedEngineArchive(env, release.RuntimeAssetZipName, func(dir string) error {
		return installRuntimePack(env, dir)
	})
}

func installRuntimePack(env engineDownloadEnv, extractDir string) error {
	installs := []binaryInstall{
		{"runtime.gdextension", filepath.Join(env.goBinDir, "runtime.gdextension")},
		{"gdspxrt.pck", filepath.Join(env.goBinDir, "gdspxrt"+env.version+".pck")},
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		expected := slices.ContainsFunc(installs, func(target binaryInstall) bool {
			return target.assetName == entry.Name()
		})
		if !expected || !entry.Type().IsRegular() {
			return fmt.Errorf("runtime asset bundle contains unsupported entry %q", entry.Name())
		}
	}
	return installEngineBinaries(extractDir, "runtime asset bundle", installs)
}
