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
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type publishedRuntimeFixture struct {
	lock         release.RuntimeLock
	spec         release.HostRuntimeSpec
	manifest     release.RuntimeManifest
	manifestData []byte
	assets       map[string][]byte
}

func newPublishedRuntimeFixture(t *testing.T) publishedRuntimeFixture {
	t.Helper()
	lock, spec := defaultHostRuntimeTest(t)
	root := t.TempDir()
	assets := make(map[string][]byte, len(lock.RequiredAssets))
	for _, name := range lock.RequiredAssets {
		var data []byte
		switch name {
		case spec.ArchiveName:
			data = writeRuntimeZip(t, filepath.Join(root, name), runtimeZipEntry{
				Name: spec.BinaryName, Mode: 0o755,
				Data: []byte("fixture-engine-" + runtime.GOOS + "-" + runtime.GOARCH),
			})
		case release.RuntimeAssetZipName:
			data = writeRuntimeZip(t, filepath.Join(root, name),
				runtimeZipEntry{Name: "gdspxrt.pck", Mode: 0o600, Data: []byte("fixture-pack")},
				runtimeZipEntry{Name: "runtime.gdextension", Mode: 0o600, Data: []byte("fixture-extension")},
			)
		default:
			data = []byte("fixture-" + name)
			if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		assets[name] = data
	}

	provenance := release.RuntimeProvenance{
		SPXCommit:               strings.Repeat("a", 40),
		GodotCommit:             lock.Godot.Commit,
		ModuleTree:              strings.Repeat("b", 40),
		RuntimePackSourceSHA256: strings.Repeat("c", 64),
		BuildRecipeSHA256:       strings.Repeat("d", 64),
		Toolchain:               lock.Toolchain,
	}
	inputs := make([]release.RuntimeAssetInput, 0, len(lock.RequiredAssets))
	for _, name := range lock.RequiredAssets {
		inputs = append(inputs, release.RuntimeAssetInput{Name: name, Path: filepath.Join(root, name)})
	}
	manifest, err := release.GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return publishedRuntimeFixture{lock: lock, spec: spec, manifest: manifest, manifestData: manifestData, assets: assets}
}

type runtimeZipEntry struct {
	Name string
	Mode os.FileMode
	Data []byte
}

func writeRuntimeZip(t *testing.T, filePath string, entries ...runtimeZipEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Store}
		header.SetMode(entry.Mode)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, data.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func (f publishedRuntimeFixture) fetcher(replacements map[string][]byte, calls *int) func(context.Context, string, io.Writer) error {
	return func(ctx context.Context, url string, dst io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		(*calls)++
		name := path.Base(url)
		data, ok := replacements[name]
		if !ok {
			if name == f.lock.Manifest {
				data = f.manifestData
			} else {
				data = f.assets[name]
			}
		}
		if len(data) == 0 {
			return fmt.Errorf("fixture has no release asset %q", name)
		}
		_, err := io.Copy(dst, bytes.NewReader(data))
		return err
	}
}

func (f publishedRuntimeFixture) dependencies(cacheRoot string, fetch func(context.Context, string, io.Writer) error) runtimeAssetDependencies {
	return runtimeAssetDependencies{
		fetch: fetch, cacheRoot: func() string { return cacheRoot },
	}
}

func publishedRuntimeConfig(cacheRoot string) Config {
	return Config{RuntimeCacheRoot: cacheRoot}
}

func TestAcquireRuntimeAssetsFromPublishedRelease(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	calls := 0
	assets, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: []string{}}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("acquire published runtime: %v", err)
	}
	defer assets.Cleanup()
	if calls != 3 {
		t.Fatalf("fetch count = %d, want manifest, Engine archive, and pack archive", calls)
	}
	if got, err := os.ReadFile(assets.EnginePath); err != nil || string(got) != "fixture-engine-"+runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatalf("materialized Engine = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(assets.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("materialized pack = %q, err=%v", got, err)
	}
}

