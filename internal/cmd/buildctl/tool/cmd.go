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

package tool

import (
	"errors"
	"flag"
	"fmt"
)

type toolInstallConfig struct {
	web            bool
	opt            bool
	noEmbedRuntime bool
}

func runTool(args []string) error {
	if len(args) == 0 {
		printToolUsage()
		return errUsage
	}

	switch args[0] {
	case "clean-assets":
		return runToolCleanAssets(args[1:])
	case "install":
		return runToolInstall(args[1:])
	case "setup-emsdk":
		return runToolSetupEMSDK(args[1:])
	case "setup-jdk":
		return runToolSetupJDK(args[1:])
	case "setup-ndk":
		return runToolSetupNDK(args[1:])
	case "setup-scons":
		return runToolSetupSCons(args[1:])
	case "help", "-h", "--help":
		printToolUsage()
		return nil
	default:
		printToolUsage()
		return fmt.Errorf("unknown tool command %q", args[0])
	}
}

func printToolUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl tool <clean-assets|install|setup-emsdk|setup-jdk|setup-ndk|setup-scons> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  clean-assets Remove installed SPX/Godot runtime assets from GOPATH/bin")
	fmt.Fprintln(osStderr, "  install     Build and install spx tooling with embedded runtime by default")
	fmt.Fprintln(osStderr, "  setup-emsdk Install or activate the pinned Emscripten SDK")
	fmt.Fprintln(osStderr, "  setup-jdk   Install or verify JDK 17")
	fmt.Fprintln(osStderr, "  setup-ndk   Download and install Android NDK")
	fmt.Fprintln(osStderr, "  setup-scons Install the pinned SCons version")
}

func runToolCleanAssets(args []string) error {
	if err := parseToolCleanAssetsArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return cleanInstalledAssets()
}

func parseToolCleanAssetsArgs(args []string) error {
	fs := flag.NewFlagSet("tool clean-assets", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool clean-assets")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errUsage
	}
	return nil
}

func runToolInstall(args []string) error {
	cfg, err := parseToolInstallArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return installTools(cfg, runner)
}

func parseToolInstallArgs(args []string) (toolInstallConfig, error) {
	cfg := toolInstallConfig{}

	fs := flag.NewFlagSet("tool install", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.web, "web", false, "build web tooling and install ispx web runtime")
	fs.BoolVar(&cfg.opt, "opt", false, "pass through the optional optimized web install mode")
	fs.BoolVar(&cfg.noEmbedRuntime, "no-embed-runtime", false, "build spx without embedded desktop runtime assets")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool install [--web] [--opt] [--no-embed-runtime]")
	}

	if err := fs.Parse(args); err != nil {
		return toolInstallConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolInstallConfig{}, errUsage
	}
	if cfg.opt && !cfg.web {
		return toolInstallConfig{}, errors.New("--opt requires --web")
	}
	return cfg, nil
}

func runToolSetupNDK(args []string) error {
	cfg, err := parseToolSetupNDKArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return setupAndroidNDK(cfg)
}

func runToolSetupSCons(args []string) error {
	if _, err := parseToolSetupSConsArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return setupSCons()
}

func runToolSetupJDK(args []string) error {
	if _, err := parseToolSetupJDKArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return setupJDK()
}

func runToolSetupEMSDK(args []string) error {
	if _, err := parseToolSetupEMSDKArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return setupEMSDK()
}
