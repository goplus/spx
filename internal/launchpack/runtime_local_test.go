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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/runtimebundle"

	"github.com/goplus/spx/v4/internal/release"
)

func TestAcquireRuntimeAssetsPrefersExplicitLocalManifest(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	explicitRoot := filepath.Join(root, "explicit")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(explicitRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceManifest, err := release.LocalRuntimeManifestPath(sourceRoot, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	publishLocalRuntimeTest(t, sourceRoot, sourceManifest, spec, "source-engine", "source-pack")
	explicitManifest := publishLocalRuntimeTest(t, explicitRoot, filepath.Join(explicitRoot, "engine-manifest.json"), spec, "explicit-engine", "explicit-pack")

	cfg := Config{
		RuntimeSourceRoot: sourceRoot, RuntimeManifestPath: explicitManifest,
		RuntimeCacheRoot: filepath.Join(root, "cache"),
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{"SPX_RUNTIME_OFFLINE=1"}}, lock, runtimeAssetDependencies{
		fetch:     func(context.Context, string, io.Writer) error { return errors.New("network must not be used") },
		cacheRoot: func() string { return cfg.RuntimeCacheRoot },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	engine, err := os.ReadFile(assets.EnginePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(engine) != "explicit-engine" {
		t.Fatalf("engine = %q, want explicit manifest asset", engine)
	}

	cfg.RuntimeManifestPath = ""
	cfg.Source.SourceMode = true
	assets, err = acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{"SPX_RUNTIME_OFFLINE=1"}}, lock, runtimeAssetDependencies{
		fetch:     func(context.Context, string, io.Writer) error { return errors.New("network must not be used") },
		cacheRoot: func() string { return filepath.Join(root, "cache-source") },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	engine, err = os.ReadFile(assets.EnginePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(engine) != "source-engine" {
		t.Fatalf("engine = %q, want source-root manifest asset", engine)
	}
}

func TestRuntimeEnvironmentConfigOverridesProcess(t *testing.T) {
	t.Setenv(runtimeCacheEnv, "/process-cache")
	env := runtimeEnvironment(Config{RuntimeCacheRoot: "/config-cache"}, nil)
	value, found, duplicate := environmentValue(env, runtimeCacheEnv)
	if !found || duplicate || value != "/config-cache" {
		t.Fatalf("%s = %q, found %v, duplicate %v", runtimeCacheEnv, value, found, duplicate)
	}
}

func TestAcquireRuntimeAssetsRejectsInvalidExplicitManifestWithoutFetch(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "engine-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema":"spx-local-engine/v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("invalid local manifest must not fetch")
	}
	cacheRoot := t.TempDir()
	dependencies := localRuntimeTestDependencies(cacheRoot, fetch)
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		calls++
		return t.TempDir(), nil
	}
	cfg := Config{RuntimeCacheRoot: cacheRoot, RuntimeManifestPath: manifestPath, Source: SourceIdentity{SourceMode: true}}
	_, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{runtimeCacheEnv + "=" + cacheRoot}}, release.DefaultRuntimeLock(), dependencies)
	if err == nil {
		t.Fatal("invalid explicit local manifest was accepted")
	}
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0", calls)
	}
}

