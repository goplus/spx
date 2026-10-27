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

type acquireTestWriter func([]byte) (int, error)

func TestAcquireRejectsDownloadedSizeAndDigestMismatchWithoutPublishing(t *testing.T) {
	const payload = "verified payload"
	tests := []struct {
		name           string
		expectedSize   int64
		expectedSHA256 string
	}{
		{
			name:           "size",
			expectedSize:   int64(len(payload) + 1),
			expectedSHA256: testDigest(payload),
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
			_, err := acquirePath(context.Background(), root, FetchSpec{
				Name:   "runtime.bin",
				URL:    "https://example.invalid/runtime.bin",
				Size:   test.expectedSize,
				SHA256: test.expectedSHA256,
				Fetch: func(_ context.Context, _ string, dst io.Writer) error {
					fetches.Add(1)
					_, err := io.WriteString(dst, payload)
					return err
				},
			})
			if err == nil {
				t.Fatal("Acquire accepted an asset with mismatched verification data")
			}
			if !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("verification error = %v, want ErrDigestMismatch", err)
			}
			if fetches.Load() != 1 {
				t.Fatalf("fetch count = %d, want 1", fetches.Load())
			}
			names := acquireTestEntryNames(t, root)
			if len(names) != 0 {
				t.Fatalf("cache contents after rejected download = %v, want empty", names)
			}
		})
	}
}