func TestAcquireRuntimeAssetsDerivesURLsFromSelectedRuntimeVersion(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	fixture.manifest.ReleaseRepository = "example/runtime"
	fixture.manifest.LockSHA256 = strings.Repeat("0", 64)
	var err error
	fixture.manifestData, err = fixture.manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}

	cacheRoot := t.TempDir()
	var urls []string
	calls := 0
	fetchFixture := fixture.fetcher(nil, &calls)
	fetch := func(ctx context.Context, url string, dst io.Writer) error {
		urls = append(urls, url)
		return fetchFixture(ctx, url, dst)
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: []string{}}, fixture.lock, fixture.dependencies(cacheRoot, fetch))
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()

	want := []string{
		fixture.lock.RuntimeAssetDownloadURL(fixture.lock.Manifest),
		fixture.lock.RuntimeAssetDownloadURL(fixture.spec.ArchiveName),
		fixture.lock.RuntimeAssetDownloadURL(release.RuntimeAssetZipName),
	}
	if !slices.Equal(urls, want) {
		t.Fatalf("runtime release URLs = %v, want %v", urls, want)
	}
}

func TestAcquireRuntimeAssetsFromLocalReleaseDirectory(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	assetDir := t.TempDir()
	for name, data := range fixture.assets {
		if err := os.WriteFile(filepath.Join(assetDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(assetDir, fixture.lock.Manifest), fixture.manifestData, 0o600); err != nil {
		t.Fatal(err)
	}

	cacheRoot := t.TempDir()
	calls := 0
	cfg := Config{RuntimeAssetDir: assetDir, RuntimeCacheRoot: cacheRoot, RuntimeOffline: true}
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{}}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("acquire local release assets: %v", err)
	}
	defer assets.Cleanup()
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0", calls)
	}
	if got, err := os.ReadFile(assets.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("materialized pack = %q, err=%v", got, err)
	}
}

func TestAcquireRuntimeAssetsOfflineRejectsCorruptCachedReleaseData(t *testing.T) {
	tests := []struct {
		name string
		path func(t *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string
	}{
		{
			name: "manifest",
			path: func(_ *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string {
				return filepath.Join(cacheRoot, "release-manifests", "runtime", fixture.lock.RuntimeVersion, fixture.lock.Manifest)
			},
		},
		{
			name: "Engine archive",
			path: func(t *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string {
				asset, ok := fixture.manifest.Asset(fixture.spec.ArchiveName)
				if !ok {
					t.Fatal("fixture manifest has no host Engine asset")
				}
				return filepath.Join(cacheRoot, "release-assets", "runtime", fixture.lock.RuntimeVersion, asset.SHA256+"-"+asset.Name)
			},
		},
		{
			name: "runtime PCK archive",
			path: func(t *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string {
				asset, ok := fixture.manifest.Asset(release.RuntimeAssetZipName)
				if !ok {
					t.Fatal("fixture manifest has no runtime pack asset")
				}
				return filepath.Join(cacheRoot, "release-assets", "runtime", fixture.lock.RuntimeVersion, asset.SHA256+"-"+asset.Name)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPublishedRuntimeFixture(t)
			cacheRoot := t.TempDir()
			firstCalls := 0
			first, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: []string{}}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &firstCalls)))
			if err != nil {
				t.Fatalf("initial acquisition: %v", err)
			}
			first.Cleanup()
			if firstCalls != 3 {
				t.Fatalf("initial fetch count = %d, want 3", firstCalls)
			}
			if err := os.WriteFile(test.path(t, fixture, cacheRoot), []byte("corrupt cached release data"), 0o600); err != nil {
				t.Fatal(err)
			}

			offlineCalls := 0
			offlineFetch := func(context.Context, string, io.Writer) error {
				offlineCalls++
				return errors.New("offline cache failure must not fetch")
			}
			env := []string{runtimeOfflineEnv + "=1"}
			if _, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: env}, fixture.lock, fixture.dependencies(cacheRoot, offlineFetch)); err == nil {
				t.Fatal("offline acquisition accepted corrupt cached release data")
			}
			if offlineCalls != 0 {
				t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
			}
		})
	}
}

