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
