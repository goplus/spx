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
	"strings"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

type engineDownloadEnv struct {
	version              string
	platform             string
	arch                 string
	goBinDir             string
	templateDir          string
	cacheDir             string
	urlPrefix            string
	assetDir             string
	allowMissingManifest bool
	manifest             *release.RuntimeManifest
}

// ShouldRefreshPreparedAssets defaults to refreshing on GitHub Actions.
func ShouldRefreshPreparedAssets() bool {
	value, set := os.LookupEnv("SPX_PREPARE_FORCE_REFRESH")
	if !set {
		value = os.Getenv("GITHUB_ACTIONS")
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func prepareEngineDownloadEnv(repoRoot, platform, assetDir string, allowMissingManifest bool) (engineDownloadEnv, error) {
	buildEnv, err := shared.ResolveBuildEnvironment(repoRoot, platform)
	if err != nil {
		return engineDownloadEnv{}, err
	}
	lock := release.DefaultRuntimeLock()
	if buildEnv.Version != lock.RuntimeVersion {
		return engineDownloadEnv{}, fmt.Errorf("resolved runtime version %q does not match runtime lock %q", buildEnv.Version, lock.RuntimeVersion)
	}

	env := engineDownloadEnv{
		version:     buildEnv.Version,
		platform:    buildEnv.Platform,
		arch:        buildEnv.Arch,
		goBinDir:    filepath.Join(buildEnv.GoPath, "bin"),
		templateDir: buildEnv.TemplateDir,
		cacheDir:    filepath.Join(repoRoot, "internal", "cmd", "buildctl", "bin"),
		urlPrefix:   lock.RuntimeAssetDownloadURL(""),
	}

	for _, dir := range []string{env.goBinDir, env.templateDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return engineDownloadEnv{}, err
		}
	}
	if assetDir != "" {
		if err := setLocalAssetDir(&env, repoRoot, assetDir, allowMissingManifest); err != nil {
			return engineDownloadEnv{}, err
		}
	}
	if err := loadEngineAssetManifest(&env); err != nil {
		return engineDownloadEnv{}, err
	}
	return env, nil
}

func shouldDownloadPreparedAsset(path string) bool {
	return ShouldRefreshPreparedAssets() || !shared.FileExists(path)
}
