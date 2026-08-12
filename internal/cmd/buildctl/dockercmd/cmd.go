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

package dockercmd

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type dockerBuildImagesConfig struct {
	proxyURL string
}

type dockerBuildEngineConfig struct {
	godotSrc string
}

func Run(args []string) error {
	if len(args) == 0 {
		printDockerUsage()
		return shared.ErrUsage
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
	fmt.Fprintln(os.Stderr, "Usage: buildctl docker <build-images|build-engine> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Legacy: these unsupported container workflows use an independent historical toolchain;")
	fmt.Fprintln(os.Stderr, "        current builds use buildctl build and runtime.lock.json instead.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  build-images  Build the legacy podman base/linux container images")
	fmt.Fprintln(os.Stderr, "  build-engine  Run the legacy podman/native Godot build matrix")
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
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.proxyURL, "proxy-url", "", "proxy URL used for podman image builds")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildctl docker build-images --proxy-url <url>")
	}
	if err := fs.Parse(args); err != nil {
		return dockerBuildImagesConfig{}, err
	}
	if fs.NArg() == 1 && cfg.proxyURL == "" {
		cfg.proxyURL = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fs.Usage()
		return dockerBuildImagesConfig{}, shared.ErrUsage
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
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.godotSrc, "godot-src", "", "path to the Godot source tree (defaults to GODOT_SRC or ./godot)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildctl docker build-engine [--godot-src /abs/path/to/godot]")
	}
	if err := fs.Parse(args); err != nil {
		return dockerBuildEngineConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return dockerBuildEngineConfig{}, shared.ErrUsage
	}
	return cfg, nil
}
