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

func TestSetAssetDirSelectsOneRoot(t *testing.T) {
	resources := setupAssetPathTest(t)
	project := t.TempDir()
	session := filepath.Join(project, "project")
	for _, dir := range []string{session, filepath.Join(project, "assets")} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(session)
	assetPaths = assetResolver{}
	SetAssetDir("")
	want := "../assets/"
	if platform.IsWeb() {
		want = "assets/"
	}
	if assetPaths.assetRoot != want {
		t.Fatalf("default asset root = %q, want %q", assetPaths.assetRoot, want)
	}
	if err := os.Mkdir("assets", 0o755); err != nil {
		t.Fatal(err)
	}
	SetAssetDir("assets")
	if assetPaths.assetRoot != "assets/" || assetPaths.projectRoot != "." {
		t.Fatalf("local asset root = %+v", assetPaths)
	}
	if !slices.Equal(resources.modes, []bool{true, true}) {
		t.Fatalf("resource load modes = %v", resources.modes)
	}
}

func TestSetAssetDirPreservesExplicitRoots(t *testing.T) {
	setupAssetPathTest(t)
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetFilesystemRoots(projectDir, assetDir); err != nil {
		t.Fatal(err)
	}
	configured := assetPaths
	SetAssetDir("ignored")
	if assetPaths != configured {
		t.Fatalf("SetAssetDir changed explicit roots: %+v, want %+v", assetPaths, configured)
	}
}

func TestGeneratedProjectResolvesAssetsWithoutSymlinkEscape(t *testing.T) {
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
	for link, target := range map[string]string{
		"alias.png": "inside.png", "parent.png": "../project-file.png",
	} {
		if err := os.Symlink(target, filepath.Join(assetDir, link)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project-file.png"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sessionDir)
	resolver := nativeAssetResolver("..", "../assets")
	for name, want := range map[string]string{
		"inside.png": "../assets/inside.png", "alias.png": "../assets/alias.png",
		"parent.png": "", "linked/outside.png": "", "res://assets/alias.png": "",
	} {
		if got := resolver.filesystemPath(name); got != want {
			t.Errorf("filesystemPath(%q) = %q, want %q", name, got, want)
		}
	}
	explicit, err := newFilesystemAssetResolver(projectDir, assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := explicit.filesystemPath("alias.png"); got != "" {
		t.Errorf("explicit root accepted final symlink: %q", got)
	}
}
