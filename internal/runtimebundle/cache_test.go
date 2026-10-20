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
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
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

func TestCacheMaterializeRejectsInvalidRequestBeforeWriting(t *testing.T) {
	if result, err := (*Cache)(nil).Materialize(nil, NamespaceEngine, "missing.zip", nil); result != nil || err == nil || !strings.Contains(err.Error(), "nil cache") {
		t.Fatalf("nil Materialize = %v, %v", result, err)
	}
	t.Run("missing source", func(t *testing.T) {
		cache := NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
		if result, err := cache.Materialize(nil, NamespaceEngine, filepath.Join(t.TempDir(), "missing.zip"), nil); result != nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing source Materialize = %v, %v", result, err)
		}
		if _, err := os.Lstat(cache.Root); !os.IsNotExist(err) {
			t.Fatalf("missing source created cache storage: %v", err)
		}
	})
	zipPath := writeTestZip(t, testZipEntry{name: "x", data: "payload"})
	invalid := Bundle{Entries: []Entry{{Name: "x", Size: -1}}, Digest: strings.Repeat("0", sha256.Size*2)}
	for _, tt := range []struct {
		name     string
		limits   Limits
		expected Bundle
		message  string
	}{
		{"negative limits before expected validation", Limits{MaxEntries: -1}, invalid, "negative archive limit"},
		{"invalid expected before digest validation", Limits{}, invalid, `"x" has negative size`},
		{"invalid namespaced expected", Limits{}, Bundle{Namespace: NamespaceEngine, Entries: invalid.Entries}, `"x" has negative size`},
		{"expected manifest too large", Limits{MaxManifestBytes: 1}, Bundle{Entries: []Entry{{Name: "x", Size: 7, SHA256: testDigest("payload")}}}, "manifest size"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
			cache.Limits = tt.limits
			materialized, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &tt.expected)
			if materialized != nil {
				_ = materialized.Close()
				t.Fatal("Materialize returned a lease for an invalid request")
			}
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Materialize error = %v, want %q", err, tt.message)
			}
			if _, err := os.Lstat(cache.Root); !os.IsNotExist(err) {
				t.Fatalf("invalid request created cache storage: %v", err)
			}
		})
	}
}

func TestCacheMaterializeRejectsInvalidIdentityBeforeWriting(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name        string
		namespace   Namespace
		wrongDigest bool
		message     string
	}{
		{
			name:      "namespace mismatch",
			namespace: NamespaceBridge,
			message:   `expected namespace "bridge" does not match "engine"`,
		},
		{
			name:        "bare wrong digest",
			wrongDigest: true,
			message:     "namespace-empty expected digest",
		},
		{
			name:        "namespaced wrong digest",
			namespace:   NamespaceEngine,
			wrongDigest: true,
			message:     "expected manifest digest",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expected := bundle
			expected.Namespace = tt.namespace
			expected, err := expected.WithDigest()
			if err != nil {
				t.Fatal(err)
			}
			if tt.wrongDigest {
				expected.Digest = strings.Repeat("0", sha256.Size*2)
			}
			cache := NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
			materialized, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, &expected)
			if materialized != nil {
				_ = materialized.Close()
				t.Fatal("Materialize returned a lease for an invalid identity")
			}
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Materialize error = %v, want %q", err, tt.message)
			}
			if errors.Is(err, ErrDigestMismatch) != tt.wrongDigest {
				t.Errorf("Materialize error = %v, want digest mismatch = %v", err, tt.wrongDigest)
			}
			if _, err := os.Lstat(cache.Root); !os.IsNotExist(err) {
				t.Errorf("invalid identity created cache storage: %v", err)
			}
		})
	}
}

