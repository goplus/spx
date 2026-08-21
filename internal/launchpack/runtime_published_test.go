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
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
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
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
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
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Store}
		header.SetMode(entry.Mode)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.Data); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
		manifestPin: func(lock release.RuntimeLock) (release.RuntimeManifestPin, error) {
			if lock.RuntimeVersion != f.lock.RuntimeVersion || lock.Manifest != f.lock.Manifest {
				return release.RuntimeManifestPin{}, errors.New("fixture lock mismatch")
			}
			return release.RuntimeManifestPin{
				Schema: 1, RuntimeVersion: lock.RuntimeVersion,
				Name: lock.Manifest, Size: int64(len(f.manifestData)), SHA256: digestBytes(f.manifestData),
			}, nil
		},
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
				return filepath.Join(cacheRoot, "release-manifests", digestBytes(fixture.manifestData)+"-"+fixture.lock.Manifest)
			},
		},
		{
			name: "Engine archive",
			path: func(t *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string {
				asset, ok := fixture.manifest.Asset(fixture.spec.ArchiveName)
				if !ok {
					t.Fatal("fixture manifest has no host Engine asset")
				}
				return filepath.Join(cacheRoot, "release-assets", fixture.manifest.LockSHA256, asset.SHA256+"-"+asset.Name)
			},
		},
		{
			name: "runtime PCK archive",
			path: func(t *testing.T, fixture publishedRuntimeFixture, cacheRoot string) string {
				asset, ok := fixture.manifest.Asset(release.RuntimeAssetZipName)
				if !ok {
					t.Fatal("fixture manifest has no runtime pack asset")
				}
				return filepath.Join(cacheRoot, "release-assets", fixture.manifest.LockSHA256, asset.SHA256+"-"+asset.Name)
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
	if _, err := os.Stat(filepath.Join(cacheRoot, "release-assets", fixture.manifest.LockSHA256, engineAsset.SHA256+"-"+engineAsset.Name)); !os.IsNotExist(err) {
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

func TestAcquireRuntimeAssetsRejectsManifestOutsidePin(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	forged := fixture.manifest
	forged.Assets = append([]release.RuntimeAsset(nil), forged.Assets...)
	forged.Assets[0].SHA256 = strings.Repeat("0", 64)
	data, err := forged.JSON()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	dependencies := fixture.dependencies(t.TempDir(), fixture.fetcher(map[string][]byte{fixture.lock.Manifest: data}, &calls))
	_, err = acquireRuntimeAssetsWith(context.Background(), publishedRuntimeConfig(t.TempDir()), IO{}, fixture.lock, dependencies)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("unpinned manifest error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("fetch count = %d, want only the rejected manifest", calls)
	}
}
