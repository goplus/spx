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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

type runtimeReleaseFixture struct {
	lock         release.RuntimeLock
	spec         release.HostRuntimeSpec
	manifest     release.RuntimeManifest
	manifestData []byte
	assets       map[string][]byte
}

func newRuntimeReleaseFixture(t *testing.T) runtimeReleaseFixture {
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
				Name: spec.BinaryName, Mode: 0o755, Data: []byte("fixture-engine-" + runtime.GOOS + "-" + runtime.GOARCH),
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
	return runtimeReleaseFixture{
		lock: lock, spec: spec, manifest: manifest, manifestData: manifestData, assets: assets,
	}
}

type runtimeZipEntry struct {
	Name string
	Mode os.FileMode
	Data []byte
}

func writeRuntimeZip(t *testing.T, path string, entries ...runtimeZipEntry) []byte {
	t.Helper()
	file, err := os.Create(path)
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
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (f runtimeReleaseFixture) fetcher(replacements map[string][]byte, calls *int) func(context.Context, string, io.Writer) error {
	return func(ctx context.Context, url string, dst io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		*calls++
		name := path.Base(url)
		var data []byte
		if replacement, ok := replacements[name]; ok {
			data = replacement
		} else if name == f.lock.Manifest {
			data = f.manifestData
		} else {
			data = f.assets[name]
		}
		if len(data) == 0 {
			return fmt.Errorf("fixture has no release asset %q", name)
		}
		_, err := io.Copy(dst, bytes.NewReader(data))
		return err
	}
}

func (f runtimeReleaseFixture) dependencies(fetch func(context.Context, string, io.Writer) error) runtimeAssetDependencies {
	return runtimeAssetDependencies{
		fetch:     fetch,
		cacheRoot: func() string { return filepath.Join(os.TempDir(), "spx-runtime-test-unused") },
		manifestPin: func(lock release.RuntimeLock) (release.RuntimeManifestPin, error) {
			if lock.RuntimeVersion != f.lock.RuntimeVersion || lock.Manifest != f.lock.Manifest {
				return release.RuntimeManifestPin{}, fmt.Errorf("fixture lock mismatch")
			}
			return release.RuntimeManifestPin{
				Schema: release.RuntimeManifestPinSchema, RuntimeVersion: lock.RuntimeVersion,
				Name: lock.Manifest, Size: int64(len(f.manifestData)), SHA256: digestBytes(f.manifestData),
			}, nil
		},
	}
}

func runtimeAssetTestDependencies(fetch func(context.Context, string, io.Writer) error) runtimeAssetDependencies {
	dependencies := defaultRuntimeAssetDependencies()
	dependencies.fetch = fetch
	return dependencies
}

func runtimeAssetTestConfig(sourceDir string) Config {
	return Config{DriverOrigin: ModuleOrigin{
		Selected: ModuleRef{Path: "github.com/goplus/spx/v3"},
		Replace:  &ModuleRef{Path: "github.com/goplus/spx/v3", Dir: sourceDir},
	}}
}

func runtimeAssetTestEnv(cacheRoot, gopath string, extra ...string) []string {
	env := []string{"GOPATH=" + gopath, runtimeCacheEnv + "=" + cacheRoot}
	return append(env, extra...)
}

func TestAcquireRuntimeAssetsFromPublishedReleaseWithoutGOPATHInstall(t *testing.T) {
	fixture := newRuntimeReleaseFixture(t)
	cacheRoot := t.TempDir()
	gopath := t.TempDir()
	gopathBin := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(gopathBin, 0o700); err != nil {
		t.Fatal(err)
	}
	// These files deliberately look like the legacy source-mode assets. The
	// release driver must ignore them, even when their names match the
	// locked runtime version.
	if err := os.WriteFile(filepath.Join(gopathBin, fixture.spec.RuntimeName), []byte("old-or-replaced-engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gopathBin, fixture.spec.PackName), []byte("old-or-replaced-pack"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0

	assets, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{
		Env: runtimeAssetTestEnv(cacheRoot, gopath),
	}, fixture.lock, fixture.dependencies(fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("acquire published runtime: %v", err)
	}
	defer assets.Cleanup()
	if calls != 3 {
		t.Fatalf("fetch count = %d, want manifest, Engine archive, and pack archive", calls)
	}
	if strings.HasPrefix(filepath.Clean(assets.EnginePath), filepath.Clean(gopath)+string(filepath.Separator)) {
		t.Fatalf("Engine was sourced from GOPATH: %s", assets.EnginePath)
	}
	if strings.HasPrefix(filepath.Clean(assets.PackPath), filepath.Clean(gopath)+string(filepath.Separator)) {
		t.Fatalf("runtime PCK was sourced from GOPATH: %s", assets.PackPath)
	}
	if got, err := os.ReadFile(assets.EnginePath); err != nil || string(got) != "fixture-engine-"+runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatalf("materialized Engine = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(assets.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("materialized pack = %q, err=%v", got, err)
	}
}

func TestResolveLocalAssetsSourceModeIgnoresLegacyGOPATHAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a source bridge fixture")
	}
	fixture := newRuntimeReleaseFixture(t)
	moduleRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	modulePath := "example.test/spx"
	mustWriteDriverTestFile(t, filepath.Join(moduleRoot, "go.mod"), "module "+modulePath+"\n\ngo 1.25\n", 0o600)
	mustWriteDriverTestFile(t, filepath.Join(moduleRoot, "cmd", "ispxnative", "main.go"), "package main\nfunc main() {}\n", 0o600)
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	goCommand, err = filepath.Abs(goCommand)
	if err != nil {
		t.Fatal(err)
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	gopath := t.TempDir()
	gopathBin := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(gopathBin, 0o700); err != nil {
		t.Fatal(err)
	}
	spec := fixture.spec
	if err := os.WriteFile(filepath.Join(gopathBin, spec.RuntimeName), []byte("legacy source-mode Engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gopathBin, spec.PackName), []byte("legacy source-mode PCK"), 0o600); err != nil {
		t.Fatal(err)
	}

	cacheRoot := t.TempDir()
	calls := 0
	var stderr bytes.Buffer
	assets, err := resolveLocalAssetsWith(context.Background(), Config{
		DriverOrigin: ModuleOrigin{
			Main:     true,
			Selected: ModuleRef{Path: modulePath, Dir: moduleRoot, GoMod: filepath.Join(moduleRoot, "go.mod")},
		},
		GoCommand:    goCommand,
		GraphWorkDir: moduleRoot,
		GoWork:       "off",
	}, IO{Env: append(runtimeAssetTestEnv(cacheRoot, gopath), "GOCACHE="+t.TempDir(), "PATH="+os.Getenv("PATH")), Stderr: &stderr}, fixture.dependencies(fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("resolve source-mode runtime assets: %v\n%s", err, stderr.String())
	}
	defer assets.Cleanup()
	if calls != 3 {
		t.Fatalf("fetch count = %d, want manifest, Engine archive, and pack archive", calls)
	}
	for label, assetPath := range map[string]string{"Engine": assets.EnginePath, "runtime PCK": assets.PackPath} {
		if strings.HasPrefix(filepath.Clean(assetPath), filepath.Clean(gopath)+string(filepath.Separator)) {
			t.Fatalf("%s was sourced from GOPATH: %s", label, assetPath)
		}
	}
	if got, err := os.ReadFile(assets.EnginePath); err != nil || string(got) != "fixture-engine-"+runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatalf("materialized Engine = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(assets.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("materialized pack = %q, err=%v", got, err)
	}
}

func TestAcquireRuntimeAssetsOfflineRejectsCorruptCachedReleaseMetadata(t *testing.T) {
	tests := []struct {
		name string
		path func(t *testing.T, fixture runtimeReleaseFixture, cacheRoot string) string
	}{
		{
			name: "manifest",
			path: func(t *testing.T, fixture runtimeReleaseFixture, cacheRoot string) string {
				return filepath.Join(cacheRoot, "release-manifests", digestBytes(fixture.manifestData)+"-"+fixture.lock.Manifest)
			},
		},
		{
			name: "Engine archive",
			path: func(t *testing.T, fixture runtimeReleaseFixture, cacheRoot string) string {
				asset, ok := fixture.manifest.Asset(fixture.spec.ArchiveName)
				if !ok {
					t.Fatal("fixture manifest has no host Engine asset")
				}
				return filepath.Join(cacheRoot, "release-assets", fixture.manifest.LockSHA256, asset.SHA256+"-"+asset.Name)
			},
		},
		{
			name: "runtime PCK archive",
			path: func(t *testing.T, fixture runtimeReleaseFixture, cacheRoot string) string {
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
			fixture := newRuntimeReleaseFixture(t)
			cacheRoot := t.TempDir()
			env := runtimeAssetTestEnv(cacheRoot, t.TempDir())
			firstCalls := 0

			first, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, fixture.lock, fixture.dependencies(fixture.fetcher(nil, &firstCalls)))
			if err != nil {
				t.Fatalf("initial published runtime acquisition: %v", err)
			}
			first.Cleanup()
			if firstCalls != 3 {
				t.Fatalf("initial fetch count = %d, want 3", firstCalls)
			}
			corruptPath := test.path(t, fixture, cacheRoot)
			if err := os.WriteFile(corruptPath, []byte("corrupt cached release data"), 0o600); err != nil {
				t.Fatal(err)
			}

			gopath := t.TempDir()
			if err := os.MkdirAll(filepath.Join(gopath, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(gopath, "bin", fixture.spec.RuntimeName), []byte("untrusted fallback Engine"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(gopath, "bin", fixture.spec.PackName), []byte("untrusted fallback PCK"), 0o600); err != nil {
				t.Fatal(err)
			}

			offlineCalls := 0
			offlineFetch := func(context.Context, string, io.Writer) error {
				offlineCalls++
				return errors.New("offline cache failure must not fetch")
			}
			offlineEnv := append(append([]string(nil), env...), runtimeOfflineEnv+"=1", "GOPATH="+gopath)
			if _, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: offlineEnv}, fixture.lock, fixture.dependencies(offlineFetch)); err == nil {
				t.Fatal("offline acquisition accepted corrupt cached release data")
			}
			if offlineCalls != 0 {
				t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
			}
		})
	}
}

func TestAcquireRuntimeAssetsOfflineUsesVerifiedCacheWithoutFetch(t *testing.T) {
	fixture := newRuntimeReleaseFixture(t)
	cacheRoot := t.TempDir()
	env := runtimeAssetTestEnv(cacheRoot, t.TempDir())
	firstCalls := 0

	first, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, fixture.lock, fixture.dependencies(fixture.fetcher(nil, &firstCalls)))
	if err != nil {
		t.Fatalf("initial published runtime acquisition: %v", err)
	}
	if firstCalls != 3 {
		t.Fatalf("initial fetch count = %d, want 3", firstCalls)
	}
	first.Cleanup()

	offlineCalls := 0
	offlineFetch := func(context.Context, string, io.Writer) error {
		offlineCalls++
		return errors.New("offline cache hit unexpectedly fetched")
	}
	offlineEnv := append(append([]string(nil), env...), runtimeOfflineEnv+"=1")
	second, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: offlineEnv}, fixture.lock, fixture.dependencies(offlineFetch))
	if err != nil {
		t.Fatalf("offline cached runtime acquisition: %v", err)
	}
	defer second.Cleanup()
	if offlineCalls != 0 {
		t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
	}
}

