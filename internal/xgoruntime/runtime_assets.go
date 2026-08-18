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

package xgoruntime

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

var runtimeHTTPClient = &http.Client{Timeout: 30 * time.Minute}

type runtimeAssetDependencies struct {
	fetch       runtimebundle.FetchFunc
	cacheRoot   func() string
	manifestPin func(release.RuntimeLock) (release.RuntimeManifestPin, error)
}

func defaultRuntimeAssetDependencies() runtimeAssetDependencies {
	return runtimeAssetDependencies{
		fetch:       fetchRuntimeURL,
		cacheRoot:   runtimebundle.DefaultCacheRoot,
		manifestPin: release.RuntimeManifestPinForLock,
	}
}

type runtimeAssetSource struct {
	manifest       release.RuntimeManifest
	manifestSHA256 string
	manifestDir    string
	fetch          runtimebundle.FetchFunc
}

type publishedRuntimeArchives struct {
	engine *runtimebundle.AcquiredFile
	pack   *runtimebundle.AcquiredFile
}

type localRuntimeSource struct {
	manifest  release.LocalRuntimeManifest
	directory string
	bytes     []byte
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

// acquireRuntimeAssets obtains one verified Engine/PCK pair. Published assets
// are downloaded from the immutable runtime release; local assets are accepted
// only through an explicit, digest-bearing local manifest. GOPATH is not a
// source of truth for either path.
func acquireRuntimeAssets(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock) (localAssets, error) {
	return acquireRuntimeAssetsWith(ctx, cfg, streams, lock, defaultRuntimeAssetDependencies())
}

func acquireRuntimeAssetsWith(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock, dependencies runtimeAssetDependencies) (localAssets, error) {
	if ctx == nil {
		return localAssets{}, errors.New("xgoruntime: nil context")
	}
	if dependencies.fetch == nil || dependencies.cacheRoot == nil || dependencies.manifestPin == nil {
		return localAssets{}, errors.New("xgoruntime: incomplete runtime acquisition dependencies")
	}
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return localAssets{}, err
	}
	cacheRoot, err := resolveRuntimeCacheRoot(streams.Env, dependencies.cacheRoot)
	if err != nil {
		return localAssets{}, err
	}
	offline, err := runtimeOffline(streams.Env)
	if err != nil {
		return localAssets{}, err
	}

	if local, found, err := findLocalRuntimeManifest(cfg, streams.Env, lock, spec); err != nil {
		return localAssets{}, err
	} else if found {
		return materializeLocalRuntime(ctx, cacheRoot, lock, spec, local)
	}

	source, err := resolvePublishedRuntime(ctx, cacheRoot, lock, spec, streams.Env, offline, dependencies)
	if err != nil {
		return localAssets{}, err
	}
	return materializePublishedRuntime(ctx, cacheRoot, lock, spec, source, offline)
}

func resolveRuntimeCacheRoot(env []string, defaultRoot func() string) (string, error) {
	value, found, duplicate := environmentValue(env, runtimeCacheEnv)
	if duplicate {
		return "", fmt.Errorf("xgoruntime: duplicate %s", runtimeCacheEnv)
	}
	if !found || value == "" {
		value = filepath.Clean(defaultRoot())
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("xgoruntime: %s must be an absolute clean path", runtimeCacheEnv)
	}
	return value, nil
}

func runtimeOffline(env []string) (bool, error) {
	value, found, duplicate := environmentValue(env, runtimeOfflineEnv)
	if duplicate {
		return false, fmt.Errorf("xgoruntime: duplicate %s", runtimeOfflineEnv)
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
		return false, fmt.Errorf("xgoruntime: invalid %s value %q", runtimeOfflineEnv, value)
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

func findLocalRuntimeManifest(cfg Config, env []string, lock release.RuntimeLock, spec release.HostRuntimeSpec) (localRuntimeSource, bool, error) {
	path, explicit, duplicate := environmentValue(env, runtimeLocalManifestEnv)
	if duplicate {
		return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: duplicate %s", runtimeLocalManifestEnv)
	}
	if !explicit {
		candidate, pathErr := release.LocalRuntimeManifestPath(cfg.ProviderOrigin.Effective().Dir, lock, spec.GOOS, spec.GOARCH)
		if pathErr != nil {
			return localRuntimeSource{}, false, pathErr
		}
		if info, err := os.Lstat(candidate); err == nil {
			if !isRegularNonSymlink(info) {
				return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: discovered local runtime manifest is not a regular non-symlink file: %s", candidate)
			}
			path = candidate
			explicit = true
		} else if !os.IsNotExist(err) {
			return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: inspect local runtime manifest: %w", err)
		}
	}
	if !explicit {
		return localRuntimeSource{}, false, nil
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: %s must be an absolute clean path", runtimeLocalManifestEnv)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: inspect local runtime manifest %q: %w", path, err)
	}
	if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("xgoruntime: local runtime manifest %q is not a regular non-symlink file", path)
	}
	data, err := readRegularFile(path)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	manifest, err := release.ParseLocalRuntimeManifest(data)
	if err != nil {
		return localRuntimeSource{}, false, err
	}
	if err := manifest.ValidateForLock(lock, spec.GOOS, spec.GOARCH); err != nil {
		return localRuntimeSource{}, false, err
	}
	directory := filepath.Dir(path)
	if err := manifest.VerifyFiles(directory); err != nil {
		return localRuntimeSource{}, false, err
	}
	return localRuntimeSource{manifest: manifest, directory: directory, bytes: data}, true, nil
}

func resolvePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, env []string, offline bool, dependencies runtimeAssetDependencies) (runtimeAssetSource, error) {
	pin, err := dependencies.manifestPin(lock)
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgoruntime: resolve runtime manifest pin: %w", err)
	}
	if err := pin.ValidateForLock(lock); err != nil {
		return runtimeAssetSource{}, err
	}
	assetDir, assetDirSet, duplicate := environmentValue(env, runtimeAssetDirEnv)
	if duplicate {
		return runtimeAssetSource{}, fmt.Errorf("xgoruntime: duplicate %s", runtimeAssetDirEnv)
	}
	if assetDirSet {
		if assetDir == "" {
			return runtimeAssetSource{}, fmt.Errorf("xgoruntime: %s must not be empty", runtimeAssetDirEnv)
		}
		if !filepath.IsAbs(assetDir) || filepath.Clean(assetDir) != assetDir {
			return runtimeAssetSource{}, fmt.Errorf("xgoruntime: %s must be an absolute clean path", runtimeAssetDirEnv)
		}
		manifestPath := filepath.Join(assetDir, lock.Manifest)
		data, err := readRegularFile(manifestPath)
		if err != nil {
			return runtimeAssetSource{}, fmt.Errorf("xgoruntime: read local release manifest: %w", err)
		}
		if err := verifyRuntimeManifestPin(pin, data); err != nil {
			return runtimeAssetSource{}, err
		}
		manifest, err := release.ParseRuntimeManifest(data)
		if err != nil {
			return runtimeAssetSource{}, err
		}
		if err := manifest.ValidateForLock(lock); err != nil {
			return runtimeAssetSource{}, err
		}
		return runtimeAssetSource{manifest: manifest, manifestSHA256: pin.SHA256, manifestDir: assetDir, fetch: dependencies.fetch}, nil
	}
	manifestURL := lock.RuntimeAssetDownloadURL(lock.Manifest)
	manifestRoot := filepath.Join(cacheRoot, "release-manifests")
	manifestName := pin.SHA256 + "-" + pin.Name
	manifestFile, err := runtimebundle.AcquireFile(ctx, manifestRoot, runtimebundle.FetchSpec{
		Name: manifestName, URL: manifestURL, ExpectedSize: pin.Size, ExpectedSHA256: pin.SHA256,
		MaxSize: maxRuntimeManifestSize, Offline: offline, Fetch: dependencies.fetch,
	})
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgoruntime: acquire runtime manifest for %s/%s: %w", spec.GOOS, spec.GOARCH, err)
	}
	data, err := readRuntimeMetadata(manifestFile, manifestFile.Path)
	closeErr := manifestFile.Close()
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgoruntime: read acquired runtime manifest: %w", err)
	}
	if closeErr != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgoruntime: close acquired runtime manifest: %w", closeErr)
	}
	if err := verifyRuntimeManifestPin(pin, data); err != nil {
		return runtimeAssetSource{}, err
	}
	manifest, err := release.ParseRuntimeManifest(data)
	if err != nil {
		return runtimeAssetSource{}, err
	}
	if err := manifest.ValidateForLock(lock); err != nil {
		return runtimeAssetSource{}, err
	}
	return runtimeAssetSource{manifest: manifest, manifestSHA256: pin.SHA256, fetch: dependencies.fetch}, nil
}

func materializePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source runtimeAssetSource, offline bool) (localAssets, error) {
	engineAsset, ok := source.manifest.Asset(spec.ArchiveName)
	if !ok {
		return localAssets{}, fmt.Errorf("xgoruntime: runtime manifest has no host asset %q", spec.ArchiveName)
	}
	packAsset, ok := source.manifest.Asset(release.RuntimeAssetZipName)
	if !ok {
		return localAssets{}, fmt.Errorf("xgoruntime: runtime manifest has no runtime pack asset %q", release.RuntimeAssetZipName)
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
	archives := &publishedRuntimeArchives{engine: engineFile, pack: packFile}
	return materializeEngineFromArchives(ctx, cacheRoot, lock, spec, source.manifestSHA256, "", "", archives, 0, "", 0, "", source.manifest)
}

func (s runtimeAssetSource) manifestURL(name string) string {
	return "https://github.com/" + s.manifest.ReleaseRepository + "/releases/download/" + "runtime-v" + s.manifest.RuntimeVersion + "/" + name
}

func acquireReleaseAsset(ctx context.Context, root string, asset release.RuntimeAsset, url, localDir string, offline bool, fetch runtimebundle.FetchFunc) (*runtimebundle.AcquiredFile, error) {
	name := asset.SHA256 + "-" + asset.Name
	if localDir != "" {
		if !filepath.IsAbs(localDir) || filepath.Clean(localDir) != localDir {
			return nil, fmt.Errorf("xgoruntime: %s must be an absolute clean path", runtimeAssetDirEnv)
		}
		src := filepath.Join(localDir, asset.Name)
		return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
			Name: name, URL: src, ExpectedSize: asset.Size, ExpectedSHA256: asset.SHA256,
			MaxSize: asset.Size,
			Fetch: func(ctx context.Context, _ string, dst io.Writer) error {
				return copyLocalRuntimeAsset(ctx, src, dst)
			},
		})
	}
	return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
		Name: name, URL: url, ExpectedSize: asset.Size, ExpectedSHA256: asset.SHA256,
		MaxSize: asset.Size, Offline: offline, Fetch: fetch,
	})
}

func verifyRuntimeManifestPin(pin release.RuntimeManifestPin, data []byte) error {
	if int64(len(data)) != pin.Size {
		return fmt.Errorf("xgoruntime: runtime manifest size = %d, want pinned %d", len(data), pin.Size)
	}
	if digest := digestBytes(data); digest != pin.SHA256 {
		return fmt.Errorf("xgoruntime: runtime manifest SHA-256 = %s, want pinned %s", digest, pin.SHA256)
	}
	return nil
}

func copyLocalRuntimeAsset(ctx context.Context, path string, dst io.Writer) error {
	file, err := openPinnedFile("runtime release asset", path)
	if err != nil {
		return err
	}
	defer file.file.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := io.Copy(dst, file.file); err != nil {
		return err
	}
	return file.verify()
}

func materializeLocalRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, source localRuntimeSource) (localAssets, error) {
	return materializeEngineFromArchives(ctx, cacheRoot, lock, spec, digestBytes(source.bytes), filepath.Join(source.directory, source.manifest.Engine.Name), filepath.Join(source.directory, source.manifest.Pack.Name), nil, source.manifest.Engine.Size, source.manifest.Engine.SHA256, source.manifest.Pack.Size, source.manifest.Pack.SHA256, release.RuntimeManifest{})
}

