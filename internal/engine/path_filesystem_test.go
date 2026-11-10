//go:build !packmode

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

package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/goplus/spx/v4/internal/engine/platform"
)

func TestSetAssetDirConfiguresLegacyPathsAndDefaultDirectory(t *testing.T) {
	resources := setupAssetPathTest(t)
	t.Chdir(t.TempDir())
	prefix := defaultAssetPathPrefix
	if platform.IsWeb() {
		prefix = ""
	}
	resources.files[projectConfigPath(prefix)] = `invalid unused config`
	for _, dir := range []string{"", "images"} {
		assetPaths = assetResolver{}
		SetAssetDir(dir)
		wantDir := dir
		if wantDir == "" {
			wantDir = "assets"
		}
		if assetPaths.root != prefix+wantDir+"/" || assetPaths.extAssetDir != "" || assetPaths.compatibilityRoot == "" || assetPaths.explicitFSRoots {
			t.Fatalf("SetAssetDir(%q) = %+v", dir, assetPaths)
		}
	}
	if resources.checks != 0 || resources.reads != 0 {
		t.Fatalf("filesystem setup read unused archive config: checks=%d reads=%d", resources.checks, resources.reads)
	}
	if !slices.Equal(resources.modes, []bool{true, true}) {
		t.Fatalf("resource load modes = %v, want filesystem mode for both calls", resources.modes)
	}
}

func TestSetAssetDirPreservesExplicitRoots(t *testing.T) {
	resources := setupAssetPathTest(t)
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		if err := os.WriteFile(filepath.Join(projectDir, projectConfigFile), []byte(`{"extasset":"initial"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := setFilesystemRoots(projectDir, assetDir, legacy); err != nil {
			t.Fatal(err)
		}
		configured := assetPaths
		if err := os.WriteFile(filepath.Join(projectDir, projectConfigFile), []byte(`{"extasset":"updated"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		SetAssetDir("ignored")
		if assetPaths != configured {
			t.Fatalf("legacy=%v: SetAssetDir changed explicit roots: %+v, want %+v", legacy, assetPaths, configured)
		}
	}
	if resources.checks != 0 || resources.reads != 0 {
		t.Fatalf("filesystem setup read unused archive config: checks=%d reads=%d", resources.checks, resources.reads)
	}
	if !slices.Equal(resources.modes, []bool{true, true}) {
		t.Fatalf("resource load modes = %v, want filesystem mode for both calls", resources.modes)
	}
}

func TestLegacyFilesystemRootsResolveAssetsWithoutSymlinkEscape(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	sessionDir := filepath.Join(projectDir, "project")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetPath := filepath.Join(assetDir, "inside.png")
	if err := os.WriteFile(assetPath, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "outside.png"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDir, filepath.Join(assetDir, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Chdir(sessionDir)
	resolver := newLegacyFilesystemAssetResolver(defaultAssetPathPrefix, "assets", true)
	if got, want := resolver.filesystemPath("inside.png"), filepath.Join("..", "assets", "inside.png"); got != want {
		t.Fatalf("resolver.filesystemPath() = %q, want %q", got, want)
	}
	if got := resolver.filesystemPath("linked/outside.png"); got != "" {
		t.Fatalf("resolver.filesystemPath() followed legacy symlink escape: %q", got)
	}
}
