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
	"testing"
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

func TestSetFilesystemRootsRejectsImplicitPaths(t *testing.T) {
	if err := SetFilesystemRoots(".", "assets"); err == nil {
		t.Fatal("SetFilesystemRoots accepted relative paths")
	}
}

func TestSetFilesystemRootsRejectsAssetOutsideProject(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := t.TempDir()
	if err := SetFilesystemRoots(projectDir, assetDir); err == nil {
		t.Fatal("SetFilesystemRoots accepted AssetDir outside ProjectDir")
	}
}
