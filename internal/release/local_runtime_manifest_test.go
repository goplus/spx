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

package release

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalRuntimeManifestWriteAndVerify(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("engine-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.ValidateForLock(lock, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Dir(manifestPath)
	if err := loaded.VerifyFiles(manifestDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{loaded.Engine.Name, loaded.Pack.Name} {
		if _, err := os.Stat(filepath.Join(manifestDir, name)); err != nil {
			t.Fatalf("published local runtime file %s: %v", name, err)
		}
	}

	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, updated, enginePath, packPath); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Engine.SHA256 == loaded.Engine.SHA256 {
		t.Fatal("rewriting local runtime manifest did not replace the old digest")
	}
	if reloaded.Engine.Name == loaded.Engine.Name {
		t.Fatal("rewriting local runtime manifest reused the old content address")
	}
	if err := reloaded.VerifyFiles(manifestDir); err != nil {
		t.Fatal(err)
	}
	if err := loaded.VerifyFiles(manifestDir); err != nil {
		t.Fatalf("previous manifest stopped resolving after atomic update: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".engine-manifest-") {
			t.Fatalf("temporary manifest file was left behind: %s", entry.Name())
		}
	}
}

func TestNewLocalRuntimeManifestRejectsSymlink(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engineTarget := filepath.Join(root, "engine-target")
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(engineTarget, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(engineTarget, enginePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(packPath, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("NewLocalRuntimeManifest error = %v, want symlink rejection", err)
	}
}

func TestNewLocalRuntimeManifestRejectsUnlockedNames(t *testing.T) {
	lock := DefaultRuntimeLock()
	root := t.TempDir()
	enginePath := filepath.Join(root, "engine")
	packPath := filepath.Join(root, "runtime.pck")
	if err := os.WriteFile(enginePath, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "file names") {
		t.Fatalf("NewLocalRuntimeManifest error = %v, want locked-name rejection", err)
	}
}

func TestLocalRuntimeManifestPathRequiresAbsoluteCleanRoot(t *testing.T) {
	lock := DefaultRuntimeLock()
	for _, root := range []string{"", ".", t.TempDir() + string(filepath.Separator) + ".."} {
		if _, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH); err == nil {
			t.Fatalf("LocalRuntimeManifestPath accepted root %q", root)
		}
	}
}

func TestPublishLocalRuntimeManifestRejectsChangedSource(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("engine-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("PublishLocalRuntimeManifest error = %v, want digest mismatch", err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("manifest after failed publish: err=%v", err)
	}
}

func TestPublishLocalRuntimeManifestKeepsPreviousGenerationOnPackFailure(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	repoRoot := t.TempDir()
	enginePath := filepath.Join(sourceDir, spec.RuntimeName)
	packPath := filepath.Join(sourceDir, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("engine-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(repoRoot, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, previous, enginePath, packPath); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(enginePath, []byte("engine-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("changed-after-manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, next, enginePath, packPath); err == nil {
		t.Fatal("publish with changed PCK unexpectedly succeeded")
	}

	loaded, err := LoadLocalRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Engine.SHA256 != previous.Engine.SHA256 || loaded.Pack.SHA256 != previous.Pack.SHA256 {
		t.Fatalf("failed publish replaced previous manifest: %#v", loaded)
	}
	if err := loaded.VerifyFiles(filepath.Dir(manifestPath)); err != nil {
		t.Fatalf("failed publish damaged previous generation: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(manifestPath), next.Engine.Name)); !os.IsNotExist(err) {
		t.Fatalf("failed preflight left next Engine object: %v", err)
	}
}

func TestPublishLocalRuntimeManifestRejectsDestinationSymlink(t *testing.T) {
	lock := DefaultRuntimeLock()
	spec, err := HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	enginePath := filepath.Join(root, spec.RuntimeName)
	packPath := filepath.Join(root, spec.PackName)
	if err := os.WriteFile(enginePath, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := NewLocalRuntimeManifest(lock, runtime.GOOS, runtime.GOARCH, enginePath, packPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := LocalRuntimeManifestPath(root, lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside-engine")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(manifestDir, manifest.Engine.Name)
	if err := os.Symlink(target, destination); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("PublishLocalRuntimeManifest error = %v, want destination symlink rejection", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "outside" {
		t.Fatalf("symlink target changed: data=%q err=%v", got, err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("manifest after failed publish: err=%v", err)
	}
}
