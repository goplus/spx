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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
)

func TestParseToolSetupNDKArgs(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		want    toolSetupNDKConfig
		wantErr string
	}{
		{name: "default"},
		{name: "manual", args: []string{"--manual-install", "--ndk-path", "ndk.zip"}, want: toolSetupNDKConfig{manualInstall: true, ndkPath: "ndk.zip"}},
		{name: "missing archive", args: []string{"--manual-install"}, wantErr: "--manual-install requires --ndk-path"},
		{name: "missing manual flag", args: []string{"--ndk-path", "ndk.zip"}, wantErr: "--ndk-path requires --manual-install"},
		{name: "skip verification", args: []string{"--skip-verification"}, wantErr: "archive validation is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseToolSetupNDKArgs(test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parse error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("config = %#v, %v; want %#v", got, err, test.want)
			}
		})
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

func TestNDKArchiveRejectsNonRegularCache(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			fixture := filepath.Join(root, "fixture.zip")
			archive := writeNDKZipFixture(t, fixture)
			env := androidNDKEnv{archiveName: "android-ndk.zip", archive: archive, downloadURL: "https://example.invalid/android-ndk.zip", cacheDir: filepath.Join(root, "cache")}
			cached := filepath.Join(env.cacheDir, env.archiveName)
			if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if kind == "directory" {
				writeToolFixture(t, filepath.Join(cached, "keep"), "preserved", 0o644)
			} else if err := os.Symlink(fixture, cached); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := resolveNDKArchivePath(toolSetupNDKConfig{}, env, t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a regular non-symlink file") {
				t.Fatalf("non-regular cache error = %v", err)
			}
			if _, _, err := openNDKArchive(cached); err == nil || !strings.Contains(err.Error(), "not a regular non-symlink file") {
				t.Fatalf("non-regular archive error = %v", err)
			}
			if kind == "directory" {
				if data, err := os.ReadFile(filepath.Join(cached, "keep")); err != nil || string(data) != "preserved" {
					t.Fatalf("cache directory changed: %q, %v", data, err)
				}
			} else if target, err := os.Readlink(cached); err != nil || target != fixture {
				t.Fatalf("cache symlink changed: %q, %v", target, err)
			}
			if err := verifyNDKArchive(fixture, archive); err != nil {
				t.Fatalf("original archive changed: %v", err)
			}
		})
	}
}

