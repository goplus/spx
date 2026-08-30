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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

const versionedReleaseManifestCacheDirectory = "release-manifests"

var errVersionedReleaseManifestTooLarge = errors.New("launchpack: release manifest exceeds size limit")

// versionedReleaseManifestSpec identifies a manifest by its release version.
// Namespace, Version, and Name form its deterministic cache key. MirrorDir is
// an explicit local release mirror and always takes precedence over the cache.
type versionedReleaseManifestSpec struct {
	CacheRoot string
	Namespace string
	Version   string
	Name      string
	URL       string
	MirrorDir string
	Offline   bool
	MaxSize   int64
	Fetch     runtimebundle.FetchFunc
}

// acquireVersionedReleaseManifest loads a version-addressed release manifest.
// Every mirror or cache hit is parsed again and checked against spec.Version.
// The returned bytes let callers derive a content digest without introducing a
// second pin or trust root.
func acquireVersionedReleaseManifest[T any](
	ctx context.Context,
	spec versionedReleaseManifestSpec,
	parse func([]byte) (T, error),
	manifestVersion func(T) string,
) (T, []byte, error) {
	var zero T
	if ctx == nil {
		return zero, nil, errors.New("launchpack: nil context")
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, err
	}
	if err := validateVersionedReleaseManifestSpec(spec); err != nil {
		return zero, nil, err
	}
	if parse == nil || manifestVersion == nil {
		return zero, nil, errors.New("launchpack: release manifest parser and version accessor are required")
	}

	load := func(path string) (T, []byte, error) {
		manifest, data, err := loadVersionedReleaseManifest(path, spec.MaxSize, parse)
		if err != nil {
			return zero, nil, err
		}
		if got := manifestVersion(manifest); got != spec.Version {
			return zero, nil, fmt.Errorf("launchpack: release manifest version %q does not match %q", got, spec.Version)
		}
		return manifest, data, nil
	}

	if spec.MirrorDir != "" {
		path := filepath.Join(spec.MirrorDir, spec.Name)
		manifest, data, err := load(path)
		if err != nil {
			return zero, nil, fmt.Errorf("launchpack: read mirrored release manifest: %w", err)
		}
		return manifest, data, nil
	}

	cacheDirectory := filepath.Join(
		spec.CacheRoot,
		versionedReleaseManifestCacheDirectory,
		spec.Namespace,
		spec.Version,
	)
	cachePath := filepath.Join(cacheDirectory, spec.Name)
	if info, err := os.Lstat(cachePath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() {
			if _, data, loadErr := load(cachePath); loadErr == nil {
				manifest, data, acquireErr := acquireCachedVersionedReleaseManifest(ctx, cacheDirectory, spec, data, parse, manifestVersion, true)
				if acquireErr == nil {
					return manifest, data, nil
				}
				if spec.Offline {
					return zero, nil, fmt.Errorf("launchpack: offline cached release manifest is invalid: %w", acquireErr)
				}
			} else if spec.Offline {
				return zero, nil, fmt.Errorf("launchpack: offline cached release manifest is invalid: %w", loadErr)
			}
		} else if spec.Offline {
			return zero, nil, fmt.Errorf("launchpack: offline cached release manifest %q is not a regular non-symlink file", cachePath)
		}
	} else if !os.IsNotExist(err) {
		return zero, nil, fmt.Errorf("launchpack: inspect cached release manifest: %w", err)
	}
	if spec.Offline {
		return zero, nil, fmt.Errorf("%w for %s/%s/%s", runtimebundle.ErrOfflineCacheMiss, spec.Namespace, spec.Version, spec.Name)
	}
	if spec.Fetch == nil {
		return zero, nil, errors.New("launchpack: release manifest fetcher is required")
	}

	var downloaded bytes.Buffer
	limited := &versionedReleaseManifestWriter{destination: &downloaded, remaining: spec.MaxSize}
	if err := spec.Fetch(ctx, spec.URL, limited); err != nil {
		return zero, nil, fmt.Errorf("launchpack: fetch release manifest: %w", err)
	}
	if limited.exceeded {
		return zero, nil, errVersionedReleaseManifestTooLarge
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, err
	}
	data := downloaded.Bytes()
	manifest, err := parse(data)
	if err != nil {
		return zero, nil, fmt.Errorf("launchpack: parse downloaded release manifest: %w", err)
	}
	if got := manifestVersion(manifest); got != spec.Version {
		return zero, nil, fmt.Errorf("launchpack: downloaded release manifest version %q does not match %q", got, spec.Version)
	}
	return acquireCachedVersionedReleaseManifest(ctx, cacheDirectory, spec, data, parse, manifestVersion, false)
}

