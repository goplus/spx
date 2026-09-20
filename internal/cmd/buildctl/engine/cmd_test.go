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
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestParseEngineDownloadArgsDefault(t *testing.T) {
	cfg, err := parseEngineDownloadArgs(nil)
	if err != nil {
		t.Fatalf("parseEngineDownloadArgs returned error: %v", err)
	}

	if cfg.runtime {
		t.Fatal("runtime should default to false")
	}
	if cfg.platform != "" {
		t.Fatalf("unexpected platform: %s", cfg.platform)
	}
	if cfg.mode != "" {
		t.Fatalf("unexpected mode: %s", cfg.mode)
	}
}

func TestParseEngineDownloadArgsRuntimeSkipsPack(t *testing.T) {
	cfg, err := parseEngineDownloadArgs([]string{"--runtime", "--skip-runtime-pack", "--asset-dir", "artifacts/runtime", "--same-run-artifacts"})
	if err != nil {
		t.Fatalf("parseEngineDownloadArgs returned error: %v", err)
	}

	if !cfg.runtime {
		t.Fatal("runtime should be true")
	}
	if !cfg.skipRuntimePack {
		t.Fatal("skipRuntimePack should be true")
	}
	if cfg.assetDir != filepath.Clean("artifacts/runtime") {
		t.Fatalf("assetDir = %q, want %q", cfg.assetDir, filepath.Clean("artifacts/runtime"))
	}
	if !cfg.sameRunArtifacts {
		t.Fatal("sameRunArtifacts should be true")
	}
}

func TestParseEngineDownloadArgsRejectsSkipPackWithoutRuntime(t *testing.T) {
	if _, err := parseEngineDownloadArgs([]string{"--skip-runtime-pack"}); err == nil {
		t.Fatal("expected --skip-runtime-pack without --runtime to fail")
	}
}

func TestParseEngineDownloadArgsRejectsSameRunWithoutAssetDir(t *testing.T) {
	if _, err := parseEngineDownloadArgs([]string{"--same-run-artifacts"}); err == nil {
		t.Fatal("expected --same-run-artifacts without --asset-dir to fail")
	}
}

func TestParseEngineDownloadArgsWebDefaultMode(t *testing.T) {
	cfg, err := parseEngineDownloadArgs([]string{"--platform", "web"})
	if err != nil {
		t.Fatalf("parseEngineDownloadArgs returned error: %v", err)
	}

	if cfg.mode != "normal" {
		t.Fatalf("expected normal mode, got %s", cfg.mode)
	}
}

func TestFindLocalEngineAssetRejectsDuplicateNames(t *testing.T) {
	assetDir := t.TempDir()
	for _, subdir := range []string{"one", "two"} {
		path := filepath.Join(assetDir, subdir, "web.zip")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.WriteFile(path, []byte(subdir), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}

	if _, err := findLocalEngineAsset(assetDir, "web.zip"); err == nil {
		t.Fatal("expected duplicate local assets to be rejected")
	} else if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("findLocalEngineAsset error = %v, want ambiguity error", err)
	}
}

func TestLoadEngineAssetManifestRequiresExplicitSameRunTrust(t *testing.T) {
	root := t.TempDir()
	env := engineDownloadEnv{
		assetDir: filepath.Join(root, "artifacts"),
		cacheDir: filepath.Join(root, "cache"),
		version:  release.DefaultRuntimeLock().RuntimeVersion,
	}
	for _, dir := range []string{env.assetDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := loadEngineAssetManifest(&env); err == nil {
		t.Fatal("local assets without a manifest should be rejected by default")
	}
	env.allowMissingManifest = true
	if err := loadEngineAssetManifest(&env); err != nil {
		t.Fatalf("same-run artifacts should defer verification to final assembly: %v", err)
	}
}

func TestDownloadRuntimePackRejectsMissingPCK(t *testing.T) {
	root := t.TempDir()
	env := engineDownloadEnv{
		version:  mustDefaultRuntimeVersion(t),
		goBinDir: filepath.Join(root, "bin"),
		cacheDir: filepath.Join(root, "cache"),
		assetDir: filepath.Join(root, "assets"),
	}
	for _, dir := range []string{env.goBinDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	versionedPack := filepath.Join(env.goBinDir, "gdspxrt"+env.version+".pck")
	if err := os.WriteFile(versionedPack, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeZipFixture(filepath.Join(env.assetDir, release.RuntimeAssetZipName), map[string]string{"runtime.gdextension": "incomplete"}); err != nil {
		t.Fatal(err)
	}

	err := downloadRuntimePack(env)
	if err == nil || !strings.Contains(err.Error(), "missing gdspxrt.pck") {
		t.Fatalf("downloadRuntimePack error = %v, want missing pck error", err)
	}
	if data, err := os.ReadFile(versionedPack); err != nil {
		t.Fatal(err)
	} else if string(data) != "stable" {
		t.Fatalf("incomplete runtime pack changed installed pck to %q", data)
	}
}

func TestDownloadAndroidAssetsRequiresCompleteBundle(t *testing.T) {
	root := t.TempDir()
	env := engineDownloadEnv{
		templateDir: filepath.Join(root, "templates"),
		cacheDir:    filepath.Join(root, "cache"),
		assetDir:    filepath.Join(root, "assets"),
	}
	for _, dir := range []string{env.templateDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"android_debug.apk", "android_release.apk", "android_source.zip"} {
		if err := os.WriteFile(filepath.Join(env.templateDir, name), []byte("stable:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeZipFixture(filepath.Join(env.assetDir, "android.zip"), map[string]string{
		"android_debug.apk":   "debug",
		"android_release.apk": "release",
	}); err != nil {
		t.Fatal(err)
	}

	err := downloadAndroidAssets(env)
	if err == nil || !strings.Contains(err.Error(), "missing android_source.zip") {
		t.Fatalf("downloadAndroidAssets error = %v, want missing source archive error", err)
	}
	for _, name := range []string{"android_debug.apk", "android_release.apk", "android_source.zip"} {
		data, readErr := os.ReadFile(filepath.Join(env.templateDir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(data) != "stable:"+name {
			t.Fatalf("incomplete Android bundle changed %s to %q", name, data)
		}
	}
}

func writeZipFixture(dst string, files map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			return err
		}
	}
	return writer.Close()
}
