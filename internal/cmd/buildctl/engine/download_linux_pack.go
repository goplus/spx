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
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

const linuxRuntimePackArch = "x86_64"

type engineDownloadEnv struct {
	repoRoot              string
	version               string
	platform              string
	arch                  string
	goBinDir              string
	templateDir           string
	cacheDir              string
	urlPrefix             string
	runtimeAssetURLPrefix string
	runtimePackAsset      string
	assetDir              string
	verifyManifest        bool
	allowMissingManifest  bool
	manifest              *release.RuntimeManifest
}

var engineDownloadFetcher = fetchURLToFile
var engineDownloadResolveEnv = resolveEngineDownloadEnv

func resolveEngineDownloadEnv(repoRoot, platform string) (engineDownloadEnv, error) {
	buildEnv, err := shared.ResolveBuildEnvironment(repoRoot, platform)
	if err != nil {
		return engineDownloadEnv{}, err
	}
	lock := release.DefaultRuntimeLock()
	if buildEnv.Version != lock.RuntimeVersion {
		return engineDownloadEnv{}, fmt.Errorf("resolved runtime version %q does not match runtime lock %q", buildEnv.Version, lock.RuntimeVersion)
	}

	env := engineDownloadEnv{
		repoRoot:              buildEnv.RepoRoot,
		version:               buildEnv.Version,
		platform:              buildEnv.Platform,
		arch:                  buildEnv.Arch,
		goBinDir:              filepath.Join(buildEnv.GoPath, "bin"),
		templateDir:           buildEnv.TemplateDir,
		cacheDir:              filepath.Join(repoRoot, "internal", "cmd", "buildctl", "bin"),
		urlPrefix:             lock.RuntimeAssetDownloadURL(""),
		runtimeAssetURLPrefix: lock.RuntimeAssetDownloadURL(""),
		runtimePackAsset:      release.RuntimeAssetZipName,
		verifyManifest:        true,
	}

	for _, dir := range []string{env.goBinDir, env.templateDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return engineDownloadEnv{}, err
		}
	}
	return env, nil
}

func downloadLinuxAssets(env engineDownloadEnv, editor bool) error {
	if env.platform != "linux" {
		return fmt.Errorf("Linux runtime assets require platform linux, got %s", env.platform)
	}

	zipName := fmt.Sprintf("linux-%s.zip", env.arch)
	if editor {
		finalBinary := filepath.Join(env.goBinDir, "gdspx"+env.version)
		if !shouldDownloadPreparedAsset(finalBinary) {
			return nil
		}
		return downloadBinaryFromZip(env, "editor-"+zipName,
			fmt.Sprintf("godot.linuxbsd.editor.%s", env.arch),
			finalBinary)
	}

	templateBinary := filepath.Join(env.goBinDir, "gdspxrt"+env.version)
	debugBinary := filepath.Join(env.goBinDir, "gdspxrtdbg"+env.version)
	if shouldDownloadPreparedAsset(templateBinary) || shouldDownloadPreparedAsset(debugBinary) {
		if err := downloadBinariesFromZip(env, zipName, []binaryInstall{
			{fmt.Sprintf("godot.linuxbsd.template_release.%s", env.arch), templateBinary},
			{fmt.Sprintf("godot.linuxbsd.template_debug.%s", env.arch), debugBinary},
		}); err != nil {
			return err
		}
	}
	if err := linkOrCopyFile(debugBinary, filepath.Join(env.templateDir, "linux_debug."+env.arch)); err != nil {
		return err
	}
	return linkOrCopyFile(templateBinary, filepath.Join(env.templateDir, "linux_release."+env.arch))
}

// PrepareLinuxRuntimePackAssets installs only the Linux/amd64 assets used by
// the release runtime pack workflow.
func PrepareLinuxRuntimePackAssets(repoRoot, assetDir string) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("Linux runtime pack preparation requires linux/amd64, got %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if strings.TrimSpace(assetDir) == "" {
		return fmt.Errorf("Linux runtime pack preparation requires an engine asset directory")
	}

	env, err := engineDownloadResolveEnv(repoRoot, "linux")
	if err != nil {
		return err
	}
	env.arch = linuxRuntimePackArch
	if err := setLocalAssetDir(&env, repoRoot, assetDir, true); err != nil {
		return err
	}
	if env.verifyManifest {
		if err := loadEngineAssetManifest(&env); err != nil {
			return err
		}
	}
	if err := downloadLinuxAssets(env, false); err != nil {
		return err
	}
	return downloadLinuxAssets(env, true)
}

func shouldDownloadPreparedAsset(path string) bool {
	return shouldRefreshPreparedAssets() || !shared.FileExists(path)
}

func shouldRefreshPreparedAssets() bool {
	if value, ok := envFlagValue("SPX_PREPARE_FORCE_REFRESH"); ok {
		return flagValueEnabled(value)
	}
	return envFlagEnabled("GITHUB_ACTIONS")
}

func envFlagEnabled(name string) bool {
	value, _ := envFlagValue(name)
	return flagValueEnabled(value)
}

func envFlagValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func flagValueEnabled(value string) bool {
	switch strings.ToLower(value) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
