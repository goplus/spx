package main

import (
	"errors"
	"flag"
	"fmt"
)

type engineDownloadConfig struct {
	runtime  bool
	platform string
	mode     string
}

type engineBuildConfig struct {
	target   string
	platform string
	mode     string
}

func runEngine(args []string) error {
	if len(args) == 0 {
		printEngineUsage()
		return errUsage
	}

	switch args[0] {
	case "download":
		return runEngineDownload(args[1:])
	case "build":
		return runEngineBuild(args[1:])
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
	fmt.Fprintln(osStderr, "Usage: buildctl engine <download|build|exec> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  download   Download runtime or platform engine assets")
	fmt.Fprintln(osStderr, "  build      Build engine editor or templates")
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

	repoRoot, err := findRepoRoot()
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
	fs.StringVar(&cfg.platform, "platform", "", "download templates for android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl engine download [--runtime] [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram]")
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
	if cfg.runtime && cfg.platform != "" {
		return errors.New("--runtime cannot be combined with --platform")
	}
	if cfg.runtime && cfg.mode != "" {
		return errors.New("--runtime cannot be combined with --mode")
	}
	if err := validateOptionalPlatform(cfg.platform); err != nil {
		return err
	}
	if cfg.platform == "web" && cfg.mode == "" {
		cfg.mode = "normal"
	}
	if cfg.mode != "" {
		if cfg.platform != "web" {
			return errors.New("--mode requires --platform web")
		}
		if err := validateWebMode(cfg.mode); err != nil {
			return err
		}
	}
	return nil
}

func runEngineBuild(args []string) error {
	cfg, err := parseEngineBuildArgs(args)
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
	return buildEngine(cfg, repoRoot)
}

func parseEngineBuildArgs(args []string) (engineBuildConfig, error) {
	cfg := engineBuildConfig{}

	fs := flag.NewFlagSet("engine build", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.target, "target", "", "engine build target: editor or template")
	fs.StringVar(&cfg.platform, "platform", "", "build platform: android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl engine build --target editor|template [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return engineBuildConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return engineBuildConfig{}, errUsage
	}
	if err := cfg.validate(); err != nil {
		return engineBuildConfig{}, err
	}
	return cfg, nil
}

func (cfg *engineBuildConfig) validate() error {
	switch cfg.target {
	case "editor", "template":
	default:
		return fmt.Errorf("unsupported build target: %s", cfg.target)
	}
	if err := validateOptionalPlatform(cfg.platform); err != nil {
		return err
	}
	if cfg.platform == "web" && cfg.mode == "" {
		cfg.mode = "normal"
	}
	if cfg.mode != "" {
		if cfg.platform != "web" {
			return errors.New("--mode requires --platform web")
		}
		if err := validateWebMode(cfg.mode); err != nil {
			return err
		}
	}
	return nil
}
