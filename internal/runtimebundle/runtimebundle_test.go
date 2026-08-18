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

package runtimebundle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type testZipEntry struct {
	name   string
	mode   fs.FileMode
	data   string
	method uint16
}

func writeTestZip(t *testing.T, entries ...testZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: item.method}
		if header.Method == 0 {
			header.Method = zip.Store
		}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.data)); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func testDigest(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func TestVerifyZipBuildsFullManifestAndRejectsUnsafeNames(t *testing.T) {
	good := writeTestZip(t,
		testZipEntry{name: "bin/run", mode: 0o755, data: "run"},
		testZipEntry{name: "assets/", mode: fs.ModeDir | 0o755},
		testZipEntry{name: "assets/a.txt", mode: 0o644, data: "hello"},
	)
	bundle, err := VerifyZip(good)
	if err != nil {
		t.Fatalf("VerifyZip(good): %v", err)
	}
	if bundle.Digest == "" || len(bundle.Digest) != sha256.Size*2 {
		t.Fatalf("bundle digest = %q, want full SHA-256", bundle.Digest)
	}
	if len(bundle.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(bundle.Entries))
	}
	for _, entry := range bundle.Entries {
		if entry.SHA256 == "" || len(entry.SHA256) != sha256.Size*2 {
			t.Fatalf("entry %q digest = %q, want full SHA-256", entry.Name, entry.SHA256)
		}
	}

	unsafe := []struct {
		name string
		want error
	}{
		{"../escape", ErrInvalidEntryName},
		{"/absolute", ErrInvalidEntryName},
		{"a\\b", ErrInvalidEntryName},
		{"a/./b", ErrInvalidEntryName},
		{"a/../b", ErrInvalidEntryName},
		{"foo:bar", ErrInvalidEntryName},
		{"CON.txt", ErrInvalidEntryName},
		{"CLOCK$.txt", ErrInvalidEntryName},
		{"CONIN$.txt", ErrInvalidEntryName},
		{"COM¹.txt", ErrInvalidEntryName},
		{"bad<name", ErrInvalidEntryName},
		{"bad\x1fname", ErrInvalidEntryName},
		{"file. ", ErrInvalidEntryName},
	}
	for _, test := range unsafe {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestZip(t, testZipEntry{name: test.name, data: "x"})
			_, err := VerifyZip(path)
			if !errors.Is(err, test.want) {
				t.Fatalf("VerifyZip(%q) error = %v, want %v", test.name, err, test.want)
			}
		})
	}
}

func TestVerifyZipRejectsDuplicatesCollisionsAndSpecialFiles(t *testing.T) {
	tests := []struct {
		name    string
		entries []testZipEntry
	}{
		{"duplicate", []testZipEntry{{name: "a", data: "1"}, {name: "a", data: "2"}}},
		{"case-fold", []testZipEntry{{name: "A", data: "1"}, {name: "a", data: "2"}}},
		{"unicode-normalization", []testZipEntry{{name: "caf\u00e9", data: "1"}, {name: "cafe\u0301", data: "2"}}},
		{"file-directory-alias", []testZipEntry{{name: "a", data: "1"}, {name: "a/", mode: fs.ModeDir}}},
		{"file-parent", []testZipEntry{{name: "a", data: "1"}, {name: "a/b", data: "2"}}},
		{"reserved-complete", []testZipEntry{{name: CompleteMarkerName, data: "x"}}},
		{"reserved-manifest", []testZipEntry{{name: CacheManifestName, data: "x"}}},
		{"symlink", []testZipEntry{{name: "link", mode: fs.ModeSymlink | 0o777, data: "target"}}},
		{"device", []testZipEntry{{name: "device", mode: fs.ModeDevice | 0o600}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestZip(t, test.entries...)
			_, err := VerifyZip(path)
			if err == nil {
				t.Fatal("VerifyZip succeeded for unsafe archive")
			}
		})
	}
}

