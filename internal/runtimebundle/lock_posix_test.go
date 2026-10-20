//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

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
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOpenPlatformLockFileImplRejectsSymlink(t *testing.T) {
	rootPath := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	const name = "runtime.bin.lock"
	if err := os.Symlink(outside, filepath.Join(rootPath, name)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := openPinnedRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := openPlatformLockFileImpl(root, name)
	if file != nil {
		_ = file.close()
	}
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("open no-follow lock symlink error = %v, want ErrUnsafeArchive", err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside" {
		t.Fatalf("outside lock = %q, err=%v; want unchanged", data, err)
	}
}

func TestAcquireOpenedLockRejectsChangedIdentity(t *testing.T) {
	for _, change := range []string{"removed", "replaced", "directory", "public permissions", "closed descriptor", "moved root"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "asset.lock")
			file, err := openPlatformLockFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.close()
			rooted := file.(*rootedPlatformLockFile)
			want := ErrUnsafeArchive
			switch change {
			case "removed":
				err = os.Remove(path)
			case "replaced", "directory":
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
				if change == "replaced" {
					err = os.WriteFile(path, []byte("replacement"), 0o600)
				} else {
					err = os.Mkdir(path, 0o700)
				}
			case "public permissions":
				err = os.Chmod(path, 0o644)
			case "moved root":
				err = os.Rename(filepath.Dir(path), filepath.Dir(path)+".old")
				t.Cleanup(func() { _ = os.Rename(filepath.Dir(path)+".old", filepath.Dir(path)) })
			case "closed descriptor":
				err = rooted.file.close()
				want = unix.EBADF
			}
			if err != nil {
				t.Fatal(err)
			}
			lease, err := acquireOpenedPlatformFileLock(context.Background(), file, lockExclusive)
			if lease != nil || !errors.Is(err, want) {
				t.Fatalf("acquire changed lock = %v, %v; want %v", lease, err, want)
			}
			if _, err := rooted.file.stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("rejected lock descriptor left open: %v", err)
			}
			if _, err := rooted.root.Stat("."); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("rejected lock root left open: %v", err)
			}
		})
	}
}