func TestAcquireRejectsResponseOverPinnedSizeWithoutPublishing(t *testing.T) {
	root := t.TempDir()
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime-manifest.json",
		URL:    "https://example.invalid/runtime-manifest.json",
		Size:   8,
		SHA256: testDigest("12345678"),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, _ = io.WriteString(dst, "response larger than limit")
			return nil
		},
	})
	if !errors.Is(err, errAcquireSizeLimit) {
		t.Fatalf("Acquire over size limit error = %v", err)
	}
	if names := acquireTestEntryNames(t, root); len(names) != 0 {
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
	path, err := acquirePath(context.Background(), root, FetchSpec{
		Name:    name,
		URL:     "https://example.invalid/runtime.bin",
		Size:    int64(len(payload)),
		SHA256:  testDigest(payload),
		Offline: true,
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
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:    "runtime.bin",
		URL:     "https://example.invalid/runtime.bin",
		Size:    1,
		SHA256:  testDigest("x"),
		Offline: true,
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if err == nil {
		t.Fatal("offline cache miss unexpectedly succeeded")
	}
	if !errors.Is(err, ErrOfflineCacheMiss) {
		t.Fatalf("offline cache miss error = %v, want ErrOfflineCacheMiss", err)
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
	path, err := acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(payload)),
		SHA256: testDigest(payload),
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
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:    name,
		URL:     "https://example.invalid/runtime.bin",
		Size:    int64(len("trusted")),
		SHA256:  testDigest("trusted"),
		Offline: true,
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if err == nil {
		t.Fatal("offline invalid cache unexpectedly succeeded")
	}
	if errors.Is(err, ErrOfflineCacheMiss) {
		t.Fatalf("offline invalid cache error = %v, must not be a cache miss", err)
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
		Name:   name,
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(payload)),
		SHA256: testDigest(payload),
		Fetch:  fetch,
	}
	if _, err := acquirePath(context.Background(), root, spec); err == nil {
		t.Fatal("first fetch unexpectedly succeeded")
	}
	if names := acquireTestEntryNames(t, root); len(names) != 0 {
		t.Fatalf("cache contents after failed fetch = %v, want empty", names)
	}
	path, err := acquirePath(context.Background(), root, spec)
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
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   1,
		SHA256: testDigest("x"),
		Fetch: func(_ context.Context, _ string, _ io.Writer) error {
			fetches.Add(1)
			return nil
		},
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("Acquire cached symlink error = %v, want ErrUnsafeArchive", err)
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
	_, err = acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   1,
		SHA256: testDigest("x"),
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
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len("trusted payload")),
		SHA256: testDigest("trusted payload"),
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
	if names := acquireTestEntryNames(t, originalRoot); len(names) != 0 {
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
	_, err := acquirePath(context.Background(), root, FetchSpec{
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len("trusted payload")),
		SHA256: testDigest("trusted payload"),
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
	if names := acquireTestEntryNames(t, filepath.Join(originalParent, "assets")); len(names) != 0 {
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
		Name:   name,
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(payload)),
		SHA256: testDigest(payload),
		Fetch:  fetch,
	}
	paths := make([]string, workers)
	errs := make([]error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			paths[i], errs[i] = acquirePath(context.Background(), root, spec)
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
	names := acquireTestEntryNames(t, root)
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
		Name:   name,
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(original)),
		SHA256: testDigest(original),
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
		_, err := acquirePath(context.Background(), root, FetchSpec{
			Name:   name,
			URL:    "https://example.invalid/runtime-v2.bin",
			Size:   int64(len(next)),
			SHA256: testDigest(next),
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
		Name:   name,
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(payload)),
		SHA256: testDigest(payload),
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
		Name:   "runtime.bin",
		URL:    "https://example.invalid/runtime.bin",
		Size:   int64(len(payload)),
		SHA256: testDigest(payload),
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

func TestAcquiredFileInvalidAndClosedState(t *testing.T) {
	for _, file := range []*AcquiredFile{nil, {}} {
		if n, err := file.Read(make([]byte, 1)); n != 0 || !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid Read = %d, %v", n, err)
		}
		if n, err := file.ReadAt(make([]byte, 1), 0); n != 0 || !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid ReadAt = %d, %v", n, err)
		}
		if info, err := file.Stat(); info != nil || !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid Stat = %v, %v", info, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("empty Close = %v", err)
		}
	}
	file, err := AcquireFile(context.Background(), t.TempDir(), acquireTestSpec("asset", "payload"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() != int64(len("payload")) || !info.Mode().IsRegular() {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed Stat = %v, want os.ErrClosed", err)
	}
}

type closeOrderTestLease struct {
	file     *os.File
	closes   atomic.Int32
	closeErr error
}

func (l *closeOrderTestLease) Close() error {
	l.closes.Add(1)
	if _, err := l.file.Stat(); !errors.Is(err, os.ErrClosed) {
		return errors.New("lease released before the acquired file was closed")
	}
	return l.closeErr
}

func TestAcquiredFileConcurrentClosePreservesOrderAndError(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "asset"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	closeErr := errors.New("lease close failure")
	lease := &closeOrderTestLease{file: file, closeErr: closeErr}
	acquired := &AcquiredFile{file: file, lease: lease}
	start := make(chan struct{})
	var callers sync.WaitGroup
	results := make([]error, 32)
	for i := range results {
		callers.Go(func() {
			<-start
			results[i] = acquired.Close()
		})
	}
	close(start)
	callers.Wait()
	for i, err := range results {
		if !errors.Is(err, closeErr) || errors.Is(err, os.ErrClosed) || err != results[0] {
			t.Errorf("Close caller %d = %v, want the same lease error %v", i, err, results[0])
		}
	}
	if got := lease.closes.Load(); got != 1 {
		t.Fatalf("lease closes = %d, want 1", got)
	}
}

func TestAcquiredFileClosePreservesBothErrors(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "asset"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	leaseErr := errors.New("lease close failure")
	lease := &errorTestLease{err: leaseErr}
	acquired := &AcquiredFile{file: file, lease: lease}
	for i := 0; i < 2; i++ {
		if err := acquired.Close(); !errors.Is(err, os.ErrClosed) || !errors.Is(err, leaseErr) {
			t.Fatalf("Close = %v, want file and lease errors", err)
		}
	}
	if lease.closes != 1 {
		t.Fatalf("lease closes = %d, want 1", lease.closes)
	}
}

func TestAcquireFileInvalidRequestAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*FetchSpec)
		want   string
	}{
		{"zero size", func(s *FetchSpec) { s.Size = 0 }, "size must be positive"},
		{"negative size", func(s *FetchSpec) { s.Size = -1 }, "size must be positive"},
		{"invalid digest", func(s *FetchSpec) { s.SHA256 = "bad" }, "invalid asset SHA-256"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "cache")
			spec := acquireTestSpec("asset", "payload")
			tt.change(&spec)
			if file, err := AcquireFile(context.Background(), root, spec); file != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AcquireFile = %v, %v; want %q", file, err, tt.want)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("invalid request created root: %v", err)
			}
		})
	}
	for _, root := range []string{"", "relative", t.TempDir() + string(filepath.Separator)} {
		if _, err := AcquireFile(context.Background(), root, acquireTestSpec("asset", "payload")); err == nil || !strings.Contains(err.Error(), "absolute and clean") {
			t.Fatalf("AcquireFile root %q = %v", root, err)
		}
	}
	t.Run("missing fetcher", func(t *testing.T) {
		spec := acquireTestSpec("asset", "payload")
		spec.Fetch = nil
		if _, err := acquirePath(nil, t.TempDir(), spec); err == nil || !strings.Contains(err.Error(), "no fetcher") {
			t.Fatalf("missing fetcher = %v", err)
		}
	})
	t.Run("cancel after fetch", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		root := t.TempDir()
		spec := acquireTestSpec("asset", "payload")
		spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, "payload")
			cancel()
			return err
		}
		if _, err := AcquireFile(ctx, root, spec); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled download = %v", err)
		}
		if names := acquireTestEntryNames(t, root); len(names) != 0 {
			t.Fatalf("cancelled download left assets: %v", names)
		}
	})
}

