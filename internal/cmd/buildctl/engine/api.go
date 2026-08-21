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

type DownloadConfig struct {
	Runtime          bool
	SkipRuntimePack  bool
	Platform         string
	Mode             string
	AssetDir         string
	SameRunArtifacts bool
}

type BuildConfig struct {
	Target   string
	Platform string
	Mode     string
}

func DownloadEngineAssets(cfg DownloadConfig, repoRoot string) error {
	return downloadEngineAssets(engineDownloadConfig{runtime: cfg.Runtime, skipRuntimePack: cfg.SkipRuntimePack, platform: cfg.Platform, mode: cfg.Mode, assetDir: cfg.AssetDir, sameRunArtifacts: cfg.SameRunArtifacts}, repoRoot)
}

func ShouldRefreshPreparedAssets() bool {
	return shouldRefreshPreparedAssets()
}

func PrepareHostEditorAsset(repoRoot, assetDir string) error {
	env, err := engineDownloadResolveEnv(repoRoot, "")
	if err != nil {
		return err
	}
	if assetDir != "" {
		if err := setLocalAssetDir(&env, repoRoot, assetDir, true); err != nil {
			return err
		}
	}
	if env.verifyManifest {
		if err := loadEngineAssetManifest(&env); err != nil {
			return err
		}
	}
	return downloadPlatformAssets(env, "editor", true)
}
