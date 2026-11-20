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
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type pathResourceSpy struct {
	gdx.IResMgr
	modes []bool
}

func TestExplicitFilesystemRoots(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(assetDir, "sprites"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "sprites", "cat.svg"), []byte("svg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(projectDir, "res"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "res", "image.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetFilesystemRoots(projectDir, assetDir); err != nil {
		t.Fatal(err)
	}
	externalFile := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(externalFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(assetDir, "escape.png")
	symlinkAvailable := os.Symlink(externalFile, symlinkPath) == nil
	symlinkDir := filepath.Join(assetDir, "linked")
	symlinkDirAvailable := os.Symlink(filepath.Dir(externalFile), symlinkDir) == nil
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "asset",
			path: "sprites/cat.svg",
			want: filepath.Join(assetDir, "sprites", "cat.svg"),
		},
		{name: "missing asset", path: "sprites/missing.svg"},
		{
			name: "resource elsewhere in project",
			path: "../res/image.png",
			want: filepath.Join(projectDir, "res", "image.png"),
		},
		{
			name: "project resource URI",
			path: "res://res/image.png",
			want: filepath.Join(projectDir, "res", "image.png"),
		},
		{
			name: "reject extasset outside project",
			path: "../../custom_asset/image.png",
			want: "",
		},
		{
			name: "reject legacy shared compatibility root",
			path: "../../shared/image.png",
			want: "",
		},
		{
			name: "reject traversal",
			path: "../../../etc/passwd",
			want: "",
		},
		{
			name: "reject absolute",
			path: filepath.Join(projectDir, "secret"),
			want: "",
		},
		{
			name: "reject symlink outside project",
			path: "escape.png",
			want: "",
		},
		{
			name: "reject intermediate symlink outside project",
			path: "linked/outside.png",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.path == "escape.png" && !symlinkAvailable {
				t.Skip("symlink unavailable")
			}
			if tt.path == "linked/outside.png" && !symlinkDirAvailable {
				t.Skip("symlink unavailable")
			}
			if got := assetPaths.filesystemPath(tt.path); got != filepath.ToSlash(tt.want) {
				t.Fatalf("filesystemPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSetFilesystemRootsRejectsInvalidRootsWithoutChangingState(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() { assetPaths = original })
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetFilesystemRoots(projectDir, assetDir); err != nil {
		t.Fatal(err)
	}
	configured := assetPaths
	for _, tc := range []struct {
		name, project, asset, wantError string
	}{
		{"empty project", "", assetDir, "project root is empty"},
		{"relative project", ".", assetDir, "is not absolute"},
		{"unclean project", projectDir + string(os.PathSeparator) + ".", assetDir, "is not clean"},
		{"empty asset", projectDir, "", "asset root is empty"},
		{"relative asset", projectDir, "assets", "is not absolute"},
		{"unclean asset", projectDir, assetDir + string(os.PathSeparator) + ".", "is not clean"},
		{"missing project", filepath.Join(projectDir, "missing"), assetDir, "canonicalize project root"},
		{"missing asset", projectDir, filepath.Join(projectDir, "missing"), "canonicalize asset root"},
		{"outside project", projectDir, t.TempDir(), "must be within project root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := SetFilesystemRoots(tc.project, tc.asset); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("SetFilesystemRoots() error = %v, want %q", err, tc.wantError)
			}
			if assetPaths != configured {
				t.Fatalf("rejected roots changed active configuration: %+v", assetPaths)
			}
		})
	}
}

func TestAssetResolversKeepConfigurationSnapshots(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() { assetPaths = original })
	filesystem := newAssetResolver("project/", "assets")
	archive := newAssetResolver(projectResourcePrefix, "assets")
	assetPaths = assetResolver{assetRoot: "other/", projectRoot: "other"}
	if got := filesystem.filesystemPath("cat.png"); got != "project/assets/cat.png" {
		t.Fatalf("filesystem resolver used installed configuration: %q", got)
	}
	if got := archive.packmodePath("cat.png"); got != "res://assets/cat.png" {
		t.Fatalf("archive resolver used installed configuration: %q", got)
	}

}

func TestFilesystemAssetPathColonFilename(t *testing.T) {
	root := t.TempDir()
	resolver, err := newFilesystemAssetResolver(root, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"costume-Animation:1.svg", "sprites/Media/res:image.png", "./res:image.png"} {
		want := ""
		if runtime.GOOS != "windows" {
			file := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("asset"), 0o644); err != nil {
				t.Fatal(err)
			}
			want = filepath.ToSlash(file)
		}
		for _, reference := range []string{name, "res://" + name} {
			if got := resolver.filesystemPath(reference); got != want {
				t.Errorf("filesystemPath(%q) = %q, want %q", reference, got, want)
			}
		}
	}
}

