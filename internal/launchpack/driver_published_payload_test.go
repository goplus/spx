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

package launchpack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/projectbundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func TestLauncherPayloadRecordsPublishedDriverProvenance(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	assets := publishedPayloadAssets(t, fixture)
	cfg, project := publishedPayloadConfig(t)
	payload, payloadDigest, manifestDigest := buildLaunchpackPayloadTest(t, cfg, assets, project)
	verified, err := runtimepayload.Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Manifest.SPX.SourceMode || verified.Manifest.SPX.SelectedVersion != fixture.manifest.SPXVersion {
		t.Fatalf("published payload source identity = %#v", verified.Manifest.SPX)
	}
	engineManifest := readPayloadJSONEntry(t, payload, "engine/"+runtimepayload.EngineComponentManifestName)
	bridgeManifest := readPayloadJSONEntry(t, payload, "bridge/bridge-manifest.json")
	for name, manifest := range map[string]map[string]any{"Engine": engineManifest, "bridge": bridgeManifest} {
		if manifest["mode"] != "published" {
			t.Fatalf("%s mode = %#v", name, manifest["mode"])
		}
	}
	assertPayloadProvenance(t, engineManifest, assets)
	assertPayloadProvenance(t, bridgeManifest, assets)
}

func TestLauncherPayloadSourceRemainsLocal(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	assets := publishedPayloadAssets(t, fixture)
	assets.Published = nil
	cfg, project := publishedPayloadConfig(t)
	cfg.Source.SourceMode = true
	payload, payloadDigest, manifestDigest := buildLaunchpackPayloadTest(t, cfg, assets, project)
	verified, err := runtimepayload.Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Manifest.SPX.SourceMode {
		t.Fatalf("source payload source identity = %#v", verified.Manifest.SPX)
	}
	for name, manifest := range map[string]map[string]any{
		"Engine": readPayloadJSONEntry(t, payload, "engine/"+runtimepayload.EngineComponentManifestName),
		"bridge": readPayloadJSONEntry(t, payload, "bridge/bridge-manifest.json"),
	} {
		if manifest["mode"] != "source" {
			t.Fatalf("%s mode = %#v", name, manifest["mode"])
		}
		if manifest["schema"] != map[string]string{"Engine": "spx-local-engine/v1", "bridge": "spx-local-bridge/v1"}[name] {
			t.Fatalf("%s schema = %#v", name, manifest["schema"])
		}
		for _, key := range []string{"driver_manifest_sha256", "driver_bundle_sha256", "driver_bundle_name", "driver_spx_version"} {
			if _, found := manifest[key]; found {
				t.Fatalf("source %s manifest unexpectedly contains %q", name, key)
			}
		}
	}
}

func publishedPayloadAssets(t *testing.T, fixture publishedDriverFixture) Assets {
	t.Helper()
	root := t.TempDir()
	enginePath := filepath.Join(root, fixture.spec.RuntimeName)
	packPath := filepath.Join(root, fixture.spec.PackName)
	bridgeName, err := bridgeFileName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	bridgePath := filepath.Join(root, bridgeName)
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{{enginePath, fixture.engine, 0o755}, {packPath, fixture.pack, 0o644}, {bridgePath, fixture.bridge, 0o755}} {
		if err := os.WriteFile(file.path, file.data, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	return Assets{
		EnginePath: enginePath, PackPath: packPath, BridgePath: bridgePath, Lock: fixture.lock,
		Published: &PublishedDriverIdentity{
			ManifestSHA256: digestBytes(fixture.manifestData), BundleSHA256: fixture.bundle.SHA256,
			BundleName: fixture.bundle.Name, SPXVersion: fixture.manifest.SPXVersion,
			EngineSHA256: digestBytes(fixture.engine), PackSHA256: digestBytes(fixture.pack),
			BridgeSHA256: digestBytes(fixture.bridge), EngineInterfaceDigest: fixture.bundle.EngineInterfaceDigest,
		},
	}
}

func publishedPayloadConfig(t *testing.T) (Config, projectbundle.Config) {
	t.Helper()
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "main.spx"), []byte("onStart => {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(projectDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "assets", "index.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "launcher")
	cfg := publishedDriverTestConfig(t.TempDir())
	cfg.ProjectDir, cfg.ProjectFile, cfg.ProjectExt = projectDir, filepath.Join(projectDir, "main.spx"), ".spx"
	cfg.PackDir, cfg.PackIndex, cfg.Output = "assets", "index.json", output
	return cfg, projectbundle.Config{ProjectDir: projectDir, ProjectFiles: []string{"main.spx"}, PackDir: "assets", Output: output}
}

func buildLaunchpackPayloadTest(t *testing.T, cfg Config, assets Assets, project projectbundle.Config) ([]byte, string, string) {
	t.Helper()
	var payload bytes.Buffer
	payloadDigest, manifestDigest, err := writeLauncherPayload(t.TempDir(), &payload, cfg, assets, project, IO{})
	if err != nil {
		t.Fatal(err)
	}
	return payload.Bytes(), payloadDigest, manifestDigest
}

func readPayloadJSONEntry(t *testing.T, payload []byte, name string) map[string]any {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read %s: read=%v close=%v", name, readErr, closeErr)
		}
		var manifest map[string]any
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return manifest
	}
	t.Fatalf("payload is missing %s", name)
	return nil
}

func assertPayloadProvenance(t *testing.T, manifest map[string]any, assets Assets) {
	t.Helper()
	if assets.Published == nil {
		t.Fatal("published assets have no provenance")
	}
	want := map[string]string{
		"driver_manifest_sha256": assets.Published.ManifestSHA256,
		"driver_bundle_sha256":   assets.Published.BundleSHA256,
		"driver_bundle_name":     assets.Published.BundleName,
		"driver_spx_version":     assets.Published.SPXVersion,
	}
	for key, value := range want {
		if manifest[key] != value {
			t.Fatalf("payload provenance %s = %#v, want %q", key, manifest[key], value)
		}
	}
}