func TestAcquireRuntimeAssetsRejectsTamperedLocalRuntimeFilesWithoutFetch(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	root := t.TempDir()
	manifestPath := publishLocalRuntimeTest(t, root, filepath.Join(root, "engine-manifest.json"), spec, "local-engine", "local-pack")
	cacheRoot := filepath.Join(root, "cache")
	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("local manifest must not fetch")
	}
	cfg := Config{RuntimeManifestPath: manifestPath, RuntimeCacheRoot: cacheRoot}
	env := IO{Env: []string{}}
	dependencies := localRuntimeTestDependencies(cacheRoot, fetch)
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, env, lock, dependencies)
	if err != nil {
		t.Fatalf("valid local runtime acquisition: %v", err)
	}
	assets.Cleanup()
	if calls != 0 {
		t.Fatalf("initial fetch count = %d, want 0", calls)
	}

	manifest, err := release.ParseLocalRuntimeManifest(mustReadLaunchpackTestFile(t, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{manifest.Engine.Name, manifest.Pack.Name} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			original := mustReadLaunchpackTestFile(t, path)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := acquireRuntimeAssetsWith(context.Background(), cfg, env, lock, dependencies); err == nil {
				t.Fatalf("tampered local runtime asset %q was accepted", name)
			}
			if calls != 0 {
				t.Fatalf("fetch count = %d, want 0", calls)
			}
			if err := os.WriteFile(path, original, info.Mode().Perm()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAcquireRuntimeAssetsAutoDiscoveredManifestIdentityMismatchFailsClosed(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	sourceRoot := t.TempDir()
	buildRoot := t.TempDir()
	enginePath := filepath.Join(buildRoot, spec.RuntimeName)
	packPath := filepath.Join(buildRoot, spec.PackName)
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
	manifestPath, err := release.LocalRuntimeManifestPath(sourceRoot, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}

	cacheRoot := filepath.Join(sourceRoot, "cache")
	calls := 0
	fetch := func(context.Context, string, io.Writer) error {
		calls++
		return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
	}
	cfg := Config{RuntimeSourceRoot: sourceRoot, RuntimeCacheRoot: cacheRoot, Source: SourceIdentity{SourceMode: true}}
	env := IO{Env: []string{}}
	dependencies := localRuntimeTestDependencies(cacheRoot, fetch)
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, env, lock, dependencies)
	if err != nil {
		t.Fatalf("valid auto-discovered local runtime: %v", err)
	}
	assets.Cleanup()

	manifest.RuntimeVersion = "9.9.9"
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), cfg, env, lock, dependencies); err == nil {
		t.Fatal("identity-mismatched auto-discovered manifest fell back to published runtime")
	}
	if calls != 2 {
		t.Fatalf("fetch count = %d, want 2", calls)
	}
}

