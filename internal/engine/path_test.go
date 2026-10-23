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

	"github.com/goplus/spx/v4/internal/enginewrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestBuildFilesystemAssetPath(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	assetPaths = assetPathState{root: "../assets/", projectRoot: ".."}

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
		{name: "reject file URI", path: "file:///tmp/image.png", want: ""},
		{name: "reject malformed res URI", path: "res:res/image.png", want: ""},
		{name: "reject Windows path", path: `C:\outside\image.png`, want: ""},
		{name: "reject UNC path", path: `\\server\share\image.png`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFilesystemAssetPath(tt.path); got != tt.want {
				t.Fatalf("buildFilesystemAssetPath(%q) = %q, want %q", tt.path, got, tt.want)
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
			if got := buildFilesystemAssetPath(tt.path); got != normalizeSlashes(filepath.Clean(tt.want)) && !(got == "" && tt.want == "") {
				t.Fatalf("buildFilesystemAssetPath(%q) = %q, want %q", tt.path, got, tt.want)
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
			if got := buildFilesystemAssetPath(test.path); got != normalizeSlashes(filepath.Clean(test.want)) && !(got == "" && test.want == "") {
				t.Fatalf("buildFilesystemAssetPath(%q) = %q, want %q", test.path, got, test.want)
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

type pathResourceSpy struct {
	gdx.IResMgr
	modes []bool
	files map[string]string
}

func (s *pathResourceSpy) SetLoadMode(filesystem bool)    { s.modes = append(s.modes, filesystem) }
func (s *pathResourceSpy) HasFile(path string) bool       { _, ok := s.files[path]; return ok }
func (s *pathResourceSpy) ReadAllText(path string) string { return s.files[path] }

func setupAssetPathTest(t *testing.T) *pathResourceSpy {
	t.Helper()
	previousPaths, previousRes := assetPaths, gdx.ResMgr
	t.Cleanup(func() {
		assetPaths, gdx.ResMgr = previousPaths, previousRes
		enginewrap.Init(WaitMainThread)
	})
	resources := &pathResourceSpy{files: make(map[string]string)}
	gdx.ResMgr = resources
	enginewrap.Init(func(call func()) { call() })
	return resources
}

func TestExternalAssetSuffixMatchesOnlyConfiguredParentSegments(t *testing.T) {
	previous := assetPaths
	t.Cleanup(func() { assetPaths = previous })
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
			assetPaths.extAssetDir = tc.directory
			if suffix, ok := extAssetSuffix(tc.path); suffix != tc.want || ok != tc.match {
				t.Fatalf("external suffix = %q, %v; want %q, %v", suffix, ok, tc.want, tc.match)
			}
		})
	}
}