func TestAcquireRuntimeAssetsOfflineRepairsCorruptMaterializedBundleFromVerifiedArchives(t *testing.T) {
	fixture := newRuntimeReleaseFixture(t)
	cacheRoot := t.TempDir()
	env := runtimeAssetTestEnv(cacheRoot, t.TempDir())
	firstCalls := 0

	first, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, fixture.lock, fixture.dependencies(fixture.fetcher(nil, &firstCalls)))
	if err != nil {
		t.Fatalf("initial published runtime acquisition: %v", err)
	}
	enginePath, packPath := first.EnginePath, first.PackPath
	first.Cleanup()
	if err := os.WriteFile(packPath, []byte("corrupt materialized runtime PCK"), 0o600); err != nil {
		t.Fatal(err)
	}

	offlineCalls := 0
	offlineFetch := func(context.Context, string, io.Writer) error {
		offlineCalls++
		return errors.New("offline materialized bundle failure must not fetch")
	}
	offlineEnv := append(append([]string(nil), env...), runtimeOfflineEnv+"=1")
	repaired, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: offlineEnv}, fixture.lock, fixture.dependencies(offlineFetch))
	if err != nil {
		t.Fatalf("offline repair from verified archives: %v; Engine=%s Pack=%s", err, enginePath, packPath)
	}
	defer repaired.Cleanup()
	if got, err := os.ReadFile(repaired.PackPath); err != nil || string(got) != "fixture-pack" {
		t.Fatalf("repaired runtime PCK = %q, err=%v", got, err)
	}
	if offlineCalls != 0 {
		t.Fatalf("offline fetch count = %d, want 0", offlineCalls)
	}
}