func TestEngineBundleRejectsMissingFilesAndUnsafeDestinations(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		change     func(*testing.T, *string, *string, *string)
	}{
		{"missing destination parent", "", func(t *testing.T, out, engine, pack *string) {
			*out = filepath.Join(t.TempDir(), "missing", "bundle.zip")
		}},
		{"missing engine", "add Engine to bundle", func(t *testing.T, out, engine, pack *string) { *engine += ".missing" }},
		{"missing pack", "add runtime PCK to bundle", func(t *testing.T, out, engine, pack *string) { *pack += ".missing" }},
		{"directory destination", "not a regular non-symlink file", func(t *testing.T, out, engine, pack *string) {
			if err := os.Mkdir(*out, 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink destination", "not a regular non-symlink file", func(t *testing.T, out, engine, pack *string) {
			if err := os.Symlink(*engine, *out); err != nil {
				t.Skip(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			output, engine, pack := filepath.Join(root, "bundle.zip"), filepath.Join(root, "engine"), filepath.Join(root, "pack")
			writeProjectTestFile(t, engine, "engine")
			writeProjectTestFile(t, pack, "pack")
			tt.change(t, &output, &engine, &pack)
			err := writeEngineBundle(output, []byte("manifest"), "engine", "pack", engine, pack)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("writeEngineBundle = %v, want %q", err, tt.want)
			}
			if tt.name == "missing destination parent" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing destination parent: %v", err)
			}
			leaks, err := filepath.Glob(filepath.Join(root, ".spx-launchpack-engine-*"))
			if err != nil || len(leaks) > 0 {
				t.Fatalf("temporary bundles = %q, %v", leaks, err)
			}
		})
	}
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	writeProjectTestFile(t, src, "new")
	writeProjectTestFile(t, dst, "old")
	if err := replaceRuntimeFile(src, dst, 0600); err != nil {
		t.Fatal(err)
	}
	assertRuntimeFile(t, dst, "new")
	if err := replaceRuntimeFile(src, dst, 0600); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing replacement source: %v", err)
	}
	if err := validateRuntimeEntry(dst, "engine", runtimebundle.Bundle{}); err == nil || !strings.Contains(err.Error(), "archive is missing") {
		t.Fatalf("unlisted runtime entry: %v", err)
	}
	if err := validateRuntimeEntry(filepath.Join(root, "missing"), "engine", runtimebundle.Bundle{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing runtime file: %v", err)
	}
	if err := validateRuntimeFile(root, "Engine"); err == nil {
		t.Fatal("runtime directory accepted")
	}
	if isRegularNonSymlink(nil) {
		t.Fatal("nil metadata accepted")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := addFileToZip(zw, "engine", root, 0600); err == nil {
		t.Fatal("directory added as runtime file")
	}
	_ = zw.Close()
}

func TestPinnedRuntimeOperationPreservesFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine")
	writeProjectTestFile(t, path, "engine")
	sentinel := errors.New("inspect failed")
	if err := withPinnedFile("Engine", path, func(*pinnedFile) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v", err)
	}
	if err := withPinnedFile("Engine", path, func(f *pinnedFile) error { return f.file.Close() }); err == nil || !strings.Contains(err.Error(), "stat Engine") {
		t.Fatalf("closed pinned file = %v", err)
	}
	if _, _, err := hashRuntimeFile(path + ".missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing runtime hash = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyLocalRuntimeAsset(ctx, path, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled copy = %v", err)
	}
	if err := copyLocalRuntimeAsset(context.Background(), path, launchpackErrorWriter{sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("copy destination error = %v", err)
	}
	if err := copyLocalRuntimeAsset(context.Background(), path+".missing", io.Discard); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing copy source = %v", err)
	}
}

