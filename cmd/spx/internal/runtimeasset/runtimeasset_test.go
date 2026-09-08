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

package runtimeasset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func TestPrepareExtractsEmbeddedAssets(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":              &fstest.MapFile{Data: []byte("runtime")},
		"assets/gdspxrt9.9.9.pck":          &fstest.MapFile{Data: []byte("runtime-pck")},
		"assets/gdspx-linux-amd64.so":      &fstest.MapFile{Data: []byte("shared-lib")},
		"assets/runtime.gdextension":       &fstest.MapFile{Data: []byte("unused")},
		"assets/placeholder.txt":           &fstest.MapFile{Data: []byte("placeholder")},
		"assets/gdspx-darwin-amd64.dylib":  &fstest.MapFile{Data: []byte("unused-darwin")},
		"assets/gdspx-windows-amd64.dll":   &fstest.MapFile{Data: []byte("unused-windows")},
		"assets/gdspx-linux-arm64.so":      &fstest.MapFile{Data: []byte("unused-linux-arm64")},
		"assets/gdspxrt9.9.9.exe":          &fstest.MapFile{Data: []byte("unused-runtime-exe")},
		"assets/gdspxrt9.9.9.exe.pck":      &fstest.MapFile{Data: []byte("unused-runtime-exe-pck")},
		"assets/gdspx-windows-arm64.dll":   &fstest.MapFile{Data: []byte("unused-windows-arm64")},
		"assets/gdspx-darwin-arm64.dylib":  &fstest.MapFile{Data: []byte("unused-darwin-arm64")},
		"assets/gdspx-linux-x86_64.so":     &fstest.MapFile{Data: []byte("unused-linux-x86_64")},
		"assets/gdspx-linux-x86_32.so":     &fstest.MapFile{Data: []byte("unused-linux-x86_32")},
		"assets/gdspx-windows-x86_64.dll":  &fstest.MapFile{Data: []byte("unused-windows-x86_64")},
		"assets/gdspx-windows-x86_32.dll":  &fstest.MapFile{Data: []byte("unused-windows-x86_32")},
		"assets/gdspx-darwin-x86_64.dylib": &fstest.MapFile{Data: []byte("unused-darwin-x86_64")},
	}
	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare returned ok=false, want true")
	}

	assertFileContent(t, filepath.Join(dir, "gdspxrt9.9.9"), "runtime")
	assertFileContent(t, filepath.Join(dir, "gdspxrt9.9.9.pck"), "runtime-pck")
	assertFileContent(t, filepath.Join(dir, "gdspx-linux-amd64.so"), "shared-lib")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "gdspxrt9.9.9"))
		if err != nil {
			t.Fatalf("stat runtime executable: %v", err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("runtime executable mode = %v, want executable bit", info.Mode())
		}
	}
}

func TestPrepareReturnsFalseWhenAssetMissing(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	assetsFS = fstest.MapFS{
		"assets/placeholder.txt": &fstest.MapFile{Data: []byte("placeholder")},
	}
	cacheBaseDirFn = func() string { return t.TempDir() }

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if ok {
		t.Fatalf("Prepare returned ok=true with dir=%s, want false", dir)
	}
}

func TestPrepareUsesManifestCacheKeyWhenAvailable(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }
	assetsFS = fstest.MapFS{
		"assets/manifest.json":        &fstest.MapFile{Data: []byte(`{"cache_key":"manifest-key","names":["gdspxrt9.9.9","gdspxrt9.9.9.pck","gdspx-linux-amd64.so"]}`)},
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("runtime-pck")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-lib")},
		"assets/placeholder.txt":      &fstest.MapFile{Data: []byte("placeholder")},
	}

	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare returned ok=false, want true")
	}
	if filepath.Base(dir) != "manifest-key" {
		t.Fatalf("Prepare cache dir = %s, want suffix manifest-key", dir)
	}
	assertFileContent(t, filepath.Join(dir, "gdspx-linux-amd64.so"), "shared-lib")
}

func TestPrepareSeparatesCacheDirsByEmbeddedContent(t *testing.T) {
	oldFS := assetsFS
	oldCacheBaseDirFn := cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS = oldFS
		cacheBaseDirFn = oldCacheBaseDirFn
	})

	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime-v1")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("pack-v1")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-one")},
	}
	dirV1, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare(v1) returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare(v1) returned ok=false, want true")
	}
	assertFileContent(t, filepath.Join(dirV1, "gdspx-linux-amd64.so"), "shared-one")

	assetsFS = fstest.MapFS{
		"assets/gdspxrt9.9.9":         &fstest.MapFile{Data: []byte("runtime-v2")},
		"assets/gdspxrt9.9.9.pck":     &fstest.MapFile{Data: []byte("pack-v2")},
		"assets/gdspx-linux-amd64.so": &fstest.MapFile{Data: []byte("shared-two")},
	}
	dirV2, ok, err := Prepare("9.9.9", "gdspxrt9.9.9", "gdspxrt9.9.9.pck", "gdspx-linux-amd64.so")
	if err != nil {
		t.Fatalf("Prepare(v2) returned error: %v", err)
	}
	if !ok {
		t.Fatal("Prepare(v2) returned ok=false, want true")
	}
	if dirV1 == dirV2 {
		t.Fatalf("Prepare reused cache dir %s for different embedded content", dirV1)
	}
	assertFileContent(t, filepath.Join(dirV1, "gdspx-linux-amd64.so"), "shared-one")
	assertFileContent(t, filepath.Join(dirV2, "gdspx-linux-amd64.so"), "shared-two")
}

