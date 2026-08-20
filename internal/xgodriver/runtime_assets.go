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
	"context"
	"crypto/sha256"
	"encoding/hex"
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

type localRuntimeSource struct {
	manifest  release.LocalRuntimeManifest
	directory string
	bytes     []byte
}

// acquireRuntimeAssets obtains one verified Engine/PCK pair.
func acquireRuntimeAssets(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock) (localAssets, error) {
	return acquireRuntimeAssetsWith(ctx, cfg, streams, lock, defaultRuntimeAssetDependencies())
}

func acquireRuntimeAssetsWith(ctx context.Context, cfg Config, streams IO, lock release.RuntimeLock, dependencies runtimeAssetDependencies) (localAssets, error) {
	if ctx == nil {
		return localAssets{}, errors.New("xgodriver: nil context")
	}
	if dependencies.fetch == nil || dependencies.cacheRoot == nil || dependencies.manifestPin == nil {
		return localAssets{}, errors.New("xgodriver: incomplete runtime acquisition dependencies")
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
		return "", fmt.Errorf("xgodriver: duplicate %s", runtimeCacheEnv)
	}
	if !found || value == "" {
		value = filepath.Clean(defaultRoot())
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("xgodriver: %s must be an absolute clean path", runtimeCacheEnv)
	}
	return value, nil
}

func runtimeOffline(env []string) (bool, error) {
	value, found, duplicate := environmentValue(env, runtimeOfflineEnv)
	if duplicate {
		return false, fmt.Errorf("xgodriver: duplicate %s", runtimeOfflineEnv)
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
		return false, fmt.Errorf("xgodriver: invalid %s value %q", runtimeOfflineEnv, value)
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
		return localRuntimeSource{}, false, fmt.Errorf("xgodriver: duplicate %s", runtimeLocalManifestEnv)
	}
	if !explicit {
		candidate, pathErr := release.LocalRuntimeManifestPath(cfg.DriverOrigin.Effective().Dir, lock, spec.GOOS, spec.GOARCH)
		if pathErr != nil {
			return localRuntimeSource{}, false, pathErr
		}
		if info, err := os.Lstat(candidate); err == nil {
			if !isRegularNonSymlink(info) {
				return localRuntimeSource{}, false, fmt.Errorf("xgodriver: discovered local runtime manifest is not a regular non-symlink file: %s", candidate)
			}
			path = candidate
			explicit = true
		} else if !os.IsNotExist(err) {
			return localRuntimeSource{}, false, fmt.Errorf("xgodriver: inspect local runtime manifest: %w", err)
		}
	}
	if !explicit {
		return localRuntimeSource{}, false, nil
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return localRuntimeSource{}, false, fmt.Errorf("xgodriver: %s must be an absolute clean path", runtimeLocalManifestEnv)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return localRuntimeSource{}, false, fmt.Errorf("xgodriver: inspect local runtime manifest %q: %w", path, err)
	}
	if !isRegularNonSymlink(info) {
		return localRuntimeSource{}, false, fmt.Errorf("xgodriver: local runtime manifest %q is not a regular non-symlink file", path)
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
		return runtimeAssetSource{}, fmt.Errorf("xgodriver: resolve runtime manifest pin: %w", err)
	}
	if err := pin.ValidateForLock(lock); err != nil {
		return runtimeAssetSource{}, err
	}
	assetDir, assetDirSet, duplicate := environmentValue(env, runtimeAssetDirEnv)
	if duplicate {
		return runtimeAssetSource{}, fmt.Errorf("xgodriver: duplicate %s", runtimeAssetDirEnv)
	}
	if assetDirSet {
		if assetDir == "" {
			return runtimeAssetSource{}, fmt.Errorf("xgodriver: %s must not be empty", runtimeAssetDirEnv)
		}
		if !filepath.IsAbs(assetDir) || filepath.Clean(assetDir) != assetDir {
			return runtimeAssetSource{}, fmt.Errorf("xgodriver: %s must be an absolute clean path", runtimeAssetDirEnv)
		}
		manifestPath := filepath.Join(assetDir, lock.Manifest)
		data, err := readRegularFile(manifestPath)
		if err != nil {
			return runtimeAssetSource{}, fmt.Errorf("xgodriver: read local release manifest: %w", err)
		}
		return parseRuntimeAssetSource(lock, pin, data, assetDir, dependencies.fetch)
	}
	manifestURL := lock.RuntimeAssetDownloadURL(lock.Manifest)
	manifestRoot := filepath.Join(cacheRoot, "release-manifests")
	manifestName := pin.SHA256 + "-" + pin.Name
	manifestFile, err := runtimebundle.AcquireFile(ctx, manifestRoot, runtimebundle.FetchSpec{
		Name: manifestName, URL: manifestURL, ExpectedSize: pin.Size, ExpectedSHA256: pin.SHA256,
		MaxSize: maxRuntimeManifestSize, Offline: offline, Fetch: dependencies.fetch,
	})
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgodriver: acquire runtime manifest for %s/%s: %w", spec.GOOS, spec.GOARCH, err)
	}
	data, err := readRuntimeMetadata(manifestFile, manifestFile.Path)
	closeErr := manifestFile.Close()
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgodriver: read acquired runtime manifest: %w", err)
	}
	if closeErr != nil {
		return runtimeAssetSource{}, fmt.Errorf("xgodriver: close acquired runtime manifest: %w", closeErr)
	}
	return parseRuntimeAssetSource(lock, pin, data, "", dependencies.fetch)
}

func parseRuntimeAssetSource(lock release.RuntimeLock, pin release.RuntimeManifestPin, data []byte, manifestDir string, fetch runtimebundle.FetchFunc) (runtimeAssetSource, error) {
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
	return runtimeAssetSource{manifest: manifest, manifestSHA256: pin.SHA256, manifestDir: manifestDir, fetch: fetch}, nil
}

func (s runtimeAssetSource) manifestURL(name string) string {
	return "https://github.com/" + s.manifest.ReleaseRepository + "/releases/download/" + "runtime-v" + s.manifest.RuntimeVersion + "/" + name
}

func acquireReleaseAsset(ctx context.Context, root string, asset release.RuntimeAsset, url, localDir string, offline bool, fetch runtimebundle.FetchFunc) (*runtimebundle.AcquiredFile, error) {
	name := asset.SHA256 + "-" + asset.Name
	if localDir != "" {
		if !filepath.IsAbs(localDir) || filepath.Clean(localDir) != localDir {
			return nil, fmt.Errorf("xgodriver: %s must be an absolute clean path", runtimeAssetDirEnv)
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
		return fmt.Errorf("xgodriver: runtime manifest size = %d, want pinned %d", len(data), pin.Size)
	}
	if digest := digestBytes(data); digest != pin.SHA256 {
		return fmt.Errorf("xgodriver: runtime manifest SHA-256 = %s, want pinned %s", digest, pin.SHA256)
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
