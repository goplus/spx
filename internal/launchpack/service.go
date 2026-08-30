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
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/release"
)

// AcquireRuntimeAssets resolves and materializes one verified Engine/PCK pair.
// Explicit local settings take priority. Source checkouts may use an exact
// GOPATH/bin runtime when the selected versioned release is unavailable.
func AcquireRuntimeAssets(ctx context.Context, cfg Config) (Assets, error) {
	lock, err := runtimeLock(cfg)
	if err != nil {
		return Assets{}, err
	}
	return acquireRuntimeAssets(ctx, cfg, cfg.IO, lock)
}

// BuildSourceBridge compiles the configured source bridge and returns a path
// plus a cleanup function. Provenance checks belong to the caller.
func BuildSourceBridge(ctx context.Context, cfg Config) (string, func(), error) {
	if ctx == nil {
		return "", nil, fmt.Errorf("launchpack: nil context")
	}
	if err := cfg.validateGraphInputs(); err != nil {
		return "", nil, err
	}
	name, err := bridgeFileName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", nil, err
	}
	return buildSourceBridge(ctx, cfg, name, cfg.IO)
}

// BuildLauncher packages the project, runtime, and bridge into one executable.
func BuildLauncher(ctx context.Context, cfg Config) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("launchpack: nil context")
	}
	if err := cfg.validate(); err != nil {
		return Result{}, err
	}
	if _, err := cfg.PortableConfig.Identity(); err != nil {
		return Result{}, fmt.Errorf("launchpack: portable config: %w", err)
	}
	assets, err := AcquireRuntimeAssets(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	bridge, bridgeCleanup, err := BuildSourceBridge(ctx, cfg)
	if err != nil {
		if assets.Cleanup != nil {
			assets.Cleanup()
		}
		return Result{}, err
	}
	assets.BridgePath = bridge
	if cfg.VerifyBridge != nil {
		if err := cfg.VerifyBridge(bridge); err != nil {
			bridgeCleanup()
			if assets.Cleanup != nil {
				assets.Cleanup()
			}
			return Result{}, fmt.Errorf("launchpack: verify source bridge: %w", err)
		}
	}
	oldCleanup := assets.Cleanup
	cleanup := func() {
		bridgeCleanup()
		if oldCleanup != nil {
			oldCleanup()
		}
	}
	defer cleanup()

	payload, manifest, err := buildLauncher(ctx, cfg, assets, cfg.PortableConfig, cfg.IO)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Output: cfg.Output, PayloadSHA256: payload, ManifestSHA256: manifest,
	}, nil
}

func (c Config) verifyGraph(ctx context.Context, phase string) error {
	if c.VerifyGraph == nil {
		return nil
	}
	if err := c.VerifyGraph(ctx); err != nil {
		return fmt.Errorf("launchpack: graph changed %s: %w", phase, err)
	}
	return nil
}

func runtimeLock(cfg Config) (release.RuntimeLock, error) {
	lock := cfg.RuntimeLock
	if lock.RuntimeVersion == "" {
		lock = release.DefaultRuntimeLock()
	}
	if err := lock.Validate(); err != nil {
		return release.RuntimeLock{}, err
	}
	identity := cfg.RuntimeIdentity
	if identity.Version != "" && identity.Version != lock.RuntimeVersion {
		return release.RuntimeLock{}, fmt.Errorf("launchpack: runtime version %q does not match lock %q", identity.Version, lock.RuntimeVersion)
	}
	if identity.ABI != 0 && identity.ABI != lock.RuntimeABI {
		return release.RuntimeLock{}, fmt.Errorf("launchpack: runtime ABI %d does not match lock %d", identity.ABI, lock.RuntimeABI)
	}
	if identity.GOOS != "" && identity.GOOS != runtime.GOOS || identity.GOARCH != "" && identity.GOARCH != runtime.GOARCH {
		return release.RuntimeLock{}, fmt.Errorf("launchpack: runtime target does not match host %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if cfg.RuntimeCacheRoot != "" && (!filepath.IsAbs(cfg.RuntimeCacheRoot) || filepath.Clean(cfg.RuntimeCacheRoot) != cfg.RuntimeCacheRoot) {
		return release.RuntimeLock{}, fmt.Errorf("launchpack: runtime cache root must be an absolute clean path")
	}
	return lock, nil
}
