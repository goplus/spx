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
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRejectsDownloadedSizeAndDigestMismatchWithoutPublishing(t *testing.T) {
	const payload = "verified payload"
	tests := []struct {
		name           string
		expectedSize   int64
		expectedSHA256 string
	}{
		{
			name:         "size",
			expectedSize: int64(len(payload) + 1),
		},
		{
			name:           "digest",
			expectedSize:   int64(len(payload)),
			expectedSHA256: testDigest("different payload"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			var fetches atomic.Int32
			_, err := Acquire(context.Background(), root, FetchSpec{
				Name:           "runtime.bin",
				URL:            "https://example.invalid/runtime.bin",
				ExpectedSize:   test.expectedSize,
				ExpectedSHA256: test.expectedSHA256,
				Fetch: func(_ context.Context, _ string, dst io.Writer) error {
					fetches.Add(1)
					_, err := io.WriteString(dst, payload)
					return err
				},
			})
			if err == nil {
				t.Fatal("Acquire accepted an asset with mismatched verification data")
			}
			if fetches.Load() != 1 {
				t.Fatalf("fetch count = %d, want 1", fetches.Load())
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			names := acquireTestEntryNames(entries)
			if len(names) != 0 {
				t.Fatalf("cache contents after rejected download = %v, want empty", names)
			}
		})
	}
}

func TestAcquireRejectsResponseOverMaximumSizeWithoutPublishing(t *testing.T) {
	root := t.TempDir()
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name:    "runtime-manifest.json",
		URL:     "https://example.invalid/runtime-manifest.json",
		MaxSize: 8,
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, "response larger than limit")
			return err
		},
	})
	if !errors.Is(err, errAcquireSizeLimit) {
		t.Fatalf("Acquire over size limit error = %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if names := acquireTestEntryNames(entries); len(names) != 0 {
		t.Fatalf("cache contents after oversized response = %v, want empty", names)
	}
}

func TestAcquireOfflineHitDoesNotFetch(t *testing.T) {
	const (
		name    = "runtime.bin"
		payload = "cached payload"
	)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	path, err := Acquire(context.Background(), root, FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Offline:        true,
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return errors.New("offline cache hit must not fetch")
		},
	})
	if err != nil {
		t.Fatalf("offline cache hit: %v", err)
	}
	if path != filepath.Join(root, name) {
		t.Fatalf("path = %q, want %q", path, filepath.Join(root, name))
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0", fetches.Load())
	}
}

func TestAcquireOfflineMissDoesNotFetch(t *testing.T) {
	root := t.TempDir()
	var fetches atomic.Int32
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name:    "runtime.bin",
		URL:     "https://example.invalid/runtime.bin",
		Offline: true,
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if err == nil {
		t.Fatal("offline cache miss unexpectedly succeeded")
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0", fetches.Load())
	}
	if got := err.Error(); !strings.Contains(got, "offline cache miss") || !strings.Contains(got, "URL:") {
		t.Fatalf("offline cache miss error = %q, want an actionable English error", got)
	}
}

func TestAcquireCreatesNestedPrivateRoot(t *testing.T) {
	const payload = "nested cache payload"
	base := t.TempDir()
	spxRoot := filepath.Join(base, "spx")
	bundleRoot := filepath.Join(spxRoot, "runtimebundle")
	root := filepath.Join(bundleRoot, "release-assets")
	path, err := Acquire(context.Background(), root, FetchSpec{
		Name:           "runtime.bin",
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, payload)
			return err
		},
	})
	if err != nil {
		t.Fatalf("Acquire with nested clean cache: %v", err)
	}
	if path != filepath.Join(root, "runtime.bin") {
		t.Fatalf("path = %q, want %q", path, filepath.Join(root, "runtime.bin"))
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != payload {
		t.Fatalf("nested cache asset = %q, err=%v; want %q", data, err, payload)
	}
	if runtime.GOOS != "windows" {
		for _, current := range []string{spxRoot, bundleRoot, root} {
			info, err := os.Stat(current)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o700 {
				t.Fatalf("nested cache directory %s mode = %o, want 700", current, info.Mode().Perm())
			}
		}
	}
}