func TestCacheRejectsAndRepairsDamagedEntries(t *testing.T) {
	zipPath := writeTestZip(t,
		testZipEntry{name: "bin/", mode: os.ModeDir | 0o755},
		testZipEntry{name: "bin/runtime", mode: 0o755, data: "trusted"},
		testZipEntry{name: "empty/", mode: os.ModeDir | 0o755},
	)
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	bundle, err = bundle.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	wrongNamespace := bundle
	wrongNamespace.Namespace = NamespaceBridge
	wrongNamespace, err = wrongNamespace.WithDigest()
	if err != nil {
		t.Fatal(err)
	}
	wrongManifest, err := json.Marshal(wrongNamespace)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		path     string
		data     string
		mode     os.FileMode
		remove   bool
		symlink  bool
		unixOnly bool
	}{
		{name: "same-size content", path: "bin/runtime", data: "altered"},
		{name: "truncated content", path: "bin/runtime", data: "x"},
		{name: "missing file", path: "bin/runtime", remove: true},
		{name: "missing empty directory", path: "empty", remove: true},
		{name: "unexpected file", path: "extra", data: "extra"},
		{name: "unexpected directory", path: "extra", mode: os.ModeDir | 0o700},
		{name: "invalid manifest", path: cacheManifestName, data: "{"},
		{name: "manifest directory", path: cacheManifestName, mode: os.ModeDir | 0o700},
		{name: "marker directory", path: completeMarkerName, mode: os.ModeDir | 0o700},
		{name: "wrong namespace", path: cacheManifestName, data: string(wrongManifest)},
		{name: "oversized manifest", path: cacheManifestName, data: strings.Repeat("x", 1025)},
		{name: "file symlink", path: "bin/runtime", symlink: true},
		{name: "directory symlink", path: "empty", symlink: true},
		{name: "manifest symlink", path: cacheManifestName, symlink: true},
		{name: "marker symlink", path: completeMarkerName, symlink: true},
		{name: "missing marker", path: completeMarkerName, remove: true},
		{name: "wrong marker", path: completeMarkerName, data: strings.Repeat("0", 64)},
		{name: "oversized marker", path: completeMarkerName, data: strings.Repeat("x", 66)},
		{name: "file permissions", path: "bin/runtime", mode: 0o600, unixOnly: true},
		{name: "directory permissions", path: "bin", mode: 0o755, unixOnly: true},
		{name: "manifest permissions", path: cacheManifestName, mode: 0o644, unixOnly: true},
		{name: "target permissions", path: ".", mode: 0o755, unixOnly: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unixOnly && !runtimeIsUnix() {
				t.Skip("POSIX permissions")
			}
			cache := NewProcessLocalCache(t.TempDir())
			cache.Limits.MaxManifestBytes = 1024
			dir, err := materializeTestPath(cache, nil, NamespaceEngine, zipPath, &bundle)
			if err != nil {
				t.Fatal(err)
			}
			if dir != filepath.Join(cache.Root, string(NamespaceEngine), bundle.Digest) {
				t.Fatalf("materialized path = %s, want namespace/full digest", dir)
			}
			path := filepath.Join(dir, tt.path)
			var outside string
			switch {
			case tt.symlink:
				outside = t.TempDir()
				if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("untouched"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(outside, "sentinel")
				if tt.path == "empty" {
					target = outside
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case tt.remove:
				err = os.Remove(path)
			case tt.mode.IsDir():
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				err = os.Mkdir(path, tt.mode.Perm())
			case tt.mode != 0:
				err = os.Chmod(path, tt.mode)
			default:
				err = os.WriteFile(path, []byte(tt.data), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if items, err := cache.List(NamespaceEngine); err != nil || len(items) != 0 {
				t.Fatalf("List damaged cache = %v, %v; want empty", items, err)
			}
			repaired, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, &bundle)
			if err != nil || repaired != dir {
				t.Fatalf("repair = %q, %v; want %q", repaired, err, dir)
			}
			if data, err := os.ReadFile(filepath.Join(dir, "bin/runtime")); err != nil || string(data) != "trusted" {
				t.Fatalf("repaired content = %q, %v", data, err)
			}
			if marker, err := os.ReadFile(filepath.Join(dir, completeMarkerName)); err != nil || string(marker) != bundle.Digest+"\n" {
				t.Fatalf("repaired marker = %q, %v", marker, err)
			}
			if runtimeIsUnix() {
				info, err := os.Stat(filepath.Join(dir, "bin/runtime"))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o700 {
					t.Fatalf("repaired executable mode = %o, want 700", info.Mode().Perm())
				}
			}
			if items, err := cache.List(NamespaceEngine); err != nil || len(items) != 1 || items[0] != dir {
				t.Fatalf("List repaired cache = %v, %v; want [%s]", items, err, dir)
			}
			if outside != "" {
				if data, err := os.ReadFile(filepath.Join(outside, "sentinel")); err != nil || string(data) != "untouched" {
					t.Errorf("repair changed symlink target: %q, %v", data, err)
				}
				if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
					t.Errorf("repair wrote outside cache: %v, %v", entries, err)
				}
			}
		})
	}
}

func TestCacheMaterializeMetadataLimitDoesNotPublishPartialTarget(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Namespace = NamespaceEngine
	canonical, err := bundle.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	cache := NewProcessLocalCache(t.TempDir())
	// Identity bytes fit, but the published manifest also includes its digest.
	cache.Limits.MaxManifestBytes = int64(len(canonical))
	if result, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, nil); result != nil || !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), "cache manifest size") {
		t.Fatalf("Materialize = %v, %v; want serialized metadata limit", result, err)
	}
	if entries, err := os.ReadDir(filepath.Join(cache.Root, string(NamespaceEngine))); err != nil || len(entries) != 0 {
		t.Fatalf("failed metadata publication left entries: %v, %v", entries, err)
	}
	cache.Limits.MaxManifestBytes = 0
	target, err := materializeTestPath(cache, nil, NamespaceEngine, zipPath, nil)
	if err != nil {
		t.Fatalf("retry after metadata limit = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "runtime")); err != nil || string(data) != "trusted" {
		t.Fatalf("retried cache content = %q, %v", data, err)
	}
	if entries, err := cache.List(NamespaceEngine); err != nil || len(entries) != 1 || entries[0] != target {
		t.Fatalf("retried cache List = %v, %v", entries, err)
	}
}

