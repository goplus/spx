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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/release"
)

func acquireSourceRuntime(ctx context.Context, cfg Config, env []string, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, dependencies runtimeAssetDependencies, publishedErr error) (Assets, error) {
	if err := ctx.Err(); err != nil {
		return Assets{}, err
	}
	root := cfg.RuntimeSourceRoot
	if root == "" {
		return Assets{}, sourceRuntimeError(root, publishedErr, errors.New("SPX source root is unavailable"))
	}
	local, found, err := findSourceLocalRuntimeManifest(root, lock, spec)
	if err != nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, err)
	}
	if found {
		assets, err := materializeLocalRuntime(ctx, cacheRoot, lock, spec, local)
		if err != nil {
			return Assets{}, sourceRuntimeError(root, publishedErr, err)
		}
		return assets, nil
	}
	if dependencies.goBin == nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, errors.New("Go-bin resolver is unavailable"))
	}
	bin, err := dependencies.goBin(ctx, cfg, env)
	if err != nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, err)
	}
	enginePath := filepath.Join(bin, spec.RuntimeName)
	packPath := filepath.Join(bin, spec.PackName)
	manifest, err := release.NewLocalRuntimeManifest(lock, spec.GOOS, spec.GOARCH, enginePath, packPath)
	if err != nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, fmt.Errorf("inspect %s: %w", bin, err))
	}
	data, err := manifest.JSON()
	if err != nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, err)
	}
	local = localRuntimeSource{manifest: manifest, bytes: data, enginePath: enginePath, packPath: packPath}
	assets, err := materializeLocalRuntime(ctx, cacheRoot, lock, spec, local)
	if err != nil {
		return Assets{}, sourceRuntimeError(root, publishedErr, err)
	}
	return assets, nil
}

func sourceRuntimeError(root string, publishedErr, localErr error) error {
	hint := "run make dev in the SPX source checkout"
	if root != "" {
		hint = "run make dev in " + root
	}
	return fmt.Errorf("launchpack: published runtime unavailable: %w; local runtime unavailable: %w; %s", publishedErr, localErr, hint)
}

func resolveGoBin(ctx context.Context, cfg Config, env []string) (string, error) {
	if cfg.GoCommand == "" {
		return "", errors.New("Go command is unavailable")
	}
	command := exec.CommandContext(ctx, cfg.GoCommand, "env", "GOPATH")
	command.Dir = cfg.WorkDir
	command.Env = hostGoEnv(cfg, env)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("resolve Go bin: %w: %s", err, message)
		}
		return "", fmt.Errorf("resolve Go bin: %w", err)
	}
	paths := filepath.SplitList(strings.TrimSpace(string(output)))
	if len(paths) == 0 || paths[0] == "" {
		return "", errors.New("Go environment has no GOPATH")
	}
	bin := filepath.Join(paths[0], "bin")
	if !filepath.IsAbs(bin) || filepath.Clean(bin) != bin {
		return "", fmt.Errorf("Go bin must be an absolute clean path: %q", bin)
	}
	return bin, nil
}
