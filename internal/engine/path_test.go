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
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type pathResourceSpy struct {
	gdx.IResMgr
	modes      []bool
	files      map[string]string
	beforeRead func(string)
}

func TestSetAssetDirInstallsOnlyAfterReadingConfiguration(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "interrupted"}[interrupted], func(t *testing.T) {
			resources := setupAssetPathTest(t)
			for _, prefix := range []string{defaultAssetPathPrefix, "", packmodeAssetPrefix} {
				resources.files[projectConfigPath(prefix)] = `{"extasset":"shared"}`
			}
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
			if got := assetPaths.filesystemPath(tt.path); got != normalizeSlashes(filepath.Clean(tt.want)) && !(got == "" && tt.want == "") {
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

func TestLegacyFilesystemRootsRetainBoundedExternalAssets(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	assetDir := filepath.Join(projectDir, "assets")
	for _, dir := range []string{assetDir, filepath.Join(root, "custom_asset"), filepath.Join(root, "shared")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".config"), []byte(`{"extasset":"custom_asset"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"custom_asset/image.png", "shared/image.png"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetLegacyFilesystemRoots(projectDir, assetDir); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "extasset", path: "../../custom_asset/image.png", want: filepath.Join(root, "custom_asset", "image.png")},
		{name: "shared", path: "../../shared/image.png", want: filepath.Join(root, "shared", "image.png")},
		{name: "escape", path: "../../../outside/image.png", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := assetPaths.filesystemPath(test.path); got != normalizeSlashes(filepath.Clean(test.want)) && !(got == "" && test.want == "") {
				t.Fatalf("filesystemPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestReadFilesystemConfigHandlesMissingUnreadableAndInvalidFiles(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, projectConfigFile)
	if got := readExtAssetDirFromFilesystem(root, true); got != "" {
		t.Fatalf("missing config returned %q", got)
	}
	// A directory is reliably unreadable as a file, even when tests run as root.
	if err := os.Mkdir(config, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := readExtAssetDirFromFilesystem(root, true); got != "" {
		t.Fatalf("unreadable config returned %q", got)
	}
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, content, want string }{
		{"invalid JSON", "{", ""},
		{"wrong field type", `{"extasset":1}`, ""},
		{"missing field", `{}`, ""},
		{"configured", `{"extasset":"shared"}`, "shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(config, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := readExtAssetDirFromFilesystem(root, true); got != tc.want {
				t.Fatalf("read config = %q, want %q", got, tc.want)
			}
			if got := readExtAssetDirFromFilesystem(root, false); got != "" {
				t.Fatalf("disabled config returned %q", got)
			}
		})
	}
}

func TestAssetResolversKeepConfigurationSnapshots(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() { assetPaths = original })
	filesystem := newAssetResolver("project/", "assets")
	archive := newAssetResolver(packmodeAssetPrefix, "assets")
	archive.extAssetDir = "shared"
	assetPaths = assetResolver{root: "other/", projectRoot: "other", extAssetDir: "different"}
	if got := filesystem.filesystemPath("cat.png"); got != "project/assets/cat.png" {
		t.Fatalf("filesystem resolver used installed configuration: %q", got)
	}
	if got := archive.packmodePath("cat.png"); got != "res://assets/cat.png" {
		t.Fatalf("archive resolver used installed configuration: %q", got)
	}
	if got := archive.packmodePath("../../shared/cat.png"); got != "res://extasset/cat.png" {
		t.Fatalf("archive resolver lost its external asset configuration: %q", got)
	}
}

func TestBuildFilesystemAssetPath(t *testing.T) {
	resolver := newAssetResolver(defaultAssetPathPrefix, defaultAssetDirName)

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

func TestPackmodePathNormalizesCompatibilityPrefix(t *testing.T) {
	resolver := newAssetResolver(packmodeAssetPrefix, "assets")
	for _, tc := range []struct{ reference, want string }{
		{"../../shared/cat.svg", "res://shared/cat.svg"},
		{"./../../shared//sprites/../cat.svg", "res://shared/cat.svg"},
		{"sprites/../../../shared/cat.svg", "res://shared/cat.svg"},
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

func TestExternalAssetSuffixMatchesOnlyConfiguredParentSegments(t *testing.T) {
	for _, tc := range []struct {
		name, directory, path, want string
		match                       bool
	}{
		{"disabled", "", "../../shared/cat.svg", "", false},
		{"empty", "shared", "", "", false},
		{"parents only", "shared", "../..", "", false},
		{"absolute root", "shared", "/", "", false},
		{"not a parent reference", "shared", "shared/cat.svg", "", false},
		{"different directory", "shared", "../../other/cat.svg", "", false},
		{"prefix is not a segment", "shared", "../../shared-extra/cat.svg", "", false},
		{"one parent", "shared", "../shared/cat.svg", "cat.svg", true},
		{"multiple parents", "shared", "../../../shared/sprites/cat.svg", "sprites/cat.svg", true},
		{"directory itself", "shared", "../../shared", "", true},
		{"normalized suffix", "shared", "../../shared/sprites/../cat.svg", "cat.svg", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := assetResolver{extAssetDir: tc.directory}
			if suffix, ok := resolver.extAssetSuffix(tc.path); suffix != tc.want || ok != tc.match {
				t.Fatalf("external suffix = %q, %v; want %q, %v", suffix, ok, tc.want, tc.match)
			}
		})
	}
}

func (s *pathResourceSpy) SetLoadMode(filesystem bool) { s.modes = append(s.modes, filesystem) }

func (s *pathResourceSpy) HasFile(path string) bool { _, ok := s.files[path]; return ok }

func (s *pathResourceSpy) ReadAllText(path string) string {
	if s.beforeRead != nil {
		s.beforeRead(path)
	}
	return s.files[path]
}

func setupAssetPathTest(t *testing.T) *pathResourceSpy {
	t.Helper()
	previousPaths, previousRes := assetPaths, gdx.ResMgr
	t.Cleanup(func() {
		assetPaths, gdx.ResMgr = previousPaths, previousRes
		wrap.Init(WaitMainThread)
	})
	resources := &pathResourceSpy{files: make(map[string]string)}
	gdx.ResMgr = resources
	wrap.Init(func(call func()) { call() })
	return resources
}
