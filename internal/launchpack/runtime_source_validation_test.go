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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/release"
)

func TestSourceRuntimeRejectsInvalidInstalledPair(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "missing Engine", setup: func(t *testing.T, bin string) {
			if err := os.WriteFile(filepath.Join(bin, spec.PackName), []byte("pack"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing PCK", setup: func(t *testing.T, bin string) {
			if err := os.WriteFile(filepath.Join(bin, spec.RuntimeName), []byte("engine"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "Engine symlink", setup: func(t *testing.T, bin string) {
			target := filepath.Join(bin, "engine-target")
			if err := os.WriteFile(target, []byte("engine"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(bin, spec.RuntimeName)); err != nil {
				t.Skip(err)
			}
			if err := os.WriteFile(filepath.Join(bin, spec.PackName), []byte("pack"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "PCK directory", setup: func(t *testing.T, bin string) {
			if err := os.WriteFile(filepath.Join(bin, spec.RuntimeName), []byte("engine"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(bin, spec.PackName), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bin := t.TempDir()
			test.setup(t, bin)
			cacheRoot := t.TempDir()
			dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
				return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
			})
			dependencies.goBin = func(context.Context, Config, []string) (string, error) { return bin, nil }
			_, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, lock, dependencies)
			if err == nil || !strings.Contains(err.Error(), "make dev") {
				t.Fatalf("invalid installed pair error = %v", err)
			}
		})
	}
}

func TestLocalRuntimeMaterializationDetectsRewrite(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	bin := writeInstalledRuntimeTest(t, spec, "engine-v1", "pack-v1")
	enginePath := filepath.Join(bin, spec.RuntimeName)
	packPath := filepath.Join(bin, spec.PackName)
	manifest, err := release.NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := localRuntimeSource{manifest: manifest, bytes: data, enginePath: enginePath, packPath: packPath}
	if _, err := materializeLocalRuntime(context.Background(), t.TempDir(), lock, spec, source); err == nil || !strings.Contains(err.Error(), "changed after manifest verification") {
		t.Fatalf("rewritten local runtime error = %v", err)
	}
}

func TestSourceManifestUsesVersionValidationButExplicitStaysStrict(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manifestPath, err := release.LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	publishLocalRuntimeTest(t, root, manifestPath, spec, "source-engine", "source-pack")
	manifest, err := release.ParseLocalRuntimeManifest(mustReadLaunchpackTestFile(t, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	manifest.RuntimeABI++
	manifest.LockSHA256 = strings.Repeat("0", 64)
	data, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
	})
	binCalls := 0
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return t.TempDir(), nil
	}
	assets, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(root, cacheRoot), IO{Env: []string{}}, lock, dependencies)
	if err != nil {
		t.Fatalf("source manifest: %v", err)
	}
	assets.Cleanup()
	if binCalls != 0 {
		t.Fatalf("source manifest Go-bin calls = %d, want 0", binCalls)
	}
	explicit := sourceRuntimeConfig(root, t.TempDir())
	explicit.RuntimeManifestPath = manifestPath
	if _, err := acquireRuntimeAssetsWith(context.Background(), explicit, IO{Env: []string{}}, lock, dependencies); err == nil {
		t.Fatal("explicit manifest accepted stale lock metadata")
	}
	if binCalls != 0 {
		t.Fatalf("explicit manifest Go-bin calls = %d, want 0", binCalls)
	}
}

func TestSourceRuntimePassesAcquisitionEnvironmentToGoBin(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	bin := writeInstalledRuntimeTest(t, spec, "engine", "pack")
	cacheRoot := t.TempDir()
	dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
	})
	dependencies.goBin = func(_ context.Context, _ Config, env []string) (string, error) {
		value, found, duplicate := envutil.Lookup(env, "GOPATH")
		if !found || duplicate || value != "/frozen-gopath" {
			return "", errors.New("acquisition environment was not forwarded")
		}
		return bin, nil
	}
	cfg := sourceRuntimeConfig(t.TempDir(), cacheRoot)
	cfg.IO.Env = []string{"GOPATH=/stale-gopath"}
	assets, err := acquireRuntimeAssetsWith(context.Background(), cfg, IO{Env: []string{"GOPATH=/frozen-gopath"}}, lock, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	assets.Cleanup()
}

func TestSourceRuntimeHonorsCanceledContextBeforeResolution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	dependencies := localRuntimeTestDependencies(t.TempDir(), func(context.Context, string, io.Writer) error {
		calls++
		return nil
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		calls++
		return t.TempDir(), nil
	}
	_, err := acquireRuntimeAssetsWith(ctx, sourceRuntimeConfig(t.TempDir(), t.TempDir()), IO{Env: []string{}}, release.DefaultRuntimeLock(), dependencies)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled acquisition error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("resolution calls = %d, want 0", calls)
	}
}

func TestSourceRuntimeDoesNotFallbackAfterFetchCancellation(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	binCalls := 0
	dependencies := fixture.dependencies(t.TempDir(), func(context.Context, string, io.Writer) error {
		cancel()
		return context.Canceled
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return t.TempDir(), nil
	}
	_, err := acquireRuntimeAssetsWith(ctx, sourceRuntimeConfig(t.TempDir(), t.TempDir()), IO{Env: []string{}}, fixture.lock, dependencies)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fetch error = %v", err)
	}
	if binCalls != 0 {
		t.Fatalf("Go-bin calls = %d, want 0", binCalls)
	}
}

func TestSourceRuntimeMissingFilesSuggestsMakeDev(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	cacheRoot := t.TempDir()
	dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		return fmt.Errorf("%w: missing release", errReleaseUnavailable)
	})
	dependencies.goBin = func(context.Context, Config, []string) (string, error) { return t.TempDir(), nil }
	_, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, lock, dependencies)
	if err == nil || !strings.Contains(err.Error(), "make dev") || !strings.Contains(err.Error(), "gdspxrt"+lock.RuntimeVersion) {
		t.Fatalf("missing local runtime error = %v", err)
	}
	if !errors.Is(err, errReleaseUnavailable) {
		t.Fatalf("missing local runtime lost published cause: %v", err)
	}
}

func TestResolveGoBinUsesFirstGOPATHEntry(t *testing.T) {
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	first, second, other := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("GOPATH", strings.Join([]string{first, second}, string(os.PathListSeparator)))
	t.Setenv("GOBIN", other)
	got, err := resolveGoBin(context.Background(), Config{GoCommand: goCommand, WorkDir: t.TempDir(), GoWork: "off"}, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(first, "bin"); got != want {
		t.Fatalf("Go bin = %q, want %q", got, want)
	}
}

func sourceRuntimeConfig(sourceRoot, cacheRoot string) Config {
	return Config{
		RuntimeSourceRoot: sourceRoot,
		RuntimeCacheRoot:  cacheRoot,
		Source:            SourceIdentity{SourceMode: true},
	}
}

func writeInstalledRuntimeTest(t *testing.T, spec release.HostRuntimeSpec, engine, pack string) string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, spec.RuntimeName), []byte(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, spec.PackName), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	return bin
}

func assertRuntimeFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("runtime file = %q, want %q", data, want)
	}
}