func TestAcquireRuntimeAssetsOfflineRepairsCorruptMaterializedBundle(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	firstCalls := 0
	first, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: []string{}}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &firstCalls)))
	if err != nil {
		t.Fatalf("initial acquisition: %v", err)
	}
	packPath := first.PackPath
	first.Cleanup()
	if err := os.WriteFile(packPath, []byte("corrupt materialized runtime PCK"), 0o600); err != nil {
		t.Fatal(err)
	}

	offlineCalls := 0
	offlineFetch := func(context.Context, string, io.Writer) error {
		offlineCalls++
		return errors.New("offline repair must not fetch")
	}
	repaired, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: []string{runtimeOfflineEnv + "=1"}}, fixture.lock, fixture.dependencies(cacheRoot, offlineFetch))
	if err != nil {
		t.Fatalf("offline repair: %v", err)
	}
	defer repaired.Cleanup()
	if got, err := os.ReadFile(repaired.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("repaired runtime PCK = %q, err=%v", got, err)
	}
	if offlineCalls != 0 {
		t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
	}
}

func TestAcquireRuntimeAssetsRejectsOuterDigestMismatchAndRetries(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	env := []string{}
	badArchive := append([]byte(nil), fixture.assets[fixture.spec.ArchiveName]...)
	badArchive[len(badArchive)/2] ^= 0x01
	calls := 0

	if _, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: env}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(map[string][]byte{fixture.spec.ArchiveName: badArchive}, &calls))); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("digest-mismatched Engine archive error = %v", err)
	}
	engineAsset, ok := fixture.manifest.Asset(fixture.spec.ArchiveName)
	if !ok {
		t.Fatal("fixture manifest has no host Engine asset")
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "release-assets", "runtime", fixture.lock.RuntimeVersion, engineAsset.SHA256+"-"+engineAsset.Name)); !os.IsNotExist(err) {
		t.Fatalf("digest-mismatched archive was published, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "engine")); !os.IsNotExist(err) {
		t.Fatalf("Engine bundle was materialized after digest mismatch, stat error = %v", err)
	}

	assets, err := acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(cacheRoot), IO{Env: env}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("retry after rejected archive: %v", err)
	}
	defer assets.Cleanup()
	if calls != 4 {
		t.Fatalf("fetch count after retry = %d, want 4", calls)
	}
}

func TestAcquireRuntimeAssetsRejectsAssetOutsideVersionedManifest(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	forged := fixture.manifest
	forged.Assets = append([]release.RuntimeAsset(nil), forged.Assets...)
	for i := range forged.Assets {
		if forged.Assets[i].Name == fixture.spec.ArchiveName {
			forged.Assets[i].SHA256 = strings.Repeat("0", 64)
		}
	}
	data, err := forged.JSON()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	dependencies := fixture.dependencies(t.TempDir(), fixture.fetcher(map[string][]byte{fixture.lock.Manifest: data}, &calls))
	_, err = acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(t.TempDir()), IO{}, fixture.lock, dependencies)
	if !errors.Is(err, runtimebundle.ErrDigestMismatch) {
		t.Fatalf("versioned manifest asset error = %v, want ErrDigestMismatch", err)
	}
	if calls != 2 {
		t.Fatalf("fetch count = %d, want manifest and rejected asset", calls)
	}
}

