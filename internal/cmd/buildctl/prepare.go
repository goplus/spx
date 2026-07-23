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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/runtimecmd"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type prepareConfig struct {
	setupMode string
	webMode   string
}

func runPrepare(args []string) error {
	cfg, err := parsePrepareArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return prepareAssets(cfg, runner)
}

func parsePrepareArgs(args []string) (prepareConfig, error) {
	cfg := prepareConfig{
		setupMode: "runtime",
		webMode:   "normal",
	}

	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.setupMode, "setup-mode", cfg.setupMode, "setup mode: runtime, web, or full")
	fs.StringVar(&cfg.webMode, "web-mode", cfg.webMode, "web mode: normal, worker, minigame, or miniprogram")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildctl prepare [--setup-mode none|runtime|web|full] [--web-mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return prepareConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return prepareConfig{}, shared.ErrUsage
	}

	if err := cfg.validate(); err != nil {
		return prepareConfig{}, err
	}

	return cfg, nil
}

func (cfg prepareConfig) validate() error {
	if err := shared.ValidateSetupMode(cfg.setupMode); err != nil {
		return err
	}
	if err := shared.ValidateWebMode(cfg.webMode); err != nil {
		return err
	}
	return nil
}

func prepareAssets(cfg prepareConfig, runner shared.ScriptRunner) error {
	switch cfg.setupMode {
	case "none":
		return nil
	case "runtime":
		if err := prepareRuntimeAssets(runner); err != nil {
			return err
		}
		return runner.RunScript(filepath.Join("cmd", "spx", "install.sh"))
	case "web":
		if err := prepareHostEditorAsset(runner.RepoRootDir()); err != nil {
			return err
		}
		return prepareWebAssets(cfg.webMode, runner, false)
	case "full":
		if err := prepareRuntimeAssets(runner); err != nil {
			return err
		}
		return prepareWebAssets(cfg.webMode, runner, true)
	default:
		return fmt.Errorf("unsupported setup-mode: %s", cfg.setupMode)
	}
}

func prepareRuntimeAssets(runner shared.ScriptRunner) error {
	if err := engine.DownloadEngineAssets(engine.DownloadConfig{Runtime: true, SkipRuntimePack: true}, runner.RepoRootDir()); err != nil {
		return err
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

func prepareHostEditorAsset(repoRoot string) error {
	return engine.PrepareHostEditorAsset(repoRoot)
}

func prepareWebAssets(webMode string, runner shared.ScriptRunner, embedRuntime bool) error {
	if err := engine.DownloadEngineAssets(engine.DownloadConfig{Platform: "web", Mode: webMode}, runner.RepoRootDir()); err != nil {
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
