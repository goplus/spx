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
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

var runtimeHTTPClient = &http.Client{Timeout: 30 * time.Minute}

var errReleaseUnavailable = errors.New("launchpack: release unavailable")

func resolvePublishedRuntime(ctx context.Context, cacheRoot string, lock release.RuntimeLock, env []string, offline bool, dependencies runtimeAssetDependencies) (runtimeAssetSource, error) {
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
	}
	manifest, data, err := acquireVersionedReleaseManifest(ctx, versionedReleaseManifestSpec{
		CacheRoot: cacheRoot,
		Namespace: "runtime",
		Version:   lock.RuntimeVersion,
		Name:      lock.Manifest,
		URL:       lock.RuntimeAssetDownloadURL(lock.Manifest),
		MirrorDir: assetDir,
		Offline:   offline,
		MaxSize:   maxRuntimeManifestSize,
		Fetch:     dependencies.fetch,
	}, func(data []byte) (release.RuntimeManifest, error) {
		return release.ParseRuntimeManifestForRelease(data, lock.RuntimeVersion, lock.RequiredAssets)
	}, func(manifest release.RuntimeManifest) string {
		return manifest.RuntimeVersion
	})
	if err != nil {
		return runtimeAssetSource{}, fmt.Errorf("launchpack: acquire runtime release manifest: %w", err)
	}
	manifestDir := ""
	if assetDirSet {
		manifestDir = assetDir
	}
	return runtimeAssetSource{manifest: manifest, manifestSHA256: digestBytes(data), manifestDir: manifestDir, fetch: dependencies.fetch}, nil
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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: GET %s: %w", errReleaseUnavailable, url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: GET %s returned %s", errReleaseUnavailable, url, response.Status)
	}
	tracked := &writeErrorTracker{writer: dst}
	if _, err := io.Copy(tracked, response.Body); err != nil {
		if tracked.err != nil {
			return tracked.err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: read %s: %w", errReleaseUnavailable, url, err)
	}
	return nil
}

type writeErrorTracker struct {
	writer io.Writer
	err    error
}

func (w *writeErrorTracker) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	if err != nil {
		w.err = err
	} else if n != len(data) {
		w.err = io.ErrShortWrite
	}
	return n, err
}