func TestAcquireRuntimeAssetsRejectsOuterDigestMismatchAndRetriesCleanly(t *testing.T) {
	fixture := newRuntimeReleaseFixture(t)
	cacheRoot := t.TempDir()
	env := runtimeAssetTestEnv(cacheRoot, t.TempDir())
	badArchive := append([]byte(nil), fixture.assets[fixture.spec.ArchiveName]...)
	badArchive[len(badArchive)/2] ^= 0x01
	calls := 0

	_, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, fixture.lock, fixture.dependencies(fixture.fetcher(map[string][]byte{fixture.spec.ArchiveName: badArchive}, &calls)))
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("digest-mismatched Engine archive error = %v", err)
	}
	engineAsset, ok := fixture.manifest.Asset(fixture.spec.ArchiveName)
	if !ok {
		t.Fatal("fixture manifest has no host Engine asset")
	}
	if _, statErr := os.Stat(filepath.Join(cacheRoot, "release-assets", fixture.manifest.LockSHA256, engineAsset.SHA256+"-"+engineAsset.Name)); !os.IsNotExist(statErr) {
		t.Fatalf("digest-mismatched archive was published, stat error = %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cacheRoot, "engine")); !os.IsNotExist(statErr) {
		t.Fatalf("Engine bundle was materialized after digest mismatch, stat error = %v", statErr)
	}

	assets, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, fixture.lock, fixture.dependencies(fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatalf("retry after rejected archive: %v", err)
	}
	defer assets.Cleanup()
	if got, want := calls, 4; got != want {
		t.Fatalf("fetch count after retry = %d, want %d", got, want)
	}
}