func TestMaterializeRuntimeRejectsInvalidArchiveHandles(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	acquire := func(t *testing.T, data []byte) *runtimebundle.AcquiredFile {
		t.Helper()
		f, err := runtimebundle.AcquireFile(context.Background(), t.TempDir(), runtimebundle.FetchSpec{Name: "runtime.zip", URL: "https://example.invalid/runtime.zip", Size: int64(len(data)), SHA256: digestBytes(data), Fetch: func(_ context.Context, _ string, w io.Writer) error { _, err := w.Write(data); return err }})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	for _, tt := range []struct{ name, want string }{
		{"incomplete", "incomplete published runtime archive handles"},
		{"unavailable temporary storage", "create runtime extraction directory"},
		{"closed engine", "stat acquired Engine archive"},
		{"closed pack", "stat acquired runtime pack archive"},
		{"corrupt engine", "verify Engine archive"},
		{"corrupt pack", "verify runtime pack archive"},
		{"missing engine entry", "unavailable at"},
		{"missing pack entry", "unavailable at"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			engineData, packData := fixture.assets[fixture.spec.ArchiveName], fixture.assets[release.RuntimeAssetZipName]
			if tt.name == "corrupt engine" {
				engineData = []byte("not a zip")
			}
			if tt.name == "corrupt pack" {
				packData = []byte("not a zip")
			}
			if tt.name == "missing engine entry" {
				engineData = writeRuntimeZip(t, filepath.Join(t.TempDir(), "engine.zip"), runtimeZipEntry{Name: "other", Mode: 0600, Data: []byte("other")})
			}
			if tt.name == "missing pack entry" {
				packData = writeRuntimeZip(t, filepath.Join(t.TempDir(), "pack.zip"), runtimeZipEntry{Name: "other", Mode: 0600, Data: []byte("other")})
			}
			handles := &publishedRuntimeArchives{engine: acquire(t, engineData), pack: acquire(t, packData)}
			switch tt.name {
			case "incomplete":
				handles.engine = nil
			case "closed engine":
				if err := handles.engine.Close(); err != nil {
					t.Fatal(err)
				}
			case "closed pack":
				if err := handles.pack.Close(); err != nil {
					t.Fatal(err)
				}
			}
			temp, cache := t.TempDir(), t.TempDir()
			storage := temp
			if tt.name == "unavailable temporary storage" {
				storage = filepath.Join(temp, "missing")
			}
			for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
				t.Setenv(key, storage)
			}
			_, err := materializeRuntimeBundle(context.Background(), cache, fixture.lock, fixture.spec, runtimeMaterializationInput{archives: handles})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("materialization = %v, want %q", err, tt.want)
			}
			entries, err := os.ReadDir(temp)
			if err != nil || len(entries) != 0 {
				t.Fatalf("extraction cleanup = %v, %v", entries, err)
			}
		})
	}
}

func TestMaterializeRuntimeRejectsInvalidLocalInputs(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	for _, tt := range []struct{ name, want string }{
		{"missing engine", "hash Engine"}, {"missing pack", "hash runtime PCK"}, {"unavailable temporary storage", "create Engine bundle directory"}, {"invalid lock", "runtime"}, {"invalid entry name", "invalid"}, {"blocked cache", "materialize verified Engine bundle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := writeInstalledRuntimeTest(t, spec, "engine", "pack")
			input := runtimeMaterializationInput{enginePath: filepath.Join(root, spec.RuntimeName), packPath: filepath.Join(root, spec.PackName), engineSize: 6, engineSHA256: digestBytes([]byte("engine")), packSize: 4, packSHA256: digestBytes([]byte("pack"))}
			testLock, testSpec := lock, spec
			cache := filepath.Join(t.TempDir(), "cache")
			switch tt.name {
			case "missing engine":
				input.enginePath += ".missing"
			case "missing pack":
				input.packPath += ".missing"
			case "invalid lock":
				testLock.RuntimeVersion = "invalid version"
			case "invalid entry name":
				testSpec.RuntimeName = "../engine"
			case "blocked cache":
				writeProjectTestFile(t, cache, "blocked")
			case "unavailable temporary storage":
				for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
					t.Setenv(key, filepath.Join(root, "missing"))
				}
			}
			if _, err := materializeRuntimeBundle(context.Background(), cache, testLock, testSpec, input); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("materialization = %v, want %q", err, tt.want)
			}
		})
	}
	for _, without := range []string{spec.ArchiveName, release.RuntimeAssetZipName} {
		fixture := newPublishedRuntimeFixture(t)
		manifest := fixture.manifest
		manifest.Assets = slices.DeleteFunc(manifest.Assets, func(a release.RuntimeAsset) bool { return a.Name == without })
		if _, err := materializePublishedRuntime(context.Background(), t.TempDir(), lock, spec, runtimeAssetSource{manifest: manifest}, true); err == nil || !strings.Contains(err.Error(), "manifest has no") {
			t.Fatalf("missing %s = %v", without, err)
		}
	}
}
