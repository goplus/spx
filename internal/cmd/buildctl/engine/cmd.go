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

func (cfg *DownloadConfig) validate() error {
	if cfg.AssetDir != "" {
		cfg.AssetDir = filepath.Clean(cfg.AssetDir)
	}
	if cfg.SameRunArtifacts && cfg.AssetDir == "" {
		return errors.New("--same-run-artifacts requires --asset-dir")
	}
	if cfg.SkipRuntimePack && !cfg.Runtime {
		return errors.New("--skip-runtime-pack requires --runtime")
	}
	if cfg.Runtime && cfg.Platform != "" {
		return errors.New("--runtime cannot be combined with --platform")
	}
	if cfg.Runtime && cfg.Mode != "" {
		return errors.New("--runtime cannot be combined with --mode")
	}
	if err := shared.ValidateOptionalPlatform(cfg.Platform); err != nil {
		return err
	}
	if cfg.Platform == "web" && cfg.Mode == "" {
		cfg.Mode = "normal"
	}
	if cfg.Mode != "" {
		if cfg.Platform != "web" {
			return errors.New("--mode requires --platform web")
		}
		if err := shared.ValidateWebMode(cfg.Mode); err != nil {
			return err
		}
	}
	return nil
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

	return DownloadEngineAssets(cfg, repoRoot)
}

func parseEngineDownloadArgs(args []string) (DownloadConfig, error) {
	cfg := DownloadConfig{}

	fs := flag.NewFlagSet("engine download", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.Runtime, "runtime", false, "download runtime assets for the current host platform")
	fs.BoolVar(&cfg.SkipRuntimePack, "skip-runtime-pack", false, "skip downloading the published runtime asset bundle")
	fs.StringVar(&cfg.Platform, "platform", "", "download templates for android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.Mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.StringVar(&cfg.AssetDir, "asset-dir", "", "read release assets from a local directory instead of GitHub")
	fs.BoolVar(&cfg.SameRunArtifacts, "same-run-artifacts", false, "allow a local current-workflow artifact directory without a final runtime manifest")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl engine download [--runtime] [--skip-runtime-pack] [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram] [--asset-dir path] [--same-run-artifacts]")
	}

	if err := fs.Parse(args); err != nil {
		return DownloadConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return DownloadConfig{}, errUsage
	}
	if err := cfg.validate(); err != nil {
		return DownloadConfig{}, err
	}
	return cfg, nil
}