func TestRuntimeAcquisitionRejectsInvalidEnvironmentBeforeFetch(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		env        []string
	}{
		{"duplicate cache", "duplicate SPX_RUNTIME_CACHE", []string{runtimeCacheEnv + "=/one", runtimeCacheEnv + "=/two"}},
		{"relative cache", "absolute clean path", []string{runtimeCacheEnv + "=relative"}},
		{"duplicate offline", "duplicate SPX_RUNTIME_OFFLINE", []string{runtimeOfflineEnv + "=1", runtimeOfflineEnv + "=0"}},
		{"invalid offline", "invalid SPX_RUNTIME_OFFLINE", []string{runtimeOfflineEnv + "=perhaps"}},
		{"duplicate manifest", "duplicate SPX_RUNTIME_LOCAL_MANIFEST", []string{runtimeLocalManifestEnv + "=/one", runtimeLocalManifestEnv + "=/two"}},
		{"relative manifest", "absolute clean path", []string{runtimeLocalManifestEnv + "=relative"}},
		{"duplicate mirror", "duplicate SPX_RUNTIME_ASSET_DIR", []string{runtimeAssetDirEnv + "=/one", runtimeAssetDirEnv + "=/two"}},
		{"empty mirror", "must not be empty", []string{runtimeAssetDirEnv + "="}},
		{"relative mirror", "absolute clean path", []string{runtimeAssetDirEnv + "=relative"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deps := localRuntimeTestDependencies(t.TempDir(), func(context.Context, string, io.Writer) error {
				t.Fatal("invalid environment triggered a fetch")
				return nil
			})
			if _, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{Env: tt.env}, release.DefaultRuntimeLock(), deps); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("acquisition error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, value := range []string{"", "  ", "0", "false", "NO", "off"} {
		if offline, err := runtimeOffline([]string{runtimeOfflineEnv + "=" + value}); err != nil || offline {
			t.Fatalf("runtimeOffline(%q) = %v, %v", value, offline, err)
		}
	}
	for _, value := range []string{"1", "true", " YES ", "on"} {
		if offline, err := runtimeOffline([]string{runtimeOfflineEnv + "=" + value}); err != nil || !offline {
			t.Fatalf("runtimeOffline(%q) = %v, %v", value, offline, err)
		}
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{}, release.DefaultRuntimeLock(), runtimeAssetDependencies{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("missing acquisition dependencies = %v", err)
	}
}

func TestLocalRuntimeManifestRejectsUnsafeDiscoveryPaths(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	for _, name := range []string{"directory", "symlink", "blocked ancestor", "oversized metadata"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path, err := release.LocalRuntimeManifestPath(root, lock, spec.GOOS, spec.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				writeProjectTestFile(t, path+".target", "{}")
				if err := os.Symlink(path+".target", path); err != nil {
					t.Skip(err)
				}
			case "blocked ancestor":
				if err := os.Remove(filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(filepath.Dir(path), []byte("blocked"), 0600)
			case "oversized metadata":
				writeProjectTestFile(t, path, "{}")
				err = os.Truncate(path, maxRuntimeManifestSize+1)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, explicit := range []bool{false, true} {
				var found bool
				if explicit {
					_, found, err = readLocalRuntimeManifest(path, lock, spec, true)
				} else {
					_, found, err = findSourceLocalRuntimeManifest(root, lock, spec)
				}
				want := "not a regular non-symlink file"
				if name == "blocked ancestor" {
					want = "inspect local runtime manifest"
				} else if name == "oversized metadata" {
					want = "exceeds"
				}
				if err == nil || found || !strings.Contains(err.Error(), want) {
					t.Fatalf("explicit=%v: found=%v, error=%v, want %q", explicit, found, err, want)
				}
			}
		})
	}
	if _, found, err := findSourceLocalRuntimeManifest("relative", lock, spec); err == nil || found || !strings.Contains(err.Error(), "absolute and clean") {
		t.Fatalf("relative source root: found=%v, error=%v", found, err)
	}
}

func TestRuntimeAcquisitionRejectsInvalidLockBeforeResolution(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	lock.RuntimeVersion = "invalid version"
	deps := localRuntimeTestDependencies(t.TempDir(), func(context.Context, string, io.Writer) error {
		t.Fatal("invalid lock caused a download")
		return nil
	})
	deps.cacheRoot = func() string {
		t.Fatal("invalid lock resolved cache storage")
		return ""
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{Env: []string{}}, lock, deps); err == nil {
		t.Fatal("invalid runtime lock accepted")
	}
}

func TestRuntimeNamesMatchReleaseAndDevelopmentNames(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, version := range []string{lock.RuntimeVersion, "1.0.0-local.exe"} {
			t.Run(goos+"/"+version, func(t *testing.T) {
				names, err := RuntimeNamesFor(version, goos, "amd64")
				if err != nil {
					t.Fatal(err)
				}
				versionLock := lock
				versionLock.RuntimeVersion = version
				spec, err := release.HostRuntimeSpecFor(versionLock, goos, "amd64")
				if err != nil {
					t.Fatal(err)
				}
				if names.Engine != spec.RuntimeName || names.Pack != spec.PackName {
					t.Fatalf("names = %+v, release spec = %+v", names, spec)
				}
				assets := names.inDirectory(t.TempDir())
				for _, path := range []string{assets.EnginePath, assets.PackPath, assets.BridgePath} {
					writeProjectTestFile(t, path, "asset")
				}
				if err := assets.ValidateFiles(names); err != nil {
					t.Fatal(err)
				}
				mixed := names
				mixed.Pack = "gdspxrt2.0.0-other.exe.pck"
				if err := assets.ValidateFiles(mixed); err == nil || !strings.Contains(err.Error(), "same version") {
					t.Fatalf("mixed runtime versions accepted: %v", err)
				}
			})
		}
	}
	names, err := RuntimeNamesFor("dev-test", "windows", "arm64")
	if err != nil || names.Engine != "gdspxrtdev-test.exe" || names.Pack != "gdspxrtdev-test.pck" || names.Bridge != "gdspx-windows-arm64.dll" {
		t.Fatalf("development names = %+v, %v", names, err)
	}
	for _, version := range []string{"", "../other", `a\b`} {
		if _, err := RuntimeNamesFor(version, "linux", "amd64"); err == nil {
			t.Fatalf("accepted version %q", version)
		}
	}
	if names, err := RuntimeNamesFor("dev-test", "plan9", "amd64"); err == nil || !strings.Contains(err.Error(), "host platform plan9/amd64 is not supported") || names != (RuntimeNames{}) {
		t.Fatalf("unsupported platform names = %+v, %v", names, err)
	}
}

func TestResolveInstalledRuntimeRejectsInvalidConfiguration(t *testing.T) {
	valid := RuntimeNames{Engine: "gdspxrt1", Pack: "gdspxrt1.pck", Bridge: "gdspx-linux-amd64.so"}
	for _, tt := range []struct {
		name, want string
		change     func(*RuntimeNames)
	}{
		{"no directories", "no runtime installation directories configured", func(*RuntimeNames) {}},
		{"missing engine name", "invalid runtime file name", func(n *RuntimeNames) { n.Engine = "" }},
		{"pack outside installation", "invalid runtime file name", func(n *RuntimeNames) { n.Pack = "../gdspxrt1.pck" }},
		{"bridge path separator", "invalid runtime file name", func(n *RuntimeNames) { n.Bridge = `lib\gdspx.dll` }},
		{"mixed version names", "must identify the same version", func(n *RuntimeNames) { n.Pack = "gdspxrt2.pck" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			names := valid
			tt.change(&names)
			assets, err := ResolveInstalledRuntime(nil, names)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ResolveInstalledRuntime = %+v, %v; want %q", assets, err, tt.want)
			}
			if assets.EnginePath != "" || assets.PackPath != "" || assets.BridgePath != "" || assets.Cleanup != nil {
				t.Fatalf("invalid configuration returned runtime assets: %+v", assets)
			}
		})
	}
}

