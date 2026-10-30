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

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

type manifestFailWriter struct{ err error }

func TestRunReleaseManifestGenerateAndVerify(t *testing.T) {
	t.Parallel()

	lock := release.DefaultRuntimeLock()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, lock.Manifest)
	checksumsPath := filepath.Join(dir, release.SHA256SumsFileName)
	args := []string{
		"--output", manifestPath,
		"--checksums", checksumsPath,
		"--spx-commit", strings.Repeat("1", 40),
		"--module-tree", strings.Repeat("2", 40),
		"--runtime-pack-source-sha256", strings.Repeat("3", 64),
		"--build-recipe-sha256", strings.Repeat("4", 64),
	}
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("asset:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--asset", name+"="+path)
	}
	if err := runReleaseManifest(args); err != nil {
		t.Fatalf("generate manifest: %v", err)
	}
	if err := runReleaseManifest([]string{
		"--verify-manifest", manifestPath,
		"--asset-directory", dir,
	}); err != nil {
		t.Fatalf("verify generated manifest: %v", err)
	}
	manifest, err := release.LoadRuntimeManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.RuntimeABI++
	manifest.ReleaseRepository = "example/runtime"
	manifest.LockSHA256 = strings.Repeat("0", 64)
	manifest.Provenance.GodotCommit = strings.Repeat("5", 40)
	if err := release.WriteRuntimeManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	if err := runReleaseManifest([]string{"--verify-manifest", manifestPath}); err != nil {
		t.Fatalf("validate same-version manifest with different build metadata: %v", err)
	}

	corruptPath := filepath.Join(dir, lock.RequiredAssets[0])
	if err := os.WriteFile(corruptPath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runReleaseManifest([]string{
		"--verify-manifest", manifestPath,
		"--asset-directory", dir,
	}); err == nil {
		t.Fatal("verify corrupted asset: got nil error")
	}
}

func TestRunReleaseManifestRejectsMixedModes(t *testing.T) {
	t.Parallel()

	err := runReleaseManifest([]string{
		"--verify-manifest", "runtime-manifest.json",
		"--asset-directory", ".",
		"--asset", "runtime.zip=runtime.zip",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("mixed modes error = %v", err)
	}
	for _, flag := range []string{"--spx-commit", "--module-tree", "--runtime-pack-source-sha256", "--build-recipe-sha256", "--verify-manifest"} {
		for _, value := range []string{"", "explicit"} {
			args := []string{"--revision", "HEAD", flag, value}
			if err := runReleaseManifest(args); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
				t.Errorf("runReleaseManifest(%q) = %v; want incompatible inputs error", args, err)
			}
		}
	}
}

func TestRevisionManifestMatchesExplicitProvenance(t *testing.T) {
	lock, git := newManifestRepository(t)
	commit := git("rev-parse", "HEAD")
	moduleTree := git("rev-parse", commit+":"+lock.Module.Path)
	packDigest, err := release.RuntimePackSourceSHA256(".", commit)
	if err != nil {
		t.Fatal(err)
	}
	recipeDigest, err := release.RuntimeBuildRecipeSHA256(".", commit)
	if err != nil {
		t.Fatal(err)
	}
	// HEAD and dirty files differ from the requested revision in all three input
	// groups, so reading a worktree or silently using HEAD changes the result.
	for _, name := range []string{"game.go", lock.Module.Path + "/register.cpp", "internal/cmd/buildctl/main.go"} {
		writeManifestFixtureFile(t, name, "// second commit\n")
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "later inputs")
	git("tag", "historical", commit)
	git("branch", "historical", "HEAD")
	writeManifestFixtureFile(t, "game.go", "// dirty working tree\n")
	args := manifestFixtureAssetArgs(t, lock)
	outputs := t.TempDir()
	var expectedManifest, expectedChecksums []byte
	for _, mode := range []struct {
		name string
		args []string
	}{
		{"explicit", []string{"--spx-commit", commit, "--module-tree", moduleTree,
			"--runtime-pack-source-sha256", packDigest, "--build-recipe-sha256", recipeDigest}},
		{"revision", []string{"--revision", "HEAD^"}},
		// Git resolves this short name to the tag and warns on stderr; the
		// warning must not become part of the commit's object ID.
		{"ambiguous alias", []string{"--revision", "historical"}},
	} {
		manifestPath := filepath.Join(outputs, mode.name+".json")
		checksumsPath := filepath.Join(outputs, mode.name+".sums")
		command := append(append([]string(nil), args...), mode.args...)
		command = append(command, "--output", manifestPath, "--checksums", checksumsPath)
		if err := runReleaseManifest(command); err != nil {
			t.Fatalf("%s generation: %v", mode.name, err)
		}
		manifest, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		checksums, err := os.ReadFile(checksumsPath)
		if err != nil {
			t.Fatal(err)
		}
		if mode.name == "explicit" {
			expectedManifest, expectedChecksums = manifest, checksums
		} else if !bytes.Equal(manifest, expectedManifest) || !bytes.Equal(checksums, expectedChecksums) {
			t.Fatal("revision mode changed manifest or SHA256SUMS bytes from explicit provenance")
		}
	}
}

