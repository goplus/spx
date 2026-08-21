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

	"github.com/goplus/spx/v3/internal/release"
)

func downloadRuntimePack(env engineDownloadEnv) error {
	versionedPack := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrt%s.pck", env.version))
	zipName := env.runtimePackAsset
	if zipName == "" {
		zipName = release.RuntimeAssetZipName
	}
	urlPrefix := env.runtimeAssetURLPrefix
	if urlPrefix == "" {
		urlPrefix = env.urlPrefix
	}
	url := urlPrefix + zipName
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := fetchEngineAsset(env, zipName, url, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "runtime-pck-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}
	destinations := map[string]string{
		"gdspxrt.pck":         versionedPack,
		"runtime.gdextension": filepath.Join(env.goBinDir, "runtime.gdextension"),
	}
	seen := make(map[string]struct{}, len(destinations))
	for _, entry := range entries {
		_, ok := destinations[entry.Name()]
		if !ok || entry.IsDir() {
			return fmt.Errorf("runtime asset bundle contains unsupported entry %q", entry.Name())
		}
		src := filepath.Join(extractDir, entry.Name())
		if info, err := os.Stat(src); err != nil {
			return err
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("runtime asset bundle entry %q is not a regular file", entry.Name())
		}
		seen[entry.Name()] = struct{}{}
	}
	for name := range destinations {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("runtime asset bundle is missing %s", name)
		}
	}
	for _, name := range []string{"runtime.gdextension", "gdspxrt.pck"} {
		if err := copyEngineAssetAtomically(filepath.Join(extractDir, name), destinations[name]); err != nil {
			return err
		}
	}
	return nil
}
