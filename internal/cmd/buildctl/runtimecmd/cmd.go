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

package runtimecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

type runtimeExportWebConfig struct {
	mode string
}

type BuildWasmConfig struct {
	Opt bool
}

func runOtherRuntimeCommand(args []string) error {
	if len(args) == 0 {
		printRuntimeUsage()
		return errUsage
	}

	switch args[0] {
	case "build-wasm":
		return runRuntimeBuildWasm(args[1:])
	case "compress-wasm":
		return runRuntimeCompressWasm(args[1:])
	case "export-web-template":
		return runRuntimeExportWebTemplate(args[1:])
	case "export-web":
		return runRuntimeExportWeb(args[1:])
	case "help", "-h", "--help":
		printRuntimeUsage()
		return nil
	default:
		printRuntimeUsage()
		return fmt.Errorf("unknown runtime command %q", args[0])
	}
}

func printRuntimeUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl runtime <build-wasm|compress-wasm|export-pack|export-web|export-web-template> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  build-wasm   Build ispx.wasm assets")
	fmt.Fprintln(osStderr, "  compress-wasm Compress prebuilt web wasm assets with brotli")
	fmt.Fprintln(osStderr, "  export-pack  Export runtime asset bundle")
	fmt.Fprintln(osStderr, "  export-web   Export web runtime bundles")
	fmt.Fprintln(osStderr, "  export-web-template Export web runtime template bundles")
}

func runRuntimeBuildWasm(args []string) error {
	cfg, err := parseRuntimeBuildWasmArgs(args)
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
	return BuildWasmRuntime(cfg, runner)
}

func parseRuntimeBuildWasmArgs(args []string) (BuildWasmConfig, error) {
	cfg := BuildWasmConfig{}

	fs := flag.NewFlagSet("runtime build-wasm", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.Opt, "opt", false, "compress wasm artifacts with brotli after building")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime build-wasm [--opt]")
	}

	if err := fs.Parse(args); err != nil {
		return BuildWasmConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return BuildWasmConfig{}, errUsage
	}
	return cfg, nil
}

func runRuntimeCompressWasm(args []string) error {
	if err := shared.ParseNoArgs("runtime compress-wasm", "Usage: buildctl runtime compress-wasm", args, osStderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return compressWasmArtifacts()
}

func runRuntimeExportWeb(args []string) error {
	cfg, err := parseRuntimeExportWebArgs(args)
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
	return exportWebRuntime(cfg, runner)
}

func runRuntimeExportWebTemplate(args []string) error {
	cfg, err := parseRuntimeExportWebArgs(args)
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
	return ExportWebTemplateRuntime(cfg.mode, runner)
}

func parseRuntimeExportWebArgs(args []string) (runtimeExportWebConfig, error) {
	cfg := runtimeExportWebConfig{mode: "normal"}

	fs := flag.NewFlagSet("runtime export-web", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.mode, "mode", cfg.mode, "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime export-web [--mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return runtimeExportWebConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return runtimeExportWebConfig{}, errUsage
	}
	if err := shared.ValidateWebMode(cfg.mode); err != nil {
		return runtimeExportWebConfig{}, err
	}
	return cfg, nil
}

func BuildWasmRuntime(cfg BuildWasmConfig, runner shared.ScriptRunner) error {
	if err := toolpkg.InstallTools(toolpkg.InstallConfig{Web: true, NoEmbedRuntime: true}, runner); err != nil {
		return err
	}
	if !cfg.Opt {
		return nil
	}
	return compressWasmArtifacts()
}
