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
	"errors"
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

func TestDownloadEngineAssetsRuntime(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	version := mustDefaultRuntimeVersion(t)

	if err := downloadEngineAssets(engineDownloadConfig{runtime: true}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if !fileExists(filepath.Join(gopathBin, "gdspx"+version)) {
		t.Fatalf("expected host editor binary to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version)) {
		t.Fatalf("expected host template binary to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version+".pck")) {
		t.Fatalf("expected runtime pck to exist")
	}
	if !fileExists(filepath.Join(runner.repoRoot, "templates", "linux_release.x86_64")) {
		t.Fatalf("expected linux template fanout to exist")
	}
}

func TestDownloadEngineAssetsRuntimeSkipsPack(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	version := mustDefaultRuntimeVersion(t)

	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		if filepath.Base(url) == release.RuntimeAssetZipName {
			return errors.New("runtime pack should not be downloaded")
		}
		return oldFetcher(url, dst)
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadEngineAssets(engineDownloadConfig{runtime: true, skipRuntimePack: true}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if !fileExists(filepath.Join(gopathBin, "gdspx"+version)) {
		t.Fatalf("expected host editor binary to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version)) {
		t.Fatalf("expected host template binary to exist")
	}
	if fileExists(filepath.Join(gopathBin, "gdspxrt"+version+".pck")) {
		t.Fatalf("expected runtime pck not to exist")
	}
}

func TestDownloadEngineAssetsRuntimeRefreshesPackLocally(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	t.Setenv("GITHUB_ACTIONS", "")
	version := mustDefaultRuntimeVersion(t)

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	editorPath := filepath.Join(gopathBin, "gdspx"+version)
	templatePath := filepath.Join(gopathBin, "gdspxrt"+version)
	debugTemplatePath := filepath.Join(gopathBin, "gdspxrtdbg"+version)
	packPath := filepath.Join(gopathBin, "gdspxrt"+version+".pck")
	templateFanout := filepath.Join(runner.repoRoot, "templates", "linux_release.x86_64")

	if err := os.MkdirAll(gopathBin, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", gopathBin, err)
	}
	if err := os.WriteFile(editorPath, []byte("local-editor"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", editorPath, err)
	}
	if err := os.WriteFile(templatePath, []byte("local-template"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", templatePath, err)
	}
	if err := os.WriteFile(debugTemplatePath, []byte("local-debug-template"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", debugTemplatePath, err)
	}
	if err := os.WriteFile(packPath, []byte("local-pack"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", packPath, err)
	}

	downloads := 0
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		downloads++
		return oldFetcher(url, dst)
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadEngineAssets(engineDownloadConfig{runtime: true}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	if content, err := os.ReadFile(editorPath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", editorPath, err)
	} else if string(content) != "local-editor" {
		t.Fatalf("editor content = %q, want existing engine binary reused", string(content))
	}
	if content, err := os.ReadFile(templatePath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", templatePath, err)
	} else if string(content) != "local-template" {
		t.Fatalf("template content = %q, want existing runtime binary reused", string(content))
	}
	if content, err := os.ReadFile(packPath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", packPath, err)
	} else if string(content) != "runtime-pck" {
		t.Fatalf("pack content = %q, want runtime pack refreshed", string(content))
	}
	if content, err := os.ReadFile(templateFanout); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", templateFanout, err)
	} else if string(content) != "local-template" {
		t.Fatalf("fanout content = %q, want existing runtime binary copied to template fanout", string(content))
	}
	if downloads != 1 {
		t.Fatalf("download count = %d, want 1 when local runtime pack is refreshed", downloads)
	}
}

func TestDownloadEngineAssetsWeb(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	version := mustDefaultRuntimeVersion(t)

	if err := downloadEngineAssets(engineDownloadConfig{platform: "web", mode: "worker"}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if !fileExists(filepath.Join(gopathBin, "gdspx"+version+"_webworker.zip")) {
		t.Fatalf("expected cached web template zip to exist")
	}
	if !fileExists(filepath.Join(runner.repoRoot, "templates", "web_release.zip")) {
		t.Fatalf("expected web template copy to exist")
	}
}

func TestDownloadEngineAssetsUsesLocalArtifactDirectory(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	assetDir := filepath.Join(runner.repoRoot, "workflow-artifacts")
	version := mustDefaultRuntimeVersion(t)
	t.Setenv("GITHUB_ACTIONS", "true")

	oldResolve := engineDownloadResolveEnv
	engineDownloadResolveEnv = func(repoRootArg, platform string) (engineDownloadEnv, error) {
		return engineDownloadEnv{
			repoRoot:       repoRootArg,
			version:        version,
			platform:       "linux",
			arch:           "x86_64",
			goBinDir:       filepath.Join(os.Getenv("GOPATH"), "bin"),
			templateDir:    filepath.Join(repoRootArg, "templates"),
			cacheDir:       filepath.Join(repoRootArg, "cache"),
			urlPrefix:      "https://example.invalid/",
			verifyManifest: true,
		}, nil
	}
	t.Cleanup(func() { engineDownloadResolveEnv = oldResolve })

	if err := writeZipFixture(filepath.Join(assetDir, "nested", "linux-x86_64.zip"), map[string]string{
		"godot.linuxbsd.template_release.x86_64": "local-template",
		"godot.linuxbsd.template_debug.x86_64":   "local-debug-template",
	}); err != nil {
		t.Fatalf("write template fixture: %v", err)
	}
	if err := writeZipFixture(filepath.Join(assetDir, "nested", "editor-linux-x86_64.zip"), map[string]string{
		"godot.linuxbsd.editor.x86_64": "local-editor",
	}); err != nil {
		t.Fatalf("write editor fixture: %v", err)
	}

	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		return errors.New("network fetch should not be used with --asset-dir")
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadEngineAssets(engineDownloadConfig{runtime: true, skipRuntimePack: true, assetDir: assetDir, sameRunArtifacts: true}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if content, err := os.ReadFile(filepath.Join(gopathBin, "gdspxrt"+version)); err != nil {
		t.Fatalf("read installed template: %v", err)
	} else if string(content) != "local-template" {
		t.Fatalf("template content = %q, want local-template", content)
	}
	if content, err := os.ReadFile(filepath.Join(gopathBin, "gdspx"+version)); err != nil {
		t.Fatalf("read installed editor: %v", err)
	} else if string(content) != "local-editor" {
		t.Fatalf("editor content = %q, want local-editor", content)
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

func TestDownloadEngineAssetsRuntimeOverwritesStaleDesktopBinariesInGitHubActions(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	t.Setenv("GITHUB_ACTIONS", "true")
	version := mustDefaultRuntimeVersion(t)

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	editorPath := filepath.Join(gopathBin, "gdspx"+version)
	templatePath := filepath.Join(gopathBin, "gdspxrt"+version)
	packPath := filepath.Join(gopathBin, "gdspxrt"+version+".pck")
	templateFanout := filepath.Join(runner.repoRoot, "templates", "linux_release.x86_64")

	if err := os.MkdirAll(gopathBin, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", gopathBin, err)
	}
	if err := os.MkdirAll(filepath.Dir(templateFanout), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", filepath.Dir(templateFanout), err)
	}
	if err := os.WriteFile(editorPath, []byte("stale-editor"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", editorPath, err)
	}
	if err := os.WriteFile(templatePath, []byte("stale-template"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", templatePath, err)
	}
	if err := os.WriteFile(packPath, []byte("stale-pack"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", packPath, err)
	}
	if err := os.WriteFile(templateFanout, []byte("stale-fanout"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", templateFanout, err)
	}

	if err := downloadEngineAssets(engineDownloadConfig{runtime: true}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	if content, err := os.ReadFile(editorPath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", editorPath, err)
	} else if string(content) != "linux-editor" {
		t.Fatalf("editor content = %q, want refreshed engine binary", string(content))
	}
	if content, err := os.ReadFile(templatePath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", templatePath, err)
	} else if string(content) != "linux-template" {
		t.Fatalf("template content = %q, want refreshed runtime binary", string(content))
	}
	if content, err := os.ReadFile(packPath); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", packPath, err)
	} else if string(content) != "runtime-pck" {
		t.Fatalf("pack content = %q, want refreshed runtime pack", string(content))
	}
	if content, err := os.ReadFile(templateFanout); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", templateFanout, err)
	} else if string(content) != "linux-template" {
		t.Fatalf("fanout content = %q, want refreshed template fanout", string(content))
	}
}

func TestDownloadRuntimePackUsesAtomicRuntimeRelease(t *testing.T) {
	root := t.TempDir()
	meta := release.DefaultReleaseMeta()
	env := engineDownloadEnv{
		version:               meta.Runtime.Version,
		goBinDir:              filepath.Join(root, "bin"),
		cacheDir:              filepath.Join(root, "cache"),
		runtimeAssetURLPrefix: meta.Runtime.RuntimeAssets.DownloadURL(""),
		runtimePackAsset:      meta.RuntimePackAssetName(),
	}
	if err := os.MkdirAll(env.goBinDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", env.goBinDir, err)
	}
	if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", env.cacheDir, err)
	}

	var gotURL string
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		gotURL = url
		return writeZipFixture(dst, map[string]string{
			"gdspxrt.pck":         "runtime-pck",
			"runtime.gdextension": "runtime-extension",
		})
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadRuntimePack(env); err != nil {
		t.Fatalf("downloadRuntimePack returned error: %v", err)
	}

	wantURL := meta.RuntimeAssetDownloadURL(release.RuntimeAssetZipName)
	if gotURL != wantURL {
		t.Fatalf("download url = %q, want %q", gotURL, wantURL)
	}
	if !fileExists(filepath.Join(env.goBinDir, "gdspxrt"+meta.Runtime.Version+".pck")) {
		t.Fatalf("expected versioned runtime pck to exist")
	}
}

func TestDownloadRuntimePackUsesLegacyReleaseLocation(t *testing.T) {
	root := t.TempDir()
	meta, err := release.ResolveReleaseMetaForSPXVersion("v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	env := engineDownloadEnv{
		version:               meta.Runtime.Version,
		goBinDir:              filepath.Join(root, "bin"),
		cacheDir:              filepath.Join(root, "cache"),
		urlPrefix:             meta.Runtime.EngineAssets.DownloadURL(""),
		runtimeAssetURLPrefix: meta.Runtime.RuntimeAssets.DownloadURL(""),
		runtimePackAsset:      meta.RuntimePackAssetName(),
	}
	for _, dir := range []string{env.goBinDir, env.cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var gotURL string
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		gotURL = url
		return writeZipFixture(dst, map[string]string{
			"gdspxrt.pck":         "legacy-runtime-pck",
			"runtime.gdextension": "legacy-runtime-extension",
		})
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadRuntimePack(env); err != nil {
		t.Fatalf("downloadRuntimePack returned error: %v", err)
	}
	wantURL := meta.RuntimeAssetDownloadURL(meta.RuntimePackAssetName())
	if gotURL != wantURL {
		t.Fatalf("download URL = %q, want %q", gotURL, wantURL)
	}
	if content, err := os.ReadFile(filepath.Join(env.goBinDir, "gdspxrt"+meta.Runtime.Version+".pck")); err != nil {
		t.Fatal(err)
	} else if string(content) != "legacy-runtime-pck" {
		t.Fatalf("legacy runtime pck = %q", content)
	}
}

func TestDownloadRuntimePackRejectsMissingPCK(t *testing.T) {
	root := t.TempDir()
	env := engineDownloadEnv{
		version:   mustDefaultRuntimeVersion(t),
		goBinDir:  filepath.Join(root, "bin"),
		cacheDir:  filepath.Join(root, "cache"),
		urlPrefix: "https://example.invalid/",
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
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(_, dst string) error {
		return writeZipFixture(dst, map[string]string{"runtime.gdextension": "incomplete"})
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

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
		urlPrefix:   "https://example.invalid/",
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
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(_, dst string) error {
		return writeZipFixture(dst, map[string]string{
			"android_debug.apk":   "debug",
			"android_release.apk": "release",
		})
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

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

func TestDownloadEngineAssetsWebSkipsExistingCachedTemplateLocally(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	t.Setenv("GITHUB_ACTIONS", "")
	version := mustDefaultRuntimeVersion(t)

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	cachedZip := filepath.Join(gopathBin, "gdspx"+version+"_webworker.zip")
	if err := os.MkdirAll(gopathBin, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", gopathBin, err)
	}
	if err := os.WriteFile(cachedZip, []byte("local-web-template"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", cachedZip, err)
	}

	downloads := 0
	oldFetcher := engineDownloadFetcher
	engineDownloadFetcher = func(url, dst string) error {
		downloads++
		return oldFetcher(url, dst)
	}
	t.Cleanup(func() { engineDownloadFetcher = oldFetcher })

	if err := downloadEngineAssets(engineDownloadConfig{platform: "web", mode: "worker"}, runner.repoRoot); err != nil {
		t.Fatalf("downloadEngineAssets returned error: %v", err)
	}

	templateCopy := filepath.Join(runner.repoRoot, "templates", "web_release.zip")
	if content, err := os.ReadFile(templateCopy); err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", templateCopy, err)
	} else if string(content) != "local-web-template" {
		t.Fatalf("template content = %q, want existing cached web template reused", string(content))
	}
	if downloads != 0 {
		t.Fatalf("download count = %d, want 0 when cached web template already exists", downloads)
	}
}

func installFakeEngineDownload(t *testing.T, repoRoot, defaultPlatform, arch string) {
	t.Helper()

	oldResolve := engineDownloadResolveEnv
	engineDownloadResolveEnv = func(repoRootArg, platform string) (engineDownloadEnv, error) {
		if platform == "" {
			platform = defaultPlatform
		}
		version := mustDefaultRuntimeVersion(t)
		return engineDownloadEnv{
			repoRoot:    repoRootArg,
			version:     version,
			platform:    platform,
			arch:        arch,
			goBinDir:    filepath.Join(os.Getenv("GOPATH"), "bin"),
			templateDir: filepath.Join(repoRootArg, "templates"),
			cacheDir:    filepath.Join(repoRootArg, "cache"),
			urlPrefix:   "https://example.invalid/",
		}, nil
	}
	oldFetch := engineDownloadFetcher
	engineDownloadFetcher = fakeEngineDownloadFetcher

	t.Cleanup(func() {
		engineDownloadResolveEnv = oldResolve
		engineDownloadFetcher = oldFetch
	})
}

func fakeEngineDownloadFetcher(url, dst string) error {
	base := filepath.Base(url)
	runtimePackZip := release.RuntimeAssetZipName

	switch base {
	case "linux-x86_64.zip":
		return writeZipFixture(dst, map[string]string{
			"godot.linuxbsd.template_release.x86_64": "linux-template",
			"godot.linuxbsd.template_debug.x86_64":   "linux-debug-template",
		})
	case "editor-linux-x86_64.zip":
		return writeZipFixture(dst, map[string]string{
			"godot.linuxbsd.editor.x86_64": "linux-editor",
		})
	case "web-worker.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "worker-web-template",
		})
	case "web.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "normal-web-template",
		})
	case "web-minigame.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "minigame-web-template",
		})
	case "web-miniprogram.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "miniprogram-web-template",
		})
	case runtimePackZip:
		return writeZipFixture(dst, map[string]string{
			"gdspxrt.pck":         "runtime-pck",
			"runtime.gdextension": "runtime-extension",
		})
	default:
		return errors.New("unexpected download URL: " + url)
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
