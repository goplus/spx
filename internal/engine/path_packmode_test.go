//go:build packmode

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
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPackmodeAssetPathStaysWithinProject(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})
	assetPaths = assetResolver{
		root:        joinAssetRoot(packmodeAssetPrefix, "assets"),
		projectRoot: packmodeAssetPrefix,
		extAssetDir: "custom_asset",
	}

	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "asset", path: "sprites/cat.svg", want: "res://assets/sprites/cat.svg"},
		{name: "project resource", path: "../res/image.png", want: "res://res/image.png"},
		{name: "project URI", path: "res://media/image.png", want: "res://media/image.png"},
		{name: "shared legacy asset", path: "../../shared/image.png", want: "res://shared/image.png"},
		{name: "extasset legacy asset", path: "../../custom_asset/image.png", want: "res://extasset/image.png"},
		{name: "escaping project URI", path: "res://../outside/image.png", want: ""},
		{name: "Windows absolute project URI", path: "res://C:/outside/image.png", want: ""},
		{name: "triple slash project URI", path: "res:///etc/passwd", want: ""},
		{name: "malformed project URI", path: "res:media/image.png", want: ""},
		{name: "backslash project URI", path: `res:\media\image.png`, want: ""},
		{name: "UNC path", path: `\\server\share\image.png`, want: ""},
		{name: "absolute", path: "/tmp/image.png", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ToAssetPath(test.path); got != test.want {
				t.Fatalf("ToAssetPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestSetAssetDirConfiguresPackmodeRoots(t *testing.T) {
	resources := setupAssetPathTest(t)
	resources.files["res://.config"] = `{"extasset":"shared"}`
	for _, dir := range []string{"", "images"} {
		assetPaths = assetResolver{explicitFSRoots: true}
		SetAssetDir(dir)
		wantDir := dir
		if wantDir == "" {
			wantDir = "assets"
		}
		if assetPaths.root != "res://"+wantDir+"/" || assetPaths.projectRoot != "res://" || assetPaths.extAssetDir != "shared" || assetPaths.explicitFSRoots {
			t.Fatalf("SetAssetDir(%q) = %+v", dir, assetPaths)
		}
	}
	if !slices.Equal(resources.modes, []bool{false, false}) {
		t.Fatalf("resource load modes = %v, want archive mode for both calls", resources.modes)
	}
}

func TestSetAssetDirInstallsOnlyAfterReadingConfiguration(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "interrupted"}[interrupted], func(t *testing.T) {
			resources := setupAssetPathTest(t)
			resources.files[projectConfigPath(packmodeAssetPrefix)] = `{"extasset":"shared"}`
			previous := assetResolver{root: "previous/assets/", projectRoot: "previous"}
			assetPaths = previous
			read := false
			resources.beforeRead = func(string) {
				read = true
				if assetPaths != previous {
					t.Error("configuration changed before the project config was read")
				}
				if interrupted {
					panic("configuration interrupted")
				}
			}
			func() {
				defer func() {
					got := recover()
					if interrupted && got != "configuration interrupted" || !interrupted && got != nil {
						t.Errorf("configuration panic = %v, interrupted = %v", got, interrupted)
					}
				}()
				SetAssetDir("next")
			}()
			if !read {
				t.Fatal("project configuration was not read")
			}
			if interrupted {
				if assetPaths != previous {
					t.Fatal("interrupted configuration changed the installed resolver")
				}
			} else if assetPaths.root == previous.root || assetPaths.extAssetDir != "shared" {
				t.Fatalf("completed configuration was not installed: %+v", assetPaths)
			}
		})
	}
}

func TestPackmodePreservesLegacyConfigUntilAssetDirectoryIsSet(t *testing.T) {
	resources := setupAssetPathTest(t)
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, projectConfigFile), []byte(`{"extasset":"shared"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetLegacyFilesystemRoots(projectDir, assetDir); err != nil {
		t.Fatal(err)
	}
	if got := ToAssetPath("../../shared/cat.svg"); got != "res://extasset/cat.svg" {
		t.Fatalf("legacy config path = %q, want res://extasset/cat.svg", got)
	}
	resources.files[projectConfigPath(packmodeAssetPrefix)] = `{"extasset":"archive"}`
	SetAssetDir("assets")
	for reference, want := range map[string]string{
		"../../shared/cat.svg":  "res://shared/cat.svg",
		"../../archive/cat.svg": "res://extasset/cat.svg",
	} {
		if got := ToAssetPath(reference); got != want {
			t.Fatalf("archive path %q = %q, want %q", reference, got, want)
		}
	}
}
