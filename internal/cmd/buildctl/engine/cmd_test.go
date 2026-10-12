/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package engine

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func TestParseEngineDownloadArgs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		want    DownloadConfig
		wantErr error
		message string
	}{
		{name: "defaults"},
		{name: "runtime", args: []string{"--runtime"}, want: DownloadConfig{Runtime: true}},
		{
			name: "same-run runtime without pack",
			args: []string{"--runtime", "--skip-runtime-pack", "--asset-dir", "artifacts/./runtime", "--same-run-artifacts"},
			want: DownloadConfig{Runtime: true, SkipRuntimePack: true, AssetDir: filepath.Clean("artifacts/runtime"), SameRunArtifacts: true},
		},
		{name: "web default", args: []string{"--platform", "web"}, want: DownloadConfig{Platform: "web", Mode: "normal"}},
		{name: "web worker", args: []string{"--platform", "web", "--mode", "worker"}, want: DownloadConfig{Platform: "web", Mode: "worker"}},
		{name: "desktop", args: []string{"--platform", "linux"}, want: DownloadConfig{Platform: "linux"}},
		{name: "skip pack without runtime", args: []string{"--skip-runtime-pack"}, message: "--skip-runtime-pack requires --runtime"},
		{name: "same-run without directory", args: []string{"--same-run-artifacts"}, message: "--same-run-artifacts requires --asset-dir"},
		{name: "runtime with platform", args: []string{"--runtime", "--platform", "linux"}, message: "--runtime cannot be combined with --platform"},
		{name: "runtime with mode", args: []string{"--runtime", "--mode", "worker"}, message: "--runtime cannot be combined with --mode"},
		{name: "mode without web", args: []string{"--platform", "android", "--mode", "worker"}, message: "--mode requires --platform web"},
		{name: "invalid platform", args: []string{"--platform", "invalid"}, message: "platform"},
		{name: "invalid web mode", args: []string{"--platform", "web", "--mode", "invalid"}, message: "mode"},
		{name: "unknown flag", args: []string{"--unknown"}, message: "flag provided but not defined"},
		{name: "missing flag value", args: []string{"--asset-dir"}, message: "flag needs an argument"},
		{name: "positional argument", args: []string{"--runtime", "unexpected"}, wantErr: errUsage},
		{name: "help", args: []string{"--help"}, wantErr: flag.ErrHelp},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseEngineDownloadArgs(tt.args)
			wantFailure := tt.wantErr != nil || tt.message != ""
			if (err != nil) != wantFailure {
				t.Fatalf("parseEngineDownloadArgs error = %v, want failure = %v", err, wantFailure)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("parseEngineDownloadArgs error = %v, want %v", err, tt.wantErr)
			}
			if tt.message != "" && !strings.Contains(err.Error(), tt.message) {
				t.Errorf("parseEngineDownloadArgs error = %v, want %q", err, tt.message)
			}
			if cfg != tt.want {
				t.Errorf("config = %#v, want %#v", cfg, tt.want)
			}
		})
	}
}

func TestFindLocalEngineAsset(t *testing.T) {
	for _, tt := range []struct {
		name, asset, want, message string
		files                      []string
		missingRoot                bool
		wantErr                    error
	}{
		{name: "direct", asset: "web.zip", files: []string{"web.zip"}, want: "web.zip"},
		{name: "nested", asset: "web.zip", files: []string{"one/web.zip"}, want: "one/web.zip"},
		{name: "direct takes precedence", asset: "web.zip", files: []string{"web.zip", "one/web.zip", "two/web.zip"}, want: "web.zip"},
		{name: "ambiguous", asset: "web.zip", files: []string{"one/web.zip", "two/web.zip"}, message: "ambiguous"},
		{name: "missing", asset: "web.zip", wantErr: os.ErrNotExist},
		{name: "directory is not an asset", asset: "web.zip", files: []string{"web.zip/marker"}, wantErr: os.ErrNotExist},
		{name: "missing root", asset: "web.zip", missingRoot: true, wantErr: os.ErrNotExist, message: "scan engine asset directory"},
		{name: "empty name", message: "invalid engine asset name"},
		{name: "dot", asset: ".", message: "invalid engine asset name"},
		{name: "parent", asset: "..", message: "invalid engine asset name"},
		{name: "traversal", asset: "../web.zip", message: "invalid engine asset name"},
		{name: "nested name", asset: "one/web.zip", files: []string{"one/web.zip"}, message: "invalid engine asset name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range tt.files {
				mustWriteFile(t, filepath.Join(root, name), []byte(name))
			}
			if tt.missingRoot {
				root = filepath.Join(root, "missing")
			}
			got, err := findLocalEngineAsset(root, tt.asset)
			wantFailure := tt.wantErr != nil || tt.message != ""
			if (err != nil) != wantFailure {
				t.Fatalf("findLocalEngineAsset error = %v, want failure = %v", err, wantFailure)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("findLocalEngineAsset error = %v, want %v", err, tt.wantErr)
			}
			if tt.message != "" && !strings.Contains(err.Error(), tt.message) {
				t.Errorf("findLocalEngineAsset error = %v, want %q", err, tt.message)
			}
			want := ""
			if tt.want != "" {
				want = filepath.Join(root, tt.want)
			}
			if got != want {
				t.Errorf("asset path = %q, want %q", got, want)
			}
		})
	}
}

