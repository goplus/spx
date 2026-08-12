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

package main

import (
	"fmt"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/runtimecmd"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type setupConfig struct {
	target           string
	mode             string
	assetDir         string
	publishedRuntime bool
}

var (
	downloadEngineAssets   = engine.DownloadEngineAssets
	prepareHostEditorAsset = engine.PrepareHostEditorAsset
)

func setupAssets(cfg setupConfig, runner shared.ScriptRunner) error {
	switch cfg.target {
	case "host":
		if err := prepareRuntimeAssets(runner, cfg.assetDir, cfg.publishedRuntime); err != nil {
			return err
		}
		return runner.RunScript(filepath.Join("cmd", "spx", "install.sh"))
	case "web":
		if err := prepareHostEditorAsset(runner.RepoRootDir(), cfg.assetDir); err != nil {
			return err
		}
		return prepareWebAssets(cfg.mode, runner, false, cfg.assetDir)
	case "full":
		if err := prepareRuntimeAssets(runner, cfg.assetDir, cfg.publishedRuntime); err != nil {
			return err
		}
		return prepareWebAssets(cfg.mode, runner, true, cfg.assetDir)
	default:
		return fmt.Errorf("unsupported setup target: %s", cfg.target)
	}
}

func prepareRuntimeAssets(runner shared.ScriptRunner, assetDir string, publishedRuntime bool) error {
	// A same-run release directory contains the canonical runtime pack built by
	// release_runtime_assets.yml. Install that exact pack so every standalone
	// product consumes the bytes that will be published in runtime-v*.
	useLockedRuntimePack := assetDir != "" || publishedRuntime
	if err := downloadEngineAssets(engine.DownloadConfig{
		Runtime:          true,
		SkipRuntimePack:  !useLockedRuntimePack,
		AssetDir:         assetDir,
		SameRunArtifacts: assetDir != "",
	}, runner.RepoRootDir()); err != nil {
		return err
	}
	if useLockedRuntimePack {
		return nil
	}
	return ensureRuntimePack(runner)
}

func ensureRuntimePack(runner shared.ScriptRunner) error {
	goPath, err := shared.EnsureGoPath()
	if err != nil {
		return err
	}
	version, err := shared.DefaultRuntimeVersion()
	if err != nil {
		return err
	}
	packPath := filepath.Join(goPath, "bin", fmt.Sprintf("gdspxrt%s.pck", version))
	if !engine.ShouldRefreshPreparedAssets() && shared.FileExists(packPath) {
		return nil
	}
	return runtimecmd.ExportPackRuntime(runner)
}

func prepareWebAssets(webMode string, runner shared.ScriptRunner, embedRuntime bool, assetDir string) error {
	if err := downloadEngineAssets(engine.DownloadConfig{Platform: "web", Mode: webMode, AssetDir: assetDir, SameRunArtifacts: assetDir != ""}, runner.RepoRootDir()); err != nil {
		return err
	}
	args := []string{"--web"}
	if !embedRuntime {
		args = append(args, "--no-embed-runtime")
	}
	if err := runner.RunScript(filepath.Join("cmd", "spx", "install.sh"), args...); err != nil {
		return err
	}
	return runtimecmd.ExportWebTemplateRuntime(webMode, runner)
}