func TestOpenAcquiredFileCancelsWhileWaitingForLease(t *testing.T) {
	root := t.TempDir()
	spec := acquireTestSpec("asset", "trusted")
	path, err := acquirePath(t.Context(), root, spec)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := (CrossProcessLockProvider{}).AcquireExclusive(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()

	// The exclusive lease stays held until the shared acquisition has returned;
	// its deadline cannot race with an unlock and accidentally open the asset.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	acquired, err := openAcquiredFile(ctx, root, spec)
	if acquired != nil {
		defer acquired.Close()
	}
	if acquired != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("open while exclusively leased = %v, %v; want deadline exceeded", acquired, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}

	spec.Offline = true
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Error("offline reopen fetched the cached asset")
		return nil
	}
	acquired, err = AcquireFile(t.Context(), root, spec)
	if err != nil {
		t.Fatalf("offline reopen after cancellation: %v", err)
	}
	defer acquired.Close()
	if data, err := io.ReadAll(acquired); err != nil || string(data) != "trusted" {
		t.Fatalf("offline reopen = %q, %v; want trusted", data, err)
	}
}

func TestAcquireLockedPathCancellationDoesNotFetch(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	lease, err := acquirePlatformRootFileLock(t.Context(), path, root, "asset", lockExclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	spec := acquireTestSpec("asset", "trusted")
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Error("cancelled acquisition fetched a payload")
		return nil
	}
	if got, err := acquireLockedPath(ctx, path, root, spec); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquisition = %q, %v; want context.Canceled", got, err)
	}
	if names := acquireTestEntryNames(t, path); len(names) != 0 {
		t.Fatalf("cancelled acquisition left assets: %v", names)
	}
}

func TestAcquireLockedPathPreservesInspectionError(t *testing.T) {
	root := t.TempDir()
	cacheRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := cacheRoot.Close(); err != nil {
		t.Fatal(err)
	}
	spec := acquireTestSpec("asset", "payload")
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Error("inspection failure triggered a download")
		return nil
	}
	path, err := acquireLockedPath(t.Context(), root, cacheRoot, spec)
	if path != "" || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed acquisition root = %q, %v; want empty path and os.ErrClosed", path, err)
	}
}

func TestAcquireLimitWriter(t *testing.T) {
	writeErr := errors.New("destination failed")
	for _, tt := range []struct {
		name         string
		remaining    int64
		data         string
		write        acquireTestWriter
		wantN        int
		wantErr      error
		wantExceeded bool
	}{
		{name: "empty after limit", data: ""},
		{name: "exhausted", data: "x", wantErr: errAcquireSizeLimit, wantExceeded: true},
		{name: "over limit", remaining: 2, data: "abc", wantN: 2, wantErr: errAcquireSizeLimit, wantExceeded: true},
		{name: "exact limit", remaining: 3, data: "abc", wantN: 3},
		{name: "short write", remaining: 3, data: "abc", write: func([]byte) (int, error) { return 1, nil }, wantN: 1, wantErr: io.ErrShortWrite},
		{name: "writer error", remaining: 3, data: "abc", write: func([]byte) (int, error) { return 1, writeErr }, wantN: 1, wantErr: writeErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var dst bytes.Buffer
			var writer io.Writer = &dst
			if tt.write != nil {
				writer = tt.write
			}
			limited := &acquireLimitWriter{destination: writer, remaining: tt.remaining}
			n, err := limited.Write([]byte(tt.data))
			if n != tt.wantN || !errors.Is(err, tt.wantErr) || limited.remaining != tt.remaining-int64(n) || limited.exceeded != tt.wantExceeded {
				t.Fatalf("Write = %d, %v; remaining=%d, exceeded=%v", n, err, limited.remaining, limited.exceeded)
			}
			if tt.write == nil && dst.String() != tt.data[:tt.wantN] {
				t.Fatalf("destination = %q, want %q", dst.String(), tt.data[:tt.wantN])
			}
		})
	}
}