func TestSetLocalAssetDir(t *testing.T) {
	for _, tt := range []struct {
		name, path, message string
		absolute, allow     bool
		wantErr             error
	}{
		{name: "relative", path: "assets"},
		{name: "cleaned", path: "assets/../assets", allow: true},
		{name: "absolute", absolute: true, allow: true},
		{name: "missing", path: "missing", wantErr: os.ErrNotExist, message: "open engine asset directory"},
		{name: "regular file", path: "plain", message: "engine asset path is not a directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := newEngineDownloadFixture(t, "")
			root := filepath.Dir(env.assetDir)
			mustWriteFile(t, filepath.Join(root, "plain"), []byte("not a directory"))
			path := tt.path
			if tt.absolute {
				path = env.assetDir
			}
			env.allowMissingManifest = !tt.allow
			err := setLocalAssetDir(&env, root, path, tt.allow)
			wantFailure := tt.wantErr != nil || tt.message != ""
			if (err != nil) != wantFailure {
				t.Fatalf("setLocalAssetDir error = %v, want failure = %v", err, wantFailure)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("setLocalAssetDir error = %v, want %v", err, tt.wantErr)
			}
			if tt.message != "" && !strings.Contains(err.Error(), tt.message) {
				t.Errorf("setLocalAssetDir error = %v, want %q", err, tt.message)
			}
			if !wantFailure && (env.assetDir != filepath.Join(root, "assets") || env.allowMissingManifest != tt.allow) {
				t.Errorf("local asset settings = (%q, %v), want (%q, %v)", env.assetDir, env.allowMissingManifest, filepath.Join(root, "assets"), tt.allow)
			}
		})
	}
}

func TestLoadEngineAssetManifestRequiresExplicitSameRunTrust(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		allow         bool
		wantFailure   bool
	}{
		{name: "missing manifest", wantFailure: true},
		{name: "same-run missing manifest", allow: true},
		{name: "malformed manifest", content: "not JSON", wantFailure: true},
		{name: "same-run malformed manifest", content: "not JSON", allow: true, wantFailure: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := newEngineDownloadFixture(t, "")
			lock := release.DefaultRuntimeLock()
			env.version, env.allowMissingManifest = lock.RuntimeVersion, tt.allow
			path := filepath.Join(env.assetDir, lock.Manifest)
			if tt.content != "" {
				mustWriteFile(t, path, []byte(tt.content))
			}
			err := loadEngineAssetManifest(&env)
			if (err != nil) != tt.wantFailure {
				t.Fatalf("loadEngineAssetManifest error = %v, want failure = %v", err, tt.wantFailure)
			}
			if tt.content == "" && tt.wantFailure && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("missing manifest error = %v, want ErrNotExist", err)
			}
			if env.manifest != nil {
				t.Fatalf("missing or malformed manifest was installed: %#v", env.manifest)
			}
			assertEngineDownloadFiles(t, env, nil)
			if tt.content != "" {
				if data, err := os.ReadFile(path); err != nil || string(data) != tt.content {
					t.Errorf("local manifest = %q, err = %v; want source preserved", data, err)
				}
			}
		})
	}
}

