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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func TestAcquirePublishedDriverUsesOneCombinedBundle(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	calls := 0
	assets, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("acquire published driver: %v", err)
	}
	if calls != 2 {
		t.Fatalf("fetch count = %d, want manifest and one combined bundle", calls)
	}
	if got, err := os.ReadFile(assets.EnginePath); err != nil || !bytes.Equal(got, fixture.engine) {
		t.Fatalf("Engine = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(assets.PackPath); err != nil || !bytes.Equal(got, fixture.pack) {
		t.Fatalf("PCK = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(assets.BridgePath); err != nil || !bytes.Equal(got, fixture.bridge) {
		t.Fatalf("bridge = %q, err=%v", got, err)
	}
	if assets.Published == nil || assets.Published.ManifestSHA256 != digestBytes(fixture.manifestData) || assets.Published.BundleSHA256 != fixture.bundle.SHA256 || assets.Published.BundleName != fixture.bundle.Name {
		t.Fatalf("published provenance = %#v", assets)
	}
	assets.Cleanup()

	// A materialized cache hit is checked before the downloadable ZIP.
	bundleCache := filepath.Join(cacheRoot, "downloads", "driver", fixture.manifest.SPXVersion, fixture.bundle.Name)
	if err := os.Remove(bundleCache); err != nil {
		t.Fatal(err)
	}
	offlineCalls := 0
	offline, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{Env: []string{runtimeOfflineEnv + "=1"}}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &offlineCalls)))
	if err != nil {
		t.Fatalf("offline materialized cache hit: %v", err)
	}
	offline.Cleanup()
	if offlineCalls != 0 {
		t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
	}
}

func TestAcquirePublishedDriverUsesSPXReleaseAssetURLs(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	var urls []string
	calls := 0
	baseFetch := fixture.fetcher(nil, &calls)
	fetch := func(ctx context.Context, url string, dst io.Writer) error {
		urls = append(urls, url)
		return baseFetch(ctx, url, dst)
	}
	assets, err := acquirePublishedDriverWith(
		context.Background(),
		publishedDriverTestConfig(cacheRoot),
		IO{},
		fixture.lock,
		fixture.dependencies(cacheRoot, fetch),
	)
	if err != nil {
		t.Fatalf("acquire published driver: %v", err)
	}
	assets.Cleanup()

	want := map[string]bool{
		"https://github.com/goplus/spx/releases/download/v3.2.4/driver-manifest.json":   false,
		"https://github.com/goplus/spx/releases/download/v3.2.4/" + fixture.bundle.Name: false,
	}
	if len(urls) != len(want) {
		t.Fatalf("download URLs = %v, want %d URLs", urls, len(want))
	}
	for _, url := range urls {
		if strings.Contains(url, "/driver-v3.2.4/") {
			t.Fatalf("downloaded obsolete driver release URL: %s", url)
		}
		if _, ok := want[url]; !ok {
			t.Fatalf("unexpected driver download URL: %s", url)
		}
		if want[url] {
			t.Fatalf("downloaded driver asset twice: %s", url)
		}
		want[url] = true
	}
	for url, seen := range want {
		if !seen {
			t.Fatalf("did not download expected driver asset URL: %s", url)
		}
	}
}

func TestAcquirePublishedDriverUsesExplicitDriverMirrorOnly(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	assetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetDir, driverbundle.ManifestName), fixture.manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, fixture.bundle.Name), fixture.bundleData, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	cfg := publishedDriverTestConfig(t.TempDir())
	cfg.DriverAssetDir = assetDir
	assets, err := acquirePublishedDriverWith(context.Background(), cfg, IO{Env: []string{"SPX_RUNTIME_ASSET_DIR=/must-not-be-used"}}, fixture.lock, fixture.dependencies(cfg.RuntimeCacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("acquire from explicit driver mirror: %v", err)
	}
	assets.Cleanup()
	if calls != 0 {
		t.Fatalf("mirror fetch count = %d, want 0", calls)
	}
}

func TestAcquirePublishedDriverDoesNotFetchOversizedBundle(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	changed := false
	for i := range fixture.manifest.Bundles {
		bundle := &fixture.manifest.Bundles[i]
		if bundle.GOOS != fixture.bundle.GOOS || bundle.GOARCH != fixture.bundle.GOARCH {
			bundle.Size = runtimebundle.MaxArchiveBytes + 1
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("fixture has no non-host driver bundle")
	}
	var err error
	fixture.manifestData, err = fixture.manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	cacheRoot := t.TempDir()
	deps := fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls))
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, deps); err == nil || !strings.Contains(err.Error(), "archive limit") {
		t.Fatalf("oversized bundle error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("fetch count = %d, want manifest only", calls)
	}
}

