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
)

func downloadAndroidAssets(env engineDownloadEnv) error {
	url := env.urlPrefix + "android.zip"
	zipPath := filepath.Join(env.cacheDir, "android.zip")
	if err := fetchEngineAsset(env, "android.zip", url, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "android-assets-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}

	requiredFiles := []string{"android_debug.apk", "android_release.apk", "android_source.zip"}
	for _, name := range requiredFiles {
		src := filepath.Join(extractDir, name)
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("Android asset bundle is missing %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Android asset bundle entry %s is not a regular file", name)
		}
	}
	for _, name := range requiredFiles {
		src := filepath.Join(extractDir, name)
		if err := copyEngineAssetAtomically(src, filepath.Join(env.templateDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func downloadIOSAssets(env engineDownloadEnv) error {
	return fetchEngineAsset(env, "ios.zip", env.urlPrefix+"ios.zip", filepath.Join(env.templateDir, "ios.zip"))
}
