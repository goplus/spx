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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type testVersionedManifest struct {
	Version string `json:"version"`
	Value   string `json:"value"`
}

func TestVersionedReleaseManifestPrefersLocalMirror(t *testing.T) {
	cacheRoot, mirrorDir := t.TempDir(), t.TempDir()
	spec := testVersionedReleaseManifestSpec(cacheRoot)
	spec.MirrorDir = mirrorDir
	writeVersionedReleaseManifestTestFile(t, testVersionedReleaseManifestPath(spec), []byte(`{"version":"1.2.3","value":"cache"}`))
	mirrorData := []byte(`{"version":"1.2.3","value":"mirror"}`)
	writeVersionedReleaseManifestTestFile(t, filepath.Join(mirrorDir, spec.Name), mirrorData)
	fetchCalls := 0
	spec.Fetch = func(context.Context, string, io.Writer) error {
		fetchCalls++
		return nil
	}

	manifest, data, err := acquireTestVersionedReleaseManifest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Value != "mirror" || !bytes.Equal(data, mirrorData) {
		t.Fatalf("mirrored manifest = %#v, %q", manifest, data)
	}
	if fetchCalls != 0 {
		t.Fatalf("fetch calls = %d, want 0", fetchCalls)
	}
}

func TestVersionedReleaseManifestRevalidatesAndRefreshesCache(t *testing.T) {
	cacheRoot := t.TempDir()
	spec := testVersionedReleaseManifestSpec(cacheRoot)
	cachePath := testVersionedReleaseManifestPath(spec)
	writeVersionedReleaseManifestTestFile(t, cachePath, []byte(`{"version":"9.9.9","value":"stale"}`))
	fresh := []byte(`{"version":"1.2.3","value":"fresh"}`)
	fetchCalls := 0
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		fetchCalls++
		_, err := dst.Write(fresh)
		return err
	}

	manifest, data, err := acquireTestVersionedReleaseManifest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Value != "fresh" || !bytes.Equal(data, fresh) || fetchCalls != 1 {
		t.Fatalf("refreshed manifest = %#v, %q; fetch calls = %d", manifest, data, fetchCalls)
	}
	if cached, err := os.ReadFile(cachePath); err != nil || !bytes.Equal(cached, fresh) {
		t.Fatalf("cached manifest = %q, err = %v", cached, err)
	}

	spec.Offline = true
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Fatal("offline cache hit fetched")
		return nil
	}
	if manifest, data, err := acquireTestVersionedReleaseManifest(spec); err != nil || manifest.Value != "fresh" || !bytes.Equal(data, fresh) {
		t.Fatalf("offline cache hit = %#v, %q, %v", manifest, data, err)
	}

	if err := os.WriteFile(cachePath, []byte(`{"version":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil {
		t.Fatal("offline corrupt cache was accepted")
	}
}

func TestVersionedReleaseManifestOfflineCacheMiss(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	spec.Offline = true
	spec.Fetch = nil
	if _, _, err := acquireTestVersionedReleaseManifest(spec); !errors.Is(err, runtimebundle.ErrOfflineCacheMiss) {
		t.Fatalf("offline cache miss = %v", err)
	}
}

func TestVersionedReleaseManifestRejectsOversizedDownloadWithoutPublishing(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	spec.MaxSize = 8
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		_, err := dst.Write([]byte(`{"version":"1.2.3"}`))
		return err
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); !errors.Is(err, errVersionedReleaseManifestTooLarge) {
		t.Fatalf("oversized manifest error = %v", err)
	}
	if _, err := os.Lstat(testVersionedReleaseManifestPath(spec)); !os.IsNotExist(err) {
		t.Fatalf("oversized manifest was published: %v", err)
	}
}

func TestVersionedReleaseManifestFailedRefreshPreservesCache(t *testing.T) {
	spec := testVersionedReleaseManifestSpec(t.TempDir())
	cachePath := testVersionedReleaseManifestPath(spec)
	stale := []byte(`{"version":"9.9.9","value":"stale"}`)
	writeVersionedReleaseManifestTestFile(t, cachePath, stale)
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		_, err := dst.Write([]byte(`{"version":"8.8.8","value":"wrong"}`))
		return err
	}
	if _, _, err := acquireTestVersionedReleaseManifest(spec); err == nil {
		t.Fatal("downloaded wrong-version manifest was accepted")
	}
	if cached, err := os.ReadFile(cachePath); err != nil || !bytes.Equal(cached, stale) {
		t.Fatalf("failed refresh changed cache to %q, err = %v", cached, err)
	}
}

func testVersionedReleaseManifestSpec(cacheRoot string) versionedReleaseManifestSpec {
	return versionedReleaseManifestSpec{
		CacheRoot: cacheRoot,
		Namespace: "runtime",
		Version:   "1.2.3",
		Name:      "manifest.json",
		URL:       "https://example.invalid/manifest.json",
		MaxSize:   1024,
	}
}

func testVersionedReleaseManifestPath(spec versionedReleaseManifestSpec) string {
	return filepath.Join(spec.CacheRoot, versionedReleaseManifestCacheDirectory, spec.Namespace, spec.Version, spec.Name)
}

func acquireTestVersionedReleaseManifest(spec versionedReleaseManifestSpec) (testVersionedManifest, []byte, error) {
	return acquireVersionedReleaseManifest(context.Background(), spec, func(data []byte) (testVersionedManifest, error) {
		var manifest testVersionedManifest
		err := json.Unmarshal(data, &manifest)
		return manifest, err
	}, func(manifest testVersionedManifest) string {
		return manifest.Version
	})
}

func writeVersionedReleaseManifestTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