func materializeEngineFromArchives(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, manifestSHA, enginePath, packPath string, published *publishedRuntimeArchives, localEngineSize int64, localEngineSHA string, localPackSize int64, localPackSHA string, releaseManifest release.RuntimeManifest) (localAssets, error) {
	var engineBundle, packBundle runtimebundle.Bundle
	var workDir string
	local := published == nil
	if !local {
		if published.engine == nil || published.pack == nil {
			return localAssets{}, errors.New("xgoruntime: incomplete published runtime archive handles")
		}
		var err error
		workDir, err = os.MkdirTemp("", "spx-runtime-source-")
		if err != nil {
			return localAssets{}, fmt.Errorf("xgoruntime: create runtime extraction directory: %w", err)
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
		engineInfo, err := published.engine.Stat()
		if err != nil {
			return localAssets{}, fmt.Errorf("xgoruntime: stat acquired Engine archive: %w", err)
		}
		packInfo, err := published.pack.Stat()
		if err != nil {
			return localAssets{}, fmt.Errorf("xgoruntime: stat acquired runtime pack archive: %w", err)
		}
		engineBundle, err = runtimebundle.ExtractZipReader(published.engine, engineInfo.Size(), hostDir)
		if err != nil {
			return localAssets{}, fmt.Errorf("xgoruntime: verify Engine archive: %w", err)
		}
		packBundle, err = runtimebundle.ExtractZipReader(published.pack, packInfo.Size(), packDir)
		if err != nil {
			return localAssets{}, fmt.Errorf("xgoruntime: verify runtime pack archive: %w", err)
		}
		enginePath = filepath.Join(hostDir, filepath.FromSlash(spec.BinaryName))
		packPath = filepath.Join(packDir, filepath.FromSlash("gdspxrt.pck"))
		if err := validateRuntimeEntry(enginePath, spec.BinaryName, engineBundle); err != nil {
			return localAssets{}, err
		}
		if err := validateRuntimeEntry(packPath, "gdspxrt.pck", packBundle); err != nil {
			return localAssets{}, err
		}
	}

	engineSize, engineSHA, err := hashRuntimeFile(enginePath)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgoruntime: hash Engine: %w", err)
	}
	packSize, packSHA, err := hashRuntimeFile(packPath)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgoruntime: hash runtime PCK: %w", err)
	}
	if local && (engineSize != localEngineSize || engineSHA != localEngineSHA || packSize != localPackSize || packSHA != localPackSHA) {
		return localAssets{}, errors.New("xgoruntime: local runtime changed after manifest verification")
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		return localAssets{}, err
	}
	origin := runtimeBundleOrigin{
		Schema: "spx-runtime-acquisition/v1", Mode: "published", RuntimeVersion: lock.RuntimeVersion,
		RuntimeABI: lock.RuntimeABI, LockSHA256: lockSHA, ManifestSHA256: manifestSHA,
		GOOS: spec.GOOS, GOARCH: spec.GOARCH, EngineName: spec.RuntimeName,
		EngineSize: engineSize, EngineSHA256: engineSHA, PackName: spec.PackName,
		PackSize: packSize, PackSHA256: packSHA,
	}
	if local {
		origin.Mode = "local"
	} else {
		origin.EngineArchive = spec.ArchiveName
		if asset, ok := releaseManifest.Asset(spec.ArchiveName); ok {
			origin.EngineArchiveSHA256 = asset.SHA256
		}
		origin.PackArchive = release.RuntimeAssetZipName
		if asset, ok := releaseManifest.Asset(release.RuntimeAssetZipName); ok {
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
		return localAssets{}, fmt.Errorf("xgoruntime: create Engine bundle directory: %w", err)
	}
	defer os.RemoveAll(bundleWork)
	bundleZip := filepath.Join(bundleWork, "engine.bundle.zip")
	if err := writeEngineBundle(bundleZip, originBytes, spec.RuntimeName, spec.PackName, enginePath, packPath); err != nil {
		return localAssets{}, err
	}
	materialized, err := runtimebundle.NewCache(cacheRoot).Materialize(ctx, runtimebundle.NamespaceEngine, bundleZip, &bundle)
	if err != nil {
		return localAssets{}, fmt.Errorf("xgoruntime: materialize verified Engine bundle: %w", err)
	}
	enginePath = filepath.Join(materialized.Path, spec.RuntimeName)
	packPath = filepath.Join(materialized.Path, spec.PackName)
	if err := validateRuntimeFile(enginePath, "Engine"); err != nil {
		_ = materialized.Close()
		return localAssets{}, err
	}
	if err := validateRuntimeFile(packPath, "runtime PCK"); err != nil {
		_ = materialized.Close()
		return localAssets{}, err
	}
	return localAssets{EnginePath: enginePath, PackPath: packPath, Lock: lock, Cleanup: func() { _ = materialized.Close() }}, nil
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
	return fmt.Errorf("xgoruntime: runtime archive is missing %s", name)
}

func isRegularNonSymlink(info os.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func validateRuntimeFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("xgoruntime: %s unavailable at %s: %w", label, path, err)
	}
	if !isRegularNonSymlink(info) {
		return fmt.Errorf("xgoruntime: %s %q is not a regular non-symlink file", label, path)
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
	defer func() {
		_ = os.Remove(tmpPath)
	}()
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

func readRegularFile(path string) ([]byte, error) {
	var data []byte
	err := withPinnedFile("runtime metadata", path, func(file *pinnedFile) error {
		var err error
		data, err = readRuntimeMetadata(file.file, path)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readRuntimeMetadata(reader io.Reader, path string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxRuntimeManifestSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRuntimeManifestSize {
		return nil, fmt.Errorf("runtime metadata %q exceeds %d bytes", path, maxRuntimeManifestSize)
	}
	return data, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fetchRuntimeURL(ctx context.Context, url string, dst io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := runtimeHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned %s", url, response.Status)
	}
	_, err = io.Copy(dst, response.Body)
	return err
}
