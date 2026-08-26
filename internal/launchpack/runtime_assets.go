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
	"runtime"

	"github.com/goplus/spx/v3/internal/envutil"
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
	fetch     runtimebundle.FetchFunc
	cacheRoot func() string
	goBin     func(context.Context, Config, []string) (string, error)
}

func defaultRuntimeAssetDependencies() runtimeAssetDependencies {
	return runtimeAssetDependencies{
		fetch:     fetchReleaseURL,
		cacheRoot: runtimebundle.DefaultCacheRoot,
		goBin:     resolveGoBin,
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
	if dependencies.fetch == nil || dependencies.cacheRoot == nil {
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

	_, assetDirSet, duplicate := envutil.Lookup(env, runtimeAssetDirEnv)
	if duplicate {
		return Assets{}, fmt.Errorf("launchpack: duplicate %s", runtimeAssetDirEnv)
	}
	source, err := resolvePublishedRuntime(ctx, cacheRoot, lock, env, offline, dependencies)
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
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Assets{}, ctxErr
	}
	if !runtimeReleaseUnavailable(err) {
		return Assets{}, err
	}
	return acquireSourceRuntime(ctx, cfg, env, cacheRoot, lock, spec, dependencies, err)
}

func runtimeReleaseUnavailable(err error) bool {
	return errors.Is(err, errReleaseUnavailable) || errors.Is(err, runtimebundle.ErrOfflineCacheMiss)
}