func TestVerifyNDKArchiveRejectsWrongIdentity(t *testing.T) {
	for _, test := range []struct {
		name     string
		root     string
		revision string
	}{
		{name: "root", root: "android-ndk-invalid", revision: androidNDKVersion},
		{name: "revision", root: testNDKRoot(t), revision: "invalid"},
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
	archive := writeNDKZipFixtureFor(t, fixture, testNDKRoot(t), androidNDKVersion, "linux-x86_64")
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

func testNDKRoot(t *testing.T) string {
	t.Helper()
	root, err := expectedNDKRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeNDKZipFixture(t *testing.T, dst string) androidNDKArchive {
	t.Helper()
	return writeNDKZipFixtureFor(t, dst, testNDKRoot(t), androidNDKVersion, testNDKHostTag())
}

func writeNDKZipFixtureFor(t *testing.T, dst, root, revision, hostTag string) androidNDKArchive {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	writeNDKZipEntry(t, writer, root+"/source.properties", []byte("Pkg.Revision = "+revision+"\n"), 0o644)
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
	writeToolFixture(t, dst, buffer.String(), 0o644)
	data := buffer.Bytes()
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
	writeNDKZipEntry(t, writer, name, []byte(target), fs.ModeSymlink|0o777)
}

func writeInstalledNDKFixture(t *testing.T, root, hostTag string, symlinkTools bool) {
	t.Helper()
	writeToolFixture(t, filepath.Join(root, "source.properties"), "Pkg.Revision = "+androidNDKVersion+"\n", 0o644)
	bin := filepath.Join(root, "toolchains", "llvm", "prebuilt", hostTag, "bin")
	if strings.HasPrefix(hostTag, "windows-") {
		for _, name := range []string{"clang.exe", "clang++.exe"} {
			writeToolFixture(t, filepath.Join(bin, name), "compiler", 0o755)
		}
	} else {
		writeToolFixture(t, filepath.Join(bin, "clang-12"), "#!/bin/sh\nexit 0\n", 0o755)
		for name, target := range map[string]string{"clang": "clang-12", "clang++": "clang"} {
			path := filepath.Join(bin, name)
			if symlinkTools {
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			} else {
				writeToolFixture(t, path, "#!/bin/sh\nexit 0\n", 0o755)
			}
		}
	}
	for _, name := range requiredNDKFiles(hostTag) {
		writeToolFixture(t, filepath.Join(root, filepath.FromSlash(name)), "required", 0o644)
	}
}

func TestAndroidNDKSetupExistingInstallation(t *testing.T) {
	for _, state := range []string{"valid", "unmanaged", "managed"} {
		t.Run(state, func(t *testing.T) {
			env := newNDKEnvironment(t)
			writeInstalledNDKFixture(t, env.ndkRoot, env.archive.hostTag, false)
			if state != "valid" {
				if err := os.Remove(filepath.Join(env.ndkRoot, "source.properties")); err != nil {
					t.Fatal(err)
				}
			}
			if state == "managed" {
				writeToolFixture(t, filepath.Join(env.ndkRoot, androidNDKOwnerFile), androidNDKVersion+"\n", 0o644)
			}
			err := Run([]string{"setup-ndk", "--manual-install", "--ndk-path", filepath.Join(t.TempDir(), "missing.zip")})
			switch state {
			case "valid":
				if err != nil || os.Getenv("ANDROID_NDK_ROOT") != env.ndkRoot {
					t.Fatalf("reuse installed NDK: %v, ANDROID_NDK_ROOT = %q", err, os.Getenv("ANDROID_NDK_ROOT"))
				}
			case "unmanaged":
				if err == nil || !strings.Contains(err.Error(), "refusing to replace unmanaged") {
					t.Fatalf("unmanaged installation error = %v", err)
				}
			case "managed":
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing archive error = %v", err)
				}
				matches, err := filepath.Glob(filepath.Join(filepath.Dir(env.ndkRoot), ".*.install-*"))
				if err != nil || len(matches) != 0 || !isBuildctlNDKInstallation(env.ndkRoot) {
					t.Fatalf("failed installation left temporary state or removed existing NDK: %v, %v", matches, err)
				}
			}
		})
	}
}

func newNDKEnvironment(t *testing.T) androidNDKEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("ANDROID_SDK_ROOT", filepath.Join(home, "sdk"))
	t.Setenv("ANDROID_NDK_ROOT", "")
	env, err := resolveAndroidNDKEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestInstallNDKReplacement(t *testing.T) {
	archive, err := androidNDKArchiveForOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"absent", "valid", "unmanaged", "managed", "rollback"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			stage, dst := filepath.Join(root, "stage"), filepath.Join(root, "installed")
			writeInstalledNDKFixture(t, stage, archive.hostTag, false)
			if state == "valid" {
				writeInstalledNDKFixture(t, dst, archive.hostTag, false)
			} else if state != "absent" {
				writeToolFixture(t, filepath.Join(dst, "keep"), "previous", 0o644)
			}
			if state == "managed" || state == "rollback" {
				writeToolFixture(t, filepath.Join(dst, androidNDKOwnerFile), androidNDKVersion+"\n", 0o644)
			}
			var err error
			if state == "rollback" {
				err = replaceNDKInstallation(filepath.Join(root, "missing"), dst, archive)
			} else {
				err = installNDK(stage, dst, archive)
			}
			if state == "unmanaged" || state == "rollback" {
				if err == nil {
					t.Fatal("replacement unexpectedly succeeded")
				}
				if data, err := os.ReadFile(filepath.Join(dst, "keep")); err != nil || string(data) != "previous" {
					t.Fatalf("previous installation was not preserved: %q, %v", data, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := verifyInstalledNDK(dst, archive); err != nil {
					t.Fatalf("installed NDK is invalid: %v", err)
				}
				if state != "valid" && !isBuildctlNDKInstallation(dst) {
					t.Fatal("installed NDK has no ownership marker")
				}
			}
			backups, err := filepath.Glob(filepath.Join(root, ".installed.backup-*"))
			if err != nil || len(backups) != 0 {
				t.Fatalf("replacement left backups: %v, %v", backups, err)
			}
		})
	}
	if err := installNDK(t.TempDir(), filepath.Join(t.TempDir(), "ndk"), archive); err == nil {
		t.Fatal("invalid staged installation was accepted")
	}
}

func TestRestoreNDKSymlinksAndLocateInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix archive symlinks require symlink privileges")
	}
	root := t.TempDir()
	archivePath := filepath.Join(root, "ndk.zip")
	archive := writeNDKZipFixture(t, archivePath)
	extracted := filepath.Join(root, "extracted")
	if err := shared.ExtractZipWithOptions(archivePath, extracted, shared.ZipExtractOptions{MaterializeSymlinksAsFiles: true}); err != nil {
		t.Fatal(err)
	}
	if err := restoreNDKSymlinks(archivePath, extracted); err != nil {
		t.Fatal(err)
	}
	ndkRoot, err := locateExtractedNDK(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyInstalledNDK(ndkRoot, archive); err != nil {
		t.Fatalf("restored installation: %v", err)
	}
	bin := filepath.Join(ndkRoot, "toolchains", "llvm", "prebuilt", archive.hostTag, "bin")
	if target, err := os.Readlink(filepath.Join(bin, "clang++")); err != nil || target != "clang" {
		t.Fatalf("clang++ symlink = %q, %v", target, err)
	}
	if _, err := locateExtractedNDK(t.TempDir()); err == nil {
		t.Fatal("missing extracted installation was accepted")
	}
	if err := os.RemoveAll(ndkRoot); err != nil {
		t.Fatal(err)
	}
	writeToolFixture(t, ndkRoot, "not a directory", 0o644)
	if _, err := locateExtractedNDK(extracted); err == nil {
		t.Fatal("regular file was accepted as extracted installation")
	}
	if err := restoreNDKSymlinks(filepath.Join(root, "missing.zip"), extracted); err == nil {
		t.Fatal("missing archive was accepted")
	}
}

