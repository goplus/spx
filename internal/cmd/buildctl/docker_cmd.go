package main

import (
	"errors"
	"flag"
	"fmt"
)

type dockerBuildImagesConfig struct {
	proxyURL string
}

type dockerBuildEngineConfig struct {
	godotSrc string
}

func runDocker(args []string) error {
	if len(args) == 0 {
		printDockerUsage()
		return errUsage
	}

	switch args[0] {
	case "build-images":
		return runDockerBuildImagesCommand(args[1:])
	case "build-engine":
		return runDockerBuildEngineCommand(args[1:])
	case "help", "-h", "--help":
		printDockerUsage()
		return nil
	default:
		printDockerUsage()
		return fmt.Errorf("unknown docker command %q", args[0])
	}
}

func printDockerUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl docker <build-images|build-engine> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  build-images  Build the podman base/linux Godot container images")
	fmt.Fprintln(osStderr, "  build-engine  Run the legacy podman/native Godot build matrix")
}

func runDockerBuildImagesCommand(args []string) error {
	cfg, err := parseDockerBuildImagesArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return runDockerBuildImages(cfg)
}

func parseDockerBuildImagesArgs(args []string) (dockerBuildImagesConfig, error) {
	cfg := dockerBuildImagesConfig{}

	fs := flag.NewFlagSet("docker build-images", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.proxyURL, "proxy-url", "", "proxy URL used for podman image builds")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl docker build-images --proxy-url <url>")
	}
	if err := fs.Parse(args); err != nil {
		return dockerBuildImagesConfig{}, err
	}
	if fs.NArg() == 1 && cfg.proxyURL == "" {
		cfg.proxyURL = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fs.Usage()
		return dockerBuildImagesConfig{}, errUsage
	}
	if cfg.proxyURL == "" {
		return dockerBuildImagesConfig{}, errors.New("proxy URL is required")
	}
	return cfg, nil
}

func runDockerBuildEngineCommand(args []string) error {
	cfg, err := parseDockerBuildEngineArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return runDockerBuildEngine(cfg)
}

func parseDockerBuildEngineArgs(args []string) (dockerBuildEngineConfig, error) {
	cfg := dockerBuildEngineConfig{}

	fs := flag.NewFlagSet("docker build-engine", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.godotSrc, "godot-src", "", "path to the Godot source tree (defaults to GODOT_SRC or ./godot)")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl docker build-engine [--godot-src /abs/path/to/godot]")
	}
	if err := fs.Parse(args); err != nil {
		return dockerBuildEngineConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return dockerBuildEngineConfig{}, errUsage
	}
	return cfg, nil
}
