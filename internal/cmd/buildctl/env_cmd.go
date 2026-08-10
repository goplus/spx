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
	"slices"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

type envExportShellConfig struct {
	platform string
}

func runEnv(args []string) error {
	if len(args) == 0 {
		printEnvUsage()
		return shared.ErrUsage
	}

	switch args[0] {
	case "ensure-engine-source":
		return runEnvEnsureEngineSource(args[1:])
	case "export-emsdk-shell":
		return runEnvExportEMSDKShell(args[1:])
	case "export-engine-build-shell":
		return runEnvExportEngineBuildShell(args[1:])
	case "export-jdk-shell":
		return runEnvExportJDKShell(args[1:])
	case "export-shell":
		return runEnvExportShell(args[1:])
	case "export-macos-vulkan-shell":
		return runEnvExportMacOSVulkanShell(args[1:])
	case "help", "-h", "--help":
		printEnvUsage()
		return nil
	default:
		printEnvUsage()
		return fmt.Errorf("unknown env command %q", args[0])
	}
}

func printEnvUsage() {
	fmt.Fprintln(os.Stderr, "Usage: buildctl env <ensure-engine-source|export-emsdk-shell|export-engine-build-shell|export-jdk-shell|export-shell|export-macos-vulkan-shell> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  ensure-engine-source      Clone the tagged Godot source tree when it is missing")
	fmt.Fprintln(os.Stderr, "  export-emsdk-shell        Print shell exports for the activated emsdk environment")
	fmt.Fprintln(os.Stderr, "  export-shell              Print shell export statements for shared build environment values")
	fmt.Fprintln(os.Stderr, "  export-engine-build-shell Print shell exports for engine build target/platform values")
	fmt.Fprintln(os.Stderr, "  export-jdk-shell          Print shell exports for the configured JDK environment")
	fmt.Fprintln(os.Stderr, "  export-macos-vulkan-shell Print shell exports for the detected macOS Vulkan SDK")
}

func runEnvExportEngineBuildShell(args []string) error {
	cfg, err := engine.ParseEnvExportEngineBuildShellArgs(args)
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
	plan, err := engine.ResolveEngineBuildShellPlan(repoRoot, cfg)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, plan.ShellExports())
	return nil
}

func runEnvExportJDKShell(args []string) error {
	if err := parseEnvNoArgs("env export-jdk-shell", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	exports, err := toolpkg.ResolveJDKShellExports()
	if err != nil {
		return err
	}
	for _, key := range []string{"JAVA_HOME", "PATH"} {
		if value, ok := exports[key]; ok && value != "" {
			fmt.Fprintf(os.Stdout, "export %s=%s\n", key, shared.ShellQuote(value))
		}
	}
	return nil
}

func runEnvExportEMSDKShell(args []string) error {
	if err := parseEnvNoArgs("env export-emsdk-shell", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	exports, err := toolpkg.ResolveEMSDKShellExports()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(exports))
	for key := range exports {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Fprintf(os.Stdout, "export %s=%s\n", key, shared.ShellQuote(exports[key]))
	}
	return nil
}

func parseEnvNoArgs(name string, args []string) error {
	return shared.ParseNoArgs(name, "Usage: buildctl "+name, args, os.Stderr)
}

func runEnvExportShell(args []string) error {
	cfg, err := parseEnvExportShellArgs(args)
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
	env, err := shared.ResolveBuildEnvironment(repoRoot, cfg.platform)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, env.ShellExports())
	return nil
}

func parseEnvExportShellArgs(args []string) (envExportShellConfig, error) {
	cfg := envExportShellConfig{}

	fs := flag.NewFlagSet("env export-shell", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.platform, "platform", "", "override detected platform: android, ios, web, linux, windows, or macos")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildctl env export-shell [--platform android|ios|web|linux|windows|macos]")
	}

	if err := fs.Parse(args); err != nil {
		return envExportShellConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return envExportShellConfig{}, shared.ErrUsage
	}
	if err := shared.ValidateOptionalPlatform(cfg.platform); err != nil {
		return envExportShellConfig{}, err
	}
	return cfg, nil
}

func runEnvEnsureEngineSource(args []string) error {
	if err := parseEnvNoArgs("env ensure-engine-source", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}
	return shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		return shared.RunStreamingCommand("", name, args...)
	})
}

func runEnvExportMacOSVulkanShell(args []string) error {
	if err := parseEnvNoArgs("env export-macos-vulkan-shell", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sdkRoot, err := shared.ResolveMacOSVulkanSDKRoot(homeDir, os.Getenv("VULKAN_SDK"))
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, shared.MacOSVulkanSDKShellExports(sdkRoot))
	return nil
}