func TestResolveInstalledRuntimeSelectsCompleteDirectory(t *testing.T) {
	names := RuntimeNames{Engine: "gdspxrt1", Pack: "gdspxrt1.pck", Bridge: "gdspx-linux-amd64.so"}
	pair := []string{names.Engine, names.Pack}
	complete := []string{names.Engine, names.Pack, names.Bridge}
	for _, test := range []struct {
		name          string
		first, second []string
		want          int
	}{
		{"first complete", complete, complete, 0},
		{"missing bridge falls through", pair, complete, 1},
		{"split installation", pair, []string{names.Bridge}, -1},
		{"different version", pair, []string{names.Engine, "gdspxrt2.pck", names.Bridge}, -1},
		{"different platform", pair, []string{names.Engine, names.Pack, "gdspx-linux-arm64.so"}, -1},
		{"pack directory", pair, []string{names.Engine, names.Bridge}, -1},
		{"bridge symlink", pair, pair, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			dirs := []string{t.TempDir(), t.TempDir()}
			for i, files := range [][]string{test.first, test.second} {
				for _, name := range files {
					writeProjectTestFile(t, filepath.Join(dirs[i], name), "legacy asset without version metadata")
				}
			}
			switch test.name {
			case "pack directory":
				if err := os.Mkdir(filepath.Join(dirs[1], names.Pack), 0o755); err != nil {
					t.Fatal(err)
				}
			case "bridge symlink":
				if err := os.Symlink(filepath.Join(dirs[1], names.Engine), filepath.Join(dirs[1], names.Bridge)); err != nil {
					t.Skip(err)
				}
			}
			assets, err := ResolveInstalledRuntime(dirs, names)
			if test.want < 0 {
				if err == nil || assets.EnginePath != "" || assets.PackPath != "" || assets.BridgePath != "" {
					t.Fatalf("invalid installation = %+v, %v", assets, err)
				}
				return
			}
			wantDir := dirs[test.want]
			if err != nil || assets.EnginePath != filepath.Join(wantDir, names.Engine) || assets.PackPath != filepath.Join(wantDir, names.Pack) || assets.BridgePath != filepath.Join(wantDir, names.Bridge) {
				t.Fatalf("installation = %+v, %v; want directory %q", assets, err, wantDir)
			}
			if assets.Lock.RuntimeVersion != "" || assets.Cleanup != nil {
				t.Fatalf("legacy installation acquired manifest identity or ownership: %+v", assets)
			}
		})
	}
}

