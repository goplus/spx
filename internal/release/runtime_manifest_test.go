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

package release

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGenerateRuntimeManifest(t *testing.T) {
	lock, provenance, inputs, assetDir := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.ValidateForLock(lock); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Assets) != len(lock.RequiredAssets) {
		t.Fatalf("asset count = %d, want %d", len(manifest.Assets), len(lock.RequiredAssets))
	}
	for i, asset := range manifest.Assets {
		if asset.Name != lock.RequiredAssets[i] {
			t.Fatalf("asset[%d] = %q, want %q", i, asset.Name, lock.RequiredAssets[i])
		}
		if asset.Size <= 0 || len(asset.SHA256) != 64 {
			t.Fatalf("invalid asset metadata: %#v", asset)
		}
	}
	if err := manifest.VerifyFiles(assetDir); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyAsset(inputs[0].Name, inputs[0].Path); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyAsset("not-published.zip", inputs[0].Path); err == nil {
		t.Fatal("VerifyAsset accepted an asset absent from the manifest")
	}

	jsonData, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRuntimeManifest(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.ValidateForLock(lock); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(parsed.Assets, manifest.Assets) {
		t.Fatalf("parsed assets differ: %#v != %#v", parsed.Assets, manifest.Assets)
	}

	sums, err := manifest.SHA256SUMS()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n")
	if len(lines) != len(lock.RequiredAssets) {
		t.Fatalf("SHA256SUMS line count = %d, want %d", len(lines), len(lock.RequiredAssets))
	}
	if !strings.HasSuffix(lines[0], "  "+lock.RequiredAssets[0]) {
		t.Fatalf("first checksum line = %q", lines[0])
	}
}

func TestRuntimeManifestFiles(t *testing.T) {
	lock, provenance, inputs, assetDir := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), lock.Manifest)
	if err := WriteRuntimeManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.ValidateForLock(lock); err != nil {
		t.Fatal(err)
	}
	sumsPath := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := WriteSHA256SUMS(sumsPath, loaded); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(sumsPath); err != nil || len(data) == 0 {
		t.Fatalf("read SHA256SUMS: bytes=%d err=%v", len(data), err)
	}

	tamperedPath := filepath.Join(assetDir, lock.RequiredAssets[0])
	if err := os.WriteFile(tamperedPath, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loaded.VerifyAsset(lock.RequiredAssets[0], tamperedPath); err == nil {
		t.Fatal("VerifyAsset accepted tampered content")
	}
}

func TestGenerateRuntimeManifestRejectsIncompleteAndDuplicateAssets(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	if _, err := GenerateRuntimeManifest(lock, provenance, inputs[:len(inputs)-1]); err == nil {
		t.Fatal("GenerateRuntimeManifest accepted an incomplete runtime release")
	}
	inputs = append(inputs, inputs[0])
	if _, err := GenerateRuntimeManifest(lock, provenance, inputs); err == nil {
		t.Fatal("GenerateRuntimeManifest accepted a duplicate asset name")
	}
}

func TestRuntimeManifestValidateForLockRequiresEveryAsset(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assets = manifest.Assets[:len(manifest.Assets)-1]
	if err := manifest.ValidateForLock(lock); err == nil || !strings.Contains(err.Error(), "exactly") {
		t.Fatalf("ValidateForLock error = %v", err)
	}
}

func TestRuntimeManifestValidateForLockRejectsExtraAsset(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assets = append(manifest.Assets, RuntimeAsset{
		Name: "zz-unexpected.zip", Size: 1, SHA256: strings.Repeat("0", 64),
	})
	if err := manifest.ValidateForLock(lock); err == nil || !strings.Contains(err.Error(), "exactly") {
		t.Fatalf("ValidateForLock error = %v", err)
	}
}

func TestRuntimeManifestValidateForVersionIgnoresBuildMetadata(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	manifest.RuntimeABI++
	manifest.ReleaseRepository = "example/runtime"
	manifest.LockSHA256 = strings.Repeat("0", 64)
	manifest.Provenance.SPXCommit = strings.Repeat("1", 40)
	manifest.Provenance.GodotCommit = strings.Repeat("2", 40)
	manifest.Provenance.ModuleTree = strings.Repeat("3", 40)
	manifest.Provenance.RuntimePackSourceSHA256 = strings.Repeat("4", 64)
	manifest.Provenance.BuildRecipeSHA256 = strings.Repeat("5", 64)
	manifest.Provenance.Toolchain = ToolchainLock{
		Go: "9.9.9", XGo: "9.9.9", SCons: "9.9.9", EMSDK: "9.9.9", AndroidNDK: "r99", JDK: "99",
	}
	if err := manifest.ValidateForVersion(lock.RuntimeVersion); err != nil {
		t.Fatalf("version-compatible manifest rejected stale build metadata: %v", err)
	}
	if err := manifest.ValidateRequiredAssets(lock.RequiredAssets); err != nil {
		t.Fatalf("required asset set rejected: %v", err)
	}
}