func TestParseManifestIsStrict(t *testing.T) {
	digest := testDigest("x")
	valid := `{"schema":"runtimebundle/v1","entries":[{"name":"x","mode":420,"size":1,"sha256":"` + digest + `"}]}`
	if _, err := ParseManifest([]byte(valid)); err != nil {
		t.Fatalf("ParseManifest(valid): %v", err)
	}
	for _, data := range []string{
		valid + " {}",
		`{"entries":[],"entries":[]}`,
		`{"entries":[],"extra":1}`,
		`null`,
	} {
		if _, err := ParseManifest([]byte(data)); err == nil {
			t.Fatalf("ParseManifest accepted invalid JSON %q", data)
		}
	}
	emptyDigest := testDigest("")
	dirMode := strconv.FormatUint(uint64(fs.ModeDir), 10)
	badDirectory := `{"entries":[{"name":"dir/","mode":` + dirMode + `,"size":0,"sha256":"` + digest + `"}]}`
	if _, err := ParseManifest([]byte(badDirectory)); err == nil {
		t.Fatal("ParseManifest accepted a directory with non-empty digest")
	}
	goodDirectory := `{"entries":[{"name":"dir/","mode":` + dirMode + `,"size":0,"sha256":"` + emptyDigest + `"}]}`
	if _, err := ParseManifest([]byte(goodDirectory)); err != nil {
		t.Fatalf("ParseManifest rejected valid directory: %v", err)
	}
}

func TestVerifyZipRejectsNilReaderAndArchiveByteLimit(t *testing.T) {
	var reader *bytes.Reader
	if _, err := VerifyZipReader(reader, 0); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("nil ReaderAt error = %v, want ErrUnsafeArchive", err)
	}
	path := writeTestZip(t, testZipEntry{name: "x", data: "payload"})
	if _, err := VerifyZip(path, VerifyOptions{Limits: Limits{MaxArchiveBytes: 1}}); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("archive byte limit error = %v, want ErrArchiveLimit", err)
	}
}

func TestExtractZipReaderUsesOpenedArchiveAfterPathReplacement(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	file, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(zipPath, zipPath+".opened"); err != nil {
		t.Skipf("cannot replace an open ZIP on this platform: %v", err)
	}
	if err := os.WriteFile(zipPath, []byte("replacement is not a ZIP"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractZipReader(file, info.Size(), dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "runtime")); err != nil || string(got) != "trusted" {
		t.Fatalf("extracted runtime = %q, err=%v; want trusted", got, err)
	}
}

func TestVerifyZipExpectedNamespaceEmptyDigestIsStrict(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "x", data: "payload"})
	bundle, err := VerifyZip(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyZip(path, VerifyOptions{Expected: &bundle}); err != nil {
		t.Fatalf("namespace-empty expected identity rejected: %v", err)
	}
	bundle.Digest = strings.Repeat("0", sha256.Size*2)
	if _, err := VerifyZip(path, VerifyOptions{Expected: &bundle}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("namespace-empty digest error = %v, want ErrDigestMismatch", err)
	}
}

