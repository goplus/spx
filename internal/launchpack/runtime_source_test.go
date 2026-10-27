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

	"github.com/goplus/spx/v4/internal/release"
	"github.com/goplus/spx/v4/internal/runtimebundle"
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
		return fmt.Errorf("%w: network unavailable", errReleaseUnavailable)
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

func TestSourceRuntimeRejectsPublishedIntegrityFailure(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	badArchive := append([]byte(nil), fixture.assets[fixture.spec.ArchiveName]...)
	badArchive[len(badArchive)/2] ^= 1
	for _, test := range []struct {
		name         string
		replacements map[string][]byte
		digestError  bool
	}{
		{name: "manifest", replacements: map[string][]byte{fixture.lock.Manifest: []byte("tampered manifest")}},
		{name: "archive", replacements: map[string][]byte{fixture.spec.ArchiveName: badArchive}, digestError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cacheRoot := t.TempDir()
			bin := writeInstalledRuntimeTest(t, fixture.spec, "local-engine", "local-pack")
			calls, binCalls := 0, 0
			dependencies := fixture.dependencies(cacheRoot, fixture.fetcher(test.replacements, &calls))
			dependencies.goBin = func(context.Context, Config, []string) (string, error) {
				binCalls++
				return bin, nil
			}
			_, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies)
			if test.digestError && !errors.Is(err, runtimebundle.ErrDigestMismatch) {
				t.Fatalf("archive integrity failure = %v, want ErrDigestMismatch", err)
			}
			if !test.digestError && (err == nil || !strings.Contains(err.Error(), "decode runtime manifest")) {
				t.Fatalf("malformed manifest error = %v", err)
			}
			if binCalls != 0 {
				t.Fatalf("Go-bin calls = %d, want 0", binCalls)
			}
		})
	}
}