func TestCacheNestedFileCacheHitDoesNotRequireExplicitParentEntry(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "bin/run", mode: 0o755, data: "trusted"})
	bundle, err := VerifyZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := &Cache{Root: t.TempDir(), LockProvider: ProcessLockProvider{}}
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
	for _, change := range []string{"removed", "directory", "file", "closed"} {
		t.Run(change, func(t *testing.T) {
			parentPath := t.TempDir()
			childPath := filepath.Join(parentPath, "child")
			if err := os.Mkdir(childPath, 0o700); err != nil {
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
			if change == "closed" {
				err = child.Close()
			} else {
				err = os.Rename(childPath, childPath+".old")
				if err == nil {
					switch change {
					case "directory":
						err = os.Mkdir(childPath, 0o700)
					case "file":
						err = os.WriteFile(childPath, []byte("untouched"), 0o600)
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := checkPinnedChildPath(parent, "child", child); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("changed child check = %v, want ErrUnsafeArchive", err)
			}
		})
	}
}

func TestGarbageCollectionFailsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewProcessLocalCache(t.TempDir()).Collect(ctx, GCOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled GC error = %v, want context.Canceled", err)
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

func TestNewCacheWithOptions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	for _, tt := range []struct {
		name    string
		root    string
		options CacheOptions
		wantErr string
	}{
		{"empty root", "", CacheOptions{}, "cache root is empty"},
		{"invalid limits", root, CacheOptions{Limits: Limits{MaxEntries: -1}}, "negative archive limit"},
		{"defaults", root + string(filepath.Separator), CacheOptions{}, ""},
		{"custom", root, CacheOptions{Limits: Limits{MaxEntries: 2}, LockProvider: ProcessLockProvider{}, Permissions: modePermissions{}}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewCacheWithOptions(tt.root, tt.options)
			if tt.wantErr != "" {
				if cache != nil || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NewCacheWithOptions = %v, %v; want %q", cache, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantLimits, err := tt.options.Limits.withDefaults()
			if err != nil {
				t.Fatal(err)
			}
			if cache.Root != root || cache.Limits != wantLimits || cache.Permissions == nil || cache.LockProvider == nil {
				t.Fatalf("cache options not applied: %+v", cache)
			}
			if tt.options.LockProvider != nil {
				if cache.LockProvider != tt.options.LockProvider || cache.Permissions != tt.options.Permissions {
					t.Fatal("custom adapters were replaced")
				}
			} else if _, ok := cache.LockProvider.(CrossProcessLockProvider); !ok {
				t.Fatalf("default lock provider = %T", cache.LockProvider)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("constructor created cache storage: %v", err)
			}
		})
	}
}

func TestCachePathRejectsInvalidIdentity(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name      string
		cache     *Cache
		namespace Namespace
		digest    string
	}{
		{"nil cache", nil, NamespaceEngine, testDigest("")},
		{"empty root", &Cache{}, NamespaceEngine, testDigest("")},
		{"relative root", &Cache{Root: "relative"}, NamespaceEngine, testDigest("")},
		{"unclean root", &Cache{Root: root + string(filepath.Separator)}, NamespaceEngine, testDigest("")},
		{"filesystem root", &Cache{Root: filepath.VolumeName(root) + string(filepath.Separator)}, NamespaceEngine, testDigest("")},
		{"file root", &Cache{Root: file}, NamespaceEngine, testDigest("")},
		{"invalid namespace", &Cache{Root: root}, "../escape", testDigest("")},
		{"short digest", &Cache{Root: root}, NamespaceEngine, "123"},
		{"non-hex digest", &Cache{Root: root}, NamespaceEngine, strings.Repeat("g", 64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if path, err := tt.cache.Path(tt.namespace, tt.digest); path != "" || err == nil {
				t.Fatalf("Path = %q, %v; want rejection", path, err)
			}
		})
	}
}

func TestCacheAdminMissingAndInvalidState(t *testing.T) {
	digest := testDigest("")
	for _, tt := range []struct {
		name      string
		cache     *Cache
		namespace Namespace
		wantErr   string
	}{
		{"nil cache", nil, NamespaceEngine, "nil cache"},
		{"nil provider", &Cache{Root: t.TempDir()}, NamespaceEngine, ErrCrossProcessLockUnsupported.Error()},
		{"invalid root", NewProcessLocalCache("relative"), NamespaceEngine, "absolute and clean"},
		{"invalid namespace", NewProcessLocalCache(t.TempDir()), "bad", "invalid cache namespace"},
		{"missing root", NewProcessLocalCache(filepath.Join(t.TempDir(), "missing")), NamespaceEngine, ""},
		{"missing namespace", NewProcessLocalCache(t.TempDir()), NamespaceEngine, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cache != nil {
				tt.cache.Permissions = nil
			}
			items, listErr := tt.cache.List(tt.namespace)
			removeErr := tt.cache.Remove(tt.namespace, digest)
			for operation, err := range map[string]error{"List": listErr, "Remove": removeErr} {
				if tt.wantErr == "" {
					if err != nil {
						t.Fatalf("%s = %v", operation, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("%s = %v; want %q", operation, err, tt.wantErr)
				}
			}
			if len(items) != 0 {
				t.Fatalf("List = %v, want empty", items)
			}
		})
	}
}

func TestDefaultCacheRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux user cache directory environment")
	}
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	if got, want := DefaultCacheRoot(), filepath.Join(cacheHome, "spx", "runtimebundle"); got != want {
		t.Fatalf("DefaultCacheRoot = %q, want %q", got, want)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if got, want := DefaultCacheRoot(), filepath.Join(os.TempDir(), "spx", "runtimebundle"); got != want {
		t.Fatalf("fallback DefaultCacheRoot = %q, want %q", got, want)
	}
}

func TestModePermissions(t *testing.T) {
	if !runtimeIsUnix() {
		t.Skip("POSIX permission adapter")
	}
	permissions := modePermissions{}
	dir := filepath.Join(t.TempDir(), "nested", "cache")
	if err := permissions.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private directory = %v, %v", info, err)
	}
	file := filepath.Join(dir, "runtime")
	if err := os.WriteFile(file, []byte("runtime"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, executable := range []bool{true, false} {
		if err := permissions.EnsureFile(file, executable); err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0o600)
		if executable {
			want = 0o700
		}
		info, err := os.Stat(file)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("EnsureFile executable=%v = %v, %v; want mode %o", executable, info, err, want)
		}
		if got := PrivateFileMode(uint32(want)); got != want {
			t.Fatalf("PrivateFileMode(%o) = %o", want, got)
		}
	}
	if err := permissions.EnsureDir(filepath.Join(file, "child")); err == nil {
		t.Fatal("EnsureDir accepted a file parent")
	}
	if err := permissions.EnsureFile(filepath.Join(dir, "missing"), false); !os.IsNotExist(err) {
		t.Fatalf("EnsureFile missing = %v", err)
	}
}

