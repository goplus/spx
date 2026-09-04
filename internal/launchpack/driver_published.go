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
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

const driverAssetDirEnv = "SPX_DRIVER_ASSET_DIR"

type driverAssetDependencies struct {
	fetch     runtimebundle.FetchFunc
	cacheRoot func() string
}

func defaultDriverAssetDependencies() driverAssetDependencies {
	return driverAssetDependencies{
		fetch:     fetchReleaseURL,
		cacheRoot: runtimebundle.DefaultCacheRoot,
	}
}

// AcquirePublishedDriver resolves one versioned combined Engine/PCK/bridge ZIP.
// Published mode never builds or borrows a bridge from the Go graph.
func AcquirePublishedDriver(ctx context.Context, cfg Config) (Assets, error) {
	if err := validatePublishedSource(cfg.Source); err != nil {
		return Assets{}, err
	}
	lock, err := runtimeLock(cfg)
	if err != nil {
		return Assets{}, err
	}
	return acquirePublishedDriverWith(ctx, cfg, cfg.IO, lock, defaultDriverAssetDependencies())
}

func acquirePublishedDriverWith(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock, deps driverAssetDependencies) (Assets, error) {
	if ctx == nil {
		return Assets{}, errors.New("launchpack: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Assets{}, err
	}
	if err := validatePublishedSource(cfg.Source); err != nil {
		return Assets{}, err
	}
	if cfg.RuntimeSourceRoot != "" || cfg.RuntimeManifestPath != "" || cfg.RuntimeAssetDir != "" || cfg.BridgePackage != "" {
		return Assets{}, errors.New("launchpack: published mode accepts only the combined driver asset inputs")
	}
	if deps.fetch == nil || deps.cacheRoot == nil {
		return Assets{}, errors.New("launchpack: incomplete published driver dependencies")
	}
	if err := lock.Validate(); err != nil {
		return Assets{}, err
	}
	env := publishedDriverEnvironment(cfg, streams.Env)
	assetDir, assetDirSet, duplicate := envutil.Lookup(env, driverAssetDirEnv)
	if duplicate {
		return Assets{}, fmt.Errorf("launchpack: duplicate %s", driverAssetDirEnv)
	}
	if assetDirSet {
		if assetDir == "" {
			return Assets{}, fmt.Errorf("launchpack: %s must not be empty", driverAssetDirEnv)
		}
		if !filepath.IsAbs(assetDir) || filepath.Clean(assetDir) != assetDir {
			return Assets{}, fmt.Errorf("launchpack: %s must be an absolute clean path", driverAssetDirEnv)
		}
	}
	cacheRoot, err := resolveRuntimeCacheRoot(env, func() string { return deps.cacheRoot() })
	if err != nil {
		return Assets{}, err
	}
	offline, err := runtimeOffline(env)
	if err != nil {
		return Assets{}, err
	}
	offline = offline || cfg.RuntimeOffline

	spxVersion := cfg.Source.SelectedVersion
	manifestURL, err := driverbundle.ManifestURL(spxVersion)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: build published driver manifest URL: %w", err)
	}
	parseManifest := func(data []byte) (driverbundle.Manifest, error) {
		manifest, err := driverbundle.ParseForVersions(data, spxVersion, lock.RuntimeVersion)
		if err != nil {
			return driverbundle.Manifest{}, err
		}
		for _, bundle := range manifest.Bundles {
			if err := validateDriverBundleSize(bundle); err != nil {
				return driverbundle.Manifest{}, err
			}
		}
		return manifest, nil
	}
	manifest, manifestData, err := acquireVersionedReleaseManifest(ctx, versionedReleaseManifestSpec{
		CacheRoot: cacheRoot, Namespace: "driver", Version: spxVersion,
		Name: driverbundle.ManifestName, URL: manifestURL, MirrorDir: assetDir,
		Offline: offline, MaxSize: driverbundle.MaxManifestSize, Fetch: deps.fetch,
	}, parseManifest, func(manifest driverbundle.Manifest) string { return manifest.SPXVersion })
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: acquire published driver manifest: %w", err)
	}
	manifestDigest := digestBytes(manifestData)
	bundle, err := manifest.BundleFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Assets{}, err
	}

	bundleURL, err := manifest.DownloadURL(bundle.Name)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: build published driver bundle URL: %w", err)
	}
	releaseRoot := filepath.Join(cacheRoot, "downloads", "driver", spxVersion)
	cache := runtimebundle.NewCache(cacheRoot)
	expected, err := expectedDriverBundle(bundle)
	if err != nil {
		return Assets{}, err
	}
	var bundleFile *runtimebundle.AcquiredFile
	if assetDirSet {
		bundleFile, err = acquireDriverFile(ctx, releaseRoot, bundle.Name, bundle.Size, bundle.SHA256, bundleURL, assetDir, offline, deps.fetch)
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: acquire published driver bundle: %w", err)
		}
		defer bundleFile.Close()
	}
	if materialized, found, err := cache.Lookup(ctx, runtimebundle.NamespaceDriver, &expected); err != nil {
		return Assets{}, fmt.Errorf("launchpack: inspect cached published driver bundle: %w", err)
	} else if found {
		assets, err := publishedDriverAssets(materialized, bundle, manifestDigest, spxVersion, lock)
		if err != nil {
			_ = materialized.Close()
			return Assets{}, err
		}
		return assets, nil
	}
	if bundleFile == nil {
		bundleFile, err = acquireDriverFile(ctx, releaseRoot, bundle.Name, bundle.Size, bundle.SHA256, bundleURL, assetDir, offline, deps.fetch)
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: acquire published driver bundle: %w", err)
		}
		defer bundleFile.Close()
	}
	materialized, err := cache.Materialize(ctx, runtimebundle.NamespaceDriver, filepath.Join(releaseRoot, bundle.Name), &expected)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: materialize published driver bundle: %w", err)
	}
	assets, err := publishedDriverAssets(materialized, bundle, manifestDigest, spxVersion, lock)
	if err != nil {
		_ = materialized.Close()
		return Assets{}, err
	}
	return assets, nil
}