func TestSourceRuntimeRejectsMalformedPublishedManifest(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	malformed := append(append([]byte(nil), fixture.manifestData...), 'x')
	cacheRoot := t.TempDir()
	bin := writeInstalledRuntimeTest(t, fixture.spec, "local-engine", "local-pack")
	calls, binCalls := 0, 0
	dependencies := fixture.dependencies(cacheRoot, fixture.fetcher(map[string][]byte{fixture.lock.Manifest: malformed}, &calls))
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return bin, nil
	}
	_, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies)
	if err == nil || errors.Is(err, errReleaseUnavailable) {
		t.Fatalf("malformed manifest error = %v", err)
	}
	if binCalls != 0 {
		t.Fatalf("Go-bin calls = %d, want 0", binCalls)
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

func TestUnpublishedSourceRuntimeFallsBackAfterManifestMiss(t *testing.T) {
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
		return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
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
	if fetchCalls != 1 || binCalls != 1 {
		t.Fatalf("fetch calls = %d, Go-bin calls = %d; want 1, 1", fetchCalls, binCalls)
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

func TestRuntimeManifestVersionMismatchDoesNotUseGoBin(t *testing.T) {
	fixture := newPublishedRuntimeFixture(t)
	wrongVersion := fixture.manifest
	wrongVersion.RuntimeVersion = "9.9.9"
	data, err := wrongVersion.JSON()
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	dependencies := fixture.dependencies(cacheRoot, fixture.fetcher(map[string][]byte{fixture.lock.Manifest: data}, new(int)))
	binCalls := 0
	dependencies.goBin = func(context.Context, Config, []string) (string, error) {
		binCalls++
		return t.TempDir(), nil
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), sourceRuntimeConfig(t.TempDir(), cacheRoot), IO{Env: []string{}}, fixture.lock, dependencies); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("runtime manifest version mismatch = %v", err)
	}
	if binCalls != 0 {
		t.Fatalf("Go-bin calls = %d, want 0", binCalls)
	}
}

func TestSourceRuntimeRejectsInvalidInstalledPair(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
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
		{name: "PCK from another version", setup: func(t *testing.T, bin string) {
			writeProjectTestFile(t, filepath.Join(bin, spec.RuntimeName), "engine")
			writeProjectTestFile(t, filepath.Join(bin, "gdspxrt0.0.0-other.pck"), "other pack")
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
	lock, spec := defaultHostRuntimeTest(t)
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
	lock, spec := defaultHostRuntimeTest(t)
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
	lock, spec := defaultHostRuntimeTest(t)
	bin := writeInstalledRuntimeTest(t, spec, "engine", "pack")
	cacheRoot := t.TempDir()
	dependencies := localRuntimeTestDependencies(cacheRoot, func(context.Context, string, io.Writer) error {
		return fmt.Errorf("%w: unpublished runtime", errReleaseUnavailable)
	})
	dependencies.goBin = func(_ context.Context, _ Config, env []string) (string, error) {
		value, found, duplicate := environmentValue(env, "GOPATH")
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

func TestResolveGoBinRejectsUnavailableCommand(t *testing.T) {
	workDir := t.TempDir()
	for _, test := range []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "missing command", cfg: Config{}, want: "Go command is unavailable"},
		{name: "missing executable", cfg: Config{GoCommand: filepath.Join(workDir, "missing-go"), WorkDir: workDir}, want: "resolve Go bin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := resolveGoBin(context.Background(), test.cfg, nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveGoBin() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestSourceRuntimeFallbackPreservesResolutionErrors(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	published := fmt.Errorf("%w: unpublished", errReleaseUnavailable)
	sentinel := errors.New("GOPATH resolver failed")
	for _, tt := range []struct {
		name, want string
		root       string
		resolver   func(context.Context, Config, []string) (string, error)
	}{
		{name: "no source root", want: "SPX source root is unavailable"},
		{name: "no resolver", want: "cannot resolve Go bin: resolver is unavailable", root: t.TempDir()},
		{name: "resolver failed", want: sentinel.Error(), root: t.TempDir(), resolver: func(context.Context, Config, []string) (string, error) { return "", sentinel }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := acquireSourceRuntime(context.Background(), Config{RuntimeSourceRoot: tt.root}, nil, t.TempDir(), lock, spec, runtimeAssetDependencies{goBin: tt.resolver}, published)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !errors.Is(err, errReleaseUnavailable) {
				t.Fatalf("source fallback = %v, want %q and published cause", err, tt.want)
			}
			if tt.name == "resolver failed" && !errors.Is(err, sentinel) {
				t.Fatalf("lost resolver cause: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireSourceRuntime(ctx, Config{}, nil, t.TempDir(), lock, spec, runtimeAssetDependencies{}, published); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled fallback = %v", err)
	}
}

func TestResolveGoBinRejectsInvalidCommandOutput(t *testing.T) {
	for _, tt := range []struct{ name, body, want string }{
		{"empty GOPATH", "exit 0", "Go environment has no GOPATH"},
		{"relative GOPATH", "printf 'relative'", "absolute clean path"},
		{"stderr context", "printf 'broken toolchain' >&2; exit 4", "broken toolchain"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			launcherGoFixture(t, &cfg, tt.body)
			if _, err := resolveGoBin(context.Background(), cfg, []string{}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("resolveGoBin = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSourceRuntimePreservesMaterializationFailures(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	for _, discovered := range []bool{false, true} {
		t.Run(fmt.Sprintf("discovered=%v", discovered), func(t *testing.T) {
			root := t.TempDir()
			bin := writeInstalledRuntimeTest(t, spec, "engine", "pack")
			if discovered {
				path, err := release.LocalRuntimeManifestPath(root, lock, spec.GOOS, spec.GOARCH)
				if err != nil {
					t.Fatal(err)
				}
				publishLocalRuntimeTest(t, root, path, spec, "engine", "pack")
			}
			cache := filepath.Join(t.TempDir(), "cache")
			writeProjectTestFile(t, cache, "blocked")
			calls := 0
			deps := runtimeAssetDependencies{goBin: func(context.Context, Config, []string) (string, error) {
				calls++
				return bin, nil
			}}
			_, err := acquireSourceRuntime(context.Background(), Config{RuntimeSourceRoot: root}, nil, cache, lock, spec, deps, errReleaseUnavailable)
			if err == nil || !errors.Is(err, errReleaseUnavailable) || !strings.Contains(err.Error(), "materialize verified Engine bundle") || !strings.Contains(err.Error(), "run make dev in "+root) {
				t.Fatalf("source materialization failure = %v", err)
			}
			if discovered && calls != 0 || !discovered && calls != 1 {
				t.Fatalf("Go-bin resolver calls = %d, discovered=%v", calls, discovered)
			}
			assertRuntimeFile(t, cache, "blocked")
		})
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