// faultLockProvider keeps real process-local leases except at one injected
// boundary, so cache error paths can be checked without timing races.
type faultLockProvider struct {
	LockProvider
	mode    lockMode
	acquire func(context.Context, string) (LockLease, error)
}

func (p faultLockProvider) AcquireShared(ctx context.Context, key string) (LockLease, error) {
	if p.mode == lockShared {
		return p.acquire(ctx, key)
	}
	return p.LockProvider.AcquireShared(ctx, key)
}

func (p faultLockProvider) AcquireExclusive(ctx context.Context, key string) (LockLease, error) {
	if p.mode == lockExclusive {
		return p.acquire(ctx, key)
	}
	return p.LockProvider.AcquireExclusive(ctx, key)
}

type errorTestLease struct {
	err    error
	closes int
}

func (l *errorTestLease) Close() error {
	l.closes++
	return l.err
}

func TestCacheLockFailuresDoNotRemoveData(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	lockErr := errors.New("lock unavailable")
	for _, operation := range []string{"List", "Remove", "Materialize shared", "Materialize exclusive"} {
		for _, failure := range []string{"acquire error", "nil lease"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				cache := NewProcessLocalCache(t.TempDir())
				dir, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil)
				if err != nil {
					t.Fatal(err)
				}
				mode := lockShared
				if operation == "Remove" || operation == "Materialize exclusive" {
					mode = lockExclusive
				}
				// Force the materialization miss to reach its exclusive lease.
				if operation == "Materialize exclusive" {
					if err := os.Remove(filepath.Join(dir, completeMarkerName)); err != nil {
						t.Fatal(err)
					}
				}
				cache.LockProvider = faultLockProvider{
					LockProvider: ProcessLockProvider{}, mode: mode,
					acquire: func(context.Context, string) (LockLease, error) {
						if failure == "acquire error" {
							return nil, lockErr
						}
						return nil, nil
					},
				}
				switch operation {
				case "List":
					_, err = cache.List(NamespaceEngine)
				case "Remove":
					err = cache.Remove(NamespaceEngine, filepath.Base(dir))
				default:
					_, err = materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil)
				}
				if failure == "acquire error" {
					if !errors.Is(err, lockErr) {
						t.Fatalf("error = %v, want injected lock error", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "nil ") || !strings.Contains(err.Error(), "lease") {
					t.Fatalf("error = %v, want nil lease rejection", err)
				}
				if data, err := os.ReadFile(filepath.Join(dir, "runtime")); err != nil || string(data) != "trusted" {
					t.Fatalf("lock failure changed cached data: %q, %v", data, err)
				}
			})
		}
	}
	t.Run("List close error", func(t *testing.T) {
		cache := NewProcessLocalCache(t.TempDir())
		if _, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil); err != nil {
			t.Fatal(err)
		}
		lease := &errorTestLease{err: lockErr}
		cache.LockProvider = faultLockProvider{mode: lockShared,
			acquire: func(context.Context, string) (LockLease, error) { return lease, nil },
		}
		if items, err := cache.List(NamespaceEngine); items != nil || !errors.Is(err, lockErr) || lease.closes != 1 {
			t.Fatalf("List = %v, %v, closes=%d; want close failure once", items, err, lease.closes)
		}
	})
}

