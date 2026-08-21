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
)

func downloadEngineAssets(cfg engineDownloadConfig, repoRoot string) error {
	env, err := engineDownloadResolveEnv(repoRoot, cfg.platform)
	if err != nil {
		return err
	}
	if cfg.assetDir != "" {
		if err := setLocalAssetDir(&env, repoRoot, cfg.assetDir, cfg.sameRunArtifacts); err != nil {
			return err
		}
	}
	if env.verifyManifest {
		if err := loadEngineAssetManifest(&env); err != nil {
			return err
		}
	}

	if cfg.runtime {
		if err := downloadHostRuntimeAssets(env); err != nil {
			return err
		}
		if cfg.skipRuntimePack {
			return nil
		}
		if err := downloadRuntimePack(env); err != nil {
			return fmt.Errorf("download runtime asset bundle: %w", err)
		}
		return nil
	}

	return downloadPlatformAssets(env, cfg.mode, false)
}

func downloadHostRuntimeAssets(env engineDownloadEnv) error {
	if err := downloadPlatformAssets(env, "", false); err != nil {
		return err
	}
	return downloadPlatformAssets(env, "editor", true)
}

func downloadPlatformAssets(env engineDownloadEnv, mode string, editor bool) error {
	switch env.platform {
	case "android":
		return downloadAndroidAssets(env)
	case "ios":
		return downloadIOSAssets(env)
	case "web":
		if mode == "" {
			mode = "normal"
		}
		return downloadWebAssets(env, mode)
	case "linux":
		return downloadLinuxAssets(env, editor)
	case "windows", "macos":
		return downloadDesktopAssets(env, editor)
	default:
		return fmt.Errorf("unsupported platform for engine download: %s", env.platform)
	}
}
