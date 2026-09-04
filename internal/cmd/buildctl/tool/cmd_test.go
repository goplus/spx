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

package tool

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
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

func TestParseToolInstallArgsDefault(t *testing.T) {
	cfg, err := parseToolInstallArgs(nil)
	if err != nil {
		t.Fatalf("parseToolInstallArgs returned error: %v", err)
	}
	if cfg.Web || cfg.NoEmbedRuntime {
		t.Fatalf("unexpected tool install flags: %#v", cfg)
	}
}

func TestParseToolInstallArgsNoEmbedRuntime(t *testing.T) {
	cfg, err := parseToolInstallArgs([]string{"--no-embed-runtime"})
	if err != nil {
		t.Fatalf("parseToolInstallArgs returned error: %v", err)
	}
	if !cfg.NoEmbedRuntime || cfg.Web {
		t.Fatalf("unexpected tool install flags: %#v", cfg)
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
	if !shared.FileExists(filepath.Join(ndkRoot, "source.properties")) {
		t.Fatalf("expected source.properties under %s", ndkRoot)
	}
	clangPath := filepath.Join(ndkRoot, "toolchains", "llvm", "prebuilt", "bin", "clang")
	info, err := os.Lstat(clangPath)
	if err != nil {
		t.Fatalf("Lstat(%s) returned error: %v", clangPath, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		t.Fatalf("materialized clang mode = %v, want regular non-symlink", info.Mode())
	}
	if content, err := os.ReadFile(clangPath); err != nil || string(content) != "clang-12" {
		t.Fatalf("materialized clang target = %q, err = %v", content, err)
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
	if !strings.Contains(got, "export ANDROID_SDK_ROOT="+shared.ShellQuote(env.sdkRoot)) {
		t.Fatalf("expected quoted ANDROID_SDK_ROOT export in shell config: %s", got)
	}
	if !strings.Contains(got, "export ANDROID_NDK_ROOT="+shared.ShellQuote(env.ndkRoot)) {
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
	header := &zip.FileHeader{
		Name:   "android-ndk-r23c/toolchains/llvm/prebuilt/bin/clang",
		Method: zip.Store,
	}
	header.SetMode(fs.ModeSymlink | 0o777)
	entry, err = writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := entry.Write([]byte("clang-12")); err != nil {
		return err
	}
	return writer.Close()
}