func TestAcquireRejectsDestinationReplacementDuringFetch(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
			spec := acquireTestSpec("asset", "payload")
			spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
				target := filepath.Join(root, spec.Name)
				if kind == "directory" {
					if err := os.Mkdir(target, 0o700); err != nil {
						return err
					}
				} else if err := os.Symlink(outside, target); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				_, err := io.WriteString(dst, "payload")
				return err
			}
			if _, err := AcquireFile(context.Background(), root, spec); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("publish onto replaced destination = %v, want ErrUnsafeArchive", err)
			}
			if data, err := os.ReadFile(outside); err != nil || string(data) != "untouched" {
				t.Fatalf("outside file changed: %q, %v", data, err)
			}
			if names := acquireTestEntryNames(t, root); len(names) != 1 || names[0] != "asset" {
				t.Fatalf("failed publication left temporary files: %v", names)
			}
		})
	}
}

func TestReplaceAcquireFilePreservesSourceOnFailure(t *testing.T) {
	for _, fault := range []string{"missing source", "directory source", "invalid destination", "missing destination parent"} {
		t.Run(fault, func(t *testing.T) {
			path := t.TempDir()
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			source, destination := "source", "destination"
			switch fault {
			case "missing source":
			case "directory source":
				err = os.Mkdir(filepath.Join(path, source), 0o700)
			default:
				err = os.WriteFile(filepath.Join(path, source), []byte("trusted"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "invalid destination":
				destination = "invalid\x00"
			case "missing destination parent":
				destination = "missing/destination"
			}
			err = replaceAcquireRootFile(root, source, destination)
			if err == nil {
				t.Fatal("invalid replacement succeeded")
			}
			if fault == "directory source" && !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("directory source = %v", err)
			}
			if fault == "missing source" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing source = %v", err)
			}
			if fault == "invalid destination" || fault == "missing destination parent" {
				if data, err := os.ReadFile(filepath.Join(path, source)); err != nil || string(data) != "trusted" {
					t.Fatalf("source changed: %q, %v", data, err)
				}
			}
		})
	}
}

func TestAcquireRejectsRenamedRootBeforePublication(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	spec := acquireTestSpec("asset", "payload")
	spec.Fetch = func(_ context.Context, _ string, dst io.Writer) error {
		if err := os.Rename(root, root+".old"); err != nil {
			return err
		}
		if err := os.Mkdir(root, 0o700); err != nil {
			return err
		}
		_, err := io.WriteString(dst, "payload")
		return err
	}
	if file, err := AcquireFile(context.Background(), root, spec); file != nil || !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("renamed acquisition root = %v, %v", file, err)
	}
	for _, path := range []string{root, root + ".old"} {
		if names := acquireTestEntryNames(t, path); len(names) != 0 {
			t.Fatalf("failed acquisition left assets in %s: %v", path, names)
		}
	}
}

func TestAcquireFileRejectsReservedAndNonportableNamesBeforeCreatingCache(t *testing.T) {
	for _, name := range []string{
		"", ".", "..", "/absolute", "nested/asset", `nested\asset`,
		"asset.lock", "asset.LOCK", "asset.locK", ".lock",
		"asset.lock.", "asset.lock ", "asset.lock:stream", "asset:stream",
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "cache")
			spec := acquireTestSpec(name, "payload")
			fetched := false
			spec.Fetch = func(context.Context, string, io.Writer) error {
				fetched = true
				return errors.New("fetch must not run")
			}
			file, err := AcquireFile(context.Background(), root, spec)
			if file != nil {
				file.Close()
			}
			if err == nil || fetched {
				t.Fatalf("AcquireFile(%q): error %v, fetched %v; want rejection before fetch", name, err, fetched)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("invalid name created cache: Lstat error = %v", err)
			}
		})
	}
}