func validateVersionedReleaseManifestSpec(spec versionedReleaseManifestSpec) error {
	if spec.CacheRoot == "" || !filepath.IsAbs(spec.CacheRoot) || filepath.Clean(spec.CacheRoot) != spec.CacheRoot {
		return fmt.Errorf("launchpack: release manifest cache root must be an absolute clean path")
	}
	for _, item := range []struct{ label, value string }{
		{"namespace", spec.Namespace}, {"version", spec.Version}, {"name", spec.Name},
	} {
		if !validVersionedReleaseManifestKeyPart(item.value) {
			return fmt.Errorf("launchpack: invalid release manifest %s %q", item.label, item.value)
		}
	}
	if spec.MirrorDir != "" && (!filepath.IsAbs(spec.MirrorDir) || filepath.Clean(spec.MirrorDir) != spec.MirrorDir) {
		return fmt.Errorf("launchpack: release manifest mirror must be an absolute clean path")
	}
	if spec.MaxSize <= 0 {
		return errors.New("launchpack: release manifest size limit must be positive")
	}
	if strings.TrimSpace(spec.URL) == "" {
		return errors.New("launchpack: release manifest URL is required")
	}
	return nil
}

func validVersionedReleaseManifestKeyPart(value string) bool {
	if value == "" || value == "." || value == ".." || value != strings.TrimSpace(value) || filepath.Base(value) != value {
		return false
	}
	return !strings.ContainsAny(value, "/\\\x00")
}

func loadVersionedReleaseManifest[T any](path string, maxSize int64, parse func([]byte) (T, error)) (T, []byte, error) {
	var zero T
	file, err := openPinnedFile("release manifest", path)
	if err != nil {
		return zero, nil, err
	}
	defer file.file.Close()
	data, err := io.ReadAll(io.LimitReader(file.file, maxSize+1))
	if err != nil {
		return zero, nil, err
	}
	if int64(len(data)) > maxSize {
		return zero, nil, errVersionedReleaseManifestTooLarge
	}
	if err := file.verify(); err != nil {
		return zero, nil, err
	}
	manifest, err := parse(data)
	if err != nil {
		return zero, nil, err
	}
	return manifest, data, nil
}

func acquireCachedVersionedReleaseManifest[T any](
	ctx context.Context,
	cacheDirectory string,
	spec versionedReleaseManifestSpec,
	data []byte,
	parse func([]byte) (T, error),
	manifestVersion func(T) string,
	offline bool,
) (T, []byte, error) {
	var zero T
	file, err := runtimebundle.AcquireFile(ctx, cacheDirectory, runtimebundle.FetchSpec{
		Name: spec.Name, URL: spec.URL, Size: int64(len(data)), SHA256: digestBytes(data), Offline: offline,
		Fetch: func(ctx context.Context, _ string, dst io.Writer) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := io.Copy(dst, bytes.NewReader(data))
			return err
		},
	})
	if err != nil {
		return zero, nil, fmt.Errorf("launchpack: cache release manifest: %w", err)
	}
	cached, readErr := io.ReadAll(io.LimitReader(file, spec.MaxSize+1))
	closeErr := file.Close()
	if readErr != nil {
		return zero, nil, fmt.Errorf("launchpack: read cached release manifest: %w", readErr)
	}
	if closeErr != nil {
		return zero, nil, fmt.Errorf("launchpack: close cached release manifest: %w", closeErr)
	}
	if int64(len(cached)) > spec.MaxSize {
		return zero, nil, errVersionedReleaseManifestTooLarge
	}
	manifest, err := parse(cached)
	if err != nil {
		return zero, nil, fmt.Errorf("launchpack: parse cached release manifest: %w", err)
	}
	if got := manifestVersion(manifest); got != spec.Version {
		return zero, nil, fmt.Errorf("launchpack: cached release manifest version %q does not match %q", got, spec.Version)
	}
	return manifest, cached, nil
}

type versionedReleaseManifestWriter struct {
	destination io.Writer
	remaining   int64
	exceeded    bool
}

func (w *versionedReleaseManifestWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		w.exceeded = true
		return 0, errVersionedReleaseManifestTooLarge
	}
	n, err := w.destination.Write(data)
	w.remaining -= int64(n)
	return n, err
}