func TestRevisionManifestFailuresPreserveOutputs(t *testing.T) {
	for _, test := range []struct {
		name     string
		revision string
		module   string
		remove   string
		want     string
	}{
		{"unknown revision", "missing-revision", "modules/fixture", "", "resolve Git object"},
		{"empty revision", "", "modules/fixture", "", "--revision must not be empty"},
		{"blank revision", " ", "modules/fixture", "", "--revision must not be empty"},
		{"option-shaped revision", "--help", "modules/fixture", "", "resolve Git object"},
		{"empty module", "HEAD", "", "", "module path"},
		{"missing module", "HEAD", "missing/module", "", "resolve Git object"},
		{"missing pack inputs", "HEAD", "modules/fixture", "game.go", "runtime pack source tree contains no tracked inputs"},
		{"missing recipe inputs", "HEAD", "modules/fixture", "internal/cmd/buildctl/main.go", "runtime build recipe tree contains no tracked inputs"},
		{"git unavailable", "HEAD", "modules/fixture", "", "resolve Git object"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lock, git := newManifestRepository(t)
			lock.Module.Path = test.module
			data, err := json.Marshal(lock)
			if err != nil {
				t.Fatal(err)
			}
			writeManifestFixtureFile(t, "runtime.lock.json", string(data))
			if test.remove != "" {
				git("rm", test.remove)
				git("commit", "--quiet", "-m", "remove inputs")
			}
			args := append(manifestFixtureAssetArgs(t, lock), "--revision", test.revision,
				"--output", "manifest.json", "--checksums", "SHA256SUMS")
			for _, path := range []string{"manifest.json", "SHA256SUMS"} {
				writeManifestFixtureFile(t, path, "previous output")
			}
			if test.name == "git unavailable" {
				t.Setenv("PATH", t.TempDir())
			}
			err = runReleaseManifest(args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("generation error = %v; want %q", err, test.want)
			}
			if test.name == "git unavailable" && !errors.Is(err, exec.ErrNotFound) {
				t.Fatalf("Git lookup error was not preserved: %v", err)
			}
			for _, path := range []string{"manifest.json", "SHA256SUMS"} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "previous output" {
					t.Fatalf("failed generation changed %s: %q, %v", path, data, err)
				}
			}
		})
	}
}

func newManifestRepository(t *testing.T) (release.RuntimeLock, func(...string) string) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Manifest Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "manifest@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Manifest Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "manifest@example.invalid")
	git := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	lock := release.DefaultRuntimeLock()
	lock.Module.Path = "modules/fixture"
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	writeManifestFixtureFile(t, "runtime.lock.json", string(data))
	writeManifestFixtureFile(t, "game.go", "package spx\n")
	writeManifestFixtureFile(t, lock.Module.Path+"/register.cpp", "// module\n")
	writeManifestFixtureFile(t, "internal/cmd/buildctl/main.go", "package main\n")
	git("init", "--quiet")
	git("config", "commit.gpgSign", "false")
	git("add", ".")
	git("commit", "--quiet", "-m", "manifest inputs")
	return lock, git
}

func manifestFixtureAssetArgs(t *testing.T, lock release.RuntimeLock) []string {
	t.Helper()
	args := []string{"--lock", "runtime.lock.json"}
	for _, name := range lock.RequiredAssets {
		path := filepath.Join("assets", name)
		writeManifestFixtureFile(t, path, "asset:"+name)
		args = append(args, "--asset", name+"="+path)
	}
	return args
}

func writeManifestFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunReleaseManifestRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--unknown"}, "flag provided but not defined"},
		{"positional argument", []string{"extra"}, "unexpected positional"},
		{"missing lock", []string{"--lock", filepath.Join(dir, "missing")}, "read runtime lock"},
		{"invalid lock", []string{"--lock", invalid}, "decode runtime lock"},
		{"missing manifest", []string{"--verify-manifest", filepath.Join(dir, "missing")}, "read runtime manifest"},
		{"invalid manifest", []string{"--verify-manifest", invalid}, "decode runtime manifest"},
		{"asset directory in generation", []string{"--asset-directory", dir}, "requires --verify-manifest"},
		{"missing assets", nil, "at least one --asset"},
		{"missing separator", []string{"--asset", "invalid"}, "want name=path"},
		{"empty name", []string{"--asset", "=path"}, "non-empty name=path"},
		{"empty path", []string{"--asset", "name="}, "non-empty name=path"},
		{"incomplete asset set", []string{"--asset", "runtime.zip=path"}, "asset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runReleaseManifest(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runReleaseManifest(%q) = %v, want %q", test.args, err, test.want)
			}
		})
	}
}

