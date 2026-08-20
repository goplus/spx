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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/interpruntime"
)

func TestValidateAssetIndex(t *testing.T) {
	assetDir := t.TempDir()
	indexPath := filepath.Join(assetDir, "index.json")
	if err := os.WriteFile(indexPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateAssetIndex(assetDir); err != nil {
		t.Fatalf("validateAssetIndex() error = %v", err)
	}
}

func TestValidateAssetIndexRejectsMissingAndSymlink(t *testing.T) {
	if err := validateAssetIndex(t.TempDir()); err == nil {
		t.Fatal("validateAssetIndex accepted a missing index")
	}

	assetDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "index.json")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(assetDir, "index.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := validateAssetIndex(assetDir); err == nil {
		t.Fatal("validateAssetIndex accepted a symlink index")
	}
}

func TestValidateAssetIndexAcceptsPackedOnly(t *testing.T) {
	assetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetDir, "index_pack.json"), []byte(`{"zorder":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateAssetIndex(assetDir); err != nil {
		t.Fatalf("validateAssetIndex() packed-only error = %v", err)
	}
}

func TestConfigureFilesystemRootsSelectsExplicitPolicy(t *testing.T) {
	previousPortable := configurePortableFilesystemRoots
	previousLegacy := configureLegacyFilesystemRoots
	t.Cleanup(func() {
		configurePortableFilesystemRoots = previousPortable
		configureLegacyFilesystemRoots = previousLegacy
	})
	var selected string
	configurePortableFilesystemRoots = func(projectDir, assetDir string) error {
		selected = "portable"
		return nil
	}
	configureLegacyFilesystemRoots = func(projectDir, assetDir string) error {
		selected = "legacy"
		return nil
	}
	roots := interpruntime.Roots{ProjectDir: "/project", AssetDir: "/project/assets"}
	if err := configureFilesystemRoots(roots, nil); err != nil {
		t.Fatal(err)
	}
	if selected != "legacy" {
		t.Fatalf("absent portable overlay selected %q, want legacy", selected)
	}
	if err := configureFilesystemRoots(roots, &portableConfigOverlay{}); err != nil {
		t.Fatal(err)
	}
	if selected != "portable" {
		t.Fatalf("present portable overlay selected %q, want portable", selected)
	}
}

func TestPinnedProjectRootRejectsEscapingSymlink(t *testing.T) {
	projectDir := t.TempDir()
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "index.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(projectDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDir, filepath.Join(projectDir, "assets", "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	root, err := openPinnedProjectRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := fs.ReadFile(root.FS(), "assets/linked/index.json"); err == nil {
		t.Fatal("project filesystem followed a symlink outside ProjectDir")
	}
}

func TestBuildPinnedProjectKeepsFilesystemAliveForDeferredResourceLoads(t *testing.T) {
	t.Cleanup(releaseProjectRoot)
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "main.spx"), []byte("onStart => {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var runtimeFS fs.FS
	if err := buildPinnedProject(projectDir, nil, func(fsys fs.FS) error {
		runtimeFS = fsys
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	if _, err := fs.ReadFile(runtimeFS, "main.spx"); err != nil {
		t.Fatalf("deferred project read failed after build returned: %v", err)
	}
}

func TestBuildPinnedProjectClosesFilesystemOnBuildFailure(t *testing.T) {
	t.Cleanup(releaseProjectRoot)
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "main.spx"), []byte("onStart => {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("build failed")
	var failedFS fs.FS
	err := buildPinnedProject(projectDir, nil, func(fsys fs.FS) error {
		failedFS = fsys
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildPinnedProject() error = %v, want %v", err, wantErr)
	}
	if _, err := fs.ReadFile(failedFS, "main.spx"); err == nil {
		t.Fatal("failed build left the project root open")
	}
}
