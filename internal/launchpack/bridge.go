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
	"path/filepath"
	"runtime"
)

// BuildSourceBridge compiles the configured source bridge and returns a path
// plus a cleanup function. Provenance checks belong to the caller.
func BuildSourceBridge(ctx context.Context, cfg Config) (string, func(), error) {
	if ctx == nil {
		return "", nil, fmt.Errorf("launchpack: nil context")
	}
	if err := cfg.validateGraphInputs(); err != nil {
		return "", nil, err
	}
	bridgeName, err := bridgeFileName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", nil, err
	}
	if cfg.BridgePackage == "" {
		return "", nil, fmt.Errorf("launchpack: bridge package is required")
	}
	if err := cfg.verifyGraph(ctx, "before source bridge build"); err != nil {
		return "", nil, err
	}
	workDir, err := os.MkdirTemp("", "spx-launchpack-bridge-")
	if err != nil {
		return "", nil, fmt.Errorf("launchpack: create source bridge work directory: %w", err)
	}
	keepWork := hasBuildFlag(cfg.BuildFlags, "work")
	cleanup := func() { _ = os.RemoveAll(workDir) }
	if keepWork {
		cleanup = func() {}
		if cfg.IO.Stderr != nil {
			_, _ = fmt.Fprintf(cfg.IO.Stderr, "SPXBRIDGEWORK=%s\n", workDir)
		}
	}
	bridgePath := filepath.Join(workDir, bridgeName)
	command := cfg.goBuildCommand(ctx, "c-shared", bridgePath, cfg.BridgePackage)
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

func bridgeFileName(goos, goarch string) (string, error) {
	extension := ""
	switch goos {
	case "darwin":
		extension = ".dylib"
	case "linux":
		extension = ".so"
	case "windows":
		extension = ".dll"
	default:
		return "", fmt.Errorf("launchpack: host platform %s/%s is not supported", goos, goarch)
	}
	return "gdspx-" + goos + "-" + goarch + extension, nil
}