func TestCacheMaterializeRechecksSourceAndCancellationAtLeaseBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name string
		want error
		mode lockMode
	}{
		{"removed", os.ErrNotExist, lockExclusive},
		{"invalid ZIP", ErrUnsafeArchive, lockExclusive},
		{"different valid ZIP", ErrDigestMismatch, lockExclusive},
		{"cancelled", context.Canceled, lockExclusive},
		{"cancelled after cache miss", context.Canceled, lockShared},
	} {
		t.Run(tt.name, func(t *testing.T) {
			zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
			original, err := os.ReadFile(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			replacement := writeTestZip(t, testZipEntry{name: "runtime", data: "changed"})
			changed, err := os.ReadFile(replacement)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cache := NewProcessLocalCache(t.TempDir())
			cache.LockProvider = faultLockProvider{LockProvider: ProcessLockProvider{}, mode: tt.mode,
				acquire: func(ctx context.Context, key string) (LockLease, error) {
					lease, err := acquireProcessLock(ctx, key, tt.mode)
					if err != nil {
						return nil, err
					}
					switch tt.name {
					case "removed":
						err = os.Remove(zipPath)
					case "invalid ZIP":
						err = os.WriteFile(zipPath, []byte("invalid ZIP"), 0o600)
					case "different valid ZIP":
						err = os.WriteFile(zipPath, changed, 0o600)
					case "cancelled", "cancelled after cache miss":
						cancel()
					}
					if err != nil {
						_ = lease.Close()
						return nil, err
					}
					return lease, nil
				},
			}
			if result, err := cache.Materialize(ctx, NamespaceEngine, zipPath, nil); result != nil || !errors.Is(err, tt.want) {
				t.Fatalf("Materialize = %v, %v; want %v", result, err, tt.want)
			}
			items, err := os.ReadDir(filepath.Join(cache.Root, string(NamespaceEngine)))
			if err != nil || len(items) != 0 {
				t.Fatalf("failed publication left cache entries: %v, %v", items, err)
			}
			if err := os.WriteFile(zipPath, original, 0o600); err != nil {
				t.Fatal(err)
			}
			cache.LockProvider = ProcessLockProvider{}
			if _, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil); err != nil {
				t.Fatalf("retry after source failure: %v", err)
			}
		})
	}
}

func TestWriteMetadataRootFailsWithoutOverwritingExistingFiles(t *testing.T) {
	bundle := Bundle{Namespace: NamespaceEngine, Entries: []Entry{{Name: "runtime", Size: 7, SHA256: testDigest("trusted")}}}
	canonical, err := bundle.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name          string
		existing      string
		limits        Limits
		invalidBundle bool
		want          string
	}{
		{name: "negative limits", limits: Limits{MaxEntries: -1}, want: "negative archive limit"},
		{name: "invalid bundle", invalidBundle: true, want: "negative size"},
		{name: "serialized metadata limit", limits: Limits{MaxManifestBytes: int64(len(canonical))}, want: "cache manifest size"},
		{name: "existing manifest", existing: cacheManifestName, want: "write cache manifest"},
		{name: "existing marker", existing: completeMarkerName, want: "write cache complete marker"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if tt.existing != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.existing), []byte("untouched"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			input := bundle
			if tt.invalidBundle {
				input.Entries = []Entry{{Name: "runtime", Size: -1}}
			}
			if err := writeMetadataRoot(root, input, tt.limits); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("writeMetadataRoot = %v, want %q", err, tt.want)
			}
			if tt.existing != "" {
				data, err := readRootRegularFile(root, tt.existing, 100)
				if err != nil || string(data) != "untouched" {
					t.Fatalf("existing metadata changed: %q, %v", data, err)
				}
			} else {
				if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
					t.Fatalf("invalid metadata request wrote files: %v, %v", entries, err)
				}
			}
			if _, err := readRootRegularFile(root, cacheManifestName, 0); err == nil || !strings.Contains(err.Error(), "invalid metadata size limit") {
				t.Fatalf("zero metadata read limit = %v", err)
			}
		})
	}
}

