package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseToolSetupNDKArgsDefault(t *testing.T) {
	cfg, err := parseToolSetupNDKArgs(nil)
	if err != nil {
		t.Fatalf("parseToolSetupNDKArgs returned error: %v", err)
	}
	if cfg.manualInstall || cfg.ndkPath != "" || cfg.skipVerification {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseToolSetupNDKArgsManualInstallRequiresPath(t *testing.T) {
	if _, err := parseToolSetupNDKArgs([]string{"--manual-install"}); err == nil {
		t.Fatal("expected error for missing --ndk-path")
	}
}

func TestSetupAndroidNDKManualInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ANDROID_SDK_ROOT", filepath.Join(home, "Library", "Android", "sdk"))

	archivePath := filepath.Join(home, "android-ndk-r23c-darwin.zip")
	if err := writeNDKZipFixture(archivePath); err != nil {
		t.Fatalf("writeNDKZipFixture returned error: %v", err)
	}

	if err := setupAndroidNDK(toolSetupNDKConfig{
		manualInstall: true,
		ndkPath:       archivePath,
	}); err != nil {
		t.Fatalf("setupAndroidNDK returned error: %v", err)
	}

	ndkRoot := filepath.Join(os.Getenv("ANDROID_SDK_ROOT"), "ndk", androidNDKVersion)
	if !fileExists(filepath.Join(ndkRoot, "source.properties")) {
		t.Fatalf("expected source.properties under %s", ndkRoot)
	}

	shellConfig := filepath.Join(home, ".zshrc")
	content, err := os.ReadFile(shellConfig)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", shellConfig, err)
	}
	if !strings.Contains(string(content), "export ANDROID_NDK_ROOT=") {
		t.Fatalf("expected ANDROID_NDK_ROOT export in shell config: %s", string(content))
	}
}

func TestUpdateNDKShellConfigQuotesPaths(t *testing.T) {
	shellConfig := filepath.Join(t.TempDir(), ".zshrc")
	env := androidNDKEnv{
		sdkRoot:     `/tmp/sdk$HOME"quoted"'single`,
		ndkRoot:     `/tmp/ndk$PATH"quoted"'single`,
		shellConfig: shellConfig,
	}

	if err := updateNDKShellConfig(env); err != nil {
		t.Fatalf("updateNDKShellConfig returned error: %v", err)
	}

	content, err := os.ReadFile(shellConfig)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", shellConfig, err)
	}
	got := string(content)
	if !strings.Contains(got, "export ANDROID_SDK_ROOT="+shellQuote(env.sdkRoot)) {
		t.Fatalf("expected quoted ANDROID_SDK_ROOT export in shell config: %s", got)
	}
	if !strings.Contains(got, "export ANDROID_NDK_ROOT="+shellQuote(env.ndkRoot)) {
		t.Fatalf("expected quoted ANDROID_NDK_ROOT export in shell config: %s", got)
	}
	if !strings.Contains(got, "export PATH=\"$ANDROID_NDK_ROOT:$PATH\"") {
		t.Fatalf("expected PATH export to preserve variable expansion: %s", got)
	}
}

func writeNDKZipFixture(dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entry, err := writer.Create("android-ndk-r23c/source.properties")
	if err != nil {
		return err
	}
	if _, err := entry.Write([]byte("Pkg.Revision=23.2.8568313\n")); err != nil {
		return err
	}
	return writer.Close()
}
