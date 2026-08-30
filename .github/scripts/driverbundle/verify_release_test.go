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

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
)

func TestVerifyReleaseChecksVersionsAndBundles(t *testing.T) {
	lock, err := release.RuntimeLockForVersion("2.4.3")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	targets := [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"windows", "amd64"}}
	bundles := make([]driverbundle.Bundle, 0, len(targets))
	for _, target := range targets {
		bundles = append(bundles, writeReleaseTestBundle(t, directory, lock, target[0], target[1]))
	}
	manifest := driverbundle.Manifest{
		Schema: driverbundle.ManifestSchema, SPXVersion: "v3.2.4", RuntimeVersion: lock.RuntimeVersion,
		Bundles: bundles,
	}
	manifestData, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, driverbundle.ManifestName), manifestData, 0o600); err != nil {
		t.Fatal(err)
	}

	args := []string{"--lock", lockPath, "--directory", directory, "--spx-version", manifest.SPXVersion}
	if err := runVerifyRelease(args); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "spx_web.zip"), []byte("product"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runVerifyRelease(args); err != nil {
		t.Fatalf("driver verification rejected another SPX release asset: %v", err)
	}
	if err := runVerifyRelease([]string{"--lock", lockPath, "--directory", directory, "--spx-version", "v9.9.9"}); err == nil {
		t.Fatal("accepted mismatched SPX version")
	}
	otherLock, err := release.RuntimeLockForVersion("2.4.2")
	if err != nil {
		t.Fatal(err)
	}
	otherLockData, err := otherLock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	otherLockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	if err := os.WriteFile(otherLockPath, otherLockData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runVerifyRelease([]string{"--lock", otherLockPath, "--directory", directory, "--spx-version", manifest.SPXVersion}); err == nil {
		t.Fatal("accepted mismatched runtime version")
	}

	if err := os.WriteFile(filepath.Join(directory, bundles[0].Name), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runVerifyRelease(args); err == nil {
		t.Fatal("accepted tampered release")
	}
}

func writeReleaseTestBundle(t *testing.T, directory string, lock release.RuntimeLock, goos, goarch string) driverbundle.Bundle {
	t.Helper()
	spec, err := release.HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		t.Fatal(err)
	}
	extension := map[string]string{"darwin": ".dylib", "linux": ".so", "windows": ".dll"}[goos]
	names := [3]string{spec.RuntimeName, spec.PackName, "gdspx-" + goos + "-" + goarch + extension}
	modes := [3]uint32{0o755, 0o644, 0o755}
	name := "spx-driver-" + goos + "-" + goarch + ".zip"
	path := filepath.Join(directory, name)
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	files := make([]driverbundle.File, 0, len(names))
	for i, name := range names {
		data := []byte(name + " bytes")
		header := &zip.FileHeader{Name: name, Method: zip.Store, Modified: zipEpoch}
		header.SetMode(os.FileMode(modes[i]))
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		files = append(files, driverbundle.File{Name: name, Mode: modes[i], Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])})
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(files[0].SHA256, files[1].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	return driverbundle.Bundle{
		GOOS: goos, GOARCH: goarch, Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]),
		EngineInterfaceDigest: interfaceDigest, Files: files,
	}
}