func TestCacheMaterializeUsesCompetingPublication(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	cache := NewProcessLocalCache(t.TempDir())
	var winner string
	cache.LockProvider = faultLockProvider{LockProvider: ProcessLockProvider{}, mode: lockExclusive,
		acquire: func(ctx context.Context, key string) (LockLease, error) {
			// Another publisher fills the miss before this caller gets its
			// exclusive lease. Reuse that verified target even without the ZIP.
			var err error
			winner, err = materializeTestPath(NewProcessLocalCache(cache.Root), ctx, NamespaceEngine, zipPath, nil)
			if err != nil {
				return nil, err
			}
			if err := os.Remove(zipPath); err != nil {
				return nil, err
			}
			return (ProcessLockProvider{}).AcquireExclusive(ctx, key)
		},
	}
	path, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil)
	if err != nil || path != winner || winner == "" {
		t.Fatalf("competing publication = %q, %v; want %q", path, err, winner)
	}
	if data, err := os.ReadFile(filepath.Join(path, "runtime")); err != nil || string(data) != "trusted" {
		t.Fatalf("competing publication content = %q, %v", data, err)
	}
	if items, err := cache.List(NamespaceEngine); err != nil || len(items) != 1 || items[0] != winner {
		t.Fatalf("competing publication List = %v, %v", items, err)
	}
}

func TestCacheHandlesChangedPathsAtLeaseBoundaries(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	for _, operation := range []string{"List", "Remove", "Materialize shared", "Materialize exclusive"} {
		for _, change := range []string{"root file", "namespace file", "missing root", "missing namespace"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				cache := NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
				target, err := materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil)
				if err != nil {
					t.Fatal(err)
				}
				if operation == "Materialize exclusive" {
					if err := os.Remove(filepath.Join(target, completeMarkerName)); err != nil {
						t.Fatal(err)
					}
				}
				mode := lockShared
				if operation == "Remove" || operation == "Materialize exclusive" {
					mode = lockExclusive
				}
				cache.LockProvider = faultLockProvider{LockProvider: ProcessLockProvider{}, mode: mode,
					acquire: func(ctx context.Context, key string) (LockLease, error) {
						lease, err := acquireProcessLock(ctx, key, mode)
						if err != nil {
							return nil, err
						}
						path := cache.Root
						if strings.Contains(change, "namespace") {
							path = filepath.Join(cache.Root, string(NamespaceEngine))
						}
						if err := os.Rename(path, path+".old"); err != nil {
							_ = lease.Close()
							return nil, err
						}
						if strings.HasSuffix(change, "file") {
							err = os.WriteFile(path, []byte("untouched"), 0o600)
						}
						if err != nil {
							_ = lease.Close()
							return nil, err
						}
						return lease, nil
					},
				}
				switch operation {
				case "List":
					_, err = cache.List(NamespaceEngine)
				case "Remove":
					err = cache.Remove(NamespaceEngine, filepath.Base(target))
				default:
					_, err = materializeTestPath(cache, context.Background(), NamespaceEngine, zipPath, nil)
				}
				// Remove treats an already absent namespace/root as an idempotent
				// success. Publication can recreate missing storage; file swaps fail.
				if (operation == "Remove" || operation == "Materialize exclusive") && strings.HasPrefix(change, "missing") {
					if err != nil {
						t.Fatalf("%s after missing storage = %v", operation, err)
					}
					if operation == "Materialize exclusive" {
						data, readErr := os.ReadFile(filepath.Join(target, "runtime"))
						if readErr != nil || string(data) != "trusted" {
							t.Fatalf("recreated cache content = %q, %v", data, readErr)
						}
					}
				} else if strings.HasSuffix(change, "file") || operation == "List" {
					if !errors.Is(err, ErrUnsafeArchive) {
						t.Fatalf("changed path = %v, want ErrUnsafeArchive", err)
					}
				} else if err == nil {
					t.Fatal("changing lock parent unexpectedly allowed publication")
				}
			})
		}
	}
}

