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
	"bytes"
	"crypto/sha256"
	"errors"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

type failingAssetHash struct {
	hash.Hash
	writes, failAt int
	err            error
}

type failingAssetFS struct {
	err error
}

func TestPrepareReturnsFalseWhenAssetMissing(t *testing.T) {
	for _, names := range [][]string{nil, {""}, {"missing-runtime-asset"}, {"placeholder.txt", "missing"}} {
		dir, ok, err := Prepare("9.9.9", names...)
		if err != nil || ok || dir != "" {
			t.Fatalf("Prepare(%v)=(%q, %v, %v), want no assets", names, dir, ok, err)
		}
	}
}

func TestExtractAssetCopiesEmbeddedContent(t *testing.T) {
	cacheDir := t.TempDir()
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	assertEmbeddedContent(t, filepath.Join(cacheDir, "placeholder.txt"))
}

func TestExtractAssetRepairsSameSizeCorruption(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	content, err := embeddedAssets.ReadFile(assetPath("placeholder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachedPath, bytes.Repeat([]byte{0}, len(content)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	assertEmbeddedContent(t, cachedPath)
}

func TestExtractAssetReusesValidCache(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(cachedPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cachedPath, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("valid asset cache was rewritten")
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0o644 {
		t.Fatalf("asset mode = %v, want 0644", after.Mode())
	}
}

func TestExtractAssetReplacesSymlinkWithoutChangingTarget(t *testing.T) {
	const name = "placeholder.txt"
	want, err := embeddedAssets.ReadFile(assetPath(name))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		content []byte
	}{
		{name: "matching content", content: want},
		{name: "different content", content: bytes.Repeat([]byte{0}, len(want))},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "external")
			if err := os.WriteFile(target, test.content, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			cacheDir := t.TempDir()
			cachedPath := filepath.Join(cacheDir, name)
			if err := os.Symlink(target, cachedPath); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := extractAsset(cacheDir, name); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Lstat(cachedPath); err != nil || !info.Mode().IsRegular() {
				t.Errorf("extracted cache must be a regular file: %v, %v", info, err)
			}
			assertEmbeddedContent(t, cachedPath)
			if after, err := os.Stat(target); err != nil || !os.SameFile(before, after) || after.Mode().Perm() != before.Mode().Perm() {
				t.Errorf("external target identity or permissions changed: before=%v after=%v err=%v", before, after, err)
			}
			if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, test.content) {
				t.Errorf("external target content = %q, %v; want %q", got, err, test.content)
			}
		})
	}
}

func TestExtractAssetReportsCacheRepairFailure(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	if err := os.Mkdir(cachedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cachedPath, "occupied"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := extractAsset(cacheDir, "placeholder.txt"); err == nil || !strings.Contains(err.Error(), cachedPath) {
		t.Fatalf("extractAsset() error = %v, want an error identifying %s", err, cachedPath)
	}
	content, err := os.ReadFile(filepath.Join(cachedPath, "occupied"))
	if err != nil || string(content) != "keep" {
		t.Fatalf("cache directory changed: content %q, error %v", content, err)
	}
	leftovers, err := filepath.Glob(cachedPath + ".tmp-*")
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary cache files remain: %v, error: %v", leftovers, err)
	}
}

func TestPrepareUsesIsolatedCacheAndReportsFailures(t *testing.T) {
	root := t.TempDir()
	setCacheEnvironment(t, root)
	base := defaultCacheBaseDir()
	if !strings.HasPrefix(base, root+string(os.PathSeparator)) && base != root {
		t.Fatalf("cache base %q escaped test directory %q", base, root)
	}
	dir, ok, err := Prepare("test-version", "placeholder.txt")
	if err != nil || !ok {
		t.Fatalf("Prepare=(%q,%v,%v)", dir, ok, err)
	}
	assertEmbeddedContent(t, filepath.Join(dir, "placeholder.txt"))
	if again, ok, err := Prepare("test-version", "placeholder.txt"); err != nil || !ok || again != dir {
		t.Fatalf("cache reuse=(%q,%v,%v), want %q", again, ok, err, dir)
	}
	if _, ok, err := Prepare("test-version", "."); ok || err == nil || !strings.Contains(err.Error(), "compute runtime cache key") {
		t.Fatalf("directory asset: ok=%v error=%v", ok, err)
	}
	blocker := filepath.Join(base, "spx", "embedded-runtime", "blocked")
	if err := os.WriteFile(blocker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Prepare("blocked", "placeholder.txt"); ok || err == nil || !strings.Contains(err.Error(), "create runtime cache dir") {
		t.Fatalf("blocked cache: ok=%v error=%v", ok, err)
	}
	cached := filepath.Join(dir, "placeholder.txt")
	if err := os.Remove(cached); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cached, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cached, "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Prepare("test-version", "placeholder.txt"); ok || err == nil || !strings.Contains(err.Error(), "runtime cache asset") {
		t.Fatalf("blocked asset: ok=%v error=%v", ok, err)
	}
}