func TestExtractVerifiedDetectsInPlaceSourceMutation(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	archive, closer, err := openAndVerify(zipPath, VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(data, []byte("trusted"))
	if index < 0 {
		t.Fatal("test ZIP does not contain stored payload")
	}
	file, err := os.OpenFile(zipPath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("changed"), int64(index)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := extractVerified(archive, dst); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("in-place source mutation error = %v, want ErrDigestMismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "runtime")); !os.IsNotExist(err) {
		t.Fatalf("mutated output still exists, stat error = %v", err)
	}
}

func TestCacheMaterializeRepairsSameSizeTamper(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	ctx := context.Background()
	dir, err := cache.MaterializeZip(ctx, NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	if filepath.Base(filepath.Dir(dir)) != string(NamespaceEngine) || filepath.Base(dir) != bundle.Digest {
		t.Fatalf("cache path = %s, want namespace/full digest", dir)
	}
	if info, err := os.Stat(filepath.Join(dir, "runtime")); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("runtime permissions = %v, err=%v; want 0700", info.Mode().Perm(), err)
	}
	if marker, err := os.ReadFile(filepath.Join(dir, CompleteMarkerName)); err != nil || strings.TrimSpace(string(marker)) != bundle.Digest {
		t.Fatalf("complete marker = %q, err=%v", marker, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "runtime"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, err := cache.MaterializeZip(ctx, NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("repair materialize: %v", err)
	}
	if repaired != dir {
		t.Fatalf("repaired path = %s, want %s", repaired, dir)
	}
	content, err := os.ReadFile(filepath.Join(dir, "runtime"))
	if err != nil || string(content) != "trusted" {
		t.Fatalf("repaired content = %q, err=%v", content, err)
	}
}

func TestCacheNestedFileCacheHitDoesNotRequireExplicitParentEntry(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "bin/run", mode: 0o755, data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	first, err := cache.MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	second, err := cache.MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("cache hit: %v", err)
	}
	if first != second {
		t.Fatalf("cache hit path = %s, want %s", second, first)
	}
}

func TestCacheRemovePinsNamespaceAndRefusesValidTarget(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	dir, err := cache.MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Remove(NamespaceEngine, filepath.Base(dir)); err == nil {
		t.Fatal("Remove deleted a valid cache target")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("valid target disappeared after Remove: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cache.Remove(NamespaceEngine, filepath.Base(dir)); err != nil {
		t.Fatalf("Remove corrupted target: %v", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("corrupted target remains, stat error = %v", err)
	}
}

func TestCacheConcurrentMaterializePublishesOneCompleteTarget(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "engine", data: "engine bytes"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	const workers = 16
	paths := make([]string, workers)
	errs := make([]error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			paths[i], errs[i] = cache.MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle)
		}(i)
	}
	group.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
		if paths[i] != paths[0] {
			t.Fatalf("worker %d path %s, want %s", i, paths[i], paths[0])
		}
	}
	items, err := cache.List(NamespaceEngine)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != paths[0] {
		t.Fatalf("cache entries = %#v, want one published target", items)
	}
	if err := os.Mkdir(filepath.Join(filepath.Dir(paths[0]), strings.Repeat("f", sha256.Size*2)), 0o700); err != nil {
		t.Fatal(err)
	}
	items, err = cache.List(NamespaceEngine)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != paths[0] {
		t.Fatalf("List included incomplete named directory: %#v", items)
	}
}

func TestCacheRejectsRootAndTargetSymlinks(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "x", data: "x"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}

	rootReal := t.TempDir()
	rootLink := filepath.Join(t.TempDir(), "cache-link")
	if err := os.Symlink(rootReal, rootLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := NewProcessLocalCache(rootLink).MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("root symlink error = %v, want ErrUnsafeArchive", err)
	}

	root := t.TempDir()
	namespace := filepath.Join(root, string(NamespaceEngine))
	if err := os.MkdirAll(namespace, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(namespace, bundle.Digest)
	outside := t.TempDir()
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := NewProcessLocalCache(root).MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("target symlink error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside target was affected: %v", err)
	}

	namespaceRoot := t.TempDir()
	namespaceOutside := t.TempDir()
	if err := os.Symlink(namespaceOutside, filepath.Join(namespaceRoot, string(NamespaceEngine))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	namespaceCache := NewProcessLocalCache(namespaceRoot)
	if _, err := namespaceCache.MaterializeZip(context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("namespace symlink error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := namespaceCache.List(NamespaceEngine); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("List namespace symlink error = %v, want ErrUnsafeArchive", err)
	}
}

func TestVerifyZipRejectsSourceSymlink(t *testing.T) {
	realZip := writeTestZip(t, testZipEntry{name: "x", data: "x"})
	link := filepath.Join(t.TempDir(), "bundle.zip")
	if err := os.Symlink(realZip, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := VerifyZip(link); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("source symlink error = %v, want ErrUnsafeArchive", err)
	}
}

func TestExtractZipRejectsDestinationIntermediateSymlink(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "bin/run", mode: 0o755, data: "run"})
	dst := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dst, "bin")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ExtractZip(zipPath, dst); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("destination intermediate symlink error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "run")); !os.IsNotExist(err) {
		t.Fatalf("symlink target was modified, stat error = %v", err)
	}
}

func TestPinnedRootsRejectPathReplacement(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := openPinnedRoot(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(dst, dst+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkPinnedRootPath(dst, root); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("replaced root check = %v, want ErrUnsafeArchive", err)
	}

	parentPath := filepath.Join(t.TempDir(), "parent")
	childPath := filepath.Join(parentPath, "child")
	if err := os.MkdirAll(childPath, 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := openPinnedRoot(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	child, err := openPinnedChildRoot(parent, "child")
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	if err := os.Rename(childPath, childPath+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(childPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkPinnedChildPath(parent, "child", child); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("replaced child check = %v, want ErrUnsafeArchive", err)
	}
}

func TestVerifyZipEnforcesCompressionRatio(t *testing.T) {
	path := writeTestZip(t, testZipEntry{name: "bomb", data: strings.Repeat("A", 16*1024), method: zip.Deflate})
	_, err := VerifyZip(path, VerifyOptions{Limits: Limits{MaxCompressionRatio: 2}})
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("compression ratio error = %v, want ErrArchiveLimit", err)
	}
}

func TestCrossProcessAndGCBoundariesFailClosed(t *testing.T) {
	if _, err := (UnsupportedCrossProcessLockProvider{}).AcquireExclusive(context.Background(), "cache-key"); !errors.Is(err, ErrCrossProcessLockUnsupported) {
		t.Fatalf("cross-process lock error = %v, want ErrCrossProcessLockUnsupported", err)
	}
	if err := NewProcessLocalCache(t.TempDir()).Collect(context.Background(), GCOptions{}); !errors.Is(err, ErrGCUnsupported) {
		t.Fatalf("GC error = %v, want ErrGCUnsupported", err)
	}
}

func TestNewCacheUsesCrossProcessLockAndNilProviderFailsClosed(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "x", data: "x"})
	materialized, err := NewCache(t.TempDir()).Materialize(context.Background(), NamespaceEngine, zipPath, nil)
	if err != nil {
		t.Fatalf("default cache materialize = %v", err)
	}
	if err := materialized.Close(); err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	cache.LockProvider = nil
	if _, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, nil); !errors.Is(err, ErrCrossProcessLockUnsupported) {
		t.Fatalf("nil provider error = %v, want ErrCrossProcessLockUnsupported", err)
	}
}

