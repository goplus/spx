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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

const (
	runtimeOfflineEnv       = "SPX_RUNTIME_OFFLINE"
	runtimeLocalManifestEnv = "SPX_RUNTIME_LOCAL_MANIFEST"
	runtimeAssetDirEnv      = "SPX_RUNTIME_ASSET_DIR"
	runtimeCacheEnv         = "SPX_RUNTIME_CACHE"
	maxRuntimeManifestSize  = 16 << 20
)

type runtimeAssetDependencies struct {
	fetch       runtimebundle.FetchFunc
	cacheRoot   func() string
	manifestPin func(release.RuntimeLock) (release.RuntimeManifestPin, error)
	goBin       func(context.Context, Config, []string) (string, error)
}

func defaultRuntimeAssetDependencies() runtimeAssetDependencies {
	return runtimeAssetDependencies{
		fetch:       fetchRuntimeURL,
		cacheRoot:   runtimebundle.DefaultCacheRoot,
		manifestPin: release.RuntimeManifestPinForLock,
		goBin:       resolveGoBin,
	}
}

type runtimeAssetSource struct {
	manifest       release.RuntimeManifest
	manifestSHA256 string
	manifestDir    string
	fetch          runtimebundle.FetchFunc
}

type localRuntimeSource struct {
	manifest   release.LocalRuntimeManifest
	bytes      []byte
	enginePath string
	packPath   string
}

// acquireRuntimeAssets obtains one verified Engine/PCK pair.
func acquireRuntimeAssets(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock) (Assets, error) {
	return acquireRuntimeAssetsWith(ctx, cfg, streams, lock, defaultRuntimeAssetDependencies())
}

func acquireRuntimeAssetsWith(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock, dependencies runtimeAssetDependencies) (Assets, error) {
	if ctx == nil {
		return Assets{}, errors.New("launchpack: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Assets{}, err
	}
	if dependencies.fetch == nil || dependencies.cacheRoot == nil || dependencies.manifestPin == nil {
		return Assets{}, errors.New("launchpack: incomplete runtime acquisition dependencies")
	}
	env := runtimeEnvironment(cfg, streams.Env)
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Assets{}, err
	}
	cacheDefault := dependencies.cacheRoot
	if cfg.RuntimeCacheRoot != "" {
		cacheDefault = func() string { return cfg.RuntimeCacheRoot }
	}
	cacheRoot, err := resolveRuntimeCacheRoot(env, cacheDefault)
	if err != nil {
		return Assets{}, err
	}
	offline, err := runtimeOffline(env)
	if err != nil {
		return Assets{}, err
	}
	offline = offline || cfg.RuntimeOffline

	if local, found, err := findExplicitLocalRuntimeManifest(env, lock, spec); err != nil {
		return Assets{}, err
	} else if found {
		return materializeLocalRuntime(ctx, cacheRoot, lock, spec, local)
	}

	_, assetDirSet, duplicate := environmentValue(env, runtimeAssetDirEnv)
	if duplicate {
		return Assets{}, fmt.Errorf("launchpack: duplicate %s", runtimeAssetDirEnv)
	}
	pin, err := dependencies.manifestPin(lock)
	if err != nil {
		publishedErr := fmt.Errorf("launchpack: resolve runtime manifest pin: %w", err)
		if !assetDirSet && cfg.Source.SourceMode && errors.Is(err, release.ErrRuntimeManifestPinNotFound) {
			return acquireSourceRuntime(ctx, cfg, env, cacheRoot, lock, spec, dependencies, publishedErr)
		}
		return Assets{}, publishedErr
	}
	if err := pin.ValidateForLock(lock); err != nil {
		return Assets{}, err
	}

	source, err := resolvePublishedRuntime(ctx, cacheRoot, lock, spec, pin, env, offline, dependencies)
	if err == nil {
		var assets Assets
		assets, err = materializePublishedRuntime(ctx, cacheRoot, lock, spec, source, offline)
		if err == nil {
			return assets, nil
		}
	}
	if assetDirSet || !cfg.Source.SourceMode {
		return Assets{}, err
	}
	return acquireSourceRuntime(ctx, cfg, env, cacheRoot, lock, spec, dependencies, err)
}

