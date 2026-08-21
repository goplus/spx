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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

var runtimeHTTPClient = &http.Client{Timeout: 30 * time.Minute}

func resolvePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, spec release.HostRuntimeSpec, env []string, offline bool, dependencies runtimeAssetDependencies) (runtimeAssetSource, error) {
	pin, err := dependencies.manifestPin(lock)
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: resolve runtime manifest pin: %w", err)
	}
	if err := pin.ValidateForLock(lock); err != nil {
		return runtimeAssetSource{}, err
	}
	assetDir, assetDirSet, duplicate := environmentValue(env, runtimeAssetDirEnv)
	if duplicate {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: duplicate %s", runtimeAssetDirEnv)
	}
	if assetDirSet {
		if assetDir == "" {
			return runtimeAssetSource{}, fmt.Errorf("launchpack: %s must not be empty", runtimeAssetDirEnv)
		}
		if !filepath.IsAbs(assetDir) || filepath.Clean(assetDir) != assetDir {
			return runtimeAssetSource{}, fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeAssetDirEnv)
		}
		data, err := readRegularFile(filepath.Join(assetDir, lock.Manifest))
		if err != nil {
			return runtimeAssetSource{}, fmt.Errorf("launchpack: read local release manifest: %w", err)
		}
		return parseRuntimeAssetSource(lock, pin, data, assetDir, dependencies.fetch)
	}
	manifestURL := lock.RuntimeAssetDownloadURL(lock.Manifest)
	manifestRoot := filepath.Join(cacheRoot, "release-manifests")
	manifestName := pin.SHA256 + "-" + pin.Name
	manifestFile, err := runtimebundle.AcquireFile(ctx, manifestRoot, runtimebundle.FetchSpec{
		Name: manifestName, URL: manifestURL, Size: pin.Size, SHA256: pin.SHA256,
		Offline: offline, Fetch: dependencies.fetch,
	})
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: acquire runtime manifest for %s/%s: %w", spec.GOOS, spec.GOARCH, err)
	}
	data, readErr := readRuntimeMetadata(manifestFile, manifestName)
	closeErr := manifestFile.Close()
	if readErr != nil {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: read acquired runtime manifest: %w", readErr)
	}
	if closeErr != nil {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: close acquired runtime manifest: %w", closeErr)
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
	return "https://github.com/" + s.manifest.ReleaseRepository + "/releases/download/runtime-v" + s.manifest.RuntimeVersion + "/" + name
}

func acquireReleaseAsset(ctx context.Context, root string, asset release.RuntimeAsset, url, localDir string, offline bool, fetch runtimebundle.FetchFunc) (*runtimebundle.AcquiredFile, error) {
	name := asset.SHA256 + "-" + asset.Name
	if localDir != "" {
		if !filepath.IsAbs(localDir) || filepath.Clean(localDir) != localDir {
			return nil, fmt.Errorf("launchpack: %s must be an absolute clean path", runtimeAssetDirEnv)
		}
		src := filepath.Join(localDir, asset.Name)
		return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
			Name: name, URL: src, Size: asset.Size, SHA256: asset.SHA256,
			Fetch: func(ctx context.Context, _ string, dst io.Writer) error { return copyLocalRuntimeAsset(ctx, src, dst) },
		})
	}
	return runtimebundle.AcquireFile(ctx, root, runtimebundle.FetchSpec{
		Name: name, URL: url, Size: asset.Size, SHA256: asset.SHA256,
		Offline: offline, Fetch: fetch,
	})
}

func verifyRuntimeManifestPin(pin release.RuntimeManifestPin, data []byte) error {
	if int64(len(data)) != pin.Size {
		return fmt.Errorf("launchpack: runtime manifest size = %d, want pinned %d", len(data), pin.Size)
	}
	if digest := digestBytes(data); digest != pin.SHA256 {
		return fmt.Errorf("launchpack: runtime manifest SHA-256 = %s, want pinned %s", digest, pin.SHA256)
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
		return err
	})
	return data, err
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