func TestClosedRootsAndUnsafeComponentsFailClosed(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func() error{
		"sync":      func() error { return syncRootDir(root) },
		"directory": func() error { return ensureRootDir(root, "nested") },
		"child": func() error {
			child, err := openPinnedChildRoot(root, "nested")
			if child != nil {
				_ = child.Close()
			}
			return err
		},
		"metadata": func() error { _, err := readRootRegularFile(root, "metadata", 100); return err },
		"asset": func() error {
			file, err := openVerifiedAcquireRootFile(root, "asset", 1, "")
			if file != nil {
				_ = file.Close()
			}
			return err
		},
		"tree": func() error { return verifyMaterializedTreeRoot(root, Bundle{}) },
	} {
		t.Run(name, func(t *testing.T) {
			if name == "sync" && !runtimeIsUnix() {
				t.Skip("Windows directory sync is a no-op")
			}
			if err := operation(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("closed root = %v, want os.ErrClosed", err)
			}
		})
	}
	root, err = os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(path, "file"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureRootDir(root, "../outside"); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("traversal = %v", err)
	}
	if runtimeIsUnix() {
		if err := chmodRootDir(root, "file"); !errors.Is(err, ErrUnsafeArchive) {
			t.Fatalf("chmod file = %v", err)
		}
		if err := chmodRootDir(root, "missing"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("chmod missing = %v", err)
		}
		if err := chmodRootDir(root, "."); err != nil {
			t.Fatalf("chmod root no-op = %v", err)
		}
	}
	if file, err := openVerifiedAcquireRootFile(root, ".", 0, ""); file != nil || !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("directory asset = %v, %v", file, err)
	}
	if ok, err := (&Cache{Limits: Limits{MaxEntries: -1}}).validCacheHitRoot(root, "engine", "missing", nil); ok || err == nil {
		t.Fatalf("negative cache limits = %v, %v", ok, err)
	}
	if ok, err := NewProcessLocalCache(path).validCacheHitRoot(root, "engine", "file", nil); ok || err != nil {
		t.Fatalf("file cache target = %v, %v", ok, err)
	}
	if err := verifyMaterializedTreeRoot(root, Bundle{Entries: []Entry{{Name: "file", Size: -1}}}); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("invalid tree manifest = %v", err)
	}
}

func TestTemporaryAllocationRetriesWithoutReplacingCollisions(t *testing.T) {
	// These tests are deliberately serial: crypto/rand.Reader is process-wide.
	for _, kind := range []string{"directory", "asset"} {
		for _, exhausted := range []bool{false, true} {
			name := kind + "/retry"
			if exhausted {
				name = kind + "/exhausted"
			}
			t.Run(name, func(t *testing.T) {
				path := t.TempDir()
				root, err := os.OpenRoot(path)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				prefix := ".asset-"
				if kind == "directory" {
					prefix = ".runtimebundle-"
				}
				collision := prefix + strings.Repeat("0", 24)
				if err := os.WriteFile(filepath.Join(path, collision), []byte("untouched"), 0o600); err != nil {
					t.Fatal(err)
				}
				entropy := make([]byte, 32*12)
				if !exhausted {
					copy(entropy[12:], bytes.Repeat([]byte{1}, 12))
				}
				original := cryptorand.Reader
				cryptorand.Reader = bytes.NewReader(entropy)
				defer func() { cryptorand.Reader = original }()
				var allocated string
				if kind == "directory" {
					allocated, err = newRootTemp(root)
				} else {
					var file *os.File
					allocated, file, err = newAcquireRootTemp(root)
					if file != nil {
						defer file.Close()
					}
				}
				if exhausted {
					if err == nil || allocated != "" || !strings.Contains(err.Error(), "unable to allocate") {
						t.Fatalf("exhausted allocation = %q, %v", allocated, err)
					}
				} else if err != nil || allocated != prefix+strings.Repeat("01", 12) {
					t.Fatalf("retry allocation = %q, %v", allocated, err)
				}
				if data, err := os.ReadFile(filepath.Join(path, collision)); err != nil || string(data) != "untouched" {
					t.Fatalf("collision was overwritten: %q, %v", data, err)
				}
				entries, err := os.ReadDir(path)
				wantCount := 2
				if exhausted {
					wantCount = 1
				}
				if err != nil || len(entries) != wantCount {
					t.Fatalf("allocated entries = %v, %v", entries, err)
				}
			})
		}
	}
}