func TestRootedLockRejectsInvalidRequests(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"", ".", "..", "nested/asset", `nested\asset`} {
		if file, err := openPlatformRootLockFile(rootPath, root, name, false); file != nil || err == nil {
			t.Fatalf("openPlatformRootLockFile(%q) = %v, %v", name, file, err)
		}
	}
	if file, err := openPlatformRootLockFile(rootPath, nil, "asset.lock", false); file != nil || err == nil {
		t.Fatalf("nil lock root = %v, %v", file, err)
	}
	for _, key := range []string{"", "nested/asset", `nested\asset`} {
		if lease, err := acquirePlatformRootFileLock(nil, rootPath, root, key, lockShared); lease != nil || err == nil {
			t.Fatalf("acquirePlatformRootFileLock(%q) = %v, %v", key, lease, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lease, err := acquirePlatformRootFileLock(ctx, rootPath, root, "asset", lockShared); lease != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled rooted lock = %v, %v", lease, err)
	}
	if entries, err := os.ReadDir(rootPath); err != nil || len(entries) != 0 {
		t.Fatalf("invalid lock requests created entries: %v, %v", entries, err)
	}
	lease, err := acquirePlatformRootFileLock(nil, rootPath, root, "asset", lockShared)
	if err != nil {
		t.Fatalf("nil context rooted lock = %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("closing lease closed caller-owned root: %v", err)
	}
}

func TestLockProvidersValidateKeyAndDefaultContext(t *testing.T) {
	for name, provider := range map[string]LockProvider{
		"process": ProcessLockProvider{}, "cross-process": CrossProcessLockProvider{},
	} {
		t.Run(name, func(t *testing.T) {
			for mode, acquire := range []func(context.Context, string) (LockLease, error){provider.AcquireShared, provider.AcquireExclusive} {
				if lease, err := acquire(nil, ""); lease != nil || err == nil {
					t.Fatalf("empty lock key = %v, %v", lease, err)
				}
				leaseKey := filepath.Join(t.TempDir(), "asset")
				lease, err := acquire(nil, leaseKey)
				if err != nil {
					t.Fatalf("nil lock context = %v", err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if blocked, err := provider.AcquireExclusive(ctx, leaseKey); blocked != nil || !errors.Is(err, context.Canceled) {
					t.Fatalf("conflicting cancelled lock = %v, %v", blocked, err)
				}
				if mode == int(lockExclusive) {
					if blocked, err := provider.AcquireShared(ctx, leaseKey); blocked != nil || !errors.Is(err, context.Canceled) {
						t.Fatalf("shared reader entered held exclusive lease: %v, %v", blocked, err)
					}
				}
				for i := 0; i < 2; i++ {
					if err := lease.Close(); err != nil {
						t.Fatalf("idempotent lease Close = %v", err)
					}
				}
			}
		})
	}
	if err := (*fileLease)(nil).Close(); err != nil {
		t.Fatalf("nil file lease Close = %v", err)
	}
	if err := (*processLease)(nil).Close(); err != nil {
		t.Fatalf("nil process lease Close = %v", err)
	}
}

func TestRootOperationsPreservePermissionErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission errors require an unprivileged user")
	}
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	asset := filepath.Join(rootPath, "asset")
	if err := os.WriteFile(asset, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(asset, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(asset, 0o600) })
	if file, err := openSourceZip(asset); file != nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("openSourceZip = %v, %v", file, err)
	}
	if file, err := openVerifiedAcquireRootFile(root, "asset", 7, testDigest("payload")); file != nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("open verified asset = %v, %v", file, err)
	}
	if err := os.Chmod(rootPath, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rootPath, 0o700) })
	for name, operation := range map[string]func() error{
		"directory temp": func() error { _, err := newRootTemp(root); return err },
		"asset temp": func() error {
			_, file, err := newAcquireRootTemp(root)
			if file != nil {
				_ = file.Close()
			}
			return err
		},
		"nested directory": func() error { return ensureRootDir(root, "nested") },
		"nested cache root": func() error {
			child, err := openOrCreateCacheRoot(filepath.Join(rootPath, "nested/cache"), modePermissions{})
			if child != nil {
				_ = child.Close()
			}
			return err
		},
		"lock sidecar": func() error {
			file, err := openPlatformRootLockFile(rootPath, root, "new.lock", false)
			if file != nil {
				_ = file.close()
			}
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, os.ErrPermission) {
				t.Fatalf("error = %v, want os.ErrPermission", err)
			}
		})
	}
	if err := os.Chmod(rootPath, 0); err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func() error{
		"pin root": func() error {
			child, err := openPinnedRoot(rootPath)
			if child != nil {
				_ = child.Close()
			}
			return err
		},
		"open cache": func() error {
			child, err := openOrCreateCacheRoot(rootPath, modePermissions{})
			if child != nil {
				_ = child.Close()
			}
			return err
		},
		"list cache":   func() error { _, err := NewProcessLocalCache(rootPath).List(NamespaceEngine); return err },
		"remove cache": func() error { return NewProcessLocalCache(rootPath).Remove(NamespaceEngine, testDigest("")) },
		"lock parent": func() error {
			child, err := openOrCreateLockParent(rootPath)
			if child != nil {
				_ = child.Close()
			}
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, os.ErrPermission) {
				t.Fatalf("error = %v, want os.ErrPermission", err)
			}
		})
	}
}

func TestCrossProcessLocksRejectUnsafeParentPaths(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "symlink ancestor", "invalid path", "relative path"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			parent := filepath.Join(root, "parent")
			key := filepath.Join(parent, "asset")
			var setupErr error
			switch kind {
			case "file":
				setupErr = os.WriteFile(parent, []byte("untouched"), 0o600)
			case "symlink", "symlink ancestor":
				setupErr = os.Symlink(outside, parent)
				if kind == "symlink ancestor" {
					key = filepath.Join(parent, "missing", "asset")
				}
			case "invalid path":
				key = filepath.Join(root, "invalid\x00", "asset")
			case "relative path":
				t.Chdir(root)
				key = "asset"
			}
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			for _, acquire := range []func(context.Context, string) (LockLease, error){
				(CrossProcessLockProvider{}).AcquireShared,
				(CrossProcessLockProvider{}).AcquireExclusive,
			} {
				lease, err := acquire(context.Background(), key)
				if lease != nil {
					_ = lease.Close()
					t.Fatal("unsafe parent returned a lock lease")
				}
				if err == nil {
					t.Fatal("unsafe lock parent accepted")
				}
				switch kind {
				case "file", "symlink", "symlink ancestor":
					if !errors.Is(err, ErrUnsafeArchive) {
						t.Fatalf("unsafe parent error = %v", err)
					}
				case "invalid path":
					var pathErr *os.PathError
					if !errors.As(err, &pathErr) {
						t.Fatalf("invalid parent lost PathError: %v", err)
					}
				}
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
				t.Fatalf("lock request touched symlink target: %v, %v", entries, err)
			}
			if kind == "file" {
				if data, err := os.ReadFile(parent); err != nil || string(data) != "untouched" {
					t.Fatalf("lock request changed parent file: %q, %v", data, err)
				}
			}
		})
	}
}
