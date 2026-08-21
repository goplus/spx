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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestAcquireRuntimeAssetsPrefersExplicitLocalManifest(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
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
		cacheRoot: func() string { return cfg.RuntimeCacheRoot }, manifestPin: release.RuntimeManifestPinForLock,
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
		cacheRoot: func() string { return filepath.Join(root, "cache-source") }, manifestPin: release.RuntimeManifestPinForLock,
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
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
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
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
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
		return errors.New("auto-discovered local manifest must not fetch")
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
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0", calls)
	}
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