func TestMaterializeRevalidatesPublicationAfterLeaseTransition(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	for _, fault := range []string{"lease error", "nil lease", "missing marker", "target symlink"} {
		t.Run(fault, func(t *testing.T) {
			cache := NewProcessLocalCache(t.TempDir())
			failure := errors.New("shared lease failed")
			calls := 0
			var target string
			cache.LockProvider = faultLockProvider{LockProvider: ProcessLockProvider{}, mode: lockShared,
				acquire: func(ctx context.Context, key string) (LockLease, error) {
					calls++
					if calls == 2 {
						target = key
						switch fault {
						case "lease error":
							return nil, failure
						case "nil lease":
							return nil, nil
						case "missing marker":
							if err := os.Remove(filepath.Join(key, completeMarkerName)); err != nil {
								return nil, err
							}
						case "target symlink":
							if err := os.Rename(key, key+".old"); err != nil {
								return nil, err
							}
							if err := os.Symlink(key+".old", key); err != nil {
								t.Skipf("symlink unavailable: %v", err)
							}
						}
					}
					return (ProcessLockProvider{}).AcquireShared(ctx, key)
				},
			}
			result, err := cache.Materialize(context.Background(), NamespaceEngine, zipPath, nil)
			if result != nil || err == nil || calls != 2 {
				t.Fatalf("unprotected publication = %v, %v; shared calls=%d", result, err, calls)
			}
			switch fault {
			case "lease error":
				if !errors.Is(err, failure) {
					t.Fatalf("lease error lost: %v", err)
				}
			case "nil lease":
				if !strings.Contains(err.Error(), "nil shared lease") {
					t.Fatalf("nil lease error = %v", err)
				}
			case "missing marker":
				if !strings.Contains(err.Error(), "failed shared-lease revalidation") {
					t.Fatalf("corrupt publication = %v", err)
				}
			case "target symlink":
				if !errors.Is(err, ErrUnsafeArchive) {
					t.Fatalf("symlink publication = %v", err)
				}
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
			}
			cache.LockProvider = ProcessLockProvider{}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := materializeTestPath(cache, ctx, NamespaceEngine, zipPath, nil); err != nil {
				t.Fatalf("retry after transition failure = %v", err)
			}
		})
	}
}

func TestCacheRejectsInvalidFilesystemObjects(t *testing.T) {
	rootPath := t.TempDir()
	invalidPath := filepath.Join(rootPath, "invalid\x00")
	for name, operation := range map[string]func() error{
		"validate root":     func() error { return validateCacheRoot(invalidPath) },
		"prepare namespace": func() error { return NewProcessLocalCache(invalidPath).prepareCacheNamespace(NamespaceEngine) },
		"acquire": func() error {
			_, err := AcquireFile(nil, invalidPath, acquireTestSpec("asset", "payload"))
			return err
		},
		"extraction": func() error { return ensureExtractionRoot(invalidPath) },
	} {
		t.Run(name, func(t *testing.T) {
			var pathErr *os.PathError
			if err := operation(); !errors.As(err, &pathErr) {
				t.Fatalf("invalid filesystem path = %v, want original PathError", err)
			}
		})
	}
	cache := NewProcessLocalCache(rootPath)
	namespace := filepath.Join(rootPath, string(NamespaceEngine))
	if err := os.WriteFile(namespace, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache.Permissions = nil
	if err := cache.Remove(NamespaceEngine, testDigest("")); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("file namespace = %v", err)
	}
	if data, err := os.ReadFile(namespace); err != nil || string(data) != "untouched" {
		t.Fatalf("namespace file changed: %q, %v", data, err)
	}
	if err := os.Remove(namespace); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(namespace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(namespace, "not-a-digest"), 0o700); err != nil {
		t.Fatal(err)
	}
	if items, err := cache.List(NamespaceEngine); err != nil || len(items) != 0 {
		t.Fatalf("invalid identity listed: %v, %v", items, err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(namespace, testDigest(""))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := cache.Remove(NamespaceEngine, testDigest("")); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("Remove symlink target = %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("Remove touched external target: %v", err)
	}
}

func TestCacheHitRequiresExpectedManifestEntries(t *testing.T) {
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", data: "trusted"})
	cache := NewProcessLocalCache(t.TempDir())
	target, err := materializeTestPath(cache, nil, NamespaceEngine, zipPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	expected := Bundle{Namespace: NamespaceEngine, Entries: []Entry{{Name: "runtime", Size: 7, SHA256: testDigest("changed")}}}
	if hit, err := cache.validCacheHitRoot(root, string(NamespaceEngine), filepath.Base(target), &expected); hit || err != nil {
		t.Fatalf("mismatched expected entries = %v, %v", hit, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if hit, err := cache.validCacheHitRoot(root, string(NamespaceEngine), filepath.Base(target), nil); hit || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed namespace = %v, %v", hit, err)
	}
	if err := (*Materialized)(nil).Close(); err != nil {
		t.Fatalf("nil materialization Close = %v", err)
	}
}
