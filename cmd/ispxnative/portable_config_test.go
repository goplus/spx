//go:build !js || !wasm

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
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func TestPortableConfigFSUsesCapturedBytes(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".config"), []byte(`{"extasset":"live"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.spx"), []byte("onStart => {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"name":"captured"}`)
	overlayFS := newPortableConfigFS(os.DirFS(projectDir), &portableConfigOverlay{present: true, data: want})

	got, err := fs.ReadFile(overlayFS, ".config")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf(".config = %q, want %q", got, want)
	}
	assertSinglePortableConfigEntry(t, mustReadDir(t, overlayFS))

	root, err := overlayFS.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	directory, ok := root.(fs.ReadDirFile)
	if !ok {
		t.Fatal("Open root does not implement fs.ReadDirFile")
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	assertSinglePortableConfigEntry(t, entries)
}

func TestPortableConfigFSHidesAbsentConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".config"), []byte(`{"name":"late"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	overlayFS := newPortableConfigFS(os.DirFS(projectDir), &portableConfigOverlay{})
	if _, err := fs.ReadFile(overlayFS, ".config"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(.config) error = %v, want not exist", err)
	}
	if _, err := overlayFS.Open(".config/child"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open(.config/child) error = %v, want not exist", err)
	}
	for _, entry := range mustReadDir(t, overlayFS) {
		if isPortableConfigName(entry.Name()) {
			t.Fatalf("root listing exposed %q", entry.Name())
		}
	}
}

func TestLoadPortableConfigOverlayValidatesSessionContainment(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"name":"captured"}`)
	if err := os.WriteFile(filepath.Join(configDir, ".config"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	roots := interpruntime.Roots{SessionDir: sessionDir}
	env := portableConfigTestEnv(t, configDir)
	overlay, err := loadPortableConfigOverlay(env, roots)
	if err != nil {
		t.Fatal(err)
	}
	if overlay == nil || !overlay.present || string(overlay.data) != string(want) {
		t.Fatalf("loaded overlay = %#v, want captured content", overlay)
	}
	legacy, err := loadPortableConfigOverlay(nil, roots)
	if err != nil || legacy != nil {
		t.Fatalf("legacy overlay = %#v, %v, want nil", legacy, err)
	}

	outside := t.TempDir()
	_, err = loadPortableConfigOverlay([]string{
		interpruntime.PortableConfigDirEnv + "=" + outside,
		env[1],
	}, roots)
	if err == nil || !strings.Contains(err.Error(), "below session") {
		t.Fatalf("outside overlay error = %v, want containment rejection", err)
	}
}

func TestOpenPortableConfigRootPinsSessionTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement semantics differ on Windows")
	}
	sessionDir := t.TempDir()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".config"), []byte(`{"name":"inside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := openPortableConfigRoot(sessionDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, ".config"), []byte(`{"name":"outside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(configDir, filepath.Join(sessionDir, "portable-config-old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, configDir); err != nil {
		t.Fatal(err)
	}
	snapshot, err := projectpolicy.SnapshotPortableConfigRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != `{"name":"inside"}` {
		t.Fatalf("pinned config = %q, want inside snapshot", got)
	}
}

func TestLoadPortableConfigOverlayRejectsMaterializedDrift(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, ".config")
	if err := os.WriteFile(configPath, []byte(`{"name":"validated"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := portableConfigTestEnv(t, configDir)
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"name":"different-but-valid"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: sessionDir})
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("loadPortableConfigOverlay() drift error = %v", err)
	}
}

func TestLoadPortableConfigOverlayBindsAbsentState(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := portableConfigTestEnv(t, configDir)
	if err := os.WriteFile(filepath.Join(configDir, ".config"), []byte(`{"name":"late"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: sessionDir})
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("loadPortableConfigOverlay() absent drift error = %v", err)
	}
}

func TestLoadPortableConfigOverlayRejectsIncompleteEnvironment(t *testing.T) {
	sessionDir := t.TempDir()
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, env := range [][]string{
		{interpruntime.PortableConfigDirEnv + "=" + configDir},
		{interpruntime.PortableConfigIdentityEnv + "=absent"},
	} {
		if _, err := loadPortableConfigOverlay(env, interpruntime.Roots{SessionDir: sessionDir}); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("loadPortableConfigOverlay(%v) error = %v", env, err)
		}
	}
}

func portableConfigTestEnv(t *testing.T, configDir string) []string {
	t.Helper()
	snapshot, err := projectpolicy.SnapshotPortableConfig(configDir)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		t.Fatal(err)
	}
	return []string{
		interpruntime.PortableConfigDirEnv + "=" + configDir,
		interpruntime.PortableConfigIdentityEnv + "=" + identity,
	}
}

func mustReadDir(t *testing.T, fsys fs.FS) []fs.DirEntry {
	t.Helper()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func assertSinglePortableConfigEntry(t *testing.T, entries []fs.DirEntry) {
	t.Helper()
	count := 0
	for _, entry := range entries {
		if isPortableConfigName(entry.Name()) {
			count++
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			if info.IsDir() {
				t.Fatal("synthetic .config is a directory")
			}
		}
	}
	if count != 1 {
		t.Fatalf("portable config entry count = %d, want 1", count)
	}
}