func TestAcquireOfflineInvalidCacheDoesNotFetch(t *testing.T) {
	const name = "runtime.bin"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len("trusted")),
		ExpectedSHA256: testDigest("trusted"),
		Offline:        true,
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if err == nil {
		t.Fatal("offline invalid cache unexpectedly succeeded")
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0", fetches.Load())
	}
	if _, statErr := os.Stat(filepath.Join(root, name)); statErr != nil {
		t.Fatalf("invalid cache disappeared, stat error = %v", statErr)
	}
}

func TestAcquireRetriesAfterFetcherFailure(t *testing.T) {
	const (
		name    = "runtime.bin"
		payload = "retry payload"
	)
	root := t.TempDir()
	var fetches atomic.Int32
	fetch := func(_ context.Context, _ string, dst io.Writer) error {
		if fetches.Add(1) == 1 {
			return errors.New("temporary fetch failure")
		}
		_, err := io.WriteString(dst, payload)
		return err
	}
	spec := FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Fetch:          fetch,
	}
	if _, err := Acquire(context.Background(), root, spec); err == nil {
		t.Fatal("first fetch unexpectedly succeeded")
	}
	if entries, err := os.ReadDir(root); err != nil {
		t.Fatal(err)
	} else if names := acquireTestEntryNames(entries); len(names) != 0 {
		t.Fatalf("cache contents after failed fetch = %v, want empty", names)
	}
	path, err := Acquire(context.Background(), root, spec)
	if err != nil {
		t.Fatalf("retry fetch: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != payload {
		t.Fatalf("retried asset = %q, err=%v; want %q", got, err, payload)
	}
	if fetches.Load() != 2 {
		t.Fatalf("fetch count = %d, want 2", fetches.Load())
	}
}

func TestAcquireRejectsCachedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.bin")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "runtime.bin")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var fetches atomic.Int32
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name: "runtime.bin",
		URL:  "https://example.invalid/runtime.bin",
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if err == nil {
		t.Fatal("Acquire accepted a cached symlink")
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0", fetches.Load())
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "outside" {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestAcquireRejectsLockSymlinkWithoutTouchingTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.WriteFile(outside, []byte("outside lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "runtime.bin.lock")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var fetches atomic.Int32
	_, err = Acquire(context.Background(), root, FetchSpec{
		Name: "runtime.bin",
		URL:  "https://example.invalid/runtime.bin",
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("Acquire lock symlink error = %v, want ErrUnsafeArchive", err)
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0", fetches.Load())
	}
	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "outside lock" || after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("outside lock changed: contents=%q mode=%o, want contents=%q mode=%o", data, after.Mode().Perm(), "outside lock", before.Mode().Perm())
	}
}

func TestAcquireFailsClosedWhenRootIsReplacedDuringFetch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an open directory is not supported on Windows")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "cache")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	originalRoot := filepath.Join(parent, "cache-original")
	replacement := t.TempDir()
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name: "runtime.bin",
		URL:  "https://example.invalid/runtime.bin",
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			if err := os.Rename(root, originalRoot); err != nil {
				return err
			}
			if err := os.Symlink(replacement, root); err != nil {
				return err
			}
			_, err := io.WriteString(dst, "trusted payload")
			return err
		},
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("Acquire replaced root error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Lstat(filepath.Join(replacement, "runtime.bin")); !os.IsNotExist(err) {
		t.Fatalf("replacement root asset stat = %v, want not exist", err)
	}
	entries, err := os.ReadDir(originalRoot)
	if err != nil {
		t.Fatal(err)
	}
	if names := acquireTestEntryNames(entries); len(names) != 0 {
		t.Fatalf("original root contents after replacement = %v, want no acquired assets", names)
	}
}

func TestAcquireFailsClosedWhenRootParentIsReplacedDuringFetch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renaming an open directory is not supported on Windows")
	}
	top := t.TempDir()
	parent := filepath.Join(top, "cache-parent")
	root := filepath.Join(parent, "assets")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	originalParent := filepath.Join(top, "cache-parent-original")
	_, err := Acquire(context.Background(), root, FetchSpec{
		Name: "runtime.bin",
		URL:  "https://example.invalid/runtime.bin",
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			if err := os.Rename(parent, originalParent); err != nil {
				return err
			}
			if err := os.MkdirAll(root, 0o700); err != nil {
				return err
			}
			_, err := io.WriteString(dst, "trusted payload")
			return err
		},
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("Acquire replaced parent error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "runtime.bin")); !os.IsNotExist(err) {
		t.Fatalf("replacement parent asset stat = %v, want not exist", err)
	}
	entries, err := os.ReadDir(filepath.Join(originalParent, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if names := acquireTestEntryNames(entries); len(names) != 0 {
		t.Fatalf("original root contents after parent replacement = %v, want no acquired assets", names)
	}
}

func TestAcquireConcurrentFetchesPublishCompleteAsset(t *testing.T) {
	const (
		workers = 16
		name    = "runtime.bin"
		payload = "complete concurrent payload"
	)
	root := t.TempDir()
	var fetches atomic.Int32
	fetch := func(_ context.Context, _ string, dst io.Writer) error {
		fetches.Add(1)
		time.Sleep(time.Millisecond)
		_, err := io.WriteString(dst, payload)
		return err
	}
	spec := FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Fetch:          fetch,
	}
	paths := make([]string, workers)
	errs := make([]error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			paths[i], errs[i] = Acquire(context.Background(), root, spec)
		}(i)
	}
	group.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
		if paths[i] != filepath.Join(root, name) {
			t.Fatalf("worker %d path = %q, want %q", i, paths[i], filepath.Join(root, name))
		}
	}
	if fetches.Load() != 1 {
		t.Fatalf("concurrent acquisition fetch count = %d, want 1", fetches.Load())
	}
	got, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("published concurrent asset = %q, want %q", got, payload)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := acquireTestEntryNames(entries)
	if len(names) != 1 || names[0] != name {
		t.Fatalf("cache contents after concurrent acquisition = %v, want only %q", names, name)
	}
}

