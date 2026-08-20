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

package xgodriver

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

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

func materializePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source runtimeAssetSource, offline bool) (localAssets, error) {
	engineAsset, ok := source.manifest.Asset(spec.ArchiveName)
	if !ok {
		return localAssets{}, fmt.Errorf("xgodriver: runtime manifest has no host asset %q", spec.ArchiveName)
	}
	packAsset, ok := source.manifest.Asset(release.RuntimeAssetZipName)
	if !ok {
		return localAssets{}, fmt.Errorf("xgodriver: runtime manifest has no runtime pack asset %q", release.RuntimeAssetZipName)
	}
	assetRoot := filepath.Join(cacheRoot, "release-assets", source.manifest.LockSHA256)
	assetDir := source.manifestDir
	engineFile, err := acquireReleaseAsset(ctx, assetRoot, engineAsset, source.manifestURL(spec.ArchiveName), assetDir, offline, source.fetch)
	if err != nil {
		return localAssets{}, err
	}
	defer engineFile.Close()
	packFile, err := acquireReleaseAsset(ctx, assetRoot, packAsset, source.manifestURL(release.RuntimeAssetZipName), assetDir, offline, source.fetch)
	if err != nil {
		return localAssets{}, err
	}
	defer packFile.Close()
	return materializeRuntimeBundle(ctx, cacheRoot, lock, spec, runtimeMaterializationInput{
		manifestSHA256:  source.manifestSHA256,
		archives:        &publishedRuntimeArchives{engine: engineFile, pack: packFile},
		releaseManifest: source.manifest,
	})
}

func materializeLocalRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source localRuntimeSource) (localAssets, error) {
	return materializeRuntimeBundle(ctx, cacheRoot, lock, spec, runtimeMaterializationInput{
		manifestSHA256: digestBytes(source.bytes),
		enginePath:     filepath.Join(source.directory, source.manifest.Engine.Name),
		packPath:       filepath.Join(source.directory, source.manifest.Pack.Name),
		engineSize:     source.manifest.Engine.Size,
		engineSHA256:   source.manifest.Engine.SHA256,
		packSize:       source.manifest.Pack.Size,
		packSHA256:     source.manifest.Pack.SHA256,
	})
}

func materializeRuntimeBundle(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, input runtimeMaterializationInput) (localAssets, error) {
	local := input.archives == nil
	if !local {
		if input.archives.engine == nil || input.archives.pack == nil {
			return localAssets{}, errors.New("xgodriver: incomplete published runtime archive handles")
		}
		workDir, err := os.MkdirTemp("", "spx-runtime-source-")
		if err != nil {
			return localAssets{}, fmt.Errorf("xgodriver: create runtime extraction directory: %w", err)
		}
		defer os.RemoveAll(workDir)
		hostDir := filepath.Join(workDir, "host")
		packDir := filepath.Join(workDir, "pack")
		if err := os.MkdirAll(hostDir, 0o700); err != nil {
			return localAssets{}, err
		}
		if err := os.MkdirAll(packDir, 0o700); err != nil {
			return localAssets{}, err
		}
		engineInfo, err := input.archives.engine.Stat()
		if err != nil {
			return localAssets{}, fmt.Errorf("xgodriver: stat acquired Engine archive: %w", err)
		}
		packInfo, err := input.archives.pack.Stat()
		if err != nil {
			return localAssets{}, fmt.Errorf("xgodriver: stat acquired runtime pack archive: %w", err)
		}
		engineBundle, err := runtimebundle.ExtractZipReader(input.archives.engine, engineInfo.Size(), hostDir)
		if err != nil {
			return localAssets{}, fmt.Errorf("xgodriver: verify Engine archive: %w", err)
		}
		packBundle, err := runtimebundle.ExtractZipReader(input.archives.pack, packInfo.Size(), packDir)
		if err != nil {
			return localAssets{}, fmt.Errorf("xgodriver: verify runtime pack archive: %w", err)
		}
		input.enginePath = filepath.Join(hostDir, filepath.FromSlash(spec.BinaryName))
		input.packPath = filepath.Join(packDir, filepath.FromSlash("gdspxrt.pck"))
		if err := validateRuntimeEntry(input.enginePath, spec.BinaryName, engineBundle); err != nil {
			return localAssets{}, err
		}
		if err := validateRuntimeEntry(input.packPath, "gdspxrt.pck", packBundle); err != nil {
			return localAssets{}, err
		}
	}

	engineSize, engineSHA, err := hashRuntimeFile(input.enginePath)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgodriver: hash Engine: %w", err)
	}
	packSize, packSHA, err := hashRuntimeFile(input.packPath)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgodriver: hash runtime PCK: %w", err)
	}
	if local && (engineSize != input.engineSize || engineSHA != input.engineSHA256 || packSize != input.packSize || packSHA != input.packSHA256) {
		return localAssets{}, errors.New("xgodriver: local runtime changed after manifest verification")
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		return localAssets{}, err
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
		return localAssets{}, err
	}
	bundle, err := expectedEngineBundle(originBytes, spec, engineSize, engineSHA, packSize, packSHA)
	if err != nil {
		return localAssets{}, err
	}
	bundleWork, err := os.MkdirTemp("", "spx-engine-bundle-")
	if err != nil {
		return localAssets{}, fmt.Errorf("xgodriver: create Engine bundle directory: %w", err)
	}
	defer os.RemoveAll(bundleWork)
	bundleZip := filepath.Join(bundleWork, "engine.bundle.zip")
	if err := writeEngineBundle(bundleZip, originBytes, spec.RuntimeName, spec.PackName, input.enginePath, input.packPath); err != nil {
		return localAssets{}, err
	}
	materialized, err := runtimebundle.NewCache(cacheRoot).Materialize(ctx, runtimebundle.NamespaceEngine, bundleZip, &bundle)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgodriver: materialize verified Engine bundle: %w", err)
	}
	materializedEnginePath := filepath.Join(materialized.Path, spec.RuntimeName)
	materializedPackPath := filepath.Join(materialized.Path, spec.PackName)
	if err := validateRuntimeFile(materializedEnginePath, "Engine"); err != nil {
		_ = materialized.Close()
		return localAssets{}, err
	}
	if err := validateRuntimeFile(materializedPackPath, "runtime PCK"); err != nil {
		_ = materialized.Close()
		return localAssets{}, err
	}
	return localAssets{EnginePath: materializedEnginePath, PackPath: materializedPackPath, Lock: lock, Cleanup: func() { _ = materialized.Close() }}, nil
}