func TestPrepareRepairsSameSizeCorruption(t *testing.T) {
	for _, withManifest := range []bool{false, true} {
		name := "without manifest"
		if withManifest {
			name = "with manifest"
		}
		t.Run(name, func(t *testing.T) {
			files := fstest.MapFS{
				"assets/gdspxrt9.9.9": {Data: []byte("runtime")},
			}
			if withManifest {
				files["assets/manifest.json"] = &fstest.MapFile{Data: []byte(`{"cache_key":"manifest-key","names":["gdspxrt9.9.9"]}`)}
			}
			setTestRuntimeAssets(t, files)
			dir := prepareTestRuntime(t)
			cachedPath := filepath.Join(dir, "gdspxrt9.9.9")
			if err := os.WriteFile(cachedPath, []byte("corrupt"), 0o755); err != nil {
				t.Fatal(err)
			}

			if repairedDir := prepareTestRuntime(t); repairedDir != dir {
				t.Fatalf("cache directory changed from %s to %s", dir, repairedDir)
			}
			assertFileContent(t, cachedPath, "runtime")
		})
	}
}

func TestPrepareReusesValidCache(t *testing.T) {
	setTestRuntimeAssets(t, fstest.MapFS{
		"assets/gdspxrt9.9.9": {Data: []byte("runtime")},
	})
	dir := prepareTestRuntime(t)
	cachedPath := filepath.Join(dir, "gdspxrt9.9.9")
	stamp := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(cachedPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cachedPath, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	prepareTestRuntime(t)
	after, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("valid runtime cache was rewritten")
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0o755 {
		t.Fatalf("runtime mode = %v, want 0755", after.Mode())
	}
}

func TestPrepareConcurrentCacheRepair(t *testing.T) {
	content := strings.Repeat("runtime", 1<<16)
	setTestRuntimeAssets(t, fstest.MapFS{
		"assets/gdspxrt9.9.9": {Data: []byte(content)},
	})
	dir := prepareTestRuntime(t)
	cachedPath := filepath.Join(dir, "gdspxrt9.9.9")
	if err := os.WriteFile(cachedPath, []byte(strings.Repeat("corrupt", 1<<16)), 0o755); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			gotDir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
			if err != nil || !ok || gotDir != dir {
				t.Errorf("Prepare() = (%q, %v, %v), want (%q, true, nil)", gotDir, ok, err, dir)
				return
			}
			data, err := os.ReadFile(cachedPath)
			if err != nil || string(data) != content {
				t.Errorf("Prepare returned an incomplete or corrupt runtime: %v", err)
			}
		})
	}
	wg.Wait()
}

func TestPrepareReportsCacheRepairFailure(t *testing.T) {
	setTestRuntimeAssets(t, fstest.MapFS{
		"assets/gdspxrt9.9.9": {Data: []byte("runtime")},
	})
	dir := prepareTestRuntime(t)
	cachedPath := filepath.Join(dir, "gdspxrt9.9.9")
	if err := os.Remove(cachedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cachedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cachedPath, "occupied"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotDir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
	if err == nil || !strings.Contains(err.Error(), cachedPath) || gotDir != "" || ok {
		t.Fatalf("Prepare() = (%q, %v, %v), want an error identifying %s", gotDir, ok, err, cachedPath)
	}
	assertFileContent(t, filepath.Join(cachedPath, "occupied"), "keep")
	leftovers, err := filepath.Glob(cachedPath + ".tmp-*")
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary cache files remain: %v, error: %v", leftovers, err)
	}
}

func TestPrepareReportsCacheVerificationFailure(t *testing.T) {
	setTestRuntimeAssets(t, fstest.MapFS{
		"assets/manifest.json": {Data: []byte(`{"cache_key":"manifest-key","names":["gdspxrt9.9.9"]}`)},
		"assets/gdspxrt9.9.9":  {Data: []byte("runtime")},
	})
	dir := prepareTestRuntime(t)
	readErr := errors.New("asset read failed")
	assetsFS = readErrorFS{FS: assetsFS, name: "assets/gdspxrt9.9.9", err: readErr}

	gotDir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
	if !errors.Is(err, readErr) || gotDir != "" || ok {
		t.Fatalf("Prepare() = (%q, %v, %v), want asset read error", gotDir, ok, err)
	}
	assertFileContent(t, filepath.Join(dir, "gdspxrt9.9.9"), "runtime")
}

type readErrorFS struct {
	fs.FS
	name string
	err  error
}

func (f readErrorFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err == nil && name == f.name {
		return readErrorFile{File: file, err: f.err}, nil
	}
	return file, err
}

type readErrorFile struct {
	fs.File
	err error
}

func (f readErrorFile) Read([]byte) (int, error) { return 0, f.err }

func setTestRuntimeAssets(t *testing.T, files fstest.MapFS) {
	t.Helper()
	oldFS, oldCacheBaseDirFn := assetsFS, cacheBaseDirFn
	t.Cleanup(func() {
		assetsFS, cacheBaseDirFn = oldFS, oldCacheBaseDirFn
	})
	assetsFS = files
	cacheRoot := t.TempDir()
	cacheBaseDirFn = func() string { return cacheRoot }
}

func prepareTestRuntime(t *testing.T) string {
	t.Helper()
	dir, ok, err := Prepare("9.9.9", "gdspxrt9.9.9")
	if err != nil || !ok {
		t.Fatalf("Prepare() = (%q, %v, %v), want an extracted runtime", dir, ok, err)
	}
	return dir
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s = %q, want %q", path, string(data), want)
	}
}