func TestDefaultCacheBaseDirFallsBackWithoutUserCache(t *testing.T) {
	setCacheEnvironment(t, "")
	if got, want := defaultCacheBaseDir(), filepath.Join(os.TempDir(), "spx-cache"); got != want {
		t.Fatalf("fallback cache=%q, want %q", got, want)
	}
}

func TestAssetModes(t *testing.T) {
	for _, tt := range []struct {
		name string
		want os.FileMode
	}{
		{"gdspxrt1.0", 0o755}, {"gdspxrt1.0.exe", 0o755}, {"gdspxrt1.0.pck", 0o644},
		{"gdspx-linux", 0o755}, {"runtime.wasm", 0o644}, {"placeholder.txt", 0o644},
	} {
		if got := assetMode(tt.name); got != tt.want {
			t.Errorf("mode(%q)=%o, want %o", tt.name, got, tt.want)
		}
	}
}

func TestAssetHashIncludesNameAndPropagatesWriteFailures(t *testing.T) {
	hasher := sha256.New()
	if err := addFileHash(hasher, "file", strings.NewReader("payload")); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte("file\x00payload\x00"))
	if !bytes.Equal(hasher.Sum(nil), want[:]) {
		t.Fatal("asset hash omitted the name or field separators")
	}
	failure := errors.New("hash writer failed")
	for i, want := range []string{"hash runtime asset name", "hash runtime asset file", "finalize runtime asset hash"} {
		h := &failingAssetHash{Hash: sha256.New(), failAt: i + 1, err: failure}
		if err := addFileHash(h, "file", strings.NewReader("payload")); !errors.Is(err, failure) || !strings.Contains(err.Error(), want) {
			t.Errorf("write %d error=%v, want %q wrapping original error", i+1, err, want)
		}
	}
	if _, err := assetCacheKey([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "open embedded runtime asset") {
		t.Fatalf("missing asset hash error=%v", err)
	}
}

