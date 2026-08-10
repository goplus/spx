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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/workflow"
	"github.com/goplus/spx/v3/internal/release"
)

func runSetup(args []string) error {
	cfg, err := parseSetupCommandArgs(args)
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
	return setupAssets(cfg, runner)
}

func parseSetupCommandArgs(args []string) (setupConfig, error) {
	if len(args) == 0 {
		printSetupCommandUsage()
		return setupConfig{}, shared.ErrUsage
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSetupCommandUsage()
		return setupConfig{}, flag.ErrHelp
	}

	cfg := setupConfig{target: args[0]}
	fs := flag.NewFlagSet("setup "+cfg.target, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.mode, "mode", "", "Web mode: normal, worker, minigame, or miniprogram")
	fs.StringVar(&cfg.assetDir, "asset-dir", "", "Read engine assets from a local workflow artifact directory")
	fs.BoolVar(&cfg.publishedRuntime, "published-runtime", false, "Install the locked runtime pack from its published release")
	fs.Usage = printSetupCommandUsage
	if err := fs.Parse(args[1:]); err != nil {
		return setupConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return setupConfig{}, shared.ErrUsage
	}

	switch cfg.target {
	case "host":
		if cfg.mode != "" {
			return setupConfig{}, errors.New("--mode is only supported by setup web and setup full")
		}
	case "web", "full":
		if cfg.mode == "" {
			cfg.mode = "normal"
		}
		if err := shared.ValidateWebMode(cfg.mode); err != nil {
			return setupConfig{}, err
		}
	default:
		fs.Usage()
		return setupConfig{}, fmt.Errorf("unsupported setup target: %s", cfg.target)
	}
	if cfg.publishedRuntime && cfg.assetDir != "" {
		return setupConfig{}, errors.New("--published-runtime cannot be combined with --asset-dir")
	}
	if cfg.target == "web" && cfg.publishedRuntime {
		return setupConfig{}, errors.New("--published-runtime is only supported by setup host and setup full")
	}
	return cfg, nil
}

func printSetupCommandUsage() {
	fmt.Fprintln(os.Stderr, "Usage: buildctl setup <host|web|full> [--mode normal|worker|minigame|miniprogram] [--asset-dir path] [--published-runtime]")
}

func runBuild(args []string) error {
	cfg, err := parseBuildCommandArgs(args)
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
	return workflow.Build(cfg, runner)
}

func parseBuildCommandArgs(args []string) (workflow.BuildConfig, error) {
	if len(args) == 0 {
		printBuildCommandUsage()
		return workflow.BuildConfig{}, shared.ErrUsage
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printBuildCommandUsage()
		return workflow.BuildConfig{}, flag.ErrHelp
	}

	cfg := workflow.BuildConfig{Target: args[0]}
	fs := flag.NewFlagSet("build "+cfg.Target, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.Mode, "mode", "", "Web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = printBuildCommandUsage
	if err := fs.Parse(args[1:]); err != nil {
		return workflow.BuildConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflow.BuildConfig{}, shared.ErrUsage
	}

	switch cfg.Target {
	case "dev", "web":
		if cfg.Mode == "" {
			cfg.Mode = "normal"
		}
		if err := shared.ValidateWebMode(cfg.Mode); err != nil {
			return workflow.BuildConfig{}, err
		}
	case "editor", "desktop", "android", "ios":
		if cfg.Mode != "" {
			return workflow.BuildConfig{}, errors.New("--mode is only supported by build dev and build web")
		}
	default:
		fs.Usage()
		return workflow.BuildConfig{}, fmt.Errorf("unsupported build target: %s", cfg.Target)
	}
	return cfg, nil
}

func printBuildCommandUsage() {
	fmt.Fprintln(os.Stderr, "Usage: buildctl build <dev|editor|desktop|web|android|ios> [--mode normal|worker|minigame|miniprogram]")
}

func runDoctor(args []string) error {
	if err := shared.ParseNoArgs("doctor", "Usage: buildctl doctor", args, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}
	return inspectBuildConfiguration(repoRoot, os.Stdout)
}

func inspectBuildConfiguration(repoRoot string, output io.Writer) error {
	lock := release.DefaultRuntimeLock()
	lockPath := filepath.Join(repoRoot, "internal", "release", "runtime.lock.json")
	lockData, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("read runtime lock %s: %w", lockPath, err)
	}
	worktreeLock, err := release.ParseRuntimeLock(lockData)
	if err != nil {
		return err
	}
	embeddedDigest, err := lock.SHA256()
	if err != nil {
		return err
	}
	worktreeDigest, err := worktreeLock.SHA256()
	if err != nil {
		return err
	}
	if embeddedDigest != worktreeDigest {
		return fmt.Errorf("buildctl embeds runtime lock %s, but %s has %s; rebuild buildctl", embeddedDigest, lockPath, worktreeDigest)
	}

	env, err := shared.ResolveBuildEnvironment(repoRoot, "")
	if err != nil {
		return err
	}

	for _, name := range []string{"SCsub", "config.py", shared.SConsProfileFilename} {
		path := filepath.Join(env.SPXModuleSrc, name)
		if !shared.FileExists(path) {
			return fmt.Errorf("SPX module is incomplete: missing %s", path)
		}
	}
	if _, err := shared.LoadSConsProfile(env.SPXModuleSrc); err != nil {
		return err
	}

	engineStatus := "missing; the next engine build will prepare it"
	if shared.DirExists(env.EngineDir) {
		headOutput, err := shared.RunCommandOutput("git", "-C", env.EngineDir, "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("inspect Godot source %s: %w", env.EngineDir, err)
		}
		head := strings.TrimSpace(string(headOutput))
		if head != env.GodotCommit {
			return fmt.Errorf("Godot source %s is at %s, but runtime.lock.json requires %s", env.EngineDir, head, env.GodotCommit)
		}
		engineStatus = "pinned commit ready"
	}

	fmt.Fprintf(output, "Repository: %s\n", env.RepoRoot)
	fmt.Fprintf(output, "Host: %s/%s\n", env.Platform, env.Arch)
	fmt.Fprintf(output, "Runtime: %s (Godot %s)\n", env.Version, env.EngineVersion)
	fmt.Fprintf(output, "Runtime lock: %s (%s)\n", lockPath, embeddedDigest)
	fmt.Fprintf(output, "Godot source: %s (%s)\n", env.EngineDir, engineStatus)
	fmt.Fprintf(output, "SPX module: %s\n", env.SPXModuleSrc)
	fmt.Fprintf(output, "SCons profile: %s (valid)\n", filepath.Join(env.SPXModuleSrc, shared.SConsProfileFilename))
	fmt.Fprintf(output, "Toolchain: go=%s xgo=%s scons=%s emsdk=%s android_ndk=%s jdk=%s\n",
		lock.Toolchain.Go,
		lock.Toolchain.XGo,
		lock.Toolchain.SCons,
		lock.Toolchain.EMSDK,
		lock.Toolchain.AndroidNDK,
		lock.Toolchain.JDK,
	)
	fmt.Fprintln(output, "Status: OK")
	return nil
}
