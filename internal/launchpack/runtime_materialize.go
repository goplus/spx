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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type publishedRuntimeArchives struct {
	engine *runtimebundle.AcquiredFile
	pack   *runtimebundle.AcquiredFile
}

type runtimeMaterializationInput struct {
	manifestSHA256  string
	enginePath      string
	packPath        string
	archives        *publishedRuntimeArchives
	engineSize      int64
	engineSHA256    string
	packSize        int64
	packSHA256      string
	releaseManifest release.RuntimeManifest
}

type runtimeBundleOrigin struct {
	Schema              string `json:"schema"`
	Mode                string `json:"mode"`
	RuntimeVersion      string `json:"runtime_version"`
	RuntimeABI          int    `json:"runtime_abi"`
	LockSHA256          string `json:"lock_sha256"`
	ManifestSHA256      string `json:"manifest_sha256"`
	GOOS                string `json:"goos"`
	GOARCH              string `json:"goarch"`
	EngineArchive       string `json:"engine_archive,omitempty"`
	EngineArchiveSHA256 string `json:"engine_archive_sha256,omitempty"`
	PackArchive         string `json:"pack_archive,omitempty"`
	PackArchiveSHA256   string `json:"pack_archive_sha256,omitempty"`
	EngineName          string `json:"engine_name"`
	EngineSize          int64  `json:"engine_size"`
	EngineSHA256        string `json:"engine_sha256"`
	PackName            string `json:"pack_name"`
	PackSize            int64  `json:"pack_size"`
	PackSHA256          string `json:"pack_sha256"`
}

func materializePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source runtimeAssetSource, offline bool) (Assets, error) {
	engineAsset, ok := source.manifest.Asset(spec.ArchiveName)
	if !ok {
		return Assets{}, fmt.Errorf("launchpack: runtime manifest has no host asset %q", spec.ArchiveName)
	}
	packAsset, ok := source.manifest.Asset(release.RuntimeAssetZipName)
	if !ok {
		return Assets{}, fmt.Errorf("launchpack: runtime manifest has no runtime pack asset %q", release.RuntimeAssetZipName)
	}
	assetRoot := filepath.Join(cacheRoot, "release-assets", "runtime", lock.RuntimeVersion)
	assetDir := source.manifestDir
	engineFile, err := acquireReleaseAsset(ctx, assetRoot, engineAsset, lock.RuntimeAssetDownloadURL(spec.ArchiveName), assetDir, offline, source.fetch)
	if err != nil {
		return Assets{}, err
	}
	defer engineFile.Close()
	packFile, err := acquireReleaseAsset(ctx, assetRoot, packAsset, lock.RuntimeAssetDownloadURL(release.RuntimeAssetZipName), assetDir, offline, source.fetch)
	if err != nil {
		return Assets{}, err
	}
	defer packFile.Close()
	return materializeRuntimeBundle(ctx, cacheRoot, lock, spec, runtimeMaterializationInput{
		manifestSHA256:  source.manifestSHA256,
		archives:        &publishedRuntimeArchives{engine: engineFile, pack: packFile},
		releaseManifest: source.manifest,
	})
}

func materializeLocalRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source localRuntimeSource) (Assets, error) {
	return materializeRuntimeBundle(ctx, cacheRoot, lock, spec, runtimeMaterializationInput{
		manifestSHA256: digestBytes(source.bytes),
		enginePath:     source.enginePath,
		packPath:       source.packPath,
		engineSize:     source.manifest.Engine.Size,
		engineSHA256:   source.manifest.Engine.SHA256,
		packSize:       source.manifest.Pack.Size,
		packSHA256:     source.manifest.Pack.SHA256,
	})
}

