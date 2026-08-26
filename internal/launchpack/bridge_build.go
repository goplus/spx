/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package launchpack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func buildSourceBridge(ctx context.Context, cfg Config, bridgeName string, streams IO) (string, func(), error) {
	if err := cfg.verifyGraph(ctx, "before source bridge build"); err != nil {
		return "", nil, err
	}
	if cfg.BridgePackage == "" {
		return "", nil, fmt.Errorf("launchpack: bridge package is required")
	}
	workDir, err := os.MkdirTemp("", "spx-launchpack-bridge-")
	if err != nil {
		return "", nil, fmt.Errorf("launchpack: create source bridge work directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }
	if hasBuildFlag(cfg.BuildFlags, "work") {
		cleanup = func() {}
		if streams.Stderr != nil {
			_, _ = fmt.Fprintf(streams.Stderr, "SPXBRIDGEWORK=%s\n", workDir)
		}
	}
	bridgePath := filepath.Join(workDir, bridgeName)
	command := exec.CommandContext(ctx, cfg.GoCommand, sourceBridgeBuildArgs(cfg, bridgePath)...)
	command.Dir, command.Env = cfg.WorkDir, sourceBridgeEnv(cfg, streams.Env)
	command.Stdin, command.Stdout, command.Stderr = streams.Stdin, streams.Stdout, streams.Stderr
	if err := command.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("launchpack: build source interpreter bridge: %w", err)
	}
	if err := cfg.verifyGraph(ctx, "after source bridge build"); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := validatePinnedFile("source interpreter bridge", bridgePath); err != nil {
		cleanup()
		return "", nil, err
	}
	return bridgePath, cleanup, nil
}

func sourceBridgeBuildArgs(cfg Config, bridgePath string) []string {
	return sourceBridgeBuildArgsForGOOS(cfg, bridgePath, runtime.GOOS)
}

func sourceBridgeBuildArgsForGOOS(cfg Config, bridgePath, goos string) []string {
	args := append([]string{"build"}, cfg.GraphFlags...)
	args = append(args, normalizedGoBuildFlags(cfg.BuildFlags)...)
	args = append(args, "-buildmode=c-shared")
	if goos == "windows" {
		args = append(args, "-ldflags=-extldflags=-Wl,--allow-multiple-definition")
	}
	return append(args, "-o", bridgePath, cfg.BridgePackage)
}

func bridgeFileName(goos, goarch string) (string, error) {
	extension := map[string]string{"darwin": ".dylib", "linux": ".so", "windows": ".dll"}[goos]
	if extension == "" {
		return "", fmt.Errorf("launchpack: host platform %s/%s is not supported", goos, goarch)
	}
	return "gdspx-" + goos + "-" + goarch + extension, nil
}
