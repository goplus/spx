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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

type engineDownloadConfig struct {
	runtime          bool
	skipRuntimePack  bool
	platform         string
	mode             string
	assetDir         string
	sameRunArtifacts bool
}

func Run(args []string) error {
	if len(args) == 0 {
		printEngineUsage()
		return errUsage
	}

	switch args[0] {
	case "download":
		return runEngineDownload(args[1:])
	case "exec":
		return runEngineExec(args[1:])
	case "help", "-h", "--help":
		printEngineUsage()
		return nil
	default:
		printEngineUsage()
		return fmt.Errorf("unknown engine command %q", args[0])
	}
}

func printEngineUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl engine <download|exec> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  download   Download runtime or platform engine assets")
	fmt.Fprintln(osStderr, "  exec       Execute a command under the engine build lock")
}

func runEngineDownload(args []string) error {
	cfg, err := parseEngineDownloadArgs(args)
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

	return downloadEngineAssets(cfg, repoRoot)
}

func parseEngineDownloadArgs(args []string) (engineDownloadConfig, error) {
	cfg := engineDownloadConfig{}

	fs := flag.NewFlagSet("engine download", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.runtime, "runtime", false, "download runtime assets for the current host platform")
	fs.BoolVar(&cfg.skipRuntimePack, "skip-runtime-pack", false, "skip downloading the published runtime asset bundle")
	fs.StringVar(&cfg.platform, "platform", "", "download templates for android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.StringVar(&cfg.assetDir, "asset-dir", "", "read release assets from a local directory instead of GitHub")
	fs.BoolVar(&cfg.sameRunArtifacts, "same-run-artifacts", false, "allow a local current-workflow artifact directory without a final runtime manifest")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl engine download [--runtime] [--skip-runtime-pack] [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram] [--asset-dir path] [--same-run-artifacts]")
	}

	if err := fs.Parse(args); err != nil {
		return engineDownloadConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return engineDownloadConfig{}, errUsage
	}
	if err := cfg.validate(); err != nil {
		return engineDownloadConfig{}, err
	}
	return cfg, nil
}

func (cfg *engineDownloadConfig) validate() error {
	if cfg.assetDir != "" {
		cfg.assetDir = filepath.Clean(cfg.assetDir)
	}
	if cfg.sameRunArtifacts && cfg.assetDir == "" {
		return errors.New("--same-run-artifacts requires --asset-dir")
	}
	if cfg.skipRuntimePack && !cfg.runtime {
		return errors.New("--skip-runtime-pack requires --runtime")
	}
	if cfg.runtime && cfg.platform != "" {
		return errors.New("--runtime cannot be combined with --platform")
	}
	if cfg.runtime && cfg.mode != "" {
		return errors.New("--runtime cannot be combined with --mode")
	}
	if err := shared.ValidateOptionalPlatform(cfg.platform); err != nil {
		return err
	}
	if cfg.platform == "web" && cfg.mode == "" {
		cfg.mode = "normal"
	}
	if cfg.mode != "" {
		if cfg.platform != "web" {
			return errors.New("--mode requires --platform web")
		}
		if err := shared.ValidateWebMode(cfg.mode); err != nil {
			return err
		}
	}
	return nil
}
