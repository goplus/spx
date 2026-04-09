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
)

type runtimeExportWebConfig struct {
	mode string
}

type runtimeBuildWasmConfig struct {
	opt bool
}

func runRuntime(args []string) error {
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
	case "export-pack":
		return runRuntimeExportPack(args[1:])
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
	fmt.Fprintln(osStderr, "  export-pack  Export runtime pck artifacts")
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

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return buildWasmRuntime(cfg, runner)
}

func parseRuntimeBuildWasmArgs(args []string) (runtimeBuildWasmConfig, error) {
	cfg := runtimeBuildWasmConfig{}

	fs := flag.NewFlagSet("runtime build-wasm", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.opt, "opt", false, "compress wasm artifacts with brotli after building")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime build-wasm [--opt]")
	}

	if err := fs.Parse(args); err != nil {
		return runtimeBuildWasmConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return runtimeBuildWasmConfig{}, errUsage
	}
	return cfg, nil
}

func runRuntimeCompressWasm(args []string) error {
	if err := parseRuntimeCompressWasmArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return compressWasmArtifacts()
}

func parseRuntimeCompressWasmArgs(args []string) error {
	fs := flag.NewFlagSet("runtime compress-wasm", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime compress-wasm")
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

func runRuntimeExportPack(args []string) error {
	if err := parseRuntimeExportPackArgs(args); err != nil {
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
	return exportPackRuntime(runner)
}

func parseRuntimeExportPackArgs(args []string) error {
	fs := flag.NewFlagSet("runtime export-pack", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime export-pack")
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

func runRuntimeExportWeb(args []string) error {
	cfg, err := parseRuntimeExportWebArgs(args)
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

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return exportWebTemplateRuntime(cfg.mode, runner)
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
	if err := validateWebMode(cfg.mode); err != nil {
		return runtimeExportWebConfig{}, err
	}
	return cfg, nil
}
