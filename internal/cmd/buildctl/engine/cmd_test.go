package engine

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
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

func TestParseEngineDownloadArgsWebDefaultMode(t *testing.T) {
	cfg, err := parseEngineDownloadArgs([]string{"--platform", "web"})
	if err != nil {
		t.Fatalf("parseEngineDownloadArgs returned error: %v", err)
	}

	if cfg.mode != "normal" {
		t.Fatalf("expected normal mode, got %s", cfg.mode)
	}
}

func TestParseEngineBuildArgsWebDefaultMode(t *testing.T) {
	cfg, err := parseEngineBuildArgs([]string{"--target", "template", "--platform", "web"})
	if err != nil {
		t.Fatalf("parseEngineBuildArgs returned error: %v", err)
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

func TestDownloadEngineAssetsRuntimeRefreshesPackLocally(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	t.Setenv("GITHUB_ACTIONS", "")
	version := mustDefaultRuntimeVersion(t)

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	editorPath := filepath.Join(gopathBin, "gdspx"+version)
	templatePath := filepath.Join(gopathBin, "gdspxrt"+version)
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
	if err := os.WriteFile(packPath, []byte("local-pack"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", packPath, err)
	}

	downloads := 0
	oldFetcher := EngineDownloadFetcher
	EngineDownloadFetcher = func(url, dst string) error {
		downloads++
		return oldFetcher(url, dst)
	}
	t.Cleanup(func() { EngineDownloadFetcher = oldFetcher })

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
	oldFetcher := EngineDownloadFetcher
	EngineDownloadFetcher = func(url, dst string) error {
		downloads++
		return oldFetcher(url, dst)
	}
	t.Cleanup(func() { EngineDownloadFetcher = oldFetcher })

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

	restoreResolve := SetEngineDownloadResolveEnv(func(repoRootArg, platform string) (DownloadEnv, error) {
		if platform == "" {
			platform = defaultPlatform
		}
		version := mustDefaultRuntimeVersion(t)
		return DownloadEnv{
			RepoRoot:    repoRootArg,
			Version:     version,
			Platform:    platform,
			Arch:        arch,
			GoBinDir:    filepath.Join(os.Getenv("GOPATH"), "bin"),
			TemplateDir: filepath.Join(repoRootArg, "templates"),
			CacheDir:    filepath.Join(repoRootArg, "cache"),
			URLPrefix:   "https://example.invalid/",
		}, nil
	})
	oldFetch := EngineDownloadFetcher
	EngineDownloadFetcher = fakeEngineDownloadFetcher

	t.Cleanup(func() {
		restoreResolve()
		EngineDownloadFetcher = oldFetch
	})
}

func fakeEngineDownloadFetcher(url, dst string) error {
	base := filepath.Base(url)

	switch base {
	case "linux-x86_64.zip":
		return writeZipFixture(dst, map[string]string{
			"godot.linuxbsd.template_release.x86_64": "linux-template",
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
	case "gdspxrt.pck.2.0.30.zip":
		return writeZipFixture(dst, map[string]string{
			"gdspxrt.pck": "runtime-pck",
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