func expectedEngineBundle(origin []byte, spec release.HostRuntimeSpec, engineSize int64, engineSHA string, packSize int64, packSHA string) (runtimebundle.Bundle, error) {
	bundle := runtimebundle.Bundle{
		Schema:    runtimebundle.SchemaV1,
		Namespace: runtimebundle.NamespaceEngine,
		Entries: []runtimebundle.Entry{
			{Name: "runtime-manifest.json", Mode: 0o600, Size: int64(len(origin)), SHA256: digestBytes(origin)},
			{Name: spec.RuntimeName, Mode: 0o700, Size: engineSize, SHA256: engineSHA},
			{Name: spec.PackName, Mode: 0o600, Size: packSize, SHA256: packSHA},
		},
	}
	return bundle.WithDigest()
}

func validateRuntimeEntry(path, name string, bundle runtimebundle.Bundle) error {
	if err := validateRuntimeFile(path, name); err != nil {
		return err
	}
	for _, entry := range bundle.Entries {
		if entry.Name == filepath.ToSlash(name) {
			return nil
		}
	}
	return fmt.Errorf("xgodriver: runtime archive is missing %s", name)
}

func isRegularNonSymlink(info os.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func validateRuntimeFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("xgodriver: %s unavailable at %s: %w", label, path, err)
	}
	if !isRegularNonSymlink(info) {
		return fmt.Errorf("xgodriver: %s %q is not a regular non-symlink file", label, path)
	}
	return nil
}

func withPinnedFile(name, path string, fn func(*pinnedFile) error) error {
	file, err := openPinnedFile(name, path)
	if err != nil {
		return err
	}
	operationErr := fn(file)
	if operationErr == nil {
		operationErr = file.verify()
	}
	closeErr := file.file.Close()
	if operationErr != nil {
		return operationErr
	}
	return closeErr
}

func hashRuntimeFile(path string) (int64, string, error) {
	var size int64
	var digest string
	err := withPinnedFile("runtime file", path, func(file *pinnedFile) error {
		hasher := sha256.New()
		var copyErr error
		size, copyErr = io.Copy(hasher, file.file)
		if copyErr != nil {
			return copyErr
		}
		if size != file.info.Size() {
			return errors.New("file changed while reading")
		}
		digest = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	return size, digest, nil
}

func writeEngineBundle(path string, origin []byte, engineName, packName, enginePath, packPath string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".engine-bundle-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	zw := zip.NewWriter(tmp)
	addBytes := func(name string, mode os.FileMode, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(mode)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}
	if err := addBytes("runtime-manifest.json", 0o600, origin); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return err
	}
	if err := addFileToZip(zw, engineName, enginePath, 0o700); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return fmt.Errorf("add Engine to bundle: %w", err)
	}
	if err := addFileToZip(zw, packName, packPath, 0o600); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return fmt.Errorf("add runtime PCK to bundle: %w", err)
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeFile(tmpPath, path, 0o600)
}

func addFileToZip(zw *zip.Writer, name, path string, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func replaceRuntimeFile(src, dst string, mode os.FileMode) error {
	if info, err := os.Lstat(dst); err == nil {
		if !isRegularNonSymlink(info) {
			return fmt.Errorf("destination %q is not a regular non-symlink file", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Chmod(dst, mode)
	}
	return nil
}