func TestRuntimeManifestValidateForVersionRejectsMismatch(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.ValidateForVersion("9.9.9"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("version mismatch error = %v", err)
	}
}

func TestRuntimeManifestValidation(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	original, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*RuntimeManifest)
	}{
		{"schema", func(m *RuntimeManifest) { m.Schema = 2 }},
		{"version", func(m *RuntimeManifest) { m.RuntimeVersion = "2.2" }},
		{"abi", func(m *RuntimeManifest) { m.RuntimeABI = 0 }},
		{"repository", func(m *RuntimeManifest) { m.ReleaseRepository = "goplus" }},
		{"lock digest", func(m *RuntimeManifest) { m.LockSHA256 = strings.Repeat("A", 64) }},
		{"SPX commit", func(m *RuntimeManifest) { m.Provenance.SPXCommit = "deadbeef" }},
		{"Godot commit", func(m *RuntimeManifest) { m.Provenance.GodotCommit = "deadbeef" }},
		{"module tree", func(m *RuntimeManifest) { m.Provenance.ModuleTree = "deadbeef" }},
		{"runtime pack source", func(m *RuntimeManifest) { m.Provenance.RuntimePackSourceSHA256 = "deadbeef" }},
		{"build recipe", func(m *RuntimeManifest) { m.Provenance.BuildRecipeSHA256 = "deadbeef" }},
		{"toolchain", func(m *RuntimeManifest) { m.Provenance.Toolchain.SCons = "" }},
		{"empty assets", func(m *RuntimeManifest) { m.Assets = nil }},
		{"unsorted assets", func(m *RuntimeManifest) { m.Assets[0], m.Assets[1] = m.Assets[1], m.Assets[0] }},
		{"duplicate assets", func(m *RuntimeManifest) { m.Assets[1] = m.Assets[0] }},
		{"asset path", func(m *RuntimeManifest) { m.Assets[0].Name = "nested/android.zip" }},
		{"asset size", func(m *RuntimeManifest) { m.Assets[0].Size = 0 }},
		{"asset digest", func(m *RuntimeManifest) { m.Assets[0].SHA256 = "deadbeef" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := original
			manifest.Assets = append([]RuntimeAsset(nil), original.Assets...)
			test.mutate(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatalf("Validate succeeded for %#v", manifest)
			}
		})
	}
}

func TestParseRuntimeManifestRejectsUnknownAndTrailingJSON(t *testing.T) {
	lock, provenance, inputs, _ := runtimeManifestFixture(t)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	data, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := bytes.Replace(data, []byte(`"schema": 1`), []byte(`"unknown": true, "schema": 1`), 1)
	if _, err := ParseRuntimeManifest(withUnknown); err == nil {
		t.Fatal("ParseRuntimeManifest accepted an unknown field")
	}
	if _, err := ParseRuntimeManifest(append(data, []byte(`{}`)...)); err == nil {
		t.Fatal("ParseRuntimeManifest accepted a second JSON value")
	}
	withDuplicate := bytes.Replace(data, []byte(`"schema": 1`), []byte(`"schema": 1, "schema": 1`), 1)
	if _, err := ParseRuntimeManifest(withDuplicate); err == nil {
		t.Fatal("ParseRuntimeManifest accepted a duplicate key")
	}
	withNestedDuplicate := bytes.Replace(data, []byte(`"size": 1`), []byte(`"size": 1, "size": 1`), 1)
	if bytes.Equal(withNestedDuplicate, data) {
		t.Fatal("manifest fixture did not contain the expected size")
	}
	if _, err := ParseRuntimeManifest(withNestedDuplicate); err == nil {
		t.Fatal("ParseRuntimeManifest accepted a nested duplicate key")
	}
}

func runtimeManifestFixture(t *testing.T) (RuntimeLock, RuntimeProvenance, []RuntimeAssetInput, string) {
	t.Helper()
	lock := DefaultRuntimeLock()
	assetDir := t.TempDir()
	inputs := make([]RuntimeAssetInput, 0, len(lock.RequiredAssets))
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(assetDir, name)
		if err := os.WriteFile(path, []byte("asset:"+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, RuntimeAssetInput{Name: name, Path: path})
	}
	slices.Reverse(inputs)
	provenance := RuntimeProvenance{
		SPXCommit:               strings.Repeat("a", 40),
		GodotCommit:             lock.Godot.Commit,
		ModuleTree:              strings.Repeat("b", 40),
		RuntimePackSourceSHA256: strings.Repeat("c", 64),
		BuildRecipeSHA256:       strings.Repeat("d", 64),
		Toolchain:               lock.Toolchain,
	}
	return lock, provenance, inputs, assetDir
}
