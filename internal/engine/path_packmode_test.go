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
		{name: "embedded NUL", path: "image\x00.png", want: ""},
		{name: "NUL before cleaning", path: "bad\x00/../image.png", want: ""},
		{name: "project URI NUL", path: "res://bad\x00/../image.png", want: ""},
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