func TestNDKArchiveValidationFailures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ndk.zip")
	valid := writeNDKZipFixture(t, path)
	for _, test := range []struct {
		name   string
		change func(*androidNDKArchive)
	}{
		{"missing specification", func(a *androidNDKArchive) { a.archiveOS = "" }},
		{"mismatched host", func(a *androidNDKArchive) { a.hostTag = "windows-x86_64"; a.archiveOS = "linux" }},
		{"malformed digest", func(a *androidNDKArchive) { a.sha256 = "not hex" }},
		{"uppercase digest", func(a *androidNDKArchive) { a.sha256 = strings.ToUpper(a.sha256) }},
		{"wrong size", func(a *androidNDKArchive) { a.size++ }},
		{"wrong digest", func(a *androidNDKArchive) { a.sha256 = strings.Repeat("0", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive := valid
			test.change(&archive)
			if err := verifyNDKArchive(path, archive); err == nil {
				t.Fatal("invalid archive specification accepted")
			}
			if _, err := persistNDKArchive(path, filepath.Join(root, "copy.zip"), archive); err == nil {
				t.Fatal("invalid archive persisted")
			}
		})
	}
	for _, path := range []string{root, filepath.Join(root, "missing.zip")} {
		if _, _, err := openNDKArchive(path); err == nil {
			t.Fatalf("openNDKArchive(%q) accepted a non-file", path)
		}
	}
	if _, err := requiredNDKTools("plan9-x86_64"); err == nil {
		t.Fatal("unsupported host tag accepted")
	}
	if err := verifyNDKRevision([]byte("Pkg.Revision = " + androidNDKVersion + "\nPkg.Revision = " + androidNDKVersion)); err == nil {
		t.Fatal("duplicate revision accepted")
	}
}