func TestRuntimeBundleProcessHelper(t *testing.T) {
	if os.Getenv("RUNTIMEBUNDLE_HELPER") == "" {
		return
	}
	ready := os.Getenv("RUNTIMEBUNDLE_HELPER_READY")
	start := os.Getenv("RUNTIMEBUNDLE_HELPER_START")
	if ready != "" {
		if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if start != "" {
		waitTestFile(t, start, 10*time.Second)
	}
	switch os.Getenv("RUNTIMEBUNDLE_HELPER") {
	case "materialize":
		materialized, err := NewCache(os.Getenv("RUNTIMEBUNDLE_HELPER_ROOT")).Materialize(
			context.Background(), NamespaceEngine, os.Getenv("RUNTIMEBUNDLE_HELPER_ZIP"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("RUNTIMEBUNDLE_HELPER_RESULT"), []byte(materialized.Path), 0600); err != nil {
			_ = materialized.Close()
			t.Fatal(err)
		}
		if err := materialized.Close(); err != nil {
			t.Fatal(err)
		}
	case "lock":
		provider := CrossProcessLockProvider{}
		var lease LockLease
		var err error
		if os.Getenv("RUNTIMEBUNDLE_HELPER_LOCK_MODE") == "shared" {
			lease, err = provider.AcquireShared(context.Background(), os.Getenv("RUNTIMEBUNDLE_HELPER_KEY"))
		} else {
			lease, err = provider.AcquireExclusive(context.Background(), os.Getenv("RUNTIMEBUNDLE_HELPER_KEY"))
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("RUNTIMEBUNDLE_HELPER_ACQUIRED"), []byte("acquired"), 0600); err != nil {
			_ = lease.Close()
			t.Fatal(err)
		}
		waitTestFile(t, os.Getenv("RUNTIMEBUNDLE_HELPER_RELEASE"), 10*time.Minute)
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown helper mode %q", os.Getenv("RUNTIMEBUNDLE_HELPER"))
	}
}

func TestCrossProcessFirstMaterialize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("subprocess cache test requires the POSIX test runner")
	}
	zipPath := writeTestZip(t, testZipEntry{name: "engine", data: "engine bytes"})
	root := t.TempDir()
	start := filepath.Join(root, "start")
	cmds := make([]*exec.Cmd, 2)
	results := make([]string, 2)
	for i := range cmds {
		ready := filepath.Join(root, "ready-"+strconv.Itoa(i))
		results[i] = filepath.Join(root, "result-"+strconv.Itoa(i))
		cmds[i] = runtimeBundleHelperCommand(t, map[string]string{
			"RUNTIMEBUNDLE_HELPER":        "materialize",
			"RUNTIMEBUNDLE_HELPER_ROOT":   root,
			"RUNTIMEBUNDLE_HELPER_ZIP":    zipPath,
			"RUNTIMEBUNDLE_HELPER_READY":  ready,
			"RUNTIMEBUNDLE_HELPER_START":  start,
			"RUNTIMEBUNDLE_HELPER_RESULT": results[i],
		})
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if cmds[i].Process != nil {
				_ = cmds[i].Process.Kill()
			}
		})
	}
	for i := range cmds {
		waitTestFile(t, filepath.Join(root, "ready-"+strconv.Itoa(i)), 10*time.Second)
	}
	if err := os.WriteFile(start, []byte("go"), 0600); err != nil {
		t.Fatal(err)
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("materialize helper %d: %v", i, err)
		}
	}
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, string(NamespaceEngine), bundle.Digest)
	for i, result := range results {
		got, err := os.ReadFile(result)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("helper %d path = %q, want %q", i, got, want)
		}
	}
	if _, err := os.Stat(want + ".lock"); err != nil {
		t.Fatalf("sidecar lock = %v", err)
	}
	items, err := NewCache(root).List(NamespaceEngine)
	if err != nil || len(items) != 1 || items[0] != want {
		t.Fatalf("cache list = %#v, %v", items, err)
	}
}

