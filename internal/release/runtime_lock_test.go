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

package release

import (
	"bytes"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDefaultRuntimeLock(t *testing.T) {
	lock := DefaultRuntimeLock()
	if err := lock.Validate(); err != nil {
		t.Fatal(err)
	}
	if lock.RuntimeVersion != "2.4.0" || lock.RuntimeReleaseTag() != "runtime-v2.4.0" || lock.RuntimeABI != 2 {
		t.Fatalf("unexpected runtime identity: %#v", lock)
	}
	if got, want := lock.RuntimeAssetDownloadURL("linux-x86_64.zip"), "https://github.com/goplus/spx/releases/download/runtime-v2.4.0/linux-x86_64.zip"; got != want {
		t.Fatalf("runtime asset URL = %q, want %q", got, want)
	}
	if lock.ReleaseRepository != "goplus/spx" || lock.Manifest != "runtime-manifest.json" {
		t.Fatalf("unexpected release target: %#v", lock)
	}
	if lock.Godot.Repository != "https://github.com/goplus/godot.git" || lock.Godot.Ref != "spx4.4.1" {
		t.Fatalf("unexpected Godot source: %#v", lock.Godot)
	}
	if lock.Godot.Version != "4.4.1.stable" {
		t.Fatalf("unexpected Godot pin: %#v", lock.Godot)
	}
	if lock.Module.Path != "godot_modules/spx" {
		t.Fatalf("module path = %q", lock.Module.Path)
	}
	if !slices.IsSorted(lock.RequiredAssets) {
		t.Fatalf("required assets are not sorted: %v", lock.RequiredAssets)
	}
	if len(lock.RequiredAssets) != 18 {
		t.Fatalf("required asset count = %d, want 18", len(lock.RequiredAssets))
	}
	if !slices.Contains(lock.RequiredAssets, RuntimeAssetZipName) {
		t.Fatalf("required assets do not include %q", RuntimeAssetZipName)
	}
}

func TestCurrentRuntimeLockSnapshotMatchesDefault(t *testing.T) {
	current := DefaultRuntimeLock()
	snapshot, err := RuntimeLockForVersion(current.RuntimeVersion)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := snapshot.JSON()
	if err != nil {
		t.Fatal(err)
	}
	embeddedSnapshotJSON, err := fs.ReadFile(embeddedRuntimeLocks, "runtime_locks/"+current.RuntimeVersion+".json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshotJSON, embeddedRuntimeLockJSON) || !bytes.Equal(embeddedSnapshotJSON, embeddedRuntimeLockJSON) {
		t.Fatalf("current runtime lock snapshot drifted from runtime.lock.json")
	}
}

func TestRuntimeLockForVersionReturnsCopy(t *testing.T) {
	version := DefaultRuntimeLock().RuntimeVersion
	first, err := RuntimeLockForVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	first.RequiredAssets[0] = "changed.zip"
	second, err := RuntimeLockForVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	if second.RequiredAssets[0] == "changed.zip" {
		t.Fatal("RuntimeLockForVersion returned shared required_assets storage")
	}
}

func TestRuntimeLockForVersionRejectsUnknownVersion(t *testing.T) {
	if _, err := RuntimeLockForVersion("9.9.9"); err == nil || !strings.Contains(err.Error(), "no runtime lock snapshot") {
		t.Fatalf("RuntimeLockForVersion error = %v", err)
	}
}

func TestLoadRuntimeLocksRequiresMatchingFilename(t *testing.T) {
	data, err := DefaultRuntimeLock().JSON()
	if err != nil {
		t.Fatal(err)
	}
	_, err = loadRuntimeLocks(fstest.MapFS{
		"runtime_locks/9.9.9.json": &fstest.MapFile{Data: data},
	})
	if err == nil || !strings.Contains(err.Error(), "declares runtime version") {
		t.Fatalf("loadRuntimeLocks error = %v", err)
	}
}

func TestRuntimeLockCanonicalJSONAndSHA256(t *testing.T) {
	lock := DefaultRuntimeLock()
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, embeddedRuntimeLockJSON) {
		t.Fatalf("embedded lock is not canonical:\n%s", data)
	}
	parsed, err := ParseRuntimeLock(data)
	if err != nil {
		t.Fatal(err)
	}
	parsedSHA, err := parsed.SHA256()
	if err != nil {
		t.Fatal(err)
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		t.Fatal(err)
	}
	if parsedSHA != lockSHA || len(lockSHA) != 64 {
		t.Fatalf("lock SHA-256 mismatch: %q != %q", parsedSHA, lockSHA)
	}

	// DefaultRuntimeLock must not expose the embedded slice to mutation.
	parsed.RequiredAssets[0] = "changed.zip"
	if DefaultRuntimeLock().RequiredAssets[0] == "changed.zip" {
		t.Fatal("DefaultRuntimeLock returned shared required_assets storage")
	}
}

func TestRuntimeLockValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RuntimeLock)
	}{
		{"schema", func(lock *RuntimeLock) { lock.Schema = 2 }},
		{"version", func(lock *RuntimeLock) { lock.RuntimeVersion = "2.2" }},
		{"abi", func(lock *RuntimeLock) { lock.RuntimeABI = 0 }},
		{"release repository", func(lock *RuntimeLock) { lock.ReleaseRepository = "https://github.com/goplus/spx" }},
		{"manifest path", func(lock *RuntimeLock) { lock.Manifest = "metadata/runtime-manifest.json" }},
		{"empty assets", func(lock *RuntimeLock) { lock.RequiredAssets = nil }},
		{"unsorted assets", func(lock *RuntimeLock) {
			lock.RequiredAssets[0], lock.RequiredAssets[1] = lock.RequiredAssets[1], lock.RequiredAssets[0]
		}},
		{"asset path", func(lock *RuntimeLock) { lock.RequiredAssets[0] = "nested/android.zip" }},
		{"asset control character", func(lock *RuntimeLock) { lock.RequiredAssets[0] = "android\nrelease.zip" }},
		{"duplicate asset", func(lock *RuntimeLock) { lock.RequiredAssets[1] = lock.RequiredAssets[0] }},
		{"Godot repository", func(lock *RuntimeLock) { lock.Godot.Repository = "https://example.com/goplus/godot.git" }},
		{"Godot repository shorthand", func(lock *RuntimeLock) { lock.Godot.Repository = "goplus/godot" }},
		{"Godot ref", func(lock *RuntimeLock) { lock.Godot.Ref = "" }},
		{"Godot ref whitespace", func(lock *RuntimeLock) { lock.Godot.Ref = " main" }},
		{"Godot ref internal whitespace", func(lock *RuntimeLock) { lock.Godot.Ref = "feature branch" }},
		{"Godot short SHA", func(lock *RuntimeLock) { lock.Godot.Commit = "31047ba3" }},
		{"Godot uppercase SHA", func(lock *RuntimeLock) { lock.Godot.Commit = strings.Repeat("A", 40) }},
		{"Godot version", func(lock *RuntimeLock) { lock.Godot.Version = "" }},
		{"Godot version whitespace", func(lock *RuntimeLock) { lock.Godot.Version = "4.4.1 " }},
		{"Godot version internal whitespace", func(lock *RuntimeLock) { lock.Godot.Version = "4.4.1 stable" }},
		{"absolute module", func(lock *RuntimeLock) { lock.Module.Path = "/tmp/spx" }},
		{"empty module", func(lock *RuntimeLock) { lock.Module.Path = "" }},
		{"module traversal", func(lock *RuntimeLock) { lock.Module.Path = "modules/../spx" }},
		{"module revision injection", func(lock *RuntimeLock) { lock.Module.Path = "godot_modules/spx:refs/heads/main" }},
		{"module backslash", func(lock *RuntimeLock) { lock.Module.Path = `godot_modules\spx` }},
		{"module whitespace", func(lock *RuntimeLock) { lock.Module.Path = "godot modules/spx" }},
		{"module unicode", func(lock *RuntimeLock) { lock.Module.Path = "godot_modules/模块" }},
		{"toolchain", func(lock *RuntimeLock) { lock.Toolchain.EMSDK = "" }},
		{"toolchain whitespace", func(lock *RuntimeLock) { lock.Toolchain.SCons = "4.8.1 latest" }},
		{"JDK major", func(lock *RuntimeLock) { lock.Toolchain.JDK = "17.0" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := DefaultRuntimeLock()
			test.mutate(&lock)
			if err := lock.Validate(); err == nil {
				t.Fatalf("Validate succeeded for %#v", lock)
			}
		})
	}
}

func TestParseRuntimeLockRejectsUnknownAndTrailingJSON(t *testing.T) {
	data, err := DefaultRuntimeLock().JSON()
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := bytes.Replace(data, []byte(`"schema": 1`), []byte(`"unknown": true, "schema": 1`), 1)
	if _, err := ParseRuntimeLock(withUnknown); err == nil {
		t.Fatal("ParseRuntimeLock accepted an unknown field")
	}
	if _, err := ParseRuntimeLock(append(data, []byte(`{}`)...)); err == nil {
		t.Fatal("ParseRuntimeLock accepted a second JSON value")
	}
	withDuplicate := bytes.Replace(data, []byte(`"schema": 1`), []byte(`"schema": 1, "schema": 1`), 1)
	if _, err := ParseRuntimeLock(withDuplicate); err == nil {
		t.Fatal("ParseRuntimeLock accepted a duplicate key")
	}
	withNestedDuplicate := bytes.Replace(data, []byte(`"repository": "https://github.com/goplus/godot.git"`), []byte(`"repository": "https://github.com/goplus/godot.git", "repository": "https://github.com/other/godot.git"`), 1)
	if _, err := ParseRuntimeLock(withNestedDuplicate); err == nil {
		t.Fatal("ParseRuntimeLock accepted a nested duplicate key")
	}
}