func TestAcquireFileHoldsSharedLeaseUntilClose(t *testing.T) {
	const (
		name     = "runtime.bin"
		original = "verified generation one"
		next     = "verified generation two"
	)
	root := t.TempDir()
	acquired, err := AcquireFile(context.Background(), root, FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(original)),
		ExpectedSHA256: testDigest(original),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, original)
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(acquired)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("acquired file = %q, want %q", data, original)
	}

	started := make(chan struct{})
	fetched := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, err := Acquire(context.Background(), root, FetchSpec{
			Name:           name,
			URL:            "https://example.invalid/runtime-v2.bin",
			ExpectedSize:   int64(len(next)),
			ExpectedSHA256: testDigest(next),
			Force:          true,
			Fetch: func(_ context.Context, _ string, dst io.Writer) error {
				close(fetched)
				_, err := io.WriteString(dst, next)
				return err
			},
		})
		done <- err
	}()
	<-started
	select {
	case <-fetched:
		t.Fatal("exclusive replacement started while AcquireFile held its shared lease")
	case <-time.After(100 * time.Millisecond):
	}
	if err := acquired.Close(); err != nil {
		t.Fatal(err)
	}
	if err := acquired.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exclusive replacement remained blocked after AcquireFile.Close")
	}
	if got, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(got) != next {
		t.Fatalf("replacement = %q, err=%v; want %q", got, err, next)
	}
}

func TestAcquireFileReadsPinnedInodeAfterRootReplacement(t *testing.T) {
	const (
		name    = "runtime.bin"
		payload = "verified pinned payload"
	)
	parent := t.TempDir()
	root := filepath.Join(parent, "cache")
	acquired, err := AcquireFile(context.Background(), root, FetchSpec{
		Name:           name,
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, payload)
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acquired.Close()

	moved := filepath.Join(parent, "moved-cache")
	if err := os.Rename(root, moved); err != nil {
		t.Skipf("cannot replace an open cache root on this platform: %v", err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte("replacement payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(acquired)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != payload {
		t.Fatalf("acquired descriptor followed replacement root: got %q, want %q", data, payload)
	}
}

func TestAcquireFileAllowsConcurrentReadAndClose(t *testing.T) {
	payload := strings.Repeat("verified payload", 1024)
	acquired, err := AcquireFile(context.Background(), t.TempDir(), FetchSpec{
		Name:           "runtime.bin",
		URL:            "https://example.invalid/runtime.bin",
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: testDigest(payload),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, payload)
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			buffer := make([]byte, 256)
			for offset := int64(0); offset < int64(len(payload)); offset += int64(len(buffer)) {
				if _, err := acquired.ReadAt(buffer, offset); err != nil {
					return
				}
			}
		}()
	}
	close(start)
	if err := acquired.Close(); err != nil {
		t.Fatal(err)
	}
	readers.Wait()
	if err := acquired.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func acquireTestEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}
		names = append(names, entry.Name())
	}
	return names
}