func TestNDKShellConfiguration(t *testing.T) {
	home := t.TempDir()
	for _, test := range []struct{ shell, want string }{{"/bin/zsh", ".zshrc"}, {"/bin/bash", ".bashrc"}, {"/bin/fish", ".bash_profile"}} {
		t.Setenv("SHELL", test.shell)
		if got := detectShellConfigFile(home); got != filepath.Join(home, test.want) {
			t.Fatalf("detectShellConfigFile(%q) = %q", test.shell, got)
		}
	}
	writeToolFixture(t, filepath.Join(home, ".bash_profile"), "# existing profile\n", 0o644)
	t.Setenv("SHELL", "/bin/bash")
	if got := detectShellConfigFile(home); got != filepath.Join(home, ".bash_profile") {
		t.Fatalf("existing profile was ignored: %q", got)
	}
	env := androidNDKEnv{sdkRoot: filepath.Join(home, "sdk"), ndkRoot: filepath.Join(home, "ndk"), shellConfig: filepath.Join(home, ".bashrc")}
	if err := updateNDKShellConfig(env); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(env.shellConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := updateNDKShellConfig(env); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(env.shellConfig)
	if err != nil || string(before) != string(after) {
		t.Fatalf("configuration was duplicated: %q, %v", after, err)
	}
	env.shellConfig = ""
	if err := updateNDKShellConfig(env); err != nil {
		t.Fatal(err)
	}
	env.shellConfig = filepath.Join(home, ".bash_profile", "blocked")
	if err := configureAndroidNDK(env); err == nil {
		t.Fatal("configuration through regular file accepted")
	}
}

type ndkRoundTripFunc func(*http.Request) (*http.Response, error)

func (f ndkRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNDKArchiveDownloadAndRetry(t *testing.T) {
	for _, test := range []struct {
		name             string
		cacheState       string
		failures         int
		invalidResponses int
	}{
		{"empty cache", "", 0, 0},
		{"corrupt cache", "corrupt", 0, 0},
		{"transient failure", "", 1, 0},
		{"exhausted retries", "", 3, 0},
		{"invalid response retry", "", 0, 1},
		{"invalid responses exhausted", "", 0, 3},
		{"cache unavailable after download", "blocked", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				root := t.TempDir()
				fixture := filepath.Join(root, "fixture.zip")
				archive := writeNDKZipFixture(t, fixture)
				data, err := os.ReadFile(fixture)
				if err != nil {
					t.Fatal(err)
				}
				env := androidNDKEnv{archiveName: "ndk.zip", archive: archive, downloadURL: "https://example.invalid/ndk.zip", cacheDir: filepath.Join(root, "cache")}
				cached := filepath.Join(env.cacheDir, env.archiveName)
				if test.cacheState == "corrupt" {
					writeToolFixture(t, cached, "corrupt", 0o644)
				}
				calls := 0
				failure := errors.New("download interrupted")
				previous := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = previous })
				http.DefaultTransport = ndkRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.URL.String() != env.downloadURL {
						t.Errorf("requested URL = %s", req.URL)
					}
					if calls <= test.failures {
						return nil, failure
					}
					body := data
					if calls <= test.invalidResponses {
						body = []byte("invalid archive")
					}
					if test.cacheState == "blocked" {
						writeToolFixture(t, env.cacheDir, "blocked", 0o644)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
				})
				path, err := resolveNDKArchivePath(toolSetupNDKConfig{}, env, t.TempDir())
				if test.cacheState == "blocked" {
					if err == nil || !strings.Contains(err.Error(), "persist Android NDK archive") || errors.Is(err, errInvalidNDKArchive) || calls != 1 || path != "" {
						t.Fatalf("cache persistence failure = %q, %v; calls = %d", path, err, calls)
					}
					if data, err := os.ReadFile(env.cacheDir); err != nil || string(data) != "blocked" {
						t.Fatalf("cache obstruction changed: %q, %v", data, err)
					}
					return
				}
				if test.failures == 3 || test.invalidResponses == 3 {
					wantErr := failure
					if test.invalidResponses == 3 {
						wantErr = errInvalidNDKArchive
					}
					if !errors.Is(err, wantErr) || calls != 3 {
						t.Fatalf("retry result = %v, calls = %d", err, calls)
					}
					if _, err := os.Stat(cached); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("failed download populated cache: %v", err)
					}
					return
				}
				if err != nil || calls != test.failures+test.invalidResponses+1 {
					t.Fatalf("download = %q, %v; calls = %d", path, err, calls)
				}
				for _, path := range []string{path, cached} {
					if err := verifyNDKArchive(path, archive); err != nil {
						t.Fatalf("downloaded archive %s: %v", path, err)
					}
				}
			})
		})
	}
}