func TestAcquireFileLockNameCannotInvalidateSharedLease(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireFile(context.Background(), root, acquireTestSpec("engine.bin", "old"))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	lockPath := filepath.Join(root, "engine.bin.lock")
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	collision, collisionErr := AcquireFile(context.Background(), root, acquireTestSpec("engine.bin.lock", "lock collision"))
	if collision != nil {
		defer collision.Close()
	}
	if collisionErr == nil {
		t.Error("AcquireFile accepted another asset's lock sidecar name")
	}
	after, err := os.Stat(lockPath)
	if err != nil || !os.SameFile(before, after) {
		t.Errorf("sidecar identity changed while lease was held: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	replacement, err := AcquireFile(ctx, root, acquireTestSpec("engine.bin", "new"))
	if replacement != nil {
		replacement.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("replacement while shared lease held = %v, want deadline exceeded", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err = AcquireFile(context.Background(), root, acquireTestSpec("engine.bin", "new"))
	if err != nil {
		t.Fatalf("replacement after lease close: %v", err)
	}
	defer replacement.Close()
	data, err := io.ReadAll(replacement)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement contents = %q, %v; want new", data, err)
	}
}

func TestAcquireFileAllowsNonreservedPortableNames(t *testing.T) {
	for _, name := range []string{"asset.bin", "asset.lock.zip", ".lockfile", "ロック.bin"} {
		t.Run(name, func(t *testing.T) {
			file, err := AcquireFile(context.Background(), t.TempDir(), acquireTestSpec(name, "payload"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || string(data) != "payload" {
				t.Fatalf("acquired contents = %q, %v; want payload", data, err)
			}
		})
	}
}

func TestOpenAcquiredFileRejectsChangedAssetsAndReleasesLease(t *testing.T) {
	for _, change := range []string{"removed", "different size", "different digest", "directory"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			spec := acquireTestSpec("asset", "trusted")
			path, err := acquirePath(t.Context(), root, spec)
			if err != nil {
				t.Fatal(err)
			}
			want := ErrDigestMismatch
			switch change {
			case "removed", "directory":
				err = os.Remove(path)
				want = os.ErrNotExist
				if err == nil && change == "directory" {
					err = os.Mkdir(path, 0o700)
					want = ErrUnsafeArchive
				}
			case "different size":
				err = os.WriteFile(path, []byte("too long"), 0o600)
			case "different digest":
				err = os.WriteFile(path, []byte("changed"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			spec.Fetch = func(context.Context, string, io.Writer) error {
				t.Error("opening an acquired file unexpectedly fetched a replacement")
				return errors.New("unexpected fetch")
			}
			file, err := openAcquiredFile(t.Context(), root, spec)
			if file != nil {
				_ = file.Close()
			}
			if file != nil || !errors.Is(err, want) {
				t.Fatalf("open changed asset = %v, %v; want %v", file, err, want)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			lease, err := (CrossProcessLockProvider{}).AcquireExclusive(ctx, path)
			if err != nil {
				t.Fatalf("failed verification retained shared lease: %v", err)
			}
			if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpenAcquiredFileRejectsReplacedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	spec := acquireTestSpec("asset", "trusted")
	if _, err := acquirePath(t.Context(), root, spec); err != nil {
		t.Fatal(err)
	}
	retired := root + ".old"
	if err := os.Rename(root, retired); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openAcquiredFile(t.Context(), root, spec)
	if file != nil {
		_ = file.Close()
	}
	if file != nil || !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("open replaced root = %v, %v; want ErrUnsafeArchive", file, err)
	}
	for path, want := range map[string]string{
		root: "replacement", filepath.Join(retired, spec.Name): "trusted",
	} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("file %q after rejected open = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestOpenAcquiredFileCancellationLeavesCacheUsable(t *testing.T) {
	root := t.TempDir()
	spec := acquireTestSpec("asset", "trusted")
	if _, err := acquirePath(t.Context(), root, spec); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	file, err := openAcquiredFile(ctx, root, spec)
	if file != nil {
		_ = file.Close()
	}
	if file != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled open = %v, %v; want context.Canceled", file, err)
	}
	spec.Offline = true
	file, err = AcquireFile(t.Context(), root, spec)
	if err != nil {
		t.Fatalf("reopen cached asset after cancellation: %v", err)
	}
	defer file.Close()
	if got, err := io.ReadAll(file); err != nil || string(got) != "trusted" {
		t.Fatalf("cached asset after cancellation = %q, %v; want trusted", got, err)
	}
}

func (w acquireTestWriter) Write(data []byte) (int, error) { return w(data) }

func acquireTestEntryNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}
		names = append(names, entry.Name())
	}
	return names
}

func acquireTestSpec(name, body string) FetchSpec {
	return FetchSpec{
		Name: name, Size: int64(len(body)), SHA256: testDigest(body),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, body)
			return err
		},
	}
}