func TestAcquireRuntimeAssetsRejectsStructurallyValidManifestOutsidePin(t *testing.T) {
	fixture := newRuntimeReleaseFixture(t)
	forged := fixture.manifest
	forged.Assets = append([]release.RuntimeAsset(nil), forged.Assets...)
	forged.Assets[0].SHA256 = strings.Repeat("0", 64)
	forgedData, err := forged.JSON()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	dependencies := fixture.dependencies(fixture.fetcher(map[string][]byte{fixture.lock.Manifest: forgedData}, &calls))
	_, err = acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{
		Env: runtimeAssetTestEnv(t.TempDir(), t.TempDir()),
	}, fixture.lock, dependencies)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("unpinned but structurally valid manifest error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("fetch count = %d, want only rejected manifest", calls)
	}
}

func TestAcquireRuntimeAssetsRejectsExplicitBadLocalManifestWithoutFetch(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "engine-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema":"spx-local-engine/v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("bad local manifest must not fetch")
	}

	_, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: runtimeAssetTestEnv(t.TempDir(), t.TempDir(), runtimeLocalManifestEnv+"="+manifestPath)}, release.DefaultRuntimeLock(), runtimeAssetTestDependencies(fetch))
	if err == nil {
		t.Fatal("invalid explicit local manifest was accepted")
	}
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0 for invalid local manifest", calls)
	}
}