func TestAcquirePublishedDriverCacheBindsOuterBundleDigest(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	firstCalls := 0
	first, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &firstCalls)))
	if err != nil {
		t.Fatal(err)
	}
	firstEnginePath := first.EnginePath
	first.Cleanup()
	if firstCalls != 2 {
		t.Fatalf("first fetch count = %d, want 2", firstCalls)
	}

	updated := fixture
	bridgeName, err := bridgeFileName(updated.bundle.GOOS, updated.bundle.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	updated.bundleData = writeRuntimeZip(t, filepath.Join(t.TempDir(), "reencoded.zip"),
		runtimeZipEntry{Name: updated.spec.PackName, Mode: 0o644, Data: updated.pack},
		runtimeZipEntry{Name: updated.spec.RuntimeName, Mode: 0o755, Data: updated.engine},
		runtimeZipEntry{Name: bridgeName, Mode: 0o755, Data: updated.bridge},
	)
	updated.bundle.Size = int64(len(updated.bundleData))
	updated.bundle.SHA256 = digestBytes(updated.bundleData)
	if updated.bundle.SHA256 == fixture.bundle.SHA256 {
		t.Fatal("re-encoded fixture unexpectedly kept the outer bundle digest")
	}
	updated.manifest.Bundles = append([]driverbundle.Bundle(nil), fixture.manifest.Bundles...)
	for i, candidate := range updated.manifest.Bundles {
		if candidate.GOOS == updated.bundle.GOOS && candidate.GOARCH == updated.bundle.GOARCH {
			updated.manifest.Bundles[i] = updated.bundle
			break
		}
	}
	updated.manifestData, err = updated.manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	mirror := t.TempDir()
	if err := os.WriteFile(filepath.Join(mirror, driverbundle.ManifestName), updated.manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mirror, updated.bundle.Name), updated.bundleData, 0o600); err != nil {
		t.Fatal(err)
	}
	secondCalls := 0
	cfg := publishedDriverTestConfig(cacheRoot)
	cfg.DriverAssetDir = mirror
	second, err := acquirePublishedDriverWith(context.Background(), cfg, IO{}, updated.lock, updated.dependencies(cacheRoot, updated.fetcher(nil, &secondCalls)))
	if err != nil {
		t.Fatalf("acquire updated published driver: %v", err)
	}
	defer second.Cleanup()
	if secondCalls != 0 {
		t.Fatalf("updated mirror fetch count = %d, want 0", secondCalls)
	}
	if second.EnginePath == firstEnginePath {
		t.Fatalf("updated outer digest reused materialized target %q", second.EnginePath)
	}
	if second.Published == nil || second.Published.BundleSHA256 != updated.bundle.SHA256 {
		t.Fatalf("updated published provenance = %#v", second.Published)
	}
}

func TestAcquirePublishedDriverRejectsManifestAndBundleMismatch(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	badManifestData := bytes.Replace(fixture.manifestData, []byte(fixture.bundle.EngineInterfaceDigest), []byte(strings.Repeat("0", 64)), 1)
	deps := fixture.dependencies(cacheRoot, fixture.fetcher(map[string][]byte{driverbundle.ManifestName: badManifestData}, new(int)))
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, deps); err == nil || !strings.Contains(err.Error(), "interface digest") {
		t.Fatalf("bundle interface mismatch error = %v", err)
	}

	badZip := append([]byte(nil), fixture.bundleData...)
	badZip[len(badZip)/2] ^= 1
	calls := 0
	deps = fixture.dependencies(t.TempDir(), fixture.fetcher(map[string][]byte{fixture.bundle.Name: badZip}, &calls))
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(deps.cacheRoot()), IO{}, fixture.lock, deps); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("outer bundle digest mismatch error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("bad bundle fetch count = %d, want manifest and bundle", calls)
	}
}

func TestAcquirePublishedDriverRejectsInvalidZIPContents(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	// The ZIP has a valid outer digest but does not satisfy the manifest entries.
	badPath := filepath.Join(t.TempDir(), "bad.zip")
	badData := writeRuntimeZip(t, badPath,
		runtimeZipEntry{Name: fixture.spec.RuntimeName, Mode: 0o755, Data: fixture.engine},
		runtimeZipEntry{Name: fixture.spec.PackName, Mode: 0o644, Data: fixture.pack},
	)
	fixture.bundle.Size = int64(len(badData))
	fixture.bundle.SHA256 = digestBytes(badData)
	calls := 0
	deps := fixture.dependencies(t.TempDir(), fixture.fetcher(map[string][]byte{fixture.bundle.Name: badData}, &calls))
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(deps.cacheRoot()), IO{}, fixture.lock, deps); err == nil {
		t.Fatal("published bundle with missing bridge was accepted")
	}
}

func TestAcquirePublishedDriverFetchErrorsStayFailClosed(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	calls := 0
	deps := fixture.dependencies(t.TempDir(), func(ctx context.Context, _ string, _ io.Writer) error {
		calls++
		if err := ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("network unavailable")
	})
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(deps.cacheRoot()), IO{}, fixture.lock, deps); err == nil {
		t.Fatal("published fetch failure was accepted")
	}
	if calls != 1 {
		t.Fatalf("fetch count = %d, want manifest only", calls)
	}
}
