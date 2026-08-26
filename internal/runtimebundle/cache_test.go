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
	"context"
	"crypto/sha256"
	"errors"
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
	dir, err := materializeTestPath(cache, ctx, NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	if filepath.Base(filepath.Dir(dir)) != string(NamespaceEngine) || filepath.Base(dir) != bundle.Digest {
		t.Fatalf("cache path = %s, want namespace/full digest", dir)
	}
	if info, err := os.Stat(filepath.Join(dir, "runtime")); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("runtime permissions = %v, err=%v; want 0700", info.Mode().Perm(), err)
	}
	if marker, err := os.ReadFile(filepath.Join(dir, completeMarkerName)); err != nil || strings.TrimSpace(string(marker)) != bundle.Digest {
		t.Fatalf("complete marker = %q, err=%v", marker, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "runtime"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, err := materializeTestPath(cache, ctx, NamespaceEngine, zipPath, &bundle)
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
	first, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, &bundle)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	second, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, &bundle)
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
	dir, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, &bundle)
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
			paths[i], errs[i] = materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, &bundle)
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
	if _, err := materializeTestPath(NewProcessLocalCache(rootLink), context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
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
	if _, err := materializeTestPath(NewProcessLocalCache(root), context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
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
	if _, err := materializeTestPath(namespaceCache, context.Background(), NamespaceEngine, zipPath, &bundle); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("namespace symlink error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := namespaceCache.List(NamespaceEngine); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("List namespace symlink error = %v, want ErrUnsafeArchive", err)
	}
}

func TestPinnedChildRootRejectsPathReplacement(t *testing.T) {
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

func TestCacheLookupReturnsOnlyVerifiedHits(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "bridge", data: "bridge"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceDriver
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(t.TempDir())
	if hit, ok, err := cache.Lookup(context.Background(), NamespaceDriver, &bundle); err != nil || ok || hit != nil {
		t.Fatalf("empty cache lookup = %#v, %t, %v", hit, ok, err)
	}
	materialized, err := cache.Materialize(context.Background(), NamespaceDriver, zipPath, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := materialized.Close(); err != nil {
		t.Fatal(err)
	}
	hit, ok, err := cache.Lookup(context.Background(), NamespaceDriver, &bundle)
	if err != nil || !ok || hit == nil || hit.Path != materialized.Path {
		t.Fatalf("materialized lookup = %#v, %t, %v", hit, ok, err)
	}
	if err := hit.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCacheMaterializeBindsArchiveDigest(t *testing.T) {
	firstZip := writeTestZip(t,
		testZipEntry{name: "engine", mode: 0o700, data: "engine"},
		testZipEntry{name: "bridge", mode: 0o700, data: "bridge"},
	)
	secondZip := writeTestZip(t,
		testZipEntry{name: "bridge", mode: 0o700, data: "bridge"},
		testZipEntry{name: "engine", mode: 0o700, data: "engine"},
	)
	firstData, err := os.ReadFile(firstZip)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(secondZip)
	if err != nil {
		t.Fatal(err)
	}
	first, err := VerifyZip(firstZip)
	if err != nil {
		t.Fatal(err)
	}
	first.Namespace = NamespaceDriver
	first.ArchiveSHA256 = testDigest(string(firstData))
	first, err = first.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	second, err := VerifyZip(secondZip)
	if err != nil {
		t.Fatal(err)
	}
	second.Namespace = NamespaceDriver
	second.ArchiveSHA256 = testDigest(string(secondData))
	second, err = second.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	if first.ArchiveSHA256 == second.ArchiveSHA256 || first.Digest == second.Digest {
		t.Fatalf("archive-bound identities did not diverge: %#v / %#v", first, second)
	}

	cache := NewCache(t.TempDir())
	firstHit, err := cache.Materialize(context.Background(), NamespaceDriver, firstZip, &first)
	if err != nil {
		t.Fatalf("materialize first archive: %v", err)
	}
	defer firstHit.Close()
	secondHit, err := cache.Materialize(context.Background(), NamespaceDriver, secondZip, &second)
	if err != nil {
		t.Fatalf("materialize second archive: %v", err)
	}
	defer secondHit.Close()
	if firstHit.Path == secondHit.Path {
		t.Fatalf("different archive digests reused cache target %q", firstHit.Path)
	}
	storedData, err := os.ReadFile(filepath.Join(firstHit.Path, cacheManifestName))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ParseManifest(storedData)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ArchiveSHA256 != first.ArchiveSHA256 {
		t.Fatalf("stored archive digest = %q, want %q", stored.ArchiveSHA256, first.ArchiveSHA256)
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

func materializeTestPath(cache *Cache, ctx context.Context, namespace Namespace, zipPath string, expected *Bundle) (string, error) {
	materialized, err := cache.Materialize(ctx, namespace, zipPath, expected)
	if err != nil {
		return "", err
	}
	path := materialized.Path
	if err := materialized.Close(); err != nil {
		return "", err
	}
	return path, nil
}