func TestRuntimeAssetsValidateSelectedNamesAndSourceBridge(t *testing.T) {
	names, err := RuntimeNamesFor("1", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	assets := names.inDirectory(t.TempDir())
	assets.BridgePath = filepath.Join(t.TempDir(), names.Bridge)
	for _, path := range []string{assets.EnginePath, assets.PackPath, assets.BridgePath} {
		writeProjectTestFile(t, path, "asset")
	}
	if err := assets.ValidateFiles(names); err != nil {
		t.Fatalf("source bridge in a separate build directory: %v", err)
	}
	for _, tt := range []struct {
		name, want string
		change     func(*Assets)
	}{
		{"relative engine", "Engine path must be absolute and clean", func(a *Assets) { a.EnginePath = names.Engine }},
		{"unclean engine", "Engine path must be absolute and clean", func(a *Assets) {
			a.EnginePath = filepath.Dir(a.EnginePath) + string(filepath.Separator) + "." + string(filepath.Separator) + names.Engine
		}},
		{"wrong engine version", "does not match selected runtime", func(a *Assets) { a.EnginePath = filepath.Join(filepath.Dir(a.EnginePath), "gdspxrt2") }},
		{"wrong pack version", "does not match selected runtime", func(a *Assets) { a.PackPath = filepath.Join(filepath.Dir(a.PackPath), "gdspxrt2.pck") }},
		{"relative bridge", "interpreter bridge path must be absolute and clean", func(a *Assets) { a.BridgePath = names.Bridge }},
		{"wrong bridge platform", "does not match selected runtime", func(a *Assets) { a.BridgePath = filepath.Join(filepath.Dir(a.BridgePath), "gdspx-linux-arm64.so") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			invalid := assets
			tt.change(&invalid)
			if err := invalid.ValidateFiles(names); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateFiles = %v, want %q", err, tt.want)
			}
		})
	}
	names.Pack = "gdspxrt2.pck"
	if err := assets.ValidateFiles(names); err == nil || !strings.Contains(err.Error(), "same version") {
		t.Fatalf("mixed version names = %v", err)
	}
	names.Pack = "gdspxrt1.pck"
	assets.PackPath = filepath.Join(t.TempDir(), names.Pack)
	writeProjectTestFile(t, assets.PackPath, "pack")
	if err := assets.ValidateFiles(names); err == nil || !strings.Contains(err.Error(), "share one directory") {
		t.Fatalf("separate engine and pack = %v", err)
	}
}

func defaultHostRuntimeTest(t *testing.T) (release.RuntimeLock, release.HostRuntimeSpec) {
	t.Helper()
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return lock, spec
}

func localRuntimeTestDependencies(cacheRoot string, fetch func(context.Context, string, io.Writer) error) runtimeAssetDependencies {
	dependencies := defaultRuntimeAssetDependencies()
	dependencies.fetch = fetch
	dependencies.cacheRoot = func() string { return cacheRoot }
	return dependencies
}

func mustReadLaunchpackTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func publishLocalRuntimeTest(t *testing.T, root, manifestPath string, spec release.HostRuntimeSpec, engine, pack string) string {
	t.Helper()
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := release.NewLocalRuntimeManifest(release.DefaultRuntimeLock(), runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}
