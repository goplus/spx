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
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
)

func TestAcquirePublishedDriverCanceledContextDoesNotFetch(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	deps := fixture.dependencies(t.TempDir(), func(context.Context, string, io.Writer) error {
		calls++
		return errors.New("canceled acquisition fetched")
	})
	_, err := acquirePublishedDriverWith(canceled, publishedDriverTestConfig(deps.cacheRoot()), IO{}, fixture.lock, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled acquisition error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("fetch count = %d, want 0", calls)
	}
}

func TestAcquirePublishedDriverOfflineColdCacheFailsClosed(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	calls := 0
	deps := fixture.dependencies(t.TempDir(), fixture.fetcher(nil, &calls))
	cfg := publishedDriverTestConfig(deps.cacheRoot())
	cfg.RuntimeOffline = true
	if _, err := acquirePublishedDriverWith(context.Background(), cfg, IO{}, fixture.lock, deps); err == nil {
		t.Fatal("offline cold published acquisition succeeded")
	}
	if calls != 0 {
		t.Fatalf("offline fetch count = %d, want 0", calls)
	}
}

func TestAcquirePublishedDriverIgnoresSourceRuntimeEnvironment(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	calls := 0
	streams := IO{Env: []string{
		runtimeLocalManifestEnv + "=/ambient/runtime.json",
		runtimeLocalManifestEnv + "=/duplicate/runtime.json",
		runtimeAssetDirEnv + "=/ambient/assets",
		runtimeAssetDirEnv + "=/duplicate/assets",
	}}
	assets, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), streams, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	if calls != 2 {
		t.Fatalf("published fetch count = %d, want 2", calls)
	}
}

func TestAcquirePublishedDriverExplicitMirrorManifestIsNeverHiddenByCache(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	calls := 0
	assets, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &calls)))
	if err != nil {
		t.Fatal(err)
	}
	assets.Cleanup()
	if calls != 2 {
		t.Fatalf("warm-cache fetch count = %d, want 2", calls)
	}

	for name, manifestData := range map[string][]byte{
		"invalid": []byte("not a driver manifest"),
		"missing": nil,
	} {
		t.Run(name, func(t *testing.T) {
			assetDir := t.TempDir()
			if manifestData != nil {
				if err := os.WriteFile(filepath.Join(assetDir, driverbundle.ManifestName), manifestData, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := publishedDriverTestConfig(cacheRoot)
			cfg.DriverAssetDir = assetDir
			localCalls := 0
			deps := fixture.dependencies(cacheRoot, fixture.fetcher(nil, &localCalls))
			if _, err := acquirePublishedDriverWith(context.Background(), cfg, IO{}, fixture.lock, deps); err == nil {
				t.Fatal("explicit mirror manifest failure was hidden by warm cache")
			}
			if localCalls != 0 {
				t.Fatalf("explicit mirror fetch count = %d, want 0", localCalls)
			}
		})
	}
}

func TestAcquirePublishedDriverExplicitMirrorBundleIsNeverHiddenByCache(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	cacheRoot := t.TempDir()
	warmCalls := 0
	warm, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(cacheRoot), IO{}, fixture.lock, fixture.dependencies(cacheRoot, fixture.fetcher(nil, &warmCalls)))
	if err != nil {
		t.Fatal(err)
	}
	warm.Cleanup()
	if warmCalls != 2 {
		t.Fatalf("warm-cache fetch count = %d, want 2", warmCalls)
	}

	tampered := append([]byte(nil), fixture.bundleData...)
	tampered[len(tampered)/2] ^= 1
	for name, bundleData := range map[string][]byte{"missing": nil, "tampered": tampered} {
		t.Run(name, func(t *testing.T) {
			assetDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(assetDir, driverbundle.ManifestName), fixture.manifestData, 0o600); err != nil {
				t.Fatal(err)
			}
			if bundleData != nil {
				if err := os.WriteFile(filepath.Join(assetDir, fixture.bundle.Name), bundleData, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := publishedDriverTestConfig(cacheRoot)
			cfg.DriverAssetDir = assetDir
			localCalls := 0
			deps := fixture.dependencies(cacheRoot, fixture.fetcher(nil, &localCalls))
			if _, err := acquirePublishedDriverWith(context.Background(), cfg, IO{}, fixture.lock, deps); err == nil {
				t.Fatal("explicit mirror bundle failure was hidden by warm cache")
			}
			if localCalls != 0 {
				t.Fatalf("explicit mirror fetch count = %d, want 0", localCalls)
			}
		})
	}
}

func TestPublishedDriverRejectsRuntimeVersionMismatch(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	historical, err := release.RuntimeLockForVersion("2.4.3")
	if err != nil {
		t.Fatal(err)
	}
	fixture.manifest.RuntimeVersion = historical.RuntimeVersion
	for i := range fixture.manifest.Bundles {
		bundle := &fixture.manifest.Bundles[i]
		names := driverFixtureFileNames(historical.RuntimeVersion, bundle.GOOS, bundle.GOARCH)
		for j := range bundle.Files {
			bundle.Files[j].Name = names[j]
		}
	}
	fixture.manifestData, err = fixture.manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	deps := fixture.dependencies(t.TempDir(), fixture.fetcher(nil, new(int)))
	if _, err := acquirePublishedDriverWith(context.Background(), publishedDriverTestConfig(deps.cacheRoot()), IO{}, fixture.lock, deps); err == nil {
		t.Fatal("published driver accepted a mismatched runtime version")
	}
}

func TestPublishedConfigRejectsRuntimeAndSourceInputs(t *testing.T) {
	base := validPublishedConfigForValidation(t)
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"runtime manifest path", func(c *Config) { c.RuntimeManifestPath = filepath.Join(t.TempDir(), "runtime.json") }},
		{"runtime asset directory", func(c *Config) { c.RuntimeAssetDir = t.TempDir() }},
		{"runtime source root", func(c *Config) { c.RuntimeSourceRoot = t.TempDir() }},
		{"source bridge package", func(c *Config) { c.BridgePackage = "./bridge" }},
		{"source mode", func(c *Config) { c.Source.SourceMode = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.mutate(&cfg)
			if err := cfg.validate(); err == nil {
				t.Fatal("invalid published configuration was accepted")
			}
		})
	}
}