func TestCachedAssetMatchFailures(t *testing.T) {
	for _, tt := range []struct {
		name, asset, want  string
		directory, missing bool
	}{
		{name: "missing cached file", asset: "placeholder.txt", want: "open runtime cache asset", missing: true},
		{name: "missing embedded asset", asset: "missing", want: "open embedded runtime asset"},
		{name: "cached directory", asset: "placeholder.txt", want: "verify runtime cache asset", directory: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cached := filepath.Join(t.TempDir(), "cached")
			if tt.directory {
				if err := os.Mkdir(cached, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if !tt.missing {
				if err := os.WriteFile(cached, []byte("cached"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if matches, err := cachedAssetMatches(cached, tt.asset); matches || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("matches=%v error=%v, want %q error", matches, err, tt.want)
			}
		})
	}
}

func TestExtractAssetRejectsMissingSourceAndInvalidCache(t *testing.T) {
	if err := extractAsset(t.TempDir(), "missing"); err == nil || !strings.Contains(err.Error(), "stat embedded runtime asset") {
		t.Fatalf("missing asset error=%v", err)
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractAsset(blocker, "placeholder.txt"); err == nil || !strings.Contains(err.Error(), "create runtime cache parent dir") {
		t.Fatalf("invalid cache error=%v", err)
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := extractAsset(dir, "placeholder.txt"); err == nil || !strings.Contains(err.Error(), "create extracted runtime asset") {
		t.Fatalf("unwritable cache error=%v", err)
	}
}

func TestManifestCacheKeyRequiresExactOrderedAssetSet(t *testing.T) {
	const valid = `{"cache_key":"release-content-id","names":["engine","engine.pck"]}`
	for _, tt := range []struct {
		name     string
		manifest string
		names    []string
		wantKey  string
		wantErr  bool
	}{
		{name: "complete bundle", manifest: valid, names: []string{"engine", "engine.pck"}, wantKey: "release-content-id"},
		{name: "missing manifest", names: []string{"engine", "engine.pck"}},
		{name: "empty cache key", manifest: `{"names":["engine","engine.pck"]}`, names: []string{"engine", "engine.pck"}},
		{name: "subset", manifest: valid, names: []string{"engine"}},
		{name: "superset", manifest: valid, names: []string{"engine", "engine.pck", "bridge"}},
		{name: "same size different names", manifest: valid, names: []string{"engine", "other.pck"}},
		{name: "reordered assets", manifest: valid, names: []string{"engine.pck", "engine"}},
		{name: "malformed manifest", manifest: `{"cache_key":`, names: []string{"engine"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assets := fstest.MapFS{}
			if tt.manifest != "" {
				assets[assetPath(manifestFileName)] = &fstest.MapFile{Data: []byte(tt.manifest)}
			}
			before := append([]string(nil), tt.names...)
			key, found, err := manifestCacheKeyFromFS(assets, tt.names)
			if (err != nil) != tt.wantErr || key != tt.wantKey || found != (tt.wantKey != "") {
				t.Fatalf("manifest cache key = %q, %v, %v; want %q, error=%v", key, found, err, tt.wantKey, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.names, before) {
				t.Fatalf("asset request changed: %v, want %v", tt.names, before)
			}
		})
	}
	if key, found, err := manifestCacheKeyFromFS(failingAssetFS{err: fs.ErrPermission}, []string{"engine"}); key != "" || found || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("unreadable manifest = %q, %v, %v; want permission error without fallback", key, found, err)
	}
}

func TestPrepareConcurrentRequestsShareCompleteCache(t *testing.T) {
	setCacheEnvironment(t, t.TempDir())
	const workers = 8
	type result struct {
		dir     string
		ok      bool
		err     error
		content []byte
	}
	want, err := embeddedAssets.ReadFile(assetPath("placeholder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan result, workers)
	start := make(chan struct{})
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			<-start
			dir, ok, err := Prepare("concurrent-version", "placeholder.txt")
			var content []byte
			if err == nil && ok {
				content, err = os.ReadFile(filepath.Join(dir, "placeholder.txt"))
			}
			results <- result{dir: dir, ok: ok, err: err, content: content}
		})
	}
	close(start)
	group.Wait()
	close(results)
	var directory string
	for got := range results {
		if got.err != nil || !got.ok || got.dir == "" {
			t.Fatalf("Prepare = %q, %v, %v", got.dir, got.ok, got.err)
		}
		if directory == "" {
			directory = got.dir
		} else if got.dir != directory {
			t.Fatalf("cache directories differ: %q and %q", directory, got.dir)
		}
		if !bytes.Equal(got.content, want) {
			t.Fatalf("Prepare exposed incomplete cached content: %q, want %q", got.content, want)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "placeholder.txt" {
		t.Fatalf("cache publication left unexpected files: %v, %v", entries, err)
	}
}

func TestExtractAssetPreservesUnreadableCache(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions and an unprivileged user")
	}
	directory := t.TempDir()
	name := "placeholder.txt"
	if err := extractAsset(directory, name); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, name)
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })
	if err := extractAsset(directory, name); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unreadable cache = %v, want permission error", err)
	}
	after, err := os.Stat(target)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("unreadable cached asset was replaced: %v, %v", after, err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatal(err)
	}
	assertEmbeddedContent(t, target)
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 1 {
		t.Fatalf("failed cache verification left temporary files: %v, %v", entries, err)
	}
}

func (h *failingAssetHash) Write(p []byte) (int, error) {
	h.writes++
	if h.writes == h.failAt {
		return 0, h.err
	}
	return h.Hash.Write(p)
}

func (f failingAssetFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: f.err}
}

func assertEmbeddedContent(t *testing.T, path string) {
	t.Helper()
	want, err := embeddedAssets.ReadFile(assetPath("placeholder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file %s = %q, want %q", path, got, want)
	}
}

func setCacheEnvironment(t *testing.T, dir string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LocalAppData", dir)
	case "darwin", "ios":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CACHE_HOME", dir)
		t.Setenv("HOME", dir)
	}
}