func TestFetchEngineAssetVerifiesBeforeReplacing(t *testing.T) {
	const verified = "verified"
	for _, source := range []string{"local", "http"} {
		t.Run(source, func(t *testing.T) {
			for _, tt := range []struct {
				name, content, message string
				unpublished, missing   bool
			}{
				{name: "verified", content: verified},
				{name: "wrong size", content: "short", message: "size ="},
				{name: "wrong digest", content: "tampered", message: "SHA-256 ="},
				{name: "unpublished asset", content: verified, unpublished: true, message: "not present in the manifest"},
				{name: "missing source", missing: true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					env := newEngineDownloadFixture(t, "")
					manifest := newEngineAssetManifestFixture(t, verified)
					env.manifest = &manifest
					name := manifest.Assets[0].Name
					if tt.unpublished {
						name = "unpublished.zip"
					}
					src := filepath.Join(env.assetDir, name)
					if !tt.missing {
						mustWriteFile(t, src, []byte(tt.content))
					}
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.URL.Path != "/"+name {
							t.Errorf("request path = %q, want /%s", r.URL.Path, name)
							http.NotFound(w, r)
							return
						}
						if tt.missing {
							http.NotFound(w, r)
							return
						}
						_, _ = w.Write([]byte(tt.content))
					}))
					defer server.Close()
					var wantRequests int32
					if source == "http" {
						env.assetDir, wantRequests = "", 1
					}
					dst := filepath.Join(env.goBinDir, "asset.zip")
					mustWriteFile(t, dst, []byte("installed"))
					err := fetchEngineAsset(env, name, server.URL+"/"+name, dst)
					wantFailure := tt.missing || tt.message != ""
					if (err != nil) != wantFailure {
						t.Fatalf("fetchEngineAsset error = %v, want failure = %v", err, wantFailure)
					}
					if tt.message != "" && !strings.Contains(err.Error(), tt.message) {
						t.Errorf("fetchEngineAsset error = %v, want %q", err, tt.message)
					}
					if source == "local" && tt.missing && !errors.Is(err, os.ErrNotExist) {
						t.Errorf("missing local asset error = %v, want ErrNotExist", err)
					}
					want := verified
					if wantFailure {
						want = "installed"
					}
					assertEngineDownloadFiles(t, env, map[string]string{dst: want})
					if got := requests.Load(); got != wantRequests {
						t.Errorf("HTTP requests = %d, want %d", got, wantRequests)
					}
					if !tt.missing {
						if data, err := os.ReadFile(src); err != nil || string(data) != tt.content {
							t.Errorf("source content = %q, err = %v; want %q", data, err, tt.content)
						}
					}
				})
			}
		})
	}
}

func TestCopyEngineAssetAtomically(t *testing.T) {
	for _, scenario := range []string{"replace", "missing source", "directory source", "directory destination", "file parent"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			src, dir := filepath.Join(root, "source"), filepath.Join(root, "output")
			dst := filepath.Join(dir, "asset")
			if scenario == "directory source" {
				mustMkdirAll(t, src)
			} else if scenario != "missing source" {
				mustWriteFile(t, src, []byte("replacement"))
				if err := os.Chmod(src, 0o751); err != nil {
					t.Fatal(err)
				}
			}
			preserved := dst
			switch scenario {
			case "directory destination":
				preserved = filepath.Join(dst, "keep")
				mustWriteFile(t, preserved, []byte("installed"))
			case "file parent":
				preserved = dir
				mustWriteFile(t, preserved, []byte("installed"))
			default:
				mustWriteFile(t, preserved, []byte("installed"))
			}
			err := copyEngineAssetAtomically(src, dst)
			wantFailure := scenario != "replace"
			if (err != nil) != wantFailure {
				t.Fatalf("copyEngineAssetAtomically error = %v, want failure = %v", err, wantFailure)
			}
			if scenario == "missing source" && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("missing source error = %v, want ErrNotExist", err)
			}
			want, path := "replacement", dst
			if wantFailure {
				want, path = "installed", preserved
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != want {
				t.Errorf("destination = %q, err = %v; want %q", data, err, want)
			}
			if scenario != "missing source" && scenario != "directory source" {
				if data, err := os.ReadFile(src); err != nil || string(data) != "replacement" {
					t.Errorf("source = %q, err = %v; want source preserved", data, err)
				}
			}
			if !wantFailure && runtime.GOOS != "windows" {
				if info, err := os.Stat(dst); err != nil {
					t.Fatal(err)
				} else if info.Mode().Perm() != 0o751 {
					t.Errorf("copied mode = %o, want 751", info.Mode().Perm())
				}
			}
			wantEntries := 1
			if scenario == "file parent" {
				dir, wantEntries = root, 2
			}
			// Inspect the entire output directory, not only one temporary-file prefix.
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != wantEntries {
				t.Errorf("output directory = %v, err = %v; want no temporary files", entries, err)
			}
		})
	}
}

