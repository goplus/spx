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

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
)

type DownloadConfig struct {
	Runtime          bool
	SkipRuntimePack  bool
	Platform         string
	Mode             string
	AssetDir         string
	SameRunArtifacts bool
}

type BuildConfig struct {
	Target     string
	Platform   string
	Mode       string
	TestModule bool
}

func ShouldRefreshPreparedAssets() bool {
	return shouldRefreshPreparedAssets()
}

func PrepareHostEditorAsset(repoRoot, assetDir string) error {
	if enabled, err := shared.SpineEnabled(); err != nil {
		return err
	} else if enabled {
		return fmt.Errorf("published editor downloads do not support SPX_SPINE=1; build the local editor with make build-editor")
	}
	env, err := prepareEngineDownloadEnv(repoRoot, "", assetDir, true)
	if err != nil {
		return err
	}
	return downloadPlatformAssets(env, "editor", true)
}