func TestVerifyNDKPropertiesRejectsUnsafeEntries(t *testing.T) {
	for _, test := range []struct {
		name, content string
		mode          fs.FileMode
	}{
		{"directory", "", fs.ModeDir | 0o755},
		{"symlink", "target", fs.ModeSymlink | 0o777},
		{"oversized", strings.Repeat("x", int(maxNDKPropertiesBytes)+1), 0o644},
		{"missing revision", "Pkg.Description = Android NDK\n", 0o644},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			writeNDKZipEntry(t, writer, "source.properties", []byte(test.content), test.mode)
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyNDKProperties(reader.File[0]); err == nil {
				t.Fatal("unsafe properties entry accepted")
			}
		})
	}
}

func TestRestoreNDKSymlinksRejectsInvalidTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix archive symlinks require symlink privileges")
	}
	for _, test := range []struct{ name, target string }{
		{"empty", ""},
		{"oversized", strings.Repeat("x", maxNDKSymlinkBytes+1)},
		{"missing", "missing"},
		{"directory", "directory"},
		{"outside root", "../outside"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			ndkRoot := testNDKRoot(t)
			archive := filepath.Join(root, "ndk.zip")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			writeNDKZipSymlink(t, writer, ndkRoot+"/link", test.target)
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			extracted := filepath.Join(root, "extracted")
			writeToolFixture(t, filepath.Join(extracted, ndkRoot, "link"), test.target, 0o644)
			writeToolFixture(t, filepath.Join(extracted, "outside"), "outside", 0o644)
			if err := os.MkdirAll(filepath.Join(extracted, ndkRoot, "directory"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := restoreNDKSymlinks(archive, extracted); err == nil {
				t.Fatal("unsafe symlink target was accepted")
			}
		})
	}
}

func TestVerifyInstalledNDKRejectsUnsafeFiles(t *testing.T) {
	archive, err := androidNDKArchiveForOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ fault, wantErr string }{
		{"root file", "root is not a regular directory"},
		{"root symlink", "root is not a regular directory"},
		{"revision", "does not match locked version"},
		{"compiler mode", "tool is not executable"},
		{"tool mode", "tool is not executable"},
		{"empty required file", "not a non-empty regular file"},
		{"properties directory", "not a bounded regular file"},
		{"properties symlink", "not a bounded regular file"},
		{"properties oversized", "not a bounded regular file"},
		{"escaping required file", "path resolves outside the NDK root"},
	} {
		t.Run(test.fault, func(t *testing.T) {
			fault := test.fault
			root := filepath.Join(t.TempDir(), "ndk")
			if fault == "root file" {
				writeToolFixture(t, root, "not a directory", 0o644)
			} else {
				writeInstalledNDKFixture(t, root, archive.hostTag, false)
				bin := filepath.Join(root, "toolchains", "llvm", "prebuilt", archive.hostTag, "bin")
				switch fault {
				case "root symlink":
					realRoot := root + "-real"
					if err := os.Rename(root, realRoot); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(realRoot, root); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				case "properties symlink", "escaping required file":
					name := "source.properties"
					if fault == "escaping required file" {
						name = filepath.FromSlash(requiredNDKFiles(archive.hostTag)[0])
					}
					path := filepath.Join(root, name)
					target := filepath.Join(t.TempDir(), "outside")
					if err := os.Rename(path, target); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				case "properties oversized":
					writeToolFixture(t, filepath.Join(root, "source.properties"), "Pkg.Revision = "+androidNDKVersion+"\n"+strings.Repeat("#", int(maxNDKPropertiesBytes)), 0o644)
				case "revision":
					writeToolFixture(t, filepath.Join(root, "source.properties"), "Pkg.Revision = wrong\n", 0o644)
				case "compiler mode", "tool mode":
					if runtime.GOOS == "windows" {
						t.Skip("executable permissions apply to Unix tools")
					}
					name := "clang-12"
					if fault == "tool mode" {
						name = "clang"
					}
					if err := os.Chmod(filepath.Join(bin, name), 0o644); err != nil {
						t.Fatal(err)
					}
				case "empty required file":
					writeToolFixture(t, filepath.Join(root, filepath.FromSlash(requiredNDKFiles(archive.hostTag)[0])), "", 0o644)
				case "properties directory":
					path := filepath.Join(root, "source.properties")
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := verifyInstalledNDK(root, archive); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("installation verification = %v, want %q", err, test.wantErr)
			}
		})
	}
	t.Run("Windows tools", func(t *testing.T) {
		root := t.TempDir()
		archive, err := androidNDKArchiveForOS("windows")
		if err != nil {
			t.Fatal(err)
		}
		writeInstalledNDKFixture(t, root, archive.hostTag, false)
		if err := verifyInstalledNDK(root, archive); err != nil {
			t.Fatalf("valid Windows installation: %v", err)
		}
	})
}

func TestResolveAndroidNDKDefaultSDKRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("ANDROID_SDK_ROOT", "")
	env, err := resolveAndroidNDKEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(env.sdkRoot, home+string(filepath.Separator)) || env.ndkRoot != filepath.Join(env.sdkRoot, "ndk", androidNDKVersion) {
		t.Fatalf("default NDK paths = %#v", env)
	}
}

func TestNDKSetupRejectsBlockedLocalStateBeforeDownload(t *testing.T) {
	for _, scenario := range []string{"invalid root", "blocked cache", "unwritable shell config"} {
		t.Run(scenario, func(t *testing.T) {
			env := newNDKEnvironment(t)
			switch scenario {
			case "invalid root":
				writeToolFixture(t, env.sdkRoot, "blocked", 0600)
			case "blocked cache":
				writeToolFixture(t, env.cacheDir, "blocked", 0600)
			case "unwritable shell config":
				writeInstalledNDKFixture(t, env.ndkRoot, env.archive.hostTag, false)
				if err := os.Mkdir(env.shellConfig, 0700); err != nil {
					t.Fatal(err)
				}
			}
			// A missing explicit archive also guards against any accidental network fallback.
			if err := setupAndroidNDK(toolSetupNDKConfig{manualInstall: true, ndkPath: filepath.Join(t.TempDir(), "missing.zip")}); err == nil {
				t.Fatal("blocked local state ignored")
			}
		})
	}
}

func TestNDKArchivePersistenceRejectsUnusableDestinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.zip")
	archive := writeNDKZipFixture(t, source)
	for _, scenario := range []string{"parent file", "directory destination", "temporary name too long", "invalid specification", "cache snapshot", "invalid cache path"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "ndk.zip")
			var err error
			switch scenario {
			case "parent file":
				writeToolFixture(t, dst, "blocked", 0600)
				_, err = persistNDKArchive(source, filepath.Join(dst, "nested.zip"), archive)
			case "directory destination":
				if err := os.Mkdir(dst, 0700); err != nil {
					t.Fatal(err)
				}
				_, err = persistNDKArchive(source, dst, archive)
			case "temporary name too long":
				// Preserve a valid destination when its atomic temporary suffix cannot fit.
				dst = filepath.Join(dir, strings.Repeat("a", 255))
				if err := os.WriteFile(dst, []byte("preserved"), 0o644); err != nil {
					t.Skipf("filesystem cannot create the long destination name: %v", err)
				}
				if probe, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp-*"); err == nil {
					if err := probe.Close(); err != nil {
						t.Fatal(err)
					}
					t.Skip("filesystem also supports the longer temporary name")
				}
				_, err = persistNDKArchive(source, dst, archive)
				if data, readErr := os.ReadFile(dst); readErr != nil || string(data) != "preserved" {
					t.Fatalf("existing archive changed: %q, %v", data, readErr)
				}
			case "invalid specification":
				_, err = resolveNDKArchivePath(toolSetupNDKConfig{}, androidNDKEnv{}, dir)
			case "cache snapshot":
				writeToolFixture(t, dst, "blocked", 0600)
				_, err = resolveNDKArchivePath(toolSetupNDKConfig{}, androidNDKEnv{archive: archive, cacheDir: root, archiveName: "source.zip"}, dst)
			case "invalid cache path":
				_, err = resolveNDKArchivePath(toolSetupNDKConfig{}, androidNDKEnv{archive: archive, cacheDir: "\x00", archiveName: "source.zip"}, dir)
			}
			if err == nil {
				t.Fatal("unusable archive destination accepted")
			}
			matches, globErr := filepath.Glob(filepath.Join(dir, "*.tmp-*"))
			if globErr != nil || len(matches) != 0 {
				t.Fatalf("temporary archive leaked: %v, %v", matches, globErr)
			}
		})
	}
	if err := verifyNDKArchive(source+".missing", archive); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing verified archive = %v", err)
	}
}