func TestEmbeddedAssetManifest(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first"), filepath.Join(dir, "second")
	for path, contents := range map[string]string{first: "one", second: "two"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := writeEmbeddedAssetManifest([]string{"first=" + first, "second=" + second}, &output); err != nil {
		t.Fatal(err)
	}
	var manifest assetManifest
	if err := json.Unmarshal(output.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("first\x00one\x00second\x00two\x00"))
	if got, want := manifest.CacheKey, hex.EncodeToString(digest[:])[:16]; got != want {
		t.Fatalf("cache key = %q, want %q", got, want)
	}
	if strings.Join(manifest.Names, ",") != "first,second" {
		t.Fatalf("names = %q", manifest.Names)
	}
	for _, args := range [][]string{nil, {"missing separator"}, {"=path"}, {"name="}, {"missing=" + filepath.Join(dir, "missing")}} {
		if err := writeEmbeddedAssetManifest(args, io.Discard); err == nil {
			t.Errorf("writeEmbeddedAssetManifest(%q) succeeded", args)
		}
	}
	failure := errors.New("writer failed")
	if err := writeEmbeddedAssetManifest([]string{"first=" + first}, manifestFailWriter{failure}); !errors.Is(err, failure) || !strings.Contains(err.Error(), "encode manifest") {
		t.Fatalf("encode error = %v", err)
	}
	if err := hashEmbeddedAsset(manifestFailWriter{failure}, "first", first); !errors.Is(err, failure) {
		t.Fatalf("hash write error = %v", err)
	}
	if err := hashEmbeddedAsset(io.Discard, "directory", dir); err == nil {
		t.Fatal("hashed a directory")
	}
}

func TestManifestMainModesAndOutputFailures(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	dir := t.TempDir()
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(dir, "runtime.lock.json")
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--lock", lockPath, "--spx-commit", strings.Repeat("1", 40), "--module-tree", strings.Repeat("2", 40), "--runtime-pack-source-sha256", strings.Repeat("3", 64), "--build-recipe-sha256", strings.Repeat("4", 64)}
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--asset", name+"="+path)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{"embedded", []string{"asset=" + filepath.Join(dir, lock.RequiredAssets[0])}},
		{"release", args},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalArgs, originalStdout := os.Args, os.Stdout
			t.Cleanup(func() { os.Args, os.Stdout = originalArgs, originalStdout })
			output, err := os.CreateTemp(dir, "stdout-")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			os.Stdout, os.Args = output, append([]string{"runtime-manifest"}, test.args...)
			main()
			data, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "embedded" {
				var got assetManifest
				if err := json.Unmarshal(data, &got); err != nil || strings.Join(got.Names, ",") != "asset" || len(got.CacheKey) != 16 {
					t.Fatalf("embedded manifest = %q, error %v", data, err)
				}
			} else {
				manifest, err := release.ParseRuntimeManifestForRelease(data, lock.RuntimeVersion, lock.RequiredAssets)
				if err != nil {
					t.Fatalf("release manifest = %q, error %v", data, err)
				}
				if err := manifest.VerifyFiles(dir); err != nil {
					t.Fatalf("release manifest assets: %v", err)
				}
			}
		})
	}
	t.Run("closed stdout", func(t *testing.T) {
		output, err := os.CreateTemp(dir, "closed-stdout-")
		if err != nil {
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		originalStdout := os.Stdout
		t.Cleanup(func() { os.Stdout = originalStdout })
		os.Stdout = output
		if err := runReleaseManifest(args); !errors.Is(err, os.ErrClosed) || !strings.Contains(err.Error(), "write runtime manifest:") {
			t.Fatalf("closed stdout error = %v; want manifest write failure", err)
		}
	})

	for _, flag := range []string{"--output", "--checksums"} {
		t.Run(flag, func(t *testing.T) {
			badArgs := append(append([]string(nil), args...), "--output", filepath.Join(dir, "manifest.json"), flag, dir)
			if err := runReleaseManifest(badArgs); err == nil {
				t.Fatalf("accepted directory for %s", flag)
			}
		})
	}
}

func (w manifestFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestManifestMainReportsFailures(t *testing.T) {
	const helper = "SPX_TEST_MANIFEST_MAIN_ERROR"
	if mode := os.Getenv(helper); mode != "" {
		os.Args = []string{"runtime-manifest", "--unknown-manifest-option"}
		if mode == "embedded" {
			os.Args = []string{"runtime-manifest", "asset=missing-asset"}
		}
		main()
		return
	}
	for _, tt := range []struct {
		name    string
		message string
	}{
		{"embedded", "missing-asset"},
		{"release", "flag provided but not defined"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestManifestMainReportsFailures$")
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), helper+"="+tt.name)
			output, err := cmd.Output()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(output) != 0 {
				t.Fatalf("main = %v, stdout = %q; want exit 1 without manifest", err, output)
			}
			if !strings.Contains(string(exit.Stderr), tt.message) {
				t.Fatalf("stderr = %q; want %q", exit.Stderr, tt.message)
			}
		})
	}
}
