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
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestSourceRuntimePrefersPublishedAssets(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	bin := writeInstalledRuntimeTest(t, fixture.spec, "local-engine", "local-pack")
	fetchCalls, binCalls := 0, 0
	dependencies := fixture.dependencies(cacheRoot, fixture.fetcher(nil, &fetchCalls))
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return bin, nil
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	assertRuntimeFile(t, assets.EnginePath, "fixture-engine-"+runtime.GOOS+"-"+runtime.GOARCH)
	if fetchCalls != 3 || binCalls != 0 {
		t.Fatalf("fetch calls = %d, Go-bin calls = %d; want 3, 0", fetchCalls, binCalls)
	}
}

func TestSourceRuntimeFallsBackAfterPublishedFetchFailure(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot, sourceRoot := t.TempDir(), t.TempDir()
	bin := writeInstalledRuntimeTest(t, fixture.spec, "local-engine", "local-pack")
	fetchCalls, binCalls := 0, 0
	dependencies := fixture.dependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		fetchCalls++
		return errors.New("network unavailable")
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return bin, nil
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(sourceRoot, cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	assertRuntimeFile(t, assets.EnginePath, "local-engine")
	if fetchCalls != 1 || binCalls != 1 {
		t.Fatalf("fetch calls = %d, Go-bin calls = %d; want 1, 1", fetchCalls, binCalls)
	}
	manifestPath, err := release.LocalRuntimeManifestPath(sourceRoot, fixture.lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("fallback wrote source manifest: %v", err)
	}
}

func TestPublishedFetchFailureDoesNotFallbackOutsideSourceMode(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	binCalls := 0
	dependencies := fixture.dependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		return errors.New("network unavailable")
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return t.TempDir(), nil
	}
	cfg := Config{RuntimeSourceRoot: t.TempDir(), RuntimeCacheRoot: cacheRoot}
	if _, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{}}, fixture.lock, dependencies); err == nil {
		t.Fatal("published fetch failure was accepted outside source mode")
	}
	if binCalls != 0 {
		t.Fatalf("Go-bin calls = %d, want 0", binCalls)
	}
}

func TestUnpublishedSourceRuntimeUsesGoBinWithoutFetch(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	bin := writeInstalledRuntimeTest(t, spec, "dev-engine", "dev-pack")
	fetchCalls, binCalls := 0, 0
	dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		fetchCalls++
		return errors.New("unpublished runtime must not fetch")
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return bin, nil
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, lock, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	assertRuntimeFile(t, assets.PackPath, "dev-pack")
	if fetchCalls != 0 || binCalls != 1 {
		t.Fatalf("fetch calls = %d, Go-bin calls = %d; want 0, 1", fetchCalls, binCalls)
	}
}

func TestOfflineSourceRuntimeFallsBackWithoutFetch(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	bin := writeInstalledRuntimeTest(t, fixture.spec, "offline-engine", "offline-pack")
	fetchCalls := 0
	dependencies := fixture.dependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		fetchCalls++
		return errors.New("offline acquisition fetched")
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) { return bin, nil }
	cfg := sourceRuntimeConfig(t.TempDir(), cacheRoot)
	cfg.RuntimeOffline = true
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{}}, fixture.lock, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	assertRuntimeFile(t, assets.EnginePath, "offline-engine")
	if fetchCalls != 0 {
		t.Fatalf("fetch calls = %d, want 0", fetchCalls)
	}
}

func TestExplicitReleaseDirectoryFailureDoesNotUseGoBin(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	cacheRoot := t.TempDir()
	binCalls := 0
	dependencies := fixture.dependencies(cacheRoot, fixture.fetcher(nil, new(int)))
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return t.TempDir(), nil
	}
	cfg := sourceRuntimeConfig(t.TempDir(), cacheRoot)
	cfg.RuntimeAssetDir = t.TempDir()
	if _, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{}}, fixture.lock, dependencies); err == nil {
		t.Fatal("missing explicit release directory assets were accepted")
	}
	if binCalls != 0 {
		t.Fatalf("Go-bin calls = %d, want 0", binCalls)
	}
}

func TestManifestPinErrorsDoNotUseGoBin(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	for _, test := range []struct {
		name   string
		mutate func(runtimeAssetDependencies) runtimeAssetDependencies
	}{
		{
			name: "resolution",
			mutate: func(dependencies runtimeAssetDependencies) runtimeAssetDependencies {
				dependencies.manifestPin = func(release.RuntimeLock) (release.RuntimeManifestPin, error) {
					return release.RuntimeManifestPin{}, errors.New("corrupt embedded pin")
				}
				return dependencies
			},
		},
		{
			name: "validation",
			mutate: func(dependencies runtimeAssetDependencies) runtimeAssetDependencies {
				valid := dependencies.manifestPin
				dependencies.manifestPin = func(lock release.RuntimeLock) (release.RuntimeManifestPin, error) {
					pin, err := valid(lock)
					pin.RuntimeVersion = "9.9.9"
					return pin, err
				}
				return dependencies
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cacheRoot := t.TempDir()
			dependencies := test.mutate(fixture.dependencies(cacheRoot, fixture.fetcher(nil, new(int))))
			binCalls := 0
			dependencies.goBin = func(context.Context, Config, []string) (string, error) {
				binCalls++
				return t.TempDir(), nil
			}
			if _, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies); err == nil {
				t.Fatal("manifest pin error was accepted")
			}
			if binCalls != 0 {
				t.Fatalf("Go-bin calls = %d, want 0", binCalls)
			}
		})
	}
}
