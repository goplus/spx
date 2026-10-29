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

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

func TestSetupWebBundleInstallsOnce(t *testing.T) {
	for _, source := range []string{"same-run", "published"} {
		for _, mode := range []string{"normal", "worker", "minigame", "miniprogram"} {
			t.Run(source+"/"+mode, func(t *testing.T) {
				root, assets := newWebBundleFixture(t)
				requests := serveWebBundleAssets(t, assets)
				assetDir := assets
				if source == "published" {
					writeWebBundleManifest(t, assets)
					assetDir = ""
				}
				runner := &setupTestRunner{t: t, root: root}
				if err := setupAssets(setupConfig{target: "web", mode: mode, assetDir: assetDir, exportBundle: true}, runner); err != nil {
					t.Fatal(err)
				}
				wantScripts := [][]string{{"cmd/spx/install.sh", "--web", "--no-embed-runtime"}}
				if !reflect.DeepEqual(runner.scripts, wantScripts) {
					t.Fatalf("installations = %v, want exactly %v", runner.scripts, wantScripts)
				}
				spec, err := shared.ResolveWebMode(mode)
				if err != nil {
					t.Fatal(err)
				}
				if want := []string{"exporttemplateweb", spec.ExportCommand}; !reflect.DeepEqual(runner.commands, want) {
					t.Fatalf("exports = %v, want %v", runner.commands, want)
				}
				bundle, err := zip.OpenReader(filepath.Join(root, spec.ExportZip))
				if err != nil {
					t.Fatal(err)
				}
				defer bundle.Close()
				if len(bundle.File) != 1 || bundle.File[0].Name != "index.html" {
					t.Fatalf("unexpected bundle contents: %v", bundle.File)
				}
				lock := release.DefaultRuntimeLock()
				cached := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx"+lock.RuntimeVersion+"_"+spec.CacheSuffix)
				if content, err := os.ReadFile(cached); err != nil || string(content) != spec.ReleaseTemplate {
					t.Fatalf("selected Web template = %q, %v", content, err)
				}
				if source == "same-run" && len(*requests) != 0 {
					t.Fatalf("same-run assets made published requests: %v", *requests)
				}
				if source == "published" && (!slices.Contains(*requests, lock.Manifest) || !slices.Contains(*requests, spec.ReleaseTemplate)) {
					t.Fatalf("published asset requests = %v", *requests)
				}
			})
		}
	}
}

func TestSetupWebBundleStopsOnFailure(t *testing.T) {
	for _, stage := range []string{"download", "install", "exporttemplateweb", "exportweb"} {
		t.Run(stage, func(t *testing.T) {
			root, assets := newWebBundleFixture(t)
			failure := errors.New("fixture failure")
			runner := &setupTestRunner{t: t, root: root}
			var wantCommands []string
			wantInstalls := 1
			switch stage {
			case "download":
				if err := os.Remove(filepath.Join(assets, "web.zip")); err != nil {
					t.Fatal(err)
				}
				wantInstalls = 0
			case "install":
				runner.scriptErr = failure
			case "exporttemplateweb":
				runner.commandErrors = map[string]error{stage: failure}
				wantCommands = []string{stage}
			case "exportweb":
				runner.commandErrors = map[string]error{stage: failure}
				wantCommands = []string{"exporttemplateweb", stage}
			}
			err := setupAssets(setupConfig{target: "web", mode: "normal", assetDir: assets, exportBundle: true}, runner)
			if err == nil || (stage != "download" && !errors.Is(err, failure)) {
				t.Fatalf("error = %v, want failure at %s", err, stage)
			}
			if len(runner.scripts) != wantInstalls || !reflect.DeepEqual(runner.commands, wantCommands) {
				t.Fatalf("installations = %v, exports = %v", runner.scripts, runner.commands)
			}
			if _, err := os.Stat(filepath.Join(root, "spx_web.zip")); !os.IsNotExist(err) {
				t.Fatalf("failed build produced a bundle: %v", err)
			}
		})
	}
}

func newWebBundleFixture(t *testing.T) (root, assets string) {
	t.Helper()
	root = newBuildctlFixture(t)
	assets = filepath.Join(root, "assets")
	lock := release.DefaultRuntimeLock()
	for _, name := range lock.RequiredAssets {
		mustWriteFile(t, filepath.Join(assets, name), []byte(name))
	}
	env, err := shared.ResolveBuildEnvironment(root, "")
	if err != nil {
		t.Fatal(err)
	}
	platform := env.Platform
	if platform == "linux" {
		platform = "linuxbsd"
	}
	binary := "godot." + platform + ".editor." + env.Arch
	if platform == "windows" {
		binary += ".exe"
	}
	binaryPath := filepath.Join(root, binary)
	mustWriteFile(t, binaryPath, []byte("editor"))
	archive := filepath.Join(assets, "editor-"+env.Platform+"-"+env.Arch+".zip")
	if err := shared.WriteNamedZip(archive, map[string]string{binary: binaryPath}); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("extension"))
	return root, assets
}

func writeWebBundleManifest(t *testing.T, assets string) {
	t.Helper()
	lock := release.DefaultRuntimeLock()
	var inputs []release.RuntimeAssetInput
	for _, name := range lock.RequiredAssets {
		inputs = append(inputs, release.RuntimeAssetInput{Name: name, Path: filepath.Join(assets, name)})
	}
	manifest, err := release.GenerateRuntimeManifest(lock, release.RuntimeProvenance{
		SPXCommit: strings.Repeat("1", 40), GodotCommit: lock.Godot.Commit,
		ModuleTree: strings.Repeat("2", 40), RuntimePackSourceSHA256: strings.Repeat("3", 64),
		BuildRecipeSHA256: strings.Repeat("4", 64), Toolchain: lock.Toolchain,
	}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := release.WriteRuntimeManifest(filepath.Join(assets, lock.Manifest), manifest); err != nil {
		t.Fatal(err)
	}
}

type webBundleTransport func(*http.Request) (*http.Response, error)

func (f webBundleTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func serveWebBundleAssets(t *testing.T, assets string) *[]string {
	t.Helper()
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	var requests []string
	http.DefaultTransport = webBundleTransport(func(req *http.Request) (*http.Response, error) {
		lock := release.DefaultRuntimeLock()
		name := strings.TrimPrefix(req.URL.String(), lock.RuntimeAssetDownloadURL(""))
		if filepath.Base(name) != name {
			return nil, fmt.Errorf("unexpected asset request: %s", req.URL)
		}
		requests = append(requests, name)
		data, err := os.ReadFile(filepath.Join(assets, name))
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req}, nil
	})
	return &requests
}