func TestNDKInstallationRejectsInvalidMetadataAndOwner(t *testing.T) {
	archive, err := androidNDKArchiveForOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyInstalledNDK(t.TempDir(), androidNDKArchive{}); err == nil {
		t.Fatal("invalid archive metadata accepted")
	}
	if err := verifyInstalledNDK(filepath.Join(t.TempDir(), "missing"), archive); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing installation = %v", err)
	}
	if err := verifyInstalledNDKTools(t.TempDir(), t.TempDir(), "unknown"); err == nil {
		t.Fatal("invalid tool host accepted")
	}
	for _, name := range []string{"missing", "file"} {
		root := filepath.Join(t.TempDir(), name)
		if name == "file" {
			writeToolFixture(t, root, "not an installation", 0600)
		}
		if isBuildctlNDKInstallation(root) {
			t.Fatalf("invalid owned installation: %s", root)
		}
	}
	root := t.TempDir()
	writeInstalledNDKFixture(t, root, archive.hostTag, false)
	if err := os.Mkdir(filepath.Join(root, androidNDKOwnerFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := installNDK(root, filepath.Join(t.TempDir(), "destination"), archive); err == nil {
		t.Fatal("invalid ownership marker accepted")
	}
	if _, err := locateExtractedNDK(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing extraction = %v", err)
	}
	t.Setenv("ANDROID_SDK_ROOT", "unchanged")
	t.Setenv("ANDROID_NDK_ROOT", "unchanged")
	if err := configureAndroidNDK(androidNDKEnv{sdkRoot: "\x00"}); err == nil || os.Getenv("ANDROID_SDK_ROOT") != "unchanged" {
		t.Fatalf("invalid SDK environment changed state: %v", err)
	}
}

func TestNDKArchiveRequiresCompleteContents(t *testing.T) {
	for _, scenario := range []string{"not a ZIP", "missing tools", "missing properties"} {
		t.Run(scenario, func(t *testing.T) {
			var buffer bytes.Buffer
			if scenario == "not a ZIP" {
				buffer.WriteString("invalid archive")
			} else {
				writer := zip.NewWriter(&buffer)
				if scenario == "missing properties" {
					tools, err := requiredNDKTools(testNDKHostTag())
					if err != nil {
						t.Fatal(err)
					}
					for _, name := range append(tools, requiredNDKFiles(testNDKHostTag())...) {
						writeNDKZipEntry(t, writer, testNDKRoot(t)+"/"+name, []byte("file"), 0755)
					}
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "fixture.zip")
			if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(buffer.Bytes())
			archiveOS, _, _ := strings.Cut(testNDKHostTag(), "-")
			archive := androidNDKArchive{archiveOS: archiveOS, hostTag: testNDKHostTag(), size: int64(buffer.Len()), sha256: hex.EncodeToString(sum[:])}
			want := map[string]string{"not a ZIP": "preflight", "missing tools": "missing required file", "missing properties": "source.properties"}[scenario]
			if err := verifyNDKArchive(path, archive); !errors.Is(err, errInvalidNDKArchive) || !strings.Contains(err.Error(), want) {
				t.Fatalf("archive verification = %v, want %q", err, want)
			}
		})
	}
}

func TestNDKArchiveReadersRejectUnreadableEntries(t *testing.T) {
	for _, kind := range []string{"properties", "symlink"} {
		for _, fault := range []string{"compression method", "checksum"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				ndkRoot := testNDKRoot(t)
				var buffer bytes.Buffer
				writer := zip.NewWriter(&buffer)
				name, content := "source.properties", "Pkg.Revision = "+androidNDKVersion+"\n"
				mode := fs.FileMode(0o644)
				if kind == "symlink" {
					name, content, mode = ndkRoot+"/link", "clang-12", fs.ModeSymlink|0o777
					// Every target must be readable before any materialized link is replaced.
					writeNDKZipSymlink(t, writer, ndkRoot+"/first", "clang-12")
				}
				header := &zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc32.ChecksumIEEE([]byte(content)), CompressedSize64: uint64(len(content)), UncompressedSize64: uint64(len(content))}
				header.SetMode(mode)
				want := zip.ErrChecksum
				if fault == "compression method" {
					header.Method = 99
					want = zip.ErrAlgorithm
				} else {
					header.CRC32++
				}
				entry, err := writer.CreateRaw(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(entry, content); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if kind == "properties" {
					reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
					if err != nil {
						t.Fatal(err)
					}
					if err := verifyNDKProperties(reader.File[0]); !errors.Is(err, want) {
						t.Fatalf("properties read = %v, want %v", err, want)
					}
					return
				}
				root := t.TempDir()
				archive := filepath.Join(root, "ndk.zip")
				writeToolFixture(t, archive, buffer.String(), 0o644)
				extracted := filepath.Join(root, "extract")
				for _, name := range []string{"first", "link"} {
					writeToolFixture(t, filepath.Join(extracted, ndkRoot, name), "materialized", 0o644)
				}
				if err := restoreNDKSymlinks(archive, extracted); !errors.Is(err, want) {
					t.Fatalf("symlink read = %v, want %v", err, want)
				}
				for _, name := range []string{"first", "link"} {
					path := filepath.Join(extracted, ndkRoot, name)
					if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
						t.Fatalf("materialized link was replaced before validation: %v, %v", info, err)
					}
					if data, err := os.ReadFile(path); err != nil || string(data) != "materialized" {
						t.Fatalf("materialized link changed: %q, %v", data, err)
					}
				}
			})
		}
	}
}