func TestAcquireRuntimeAssetsAcceptsAndThenRejectsTamperedLocalManifestFiles(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("local-engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("local-pack"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := release.NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "engine-manifest.json")
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}

	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("local manifest path must not fetch")
	}
	env := runtimeAssetTestEnv(t.TempDir(), t.TempDir(), runtimeLocalManifestEnv+"="+manifestPath)
	assets, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, lock, runtimeAssetTestDependencies(fetch))
	if err != nil {
		t.Fatalf("valid local runtime acquisition: %v", err)
	}
	assets.Cleanup()
	if calls != 0 {
		t.Fatalf("local manifest fetch count = %d, want 0", calls)
	}

	if err := os.WriteFile(filepath.Join(root, manifest.Engine.Name), []byte("tampered-engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, lock, runtimeAssetTestDependencies(fetch)); err == nil {
		t.Fatal("tampered local runtime was accepted")
	}
	if calls != 0 {
		t.Fatalf("fetch count after tampered local runtime = %d, want 0", calls)
	}

	if err := os.WriteFile(filepath.Join(root, manifest.Engine.Name), []byte("local-engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, manifest.Pack.Name), []byte("tampered-pack"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(t.TempDir()), IO{Env: env}, lock, runtimeAssetTestDependencies(fetch)); err == nil {
		t.Fatal("tampered local runtime PCK was accepted")
	}
	if calls != 0 {
		t.Fatalf("fetch count after tampered local runtime PCK = %d, want 0", calls)
	}
}

func TestAcquireRuntimeAssetsAutoDiscoveredLocalManifestFailsClosedOnIdentityMismatch(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	buildDir := t.TempDir()
	enginePath := filepath.Join(buildDir, spec.RuntimeName)
	packPath := filepath.Join(buildDir, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("auto-local-engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("auto-local-pack"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := release.NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := release.LocalRuntimeManifestPath(sourceDir, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}

	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("auto-discovered local manifest must not fetch")
	}
	env := runtimeAssetTestEnv(t.TempDir(), t.TempDir())
	assets, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(sourceDir), IO{Env: env}, lock, runtimeAssetTestDependencies(fetch))
	if err != nil {
		t.Fatalf("valid auto-discovered local manifest: %v", err)
	}
	assets.Cleanup()

	manifest.LockSHA256 = strings.Repeat("0", 64)
	if err := release.WriteLocalRuntimeManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), runtimeAssetTestConfig(sourceDir), IO{Env: env}, lock, runtimeAssetTestDependencies(fetch)); err == nil {
		t.Fatal("identity-mismatched auto-discovered local manifest fell back to another source")
	}
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0", calls)
	}
}

func TestRuntimeAssetHostNames(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	tests := []struct {
		goos, goarch, archive, binary string
	}{
		{"linux", "amd64", "linux-x86_64.zip", "godot.linuxbsd.template_release.x86_64"},
		{"darwin", "amd64", "macos-x86_64.zip", "godot.macos.template_release.x86_64"},
		{"darwin", "arm64", "macos-arm64.zip", "godot.macos.template_release.arm64"},
		{"windows", "amd64", "windows-x86_64.zip", "godot.windows.template_release.x86_64.exe"},
	}
	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			spec, err := release.HostRuntimeSpecFor(lock, test.goos, test.goarch)
			if err != nil {
				t.Fatal(err)
			}
			runtimeName := release.RuntimeTag + lock.RuntimeVersion
			if test.goos == "windows" {
				runtimeName += ".exe"
			}
			if spec.ArchiveName != test.archive || spec.BinaryName != test.binary || spec.RuntimeName != runtimeName || spec.PackName != release.RuntimeTag+lock.RuntimeVersion+".pck" {
				t.Fatalf("host runtime spec = %#v", spec)
			}
		})
	}
}

func TestRuntimeAssetFileHelpersRequireRegularNonSymlinkFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "runtime-asset")
	data := []byte("runtime asset contents")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	size, digest, err := hashRuntimeFile(path)
	if err != nil {
		t.Fatalf("hash regular runtime file: %v", err)
	}
	if size != int64(len(data)) || digest != digestBytes(data) {
		t.Fatalf("hashed runtime file = (%d, %s), want (%d, %s)", size, digest, len(data), digestBytes(data))
	}
	read, err := readRegularFile(path)
	if err != nil {
		t.Fatalf("read regular runtime file: %v", err)
	}
	if !bytes.Equal(read, data) {
		t.Fatalf("read runtime file = %q, want %q", read, data)
	}

	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := hashRuntimeFile(directory); err == nil {
		t.Fatal("hashing a directory succeeded")
	}
	if _, err := readRegularFile(directory); err == nil {
		t.Fatal("reading a directory succeeded")
	}

	symlink := filepath.Join(root, "runtime-asset-link")
	if err := os.Symlink(path, symlink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := hashRuntimeFile(symlink); err == nil {
		t.Fatal("hashing a symlink succeeded")
	}
	if _, err := readRegularFile(symlink); err == nil {
		t.Fatal("reading a symlink succeeded")
	}
}
