//go:build darwin || linux

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
	"runtime/debug"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOpenVerifiedAcquireRootFileClosesRejectedAsset(t *testing.T) {
	// os.File cleanup must not hide a descriptor the verifier failed to close.
	previousGC := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previousGC) })
	runtime.GC()
	for _, mismatch := range []string{"size", "digest"} {
		t.Run(mismatch, func(t *testing.T) {
			const payload = "trusted"
			path := filepath.Join(t.TempDir(), "asset")
			if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()

			// Confirm inspection identifies this file, independently of unrelated
			// descriptors opened or closed elsewhere in the process.
			probe, err := root.Open("asset")
			if err != nil {
				t.Fatal(err)
			}
			defer probe.Close()
			if fds := acquireTestFileDescriptors(t, path); len(fds) != 1 || fds[0] != int(probe.Fd()) {
				t.Fatalf("open asset descriptors = %v, want [%d]", fds, probe.Fd())
			}
			if err := probe.Close(); err != nil {
				t.Fatal(err)
			}

			size, digest := int64(len(payload)), testDigest(payload)
			if mismatch == "size" {
				size++
			} else {
				digest = testDigest("changed")
			}
			file, err := openVerifiedAcquireRootFile(root, "asset", size, digest)
			if file != nil {
				defer file.Close()
			}
			if file != nil || !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("open rejected asset = %v, %v; want ErrDigestMismatch", file, err)
			}
			if fds := acquireTestFileDescriptors(t, path); len(fds) != 0 {
				t.Fatalf("verification failure leaked asset descriptors: %v", fds)
			}
		})
	}
}

func TestAcquireLockedPathTemporaryFilePermissionFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission failure requires an unprivileged process")
	}
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
	if err := root.Chmod(".", 0o500); err != nil {
		t.Fatal(err)
	}
	defer root.Chmod(".", 0o700)

	spec := acquireTestSpec("asset", "trusted")
	spec.Fetch = func(context.Context, string, io.Writer) error {
		t.Error("temporary-file failure fetched a payload")
		return nil
	}
	if got, err := acquireLockedPath(t.Context(), path, root, spec); got != "" || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unwritable acquisition root = %q, %v; want permission error", got, err)
	}
	if names := acquireTestEntryNames(t, path); len(names) != 0 {
		t.Fatalf("temporary-file failure left assets: %v", names)
	}
}

func acquireTestFileDescriptors(t *testing.T, path string) []int {
	t.Helper()
	var asset unix.Stat_t
	if err := unix.Stat(path, &asset); err != nil {
		t.Fatal(err)
	}
	fdDir := "/dev/fd"
	if runtime.GOOS == "linux" {
		fdDir = "/proc/self/fd"
	}
	directory, err := os.Open(fdDir)
	if err != nil {
		t.Skipf("file descriptor inspection unavailable: %v", err)
	}
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	var fds []int
	for _, name := range names {
		fd, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		var opened unix.Stat_t
		if err := unix.Fstat(fd, &opened); errors.Is(err, unix.EBADF) {
			// An unrelated descriptor may have closed since enumeration.
			continue
		} else if err != nil {
			t.Fatalf("inspect descriptor %d: %v", fd, err)
		}
		if opened.Dev == asset.Dev && opened.Ino == asset.Ino {
			fds = append(fds, fd)
		}
	}
	return fds
}
