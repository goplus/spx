package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v2/internal/cmd/buildctl/runtimecmd"
	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
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

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return prepareAssets(cfg, runner)
}

func parsePrepareArgs(args []string) (prepareConfig, error) {
	cfg := prepareConfig{
		setupMode: "runtime",
		webMode:   "normal",
	}

	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.setupMode, "setup-mode", cfg.setupMode, "setup mode: runtime, web, or full")
	fs.StringVar(&cfg.webMode, "web-mode", cfg.webMode, "web mode: normal, worker, minigame, or miniprogram")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildctl prepare [--setup-mode runtime|web|full] [--web-mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return prepareConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return prepareConfig{}, shared.ErrUsage
	}

	if err := cfg.validate(); err != nil {
		return prepareConfig{}, err
	}

	return cfg, nil
}

func (cfg prepareConfig) validate() error {
	if err := shared.ValidateSetupMode(cfg.setupMode); err != nil {
		return err
	}
	if err := shared.ValidateWebMode(cfg.webMode); err != nil {
		return err
	}
	return nil
}

func prepareAssets(cfg prepareConfig, runner shared.ScriptRunner) error {
	switch cfg.setupMode {
	case "runtime":
		if err := runner.RunScript(filepath.Join("cmd", "spx", "install.sh")); err != nil {
			return err
		}
		return prepareRuntimeAssets(runner)
	case "web":
		if err := prepareHostEditorAsset(runner.RepoRootDir()); err != nil {
			return err
		}
		return prepareWebAssets(cfg.webMode, runner)
	case "full":
		if err := prepareRuntimeAssets(runner); err != nil {
			return err
		}
		return prepareWebAssets(cfg.webMode, runner)
	default:
		return fmt.Errorf("unsupported setup-mode: %s", cfg.setupMode)
	}
}

func prepareRuntimeAssets(runner shared.ScriptRunner) error {
	return engine.DownloadEngineAssets(engine.DownloadConfig{Runtime: true}, runner.RepoRootDir())
}

func prepareHostEditorAsset(repoRoot string) error {
	return engine.PrepareHostEditorAsset(repoRoot)
}

func prepareWebAssets(webMode string, runner shared.ScriptRunner) error {
	if err := engine.DownloadEngineAssets(engine.DownloadConfig{Platform: "web", Mode: webMode}, runner.RepoRootDir()); err != nil {
		return err
	}
	if err := runner.RunScript(filepath.Join("cmd", "spx", "install.sh"), "--web"); err != nil {
		return err
	}
	return runtimecmd.ExportWebTemplateRuntime(webMode, runner)
}