func materializeRuntimeBundle(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, input runtimeMaterializationInput) (Assets, error) {
	local := input.archives == nil
	if !local {
		if input.archives.engine == nil || input.archives.pack == nil {
			return Assets{}, errors.New("launchpack: incomplete published runtime archive handles")
		}
		workDir, err := os.MkdirTemp("", "spx-launchpack-runtime-")
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: create runtime extraction directory: %w", err)
		}
		defer os.RemoveAll(workDir)
		hostDir := filepath.Join(workDir, "host")
		packDir := filepath.Join(workDir, "pack")
		if err := os.MkdirAll(hostDir, 0o700); err != nil {
			return Assets{}, err
		}
		if err := os.MkdirAll(packDir, 0o700); err != nil {
			return Assets{}, err
		}
		engineInfo, err := input.archives.engine.Stat()
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: stat acquired Engine archive: %w", err)
		}
		packInfo, err := input.archives.pack.Stat()
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: stat acquired runtime pack archive: %w", err)
		}
		engineBundle, err := runtimebundle.ExtractZipReader(input.archives.engine, engineInfo.Size(), hostDir)
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: verify Engine archive: %w", err)
		}
		packBundle, err := runtimebundle.ExtractZipReader(input.archives.pack, packInfo.Size(), packDir)
		if err != nil {
			return Assets{}, fmt.Errorf("launchpack: verify runtime pack archive: %w", err)
		}
		input.enginePath = filepath.Join(hostDir, filepath.FromSlash(spec.BinaryName))
		input.packPath = filepath.Join(packDir, filepath.FromSlash("gdspxrt.pck"))
		if err := validateRuntimeEntry(input.enginePath, spec.BinaryName, engineBundle); err != nil {
			return Assets{}, err
		}
		if err := validateRuntimeEntry(input.packPath, "gdspxrt.pck", packBundle); err != nil {
			return Assets{}, err
		}
	}

	engineSize, engineSHA, err := hashRuntimeFile(input.enginePath)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: hash Engine: %w", err)
	}
	packSize, packSHA, err := hashRuntimeFile(input.packPath)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: hash runtime PCK: %w", err)
	}
	if local && (engineSize != input.engineSize || engineSHA != input.engineSHA256 || packSize != input.packSize || packSHA != input.packSHA256) {
		return Assets{}, errors.New("launchpack: local runtime changed after manifest verification")
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		return Assets{}, err
	}
	origin := runtimeBundleOrigin{
		Schema: "spx-runtime-acquisition/v1", Mode: "published", RuntimeVersion: lock.RuntimeVersion,
		RuntimeABI: lock.RuntimeABI, LockSHA256: lockSHA, ManifestSHA256: input.manifestSHA256,
		GOOS: spec.GOOS, GOARCH: spec.GOARCH, EngineName: spec.RuntimeName,
		EngineSize: engineSize, EngineSHA256: engineSHA, PackName: spec.PackName,
		PackSize: packSize, PackSHA256: packSHA,
	}
	if local {
		origin.Mode = "local"
	} else {
		origin.EngineArchive = spec.ArchiveName
		if asset, ok := input.releaseManifest.Asset(spec.ArchiveName); ok {
			origin.EngineArchiveSHA256 = asset.SHA256
		}
		origin.PackArchive = release.RuntimeAssetZipName
		if asset, ok := input.releaseManifest.Asset(release.RuntimeAssetZipName); ok {
			origin.PackArchiveSHA256 = asset.SHA256
		}
	}
	originBytes, err := json.Marshal(origin)
	if err != nil {
		return Assets{}, err
	}
	bundle, err := expectedEngineBundle(originBytes, spec, engineSize, engineSHA, packSize, packSHA)
	if err != nil {
		return Assets{}, err
	}
	bundleWork, err := os.MkdirTemp("", "spx-launchpack-engine-")
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: create Engine bundle directory: %w", err)
	}
	defer os.RemoveAll(bundleWork)
	bundleZip := filepath.Join(bundleWork, "engine.bundle.zip")
	if err := writeEngineBundle(bundleZip, originBytes, spec.RuntimeName, spec.PackName, input.enginePath, input.packPath); err != nil {
		return Assets{}, err
	}
	materialized, err := runtimebundle.NewCache(cacheRoot).Materialize(ctx, runtimebundle.NamespaceEngine, bundleZip, &bundle)
	if err != nil {
		return Assets{}, fmt.Errorf("launchpack: materialize verified Engine bundle: %w", err)
	}
	materializedEnginePath := filepath.Join(materialized.Path, spec.RuntimeName)
	materializedPackPath := filepath.Join(materialized.Path, spec.PackName)
	if err := validateRuntimeFile(materializedEnginePath, "Engine"); err != nil {
		_ = materialized.Close()
		return Assets{}, err
	}
	if err := validateRuntimeFile(materializedPackPath, "runtime PCK"); err != nil {
		_ = materialized.Close()
		return Assets{}, err
	}
	return Assets{EnginePath: materializedEnginePath, PackPath: materializedPackPath, Lock: lock, Cleanup: func() { _ = materialized.Close() }}, nil
}
