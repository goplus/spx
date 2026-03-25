package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
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

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return prepareAssets(cfg, runner)
}

func parsePrepareArgs(args []string) (prepareConfig, error) {
	cfg := prepareConfig{
		setupMode: "runtime",
		webMode:   "normal",
	}

	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.setupMode, "setup-mode", cfg.setupMode, "setup mode: runtime, web, or full")
	fs.StringVar(&cfg.webMode, "web-mode", cfg.webMode, "web mode: normal, worker, minigame, or miniprogram")

	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl prepare [--setup-mode runtime|web|full] [--web-mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return prepareConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return prepareConfig{}, errUsage
	}

	if err := cfg.validate(); err != nil {
		return prepareConfig{}, err
	}

	return cfg, nil
}

func (cfg prepareConfig) validate() error {
	if err := validateSetupMode(cfg.setupMode); err != nil {
		return err
	}
	if err := validateWebMode(cfg.webMode); err != nil {
		return err
	}
	return nil
}

func prepareAssets(cfg prepareConfig, runner scriptRunner) error {
	switch cfg.setupMode {
	case "runtime":
		if err := runner.runScript(filepath.Join("cmd", "gox", "install.sh")); err != nil {
			return err
		}
		return prepareRuntimeAssets(runner)
	case "web":
		if err := prepareHostEditorAsset(runner.repoRootDir()); err != nil {
			return err
		}
		if err := downloadEngineAssets(engineDownloadConfig{platform: "web", mode: cfg.webMode}, runner.repoRootDir()); err != nil {
			return err
		}
		if err := runner.runScript(filepath.Join("cmd", "gox", "install.sh"), "--web"); err != nil {
			return err
		}
		return exportWebTemplateRuntime(cfg.webMode, runner)
	case "full":
		if err := prepareRuntimeAssets(runner); err != nil {
			return err
		}
		if err := downloadEngineAssets(engineDownloadConfig{platform: "web", mode: cfg.webMode}, runner.repoRootDir()); err != nil {
			return err
		}
		if err := runner.runScript(filepath.Join("cmd", "gox", "install.sh"), "--web"); err != nil {
			return err
		}
		return exportWebTemplateRuntime(cfg.webMode, runner)
	default:
		return fmt.Errorf("unsupported setup-mode: %s", cfg.setupMode)
	}
}

func prepareRuntimeAssets(runner scriptRunner) error {
	return downloadEngineAssets(engineDownloadConfig{runtime: true}, runner.repoRootDir())
}

func prepareHostEditorAsset(repoRoot string) error {
	env, err := engineDownloadResolveEnv(repoRoot, "")
	if err != nil {
		return err
	}
	return downloadPlatformAssets(env, "editor", true)
}

func prepareWebAssets(webMode string, runner scriptRunner) error {
	if err := prepareHostEditorAsset(runner.repoRootDir()); err != nil {
		return err
	}
	if err := downloadEngineAssets(engineDownloadConfig{platform: "web", mode: webMode}, runner.repoRootDir()); err != nil {
		return err
	}
	return exportWebTemplateRuntime(webMode, runner)
}
