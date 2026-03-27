package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
)

type envExportShellConfig struct {
	platform string
}

func runEnv(args []string) error {
	if len(args) == 0 {
		printEnvUsage()
		return errUsage
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
	fmt.Fprintln(osStderr, "Usage: buildctl env <ensure-engine-source|export-emsdk-shell|export-engine-build-shell|export-jdk-shell|export-shell|export-macos-vulkan-shell> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  ensure-engine-source      Clone the tagged Godot source tree when it is missing")
	fmt.Fprintln(osStderr, "  export-emsdk-shell        Print shell exports for the activated emsdk environment")
	fmt.Fprintln(osStderr, "  export-shell              Print shell export statements for shared build environment values")
	fmt.Fprintln(osStderr, "  export-engine-build-shell Print shell exports for engine build target/platform values")
	fmt.Fprintln(osStderr, "  export-jdk-shell          Print shell exports for the configured JDK environment")
	fmt.Fprintln(osStderr, "  export-macos-vulkan-shell Print shell exports for the detected macOS Vulkan SDK")
}

func runEnvExportEngineBuildShell(args []string) error {
	cfg, err := parseEnvExportEngineBuildShellArgs(args)
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
	plan, err := resolveEngineBuildShellPlan(repoRoot, cfg)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, plan.shellExports())
	return nil
}

func runEnvExportJDKShell(args []string) error {
	if err := parseEnvExportSimpleArgs("env export-jdk-shell", "Usage: buildctl env export-jdk-shell", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	exports, err := resolveJDKShellExports()
	if err != nil {
		return err
	}
	for _, key := range []string{"JAVA_HOME", "PATH"} {
		if value, ok := exports[key]; ok && value != "" {
			fmt.Fprintf(os.Stdout, "export %s=%s\n", key, shellQuote(value))
		}
	}
	return nil
}

func runEnvExportEMSDKShell(args []string) error {
	if err := parseEnvExportSimpleArgs("env export-emsdk-shell", "Usage: buildctl env export-emsdk-shell", args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	exports, err := resolveEMSDKShellExports()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(exports))
	for key := range exports {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Fprintf(os.Stdout, "export %s=%s\n", key, shellQuote(exports[key]))
	}
	return nil
}

func parseEnvExportSimpleArgs(name, usage string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, usage)
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

func runEnvExportShell(args []string) error {
	cfg, err := parseEnvExportShellArgs(args)
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
	env, err := resolveBuildEnvironment(repoRoot, cfg.platform)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, env.shellExports())
	return nil
}

func parseEnvExportShellArgs(args []string) (envExportShellConfig, error) {
	cfg := envExportShellConfig{}

	fs := flag.NewFlagSet("env export-shell", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.platform, "platform", "", "override detected platform: android, ios, web, linux, windows, or macos")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl env export-shell [--platform android|ios|web|linux|windows|macos]")
	}

	if err := fs.Parse(args); err != nil {
		return envExportShellConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return envExportShellConfig{}, errUsage
	}
	if err := validateOptionalPlatform(cfg.platform); err != nil {
		return envExportShellConfig{}, err
	}
	return cfg, nil
}

func runEnvEnsureEngineSource(args []string) error {
	if err := parseEnvEnsureEngineSourceArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	return ensureEngineSource(repoRoot, func(name string, args ...string) error {
		return runStreamingCommand("", name, args...)
	})
}

func parseEnvEnsureEngineSourceArgs(args []string) error {
	fs := flag.NewFlagSet("env ensure-engine-source", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl env ensure-engine-source")
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

func runEnvExportMacOSVulkanShell(args []string) error {
	if err := parseEnvExportMacOSVulkanShellArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sdkRoot, err := resolveMacOSVulkanSDKRoot(homeDir, os.Getenv("VULKAN_SDK"))
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, macOSVulkanSDKShellExports(sdkRoot))
	return nil
}

func parseEnvExportMacOSVulkanShellArgs(args []string) error {
	fs := flag.NewFlagSet("env export-macos-vulkan-shell", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl env export-macos-vulkan-shell")
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