func TestReadRuntimeManifestBoundaries(t *testing.T) {
	for _, size := range []int64{0, maxRuntimeManifestBytes, maxRuntimeManifestBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "manifest.json")
			content := strings.Repeat("x", int(size))
			mustWriteFile(t, path, []byte(content))
			data, err := readRuntimeManifest(path)
			if size > maxRuntimeManifestBytes {
				if !errors.Is(err, runtimebundle.ErrArchiveLimit) {
					t.Fatalf("readRuntimeManifest error = %v, want ErrArchiveLimit", err)
				}
			} else if err != nil || string(data) != content {
				t.Fatalf("readRuntimeManifest returned %d bytes, err = %v; want %d exact bytes", len(data), err, size)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		if _, err := readRuntimeManifest(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("readRuntimeManifest error = %v, want ErrNotExist", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		if _, err := readRuntimeManifest(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("readRuntimeManifest error = %v, want non-regular file rejection", err)
		}
	})
}

func TestDownloadRuntimePackRejectsMissingPCK(t *testing.T) {
	env := newEngineDownloadFixture(t, "")
	want := map[string]string{
		filepath.Join(env.goBinDir, "gdspxrttest.pck"):     "stable pack",
		filepath.Join(env.goBinDir, "runtime.gdextension"): "stable extension",
	}
	for path, content := range want {
		mustWriteFile(t, path, []byte(content))
	}
	if err := writeZipFixture(filepath.Join(env.assetDir, release.RuntimeAssetZipName), map[string]string{"runtime.gdextension": "incomplete"}); err != nil {
		t.Fatal(err)
	}
	if err := downloadRuntimePack(env); err == nil || !strings.Contains(err.Error(), "missing gdspxrt.pck") {
		t.Fatalf("downloadRuntimePack error = %v, want missing pck error", err)
	}
	assertEngineDownloadFiles(t, env, want)
}

func TestDownloadAndroidAssetsValidatesBeforeInstalling(t *testing.T) {
	assets := []string{"android_debug.apk", "android_release.apk", "android_source.zip"}
	for _, invalid := range []string{"missing", "directory"} {
		for _, name := range assets {
			t.Run(invalid+"/"+name, func(t *testing.T) {
				env := newEngineDownloadFixture(t, "android")
				files := make(map[string]string, len(assets))
				want := make(map[string]string, len(assets))
				for _, asset := range assets {
					files[asset] = "replacement:" + asset
					path := filepath.Join(env.templateDir, asset)
					want[path] = "stable:" + asset
					mustWriteFile(t, path, []byte(want[path]))
				}
				delete(files, name)
				message := "missing " + name
				if invalid == "directory" {
					files[name+"/"] = ""
					message = "entry " + name + " is not a regular file"
				}
				archive := filepath.Join(env.assetDir, "android.zip")
				if err := writeZipFixture(archive, files); err != nil {
					t.Fatal(err)
				}
				err := downloadAndroidAssets(env)
				if err == nil || !strings.Contains(err.Error(), message) {
					t.Fatalf("downloadAndroidAssets error = %v, want %q", err, message)
				}
				if invalid == "missing" && !errors.Is(err, os.ErrNotExist) {
					t.Errorf("downloadAndroidAssets error = %v, want ErrNotExist", err)
				}
				assertEngineDownloadFiles(t, env, want)
				if _, err := os.Stat(archive); err != nil {
					t.Errorf("source archive was removed: %v", err)
				}
			})
		}
	}
}

func newEngineAssetManifestFixture(t *testing.T, content string) release.RuntimeManifest {
	t.Helper()
	lock := release.DefaultRuntimeLock()
	manifest := release.RuntimeManifest{
		Schema: release.RuntimeManifestSchema, RuntimeVersion: lock.RuntimeVersion,
		RuntimeABI: lock.RuntimeABI, ReleaseRepository: "example/runtime", LockSHA256: strings.Repeat("1", 64),
		Provenance: release.RuntimeProvenance{
			SPXCommit: strings.Repeat("2", 40), GodotCommit: strings.Repeat("3", 40), ModuleTree: strings.Repeat("4", 40),
			RuntimePackSourceSHA256: strings.Repeat("5", 64), BuildRecipeSHA256: strings.Repeat("6", 64), Toolchain: lock.Toolchain,
		},
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	for _, name := range lock.RequiredAssets {
		manifest.Assets = append(manifest.Assets, release.RuntimeAsset{Name: name, Size: int64(len(content)), SHA256: digest})
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("invalid manifest fixture: %v", err)
	}
	return manifest
}

func writeZipFixture(dst string, files map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			return err
		}
	}
	return writer.Close()
}
