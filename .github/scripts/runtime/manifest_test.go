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
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

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

type manifestFailWriter struct{ err error }

func (w manifestFailWriter) Write([]byte) (int, error) { return 0, w.err }

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
			os.Stdout, os.Args = output, append([]string{"manifest"}, test.args...)
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
	for _, flag := range []string{"--output", "--checksums"} {
		t.Run(flag, func(t *testing.T) {
			badArgs := append(append([]string(nil), args...), "--output", filepath.Join(dir, "manifest.json"), flag, dir)
			if err := runReleaseManifest(badArgs); err == nil {
				t.Fatalf("accepted directory for %s", flag)
			}
		})
	}
}