func TestNDKToolChecksRejectMissingCompilerAndFrontend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix NDK compiler fixture")
	}
	archive, err := androidNDKArchiveForOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clang-12", "clang"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeInstalledNDKFixture(t, root, archive.hostTag, false)
			if err := os.Remove(filepath.Join(root, "toolchains", "llvm", "prebuilt", archive.hostTag, "bin", name)); err != nil {
				t.Fatal(err)
			}
			if err := verifyInstalledNDK(root, archive); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("missing tool = %v", err)
			}
		})
	}
}

func TestRestoreNDKSymlinksRejectsBlockedMaterialization(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink fixture")
	}
	for _, scenario := range []string{"blocked link", "invalid target", "missing root"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			ndkRoot := testNDKRoot(t)
			archive := filepath.Join(root, "ndk.zip")
			var data bytes.Buffer
			writer := zip.NewWriter(&data)
			if scenario != "missing root" {
				writeNDKZipSymlink(t, writer, ndkRoot+"/link", "\x00")
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archive, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			extracted := filepath.Join(root, "extracted")
			link := filepath.Join(extracted, ndkRoot, "link")
			if scenario == "blocked link" {
				writeToolFixture(t, filepath.Join(link, "keep"), "original", 0600)
			} else if scenario == "invalid target" {
				writeToolFixture(t, link, "materialized target", 0600)
			}
			if err := restoreNDKSymlinks(archive, extracted); err == nil {
				t.Fatal("unusable symlink materialization accepted")
			}
			if scenario == "blocked link" {
				if content, err := os.ReadFile(filepath.Join(link, "keep")); err != nil || string(content) != "original" {
					t.Fatalf("blocked link changed: %q, %v", content, err)
				}
			}
		})
	}
}