func resolveRuntimeCacheRoot(env []string, defaultRoot func() string) (string, error) {
	value, found, duplicate := environmentValue(env, runtimeCacheEnv)
	if duplicate {
		return "", fmt.Errorf("launchpack: duplicate %s", runtimeCacheEnv)
	}
	if !found || value == "" {
		value = filepath.Clean(defaultRoot())
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeCacheEnv)
	}
	return value, nil
}

func runtimeOffline(env []string) (bool, error) {
	value, found, duplicate := environmentValue(env, runtimeOfflineEnv)
	if duplicate {
		return false, fmt.Errorf("launchpack: duplicate %s", runtimeOfflineEnv)
	}
	if !found || strings.TrimSpace(value) == "" {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("launchpack: invalid %s value %q", runtimeOfflineEnv, value)
	}
}

func environmentValue(env []string, key string) (value string, found, duplicate bool) {
	for _, entry := range env {
		name, current, ok := strings.Cut(entry, "=")
		if !ok || name != key {
			continue
		}
		if found {
			return "", true, true
		}
		value, found = current, true
	}
	return value, found, false
}

func runtimeEnvironment(cfg Config, base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := append([]string(nil), base...)
	for _, item := range []struct{ key, value string }{
		{runtimeLocalManifestEnv, cfg.RuntimeManifestPath},
		{runtimeAssetDirEnv, cfg.RuntimeAssetDir},
		{runtimeCacheEnv, cfg.RuntimeCacheRoot},
	} {
		if item.value == "" {
			continue
		}
		filtered := env[:0]
		for _, entry := range env {
			key, _, ok := strings.Cut(entry, "=")
			if !ok || key != item.key {
				filtered = append(filtered, entry)
			}
		}
		env = append(filtered, item.key+"="+item.value)
	}
	return env
}

func findExplicitLocalRuntimeManifest(env []string, lock release.RuntimeLock, spec release.HostRuntimeSpec) (localRuntimeSource, bool, error) {
	path, found, duplicate := environmentValue(env, runtimeLocalManifestEnv)
	if duplicate {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: duplicate %s", runtimeLocalManifestEnv)
	}
	if !found {
		return localRuntimeSource{}, false, nil
	}
	return readLocalRuntimeManifest(path, lock, spec, true)
}

func findSourceLocalRuntimeManifest(root string, lock release.RuntimeLock, spec release.HostRuntimeSpec) (localRuntimeSource, bool, error) {
	path, err := release.LocalRuntimeManifestPath(root, lock, spec.GOOS, spec.GOARCH)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	if info, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return localRuntimeSource{}, false, nil
		}
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: inspect local runtime manifest: %w", err)
	} else if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: discovered local runtime manifest is not a regular non-symlink file: %s", path)
	}
	return readLocalRuntimeManifest(path, lock, spec, false)
}

func readLocalRuntimeManifest(path string, lock release.RuntimeLock, spec release.HostRuntimeSpec, strict bool) (localRuntimeSource, bool, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeLocalManifestEnv)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: inspect local runtime manifest %q: %w", path, err)
	}
	if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("launchpack: local runtime manifest %q is not a regular non-symlink file", path)
	}
	data, err := readRegularFile(path)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	manifest, err := release.ParseLocalRuntimeManifest(data)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	validate := manifest.ValidateForVersion
	if strict {
		validate = manifest.ValidateForLock
	}
	if err := validate(lock, spec.GOOS, spec.GOARCH); err != nil {
		return localRuntimeSource{}, false, err
	}
	directory := filepath.Dir(path)
	if err := manifest.VerifyFiles(directory); err != nil {
		return localRuntimeSource{}, false, err
	}
	return localRuntimeSource{
		manifest: manifest, bytes: data,
		enginePath: filepath.Join(directory, manifest.Engine.Name),
		packPath:   filepath.Join(directory, manifest.Pack.Name),
	}, true, nil
}