func TestSourceOnlyEntryPointsRejectPublishedMode(t *testing.T) {
	cfg := Config{Source: SourceIdentity{SourceMode: false}}
	if _, err := AcquireRuntimeAssets(context.Background(), cfg); err == nil {
		t.Fatal("published mode entered runtime-only acquisition")
	}
	if _, _, err := BuildSourceBridge(context.Background(), cfg); err == nil {
		t.Fatal("published mode entered source bridge build")
	}
}

func validPublishedConfigForValidation(t *testing.T) Config {
	t.Helper()
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "main.spx"), []byte("onStart => {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	return Config{
		ProjectDir: projectDir, ProjectFile: filepath.Join(projectDir, "main.spx"), ProjectExt: ".spx",
		PackDir: "assets", PackIndex: "index.json", Output: filepath.Join(t.TempDir(), "launcher"),
		GoCommand: goCommand, WorkDir: projectDir, GoWork: "off",
		Source: SourceIdentity{SelectedPath: driverbundle.SPXModulePath, SelectedVersion: "v3.2.4", EffectivePath: driverbundle.SPXModulePath, EffectiveVersion: "v3.2.4"},
	}
}

func TestPrepareAssetsSourceUsesRuntimeAndBridgePath(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manifestPath := filepath.Join(root, release.EngineSourceManifestName)
	publishLocalRuntimeTest(t, root, manifestPath, spec, "source-engine", "source-pack")
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	graphChecks := 0
	cfg := Config{
		RuntimeManifestPath: manifestPath, RuntimeCacheRoot: t.TempDir(),
		Source: SourceIdentity{SourceMode: true}, GoCommand: goCommand, WorkDir: root, GoWork: "off",
		BridgePackage: "./missing-bridge", VerifyGraph: func(context.Context) error { graphChecks++; return nil },
	}
	if _, err := PrepareAssets(context.Background(), cfg); err == nil {
		t.Fatal("source bridge build unexpectedly succeeded")
	}
	if graphChecks != 1 {
		t.Fatalf("source graph checks = %d, want one source-bridge check", graphChecks)
	}
}
