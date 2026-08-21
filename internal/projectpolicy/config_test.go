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

package projectpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePortableConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := ValidatePortableConfig(projectDir); err != nil {
		t.Fatalf("ValidatePortableConfig() without .config: %v", err)
	}
	configPath := filepath.Join(projectDir, configName)
	if err := os.WriteFile(configPath, []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePortableConfig(projectDir); err != nil {
		t.Fatalf("ValidatePortableConfig() with supported fields: %v", err)
	}
	for _, key := range []string{"extasset", "ExtAsset", "EXTASSET"} {
		if err := os.WriteFile(configPath, []byte(`{"`+key+`":"../shared-assets"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ValidatePortableConfig(projectDir); err == nil || !strings.Contains(err.Error(), "unsupported extasset") {
			t.Fatalf("ValidatePortableConfig() key %q error = %v, want extasset rejection", key, err)
		}
	}
}

func TestValidatePortableConfigRejectsSymlink(t *testing.T) {
	projectDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(projectDir, configName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := ValidatePortableConfig(projectDir); err == nil {
		t.Fatal("ValidatePortableConfig accepted symlink .config")
	}
}

func TestPortableConfigSnapshotRejectsIdentityAndContentDrift(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, configName)
	if err := os.WriteFile(configPath, []byte(`{"name":"first"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != `{"name":"first"}` {
		t.Fatalf("snapshot bytes = %q", got)
	}
	digest := sha256.Sum256([]byte(`{"name":"first"}`))
	wantIdentity := portableConfigSHA256Prefix + hex.EncodeToString(digest[:])
	if identity, err := snapshot.Identity(); err != nil || identity != wantIdentity {
		t.Fatalf("snapshot identity = %q, %v, want %q", identity, err, wantIdentity)
	}
	if err := snapshot.Verify(projectDir); err != nil {
		t.Fatalf("unchanged snapshot verification: %v", err)
	}

	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"name":"first"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("replacement verification error = %v", err)
	}

	snapshot, err = SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"name":"second"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("content drift verification error = %v", err)
	}
}

func TestSnapshotPortableConfigRootUsesPinnedDirectory(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, configName)
	data := []byte(`{"name":"pinned"}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	snapshot, err := SnapshotPortableConfigRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Bytes()); got != string(data) {
		t.Fatalf("pinned snapshot bytes = %q, want %q", got, data)
	}
}

func TestAbsentPortableConfigSnapshotRejectsCreation(t *testing.T) {
	projectDir := t.TempDir()
	snapshot, err := SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Present() || snapshot.Bytes() != nil {
		t.Fatal("absent config snapshot reported content")
	}
	if identity, err := snapshot.Identity(); err != nil || identity != portableConfigAbsentIdentity {
		t.Fatalf("absent snapshot identity = %q, %v", identity, err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, configName), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Verify(projectDir); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("created config verification error = %v", err)
	}
}

func TestPortableConfigSnapshotRejectsOversizeFile(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, configName)
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxPortableConfigBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotPortableConfig(projectDir); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("SnapshotPortableConfig() oversize error = %v", err)
	}
}

func TestPortableConfigSnapshotIdentityRejectsZeroValue(t *testing.T) {
	if identity, err := (PortableConfigSnapshot{}).Identity(); err == nil || identity != "" {
		t.Fatalf("zero snapshot identity = %q, %v, want initialization error", identity, err)
	}
}