func TestMaterializeLeaseBlocksRepair(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(t.TempDir())
	first, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first.Path, "runtime"), []byte("tampered"), 0o600); err != nil {
		_ = first.Close()
		t.Fatal(err)
	}
	repaired := make(chan error, 1)
	go func() {
		materialized, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &bundle)
		if err == nil {
			err = materialized.Close()
		}
		repaired <- err
	}()
	select {
	case err := <-repaired:
		_ = first.Close()
		t.Fatalf("repair completed while shared lease was held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-repaired:
		if err != nil {
			t.Fatalf("repair after lease close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("repair did not finish after shared lease close")
	}
	content, err := os.ReadFile(filepath.Join(first.Path, "runtime"))
	if err != nil || string(content) != "trusted" {
		t.Fatalf("repaired content = %q, %v", content, err)
	}
}

func TestMaterializeValidHitSharesUseLease(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(t.TempDir())
	first, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	second, err := cache.Materialize(ctx, NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("valid cache hit waited for an existing shared use lease: %v", err)
	}
	defer second.Close()
	if second.Path != first.Path {
		t.Fatalf("shared cache hit path = %q, want %q", second.Path, first.Path)
	}
}

func TestMaterializeLeaseBlocksRemove(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(t.TempDir())
	materialized, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materialized.Path, "runtime"), []byte("tampered"), 0o600); err != nil {
		_ = materialized.Close()
		t.Fatal(err)
	}
	removed := make(chan error, 1)
	go func() { removed <- cache.Remove(NamespaceEngine, bundle.Digest) }()
	select {
	case err := <-removed:
		_ = materialized.Close()
		t.Fatalf("remove completed while shared lease was held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := materialized.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-removed:
		if err != nil {
			t.Fatalf("remove after lease close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("remove did not finish after shared lease close")
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(materialized.Path), bundle.Digest)); !os.IsNotExist(err) {
		t.Fatalf("removed target stat = %v", err)
	}
}

func TestCrossProcessLockContextCancellation(t *testing.T) {
	provider := CrossProcessLockProvider{}
	key := filepath.Join(t.TempDir(), "namespace", strings.Repeat("a", sha256.Size*2))
	first, err := provider.AcquireExclusive(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := provider.AcquireShared(ctx, key); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled shared lock = %v, want context deadline", err)
	}
}

func TestCrossProcessSharedLocksCanOverlap(t *testing.T) {
	provider := CrossProcessLockProvider{}
	key := filepath.Join(t.TempDir(), "namespace", strings.Repeat("c", sha256.Size*2))
	first, err := provider.AcquireShared(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.AcquireShared(context.Background(), key)
	if err != nil {
		_ = first.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := provider.AcquireExclusive(ctx, key); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("exclusive lock with readers = %v, want context deadline", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	lease, err := provider.AcquireExclusive(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializedCloseIsGoroutineSafe(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	cache := NewCache(t.TempDir())
	materialized, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	const closers = 16
	var group sync.WaitGroup
	errs := make(chan error, closers)
	group.Add(closers)
	for i := 0; i < closers; i++ {
		go func() {
			defer group.Done()
			errs <- materialized.Close()
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	lease, err := cache.LockProvider.AcquireExclusive(context.Background(), materialized.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCrossProcessLockKillReleases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process termination is covered by the cross-compile ACL test")
	}
	root := t.TempDir()
	key := filepath.Join(root, "namespace", strings.Repeat("b", sha256.Size*2))
	acquired := filepath.Join(root, "acquired")
	release := filepath.Join(root, "release")
	cmd := runtimeBundleHelperCommand(t, map[string]string{
		"RUNTIMEBUNDLE_HELPER":           "lock",
		"RUNTIMEBUNDLE_HELPER_KEY":       key,
		"RUNTIMEBUNDLE_HELPER_LOCK_MODE": "shared",
		"RUNTIMEBUNDLE_HELPER_ACQUIRED":  acquired,
		"RUNTIMEBUNDLE_HELPER_RELEASE":   release,
	})
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	waitTestFile(t, acquired, 10*time.Second)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lease, err := (CrossProcessLockProvider{}).AcquireExclusive(ctx, key)
	if err != nil {
		t.Fatalf("exclusive lock after killed holder: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func runtimeBundleHelperCommand(t *testing.T, values map[string]string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRuntimeBundleProcessHelper$", "-test.v")
	env := append([]string(nil), os.Environ()...)
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	return cmd
}

func waitTestFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
