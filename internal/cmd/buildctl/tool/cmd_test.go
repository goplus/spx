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
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

func TestParseToolSetupNDKArgsDefault(t *testing.T) {
	cfg, err := parseToolSetupNDKArgs(nil)
	if err != nil {
		t.Fatalf("parseToolSetupNDKArgs returned error: %v", err)
	}
	if cfg.manualInstall || cfg.ndkPath != "" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseToolSetupNDKArgsRejectsDeprecatedSkipVerification(t *testing.T) {
	if _, err := parseToolSetupNDKArgs([]string{"--skip-verification"}); err == nil {
		t.Fatal("parseToolSetupNDKArgs accepted --skip-verification")
	}
}

func TestAndroidNDKArchiveSpecifications(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		archive, err := androidNDKArchiveForOS(goos)
		if err != nil {
			t.Fatalf("androidNDKArchiveForOS(%q) returned error: %v", goos, err)
		}
		if err := archive.validate(); err != nil {
			t.Fatalf("androidNDKArchiveForOS(%q) is invalid: %v", goos, err)
		}
	}
	if _, err := androidNDKArchiveForOS("plan9"); err == nil {
		t.Fatal("androidNDKArchiveForOS accepted an unsupported OS")
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

func TestResolveNDKArchiveRejectsCacheDirectory(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.zip")
	archive := writeNDKZipFixture(t, fixture)
	env := androidNDKEnv{
		archiveName: "android-ndk.zip",
		archive:     archive,
		downloadURL: "https://example.invalid/android-ndk.zip",
		cacheDir:    filepath.Join(root, "cache"),
	}
	cached := filepath.Join(env.cacheDir, env.archiveName)
	if err := os.MkdirAll(filepath.Join(cached, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveNDKArchivePath(toolSetupNDKConfig{}, env, t.TempDir()); err == nil {
		t.Fatal("resolveNDKArchivePath accepted a cache directory")
	}
	if !shared.FileExists(filepath.Join(cached, "keep")) {
		t.Fatal("cache directory contents were removed")
	}
}

func TestVerifyNDKArchiveRejectsWrongIdentity(t *testing.T) {
	for _, test := range []struct {
		name     string
		root     string
		revision string
	}{
		{name: "root", root: "android-ndk-r24", revision: androidNDKVersion},
		{name: "revision", root: "android-ndk-r23c", revision: "24.0.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "android-ndk.zip")
			archive := writeNDKZipFixtureFor(t, path, test.root, test.revision, testNDKHostTag())
			if err := verifyNDKArchive(path, archive); err == nil {
				t.Fatal("verifyNDKArchive accepted wrong NDK identity")
			}
		})
	}
}

func TestResolveNDKArchiveReusesValidCache(t *testing.T) {
	root := t.TempDir()
	cached := filepath.Join(root, "cache", "android-ndk.zip")
	archive := writeNDKZipFixture(t, cached)
	env := androidNDKEnv{
		archiveName: "android-ndk.zip",
		archive:     archive,
		downloadURL: "https://example.invalid/android-ndk.zip",
		cacheDir:    filepath.Join(root, "cache"),
	}

	path, err := resolveNDKArchivePath(toolSetupNDKConfig{}, env, t.TempDir())
	if err != nil {
		t.Fatalf("resolveNDKArchivePath returned error: %v", err)
	}
	if path == cached {
		t.Fatalf("resolved path = %q, want a private snapshot", path)
	}
	if err := verifyNDKArchive(path, env.archive); err != nil {
		t.Fatalf("private snapshot is invalid: %v", err)
	}
}

func TestVerifyInstalledNDKRejectsMaterializedToolLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows archive has no symlink tool aliases")
	}
	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.zip")
	archive := writeNDKZipFixtureFor(t, fixture, "android-ndk-r23c", androidNDKVersion, "linux-x86_64")
	writeInstalledNDKFixture(t, root, archive.hostTag, true)
	bin := filepath.Join(root, "toolchains", "llvm", "prebuilt", archive.hostTag, "bin")
	for name, target := range map[string]string{"clang": "clang-12", "clang++": "clang"} {
		path := filepath.Join(bin, name)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(target), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := verifyInstalledNDK(root, archive); err == nil {
		t.Fatal("verifyInstalledNDK accepted materialized symlink text as a compiler")
	}
}

func TestVerifyInstalledNDKRejectsMissingToolchainFiles(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "fixture.zip")
	archive := writeNDKZipFixture(t, fixture)
	installRoot := filepath.Join(root, "ndk")
	writeInstalledNDKFixture(t, installRoot, archive.hostTag, false)
	missing := requiredNDKFiles(archive.hostTag)[2]
	if err := os.Remove(filepath.Join(installRoot, filepath.FromSlash(missing))); err != nil {
		t.Fatal(err)
	}

	if err := verifyInstalledNDK(installRoot, archive); err == nil {
		t.Fatalf("verifyInstalledNDK accepted an installation without %s", missing)
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

func TestUpdateNDKShellConfigRepairsPartialBlock(t *testing.T) {
	shellConfig := filepath.Join(t.TempDir(), ".zshrc")
	env := androidNDKEnv{
		sdkRoot:     "/tmp/sdk",
		ndkRoot:     "/tmp/ndk",
		shellConfig: shellConfig,
	}
	partial := "export ANDROID_NDK_ROOT=" + shared.ShellQuote(env.ndkRoot) + "\n"
	if err := os.WriteFile(shellConfig, []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := updateNDKShellConfig(env); err != nil {
		t.Fatalf("updateNDKShellConfig returned error: %v", err)
	}
	content, err := os.ReadFile(shellConfig)
	if err != nil {
		t.Fatal(err)
	}
	want := "export ANDROID_SDK_ROOT=" + shared.ShellQuote(env.sdkRoot) + "\n" +
		"export ANDROID_NDK_ROOT=" + shared.ShellQuote(env.ndkRoot) + "\n" +
		"export PATH=\"$ANDROID_NDK_ROOT:$PATH\"\n"
	if !strings.Contains(string(content), want) {
		t.Fatalf("repaired shell config = %q, want full export block", content)
	}
}

func testNDKHostTag() string {
	if runtime.GOOS == "windows" {
		return "windows-x86_64"
	}
	return "linux-x86_64"
}

func writeNDKZipFixture(t *testing.T, dst string) androidNDKArchive {
	t.Helper()
	return writeNDKZipFixtureFor(t, dst, "android-ndk-r23c", androidNDKVersion, testNDKHostTag())
}

func writeNDKZipFixtureFor(t *testing.T, dst, root, revision, hostTag string) androidNDKArchive {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}

	writer := zip.NewWriter(file)
	entry, err := writer.Create(root + "/source.properties")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("Pkg.Revision = " + revision + "\n")); err != nil {
		t.Fatal(err)
	}
	bin := root + "/toolchains/llvm/prebuilt/" + hostTag + "/bin/"
	if strings.HasPrefix(hostTag, "windows-") {
		writeNDKZipEntry(t, writer, bin+"clang.exe", []byte("compiler"), 0o755)
		writeNDKZipEntry(t, writer, bin+"clang++.exe", []byte("compiler"), 0o755)
	} else {
		writeNDKZipSymlink(t, writer, bin+"clang", "clang-12")
		writeNDKZipSymlink(t, writer, bin+"clang++", "clang")
		writeNDKZipEntry(t, writer, bin+"clang-12", []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	for _, name := range requiredNDKFiles(hostTag) {
		writeNDKZipEntry(t, writer, root+"/"+name, []byte("required"), 0o644)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	archiveOS, _, _ := strings.Cut(hostTag, "-")
	return androidNDKArchive{
		archiveOS: archiveOS,
		hostTag:   hostTag,
		size:      int64(len(data)),
		sha256:    hex.EncodeToString(sum[:]),
	}
}

func writeNDKZipEntry(t *testing.T, writer *zip.Writer, name string, data []byte, mode fs.FileMode) {
	t.Helper()
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatal(err)
	}
}

func writeNDKZipSymlink(t *testing.T, writer *zip.Writer, name, target string) {
	t.Helper()
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(fs.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(target)); err != nil {
		t.Fatal(err)
	}
}

func writeInstalledNDKFixture(t *testing.T, root, hostTag string, symlinkTools bool) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.properties"), []byte("Pkg.Revision = "+androidNDKVersion+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "toolchains", "llvm", "prebuilt", hostTag, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(hostTag, "windows-") {
		for _, name := range []string{"clang.exe", "clang++.exe"} {
			if err := os.WriteFile(filepath.Join(bin, name), []byte("compiler"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		if err := os.WriteFile(filepath.Join(bin, "clang-12"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if symlinkTools {
			if err := os.Symlink("clang-12", filepath.Join(bin, "clang")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("clang", filepath.Join(bin, "clang++")); err != nil {
				t.Fatal(err)
			}
		} else {
			for _, name := range []string{"clang", "clang++"} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, name := range requiredNDKFiles(hostTag) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("required"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeEmptyZipFixture(dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	if err := writer.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