func TestFilesystemAssetPathRejectsGodotSeparatorAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslashes cannot be part of Windows filenames")
	}
	project := t.TempDir()
	for _, dir := range []string{"assets", `assets/字:句\一`, `assets\dir`, `project\dir/assets`} {
		if err := os.MkdirAll(filepath.Join(project, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ project, assets, name string }{
		{".", "assets", `字:句\一/image.png`},
		{".", `assets\dir`, "image.png"},
		{`project\dir`, `project\dir/assets`, "image.png"},
	} {
		assets := filepath.Join(project, tc.assets)
		if err := os.WriteFile(filepath.Join(assets, tc.name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		resolver, err := newFilesystemAssetResolver(filepath.Join(project, tc.project), assets)
		if err != nil {
			t.Fatal(err)
		}
		if got := resolver.filesystemPath(tc.name); got != "" {
			t.Errorf("Godot path = %q for %+v", got, tc)
		}
	}
}

func TestBuildFilesystemAssetPath(t *testing.T) {
	resolver := newAssetResolver("../", defaultAssetDirName)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "keep asset path inside root",
			path: "image.png",
			want: "../assets/image.png",
		},
		{
			name: "clean nested path inside root",
			path: "sprites/../image.png",
			want: "../assets/image.png",
		},
		{
			name: "allow resource elsewhere in project",
			path: "../res/image.png",
			want: "../res/image.png",
		},
		{
			name: "allow canonical project resource URI",
			path: "res://res/image.png",
			want: "../res/image.png",
		},
		{
			name: "reject historical shared asset outside project",
			path: "../../shared-assets/image.png",
			want: "",
		},
		{
			name: "reject parent traversal",
			path: "../../../../etc/passwd",
			want: "",
		},
		{name: "reject empty reference", path: "", want: ""},
		{name: "reject empty resource URI", path: "res://", want: ""},
		{name: "reject embedded NUL", path: "image\x00.png", want: ""},
		{name: "reject NUL before cleaning", path: "bad\x00/../image.png", want: ""},
		{name: "reject NUL in project URI", path: "res://bad\x00/../image.png", want: ""},
		{name: "reject file URI", path: "file:///tmp/image.png", want: ""},
		{name: "reject malformed res URI", path: "res:res/image.png", want: ""},
		{name: "reject Windows drive absolute path", path: "C:/outside/image.png", want: ""},
		{name: "reject Windows drive absolute project URI", path: "res://z:/outside/image.png", want: ""},
		{name: "reject Windows path", path: `C:\outside\image.png`, want: ""},
		{name: "reject UNC path", path: `\\server\share\image.png`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolver.filesystemPath(tt.path); got != tt.want {
				t.Fatalf("filesystemPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilesystemPathRejectsUnresolvedRoots(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name     string
		resolver assetResolver
	}{
		{"absolute assets without project root", newAssetResolver("", root)},
		{"missing canonical project", nativeAssetResolver(filepath.Join(root, "missing"), filepath.Join(root, "missing", "assets"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.resolver.filesystemPath("image.png"); got != "" {
				t.Fatalf("filesystemPath() = %q, want unresolved root rejected", got)
			}
		})
	}
}

func TestToAssetPathRejectsNULBeforeNormalization(t *testing.T) {
	setupAssetPathTest(t)
	assetPaths = assetResolver{}
	SetAssetDir("assets")
	for _, reference := range []string{
		"image\x00.png",
		"bad\x00/../image.png",
		"res://bad\x00/../image.png",
		"../../shared/bad\x00/../image.png",
	} {
		if got := ToAssetPath(reference); got != "" {
			t.Errorf("ToAssetPath(%q) = %q, want rejected reference", reference, got)
		}
	}
}

func TestPackmodePathRejectsEscapeAfterCleaning(t *testing.T) {
	resolver := newAssetResolver(projectResourcePrefix, "assets")
	for _, tc := range []struct{ reference, want string }{
		{"../../shared/cat.svg", ""},
		{"./../../shared//sprites/../cat.svg", ""},
		{"sprites/../../../shared/cat.svg", ""},
		{"../res/cat.svg", "res://res/cat.svg"},
		{"../..", ""},
		{"../../", ""},
		{"../../shared/..", ""},
		{"../../../shared/cat.svg", ""},
		{"res://../../shared/cat.svg", ""},
	} {
		t.Run(tc.reference, func(t *testing.T) {
			if got := resolver.packmodePath(tc.reference); got != tc.want {
				t.Fatalf("packmodePath(%q) = %q, want %q", tc.reference, got, tc.want)
			}
		})
	}
}

func (s *pathResourceSpy) SetLoadMode(filesystem bool) { s.modes = append(s.modes, filesystem) }

func setupAssetPathTest(t *testing.T) *pathResourceSpy {
	t.Helper()
	previousPaths, previousRes := assetPaths, gdx.ResMgr
	t.Cleanup(func() {
		assetPaths, gdx.ResMgr = previousPaths, previousRes
		wrap.Init(WaitMainThread)
	})
	resources := &pathResourceSpy{}
	gdx.ResMgr = resources
	wrap.Init(func(call func()) { call() })
	return resources
}
