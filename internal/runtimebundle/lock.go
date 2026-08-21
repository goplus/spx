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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrCrossProcessLockUnsupported reports that the current platform cannot
// provide the cache's required cross-process lease semantics.
var ErrCrossProcessLockUnsupported = errors.New("runtimebundle: cross-process locks are unsupported")

// LockProvider coordinates publication/repair (exclusive) and use (shared)
// leases for one cache target. Implementations must keep the lease alive until
// Close; an OS-backed implementation therefore also survives a crashed
// process by letting the kernel close the underlying handle.
type LockProvider interface {
	AcquireExclusive(context.Context, string) (LockLease, error)
	AcquireShared(context.Context, string) (LockLease, error)
}

// LockLease is an idempotent, goroutine-safe lock release handle.
type LockLease interface {
	Close() error
}

// CrossProcessLockProvider uses a sidecar lock file next to (not inside) the
// target. The platform implementation uses flock on POSIX and LockFileEx on
// Windows.
type CrossProcessLockProvider struct{}

func (CrossProcessLockProvider) AcquireExclusive(ctx context.Context, key string) (LockLease, error) {
	return acquirePlatformFileLock(ctx, key, lockExclusive)
}

func (CrossProcessLockProvider) AcquireShared(ctx context.Context, key string) (LockLease, error) {
	return acquirePlatformFileLock(ctx, key, lockShared)
}

// UnsupportedCrossProcessLockProvider is retained as an explicit fail-closed
// test seam. It is not used by NewCache.
type UnsupportedCrossProcessLockProvider struct{}

func (UnsupportedCrossProcessLockProvider) AcquireExclusive(context.Context, string) (LockLease, error) {
	return nil, ErrCrossProcessLockUnsupported
}

func (UnsupportedCrossProcessLockProvider) AcquireShared(context.Context, string) (LockLease, error) {
	return nil, ErrCrossProcessLockUnsupported
}

// ProcessLockProvider is an explicitly opt-in process-local implementation.
// It is useful for tests which deliberately do not share a cache root across
// processes; production callers should use NewCache.
type ProcessLockProvider struct{}

type processGate struct {
	mu      sync.Mutex
	readers int
	writer  bool
}

var processLocks sync.Map // map[string]*processGate

func (ProcessLockProvider) AcquireExclusive(ctx context.Context, key string) (LockLease, error) {
	return acquireProcessLock(ctx, key, lockExclusive)
}

func (ProcessLockProvider) AcquireShared(ctx context.Context, key string) (LockLease, error) {
	return acquireProcessLock(ctx, key, lockShared)
}

func acquireProcessLock(ctx context.Context, key string, mode lockMode) (LockLease, error) {
	if key == "" {
		return nil, fmt.Errorf("runtimebundle: empty cache lock key")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	value, _ := processLocks.LoadOrStore(key, &processGate{})
	gate := value.(*processGate)
	for {
		if gate.tryAcquire(mode) {
			return &processLease{gate: gate, mode: mode}, nil
		}
		if err := waitForLock(ctx); err != nil {
			return nil, err
		}
	}
}

func (g *processGate) tryAcquire(mode lockMode) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if mode == lockExclusive {
		if g.writer || g.readers != 0 {
			return false
		}
		g.writer = true
		return true
	}
	if g.writer {
		return false
	}
	g.readers++
	return true
}

func (g *processGate) release(mode lockMode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if mode == lockExclusive {
		g.writer = false
		return
	}
	if g.readers > 0 {
		g.readers--
	}
}

type processLease struct {
	gate *processGate
	mode lockMode
	once sync.Once
}

func (l *processLease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() { l.gate.release(l.mode) })
	return nil
}

type lockMode uint8

const (
	lockShared lockMode = iota
	lockExclusive
)

type fileLease struct {
	file platformLockFile
	once sync.Once
	err  error
}

func (l *fileLease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		unlockErr := l.file.unlock()
		closeErr := l.file.close()
		l.err = errors.Join(unlockErr, closeErr)
	})
	return l.err
}

func acquirePlatformFileLock(ctx context.Context, key string, mode lockMode) (LockLease, error) {
	if key == "" {
		return nil, fmt.Errorf("runtimebundle: empty cache lock key")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openPlatformLockFile(lockSidecarPath(key))
	if err != nil {
		return nil, err
	}
	return acquireOpenedPlatformFileLock(ctx, file, mode)
}

func acquirePlatformRootFileLock(ctx context.Context, rootPath string, root *os.Root, key string, mode lockMode) (LockLease, error) {
	if key == "" || filepath.Base(key) != key || strings.ContainsAny(key, `/\\`) {
		return nil, fmt.Errorf("runtimebundle: invalid rooted cache lock key %q", key)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openPlatformRootLockFile(rootPath, root, lockSidecarPath(key), false)
	if err != nil {
		return nil, err
	}
	return acquireOpenedPlatformFileLock(ctx, file, mode)
}

func acquireOpenedPlatformFileLock(ctx context.Context, file platformLockFile, mode lockMode) (LockLease, error) {
	for {
		acquired, tryErr := file.tryLock(mode)
		if tryErr != nil {
			_ = file.close()
			return nil, tryErr
		}
		if acquired {
			if err := file.validate(); err != nil {
				_ = file.unlock()
				_ = file.close()
				return nil, err
			}
			return &fileLease{file: file}, nil
		}
		if err := waitForLock(ctx); err != nil {
			_ = file.close()
			return nil, err
		}
	}
}

func lockSidecarPath(key string) string { return key + ".lock" }

func openPlatformLockFile(path string) (platformLockFile, error) {
	parentPath := filepath.Dir(path)
	root, err := openOrCreateLockParent(parentPath)
	if err != nil {
		return nil, err
	}
	file, err := openPlatformRootLockFile(parentPath, root, filepath.Base(path), true)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return file, nil
}

type rootedPlatformLockFile struct {
	file     rawPlatformLockFile
	rootPath string
	root     *os.Root
	name     string
	ownsRoot bool
}

func openPlatformRootLockFile(rootPath string, root *os.Root, name string, ownsRoot bool) (platformLockFile, error) {
	if root == nil {
		return nil, fmt.Errorf("runtimebundle: nil pinned lock root")
	}
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return nil, fmt.Errorf("runtimebundle: invalid lock sidecar name %q", name)
	}
	var before os.FileInfo
	if info, err := root.Lstat(name); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: lock sidecar is not a regular non-symlink file: %s", ErrUnsafeArchive, name)
		}
		before = info
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	raw, err := openPlatformLockFileImpl(root, name)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: open lock sidecar %s: %w", name, err)
	}
	closeRaw := true
	defer func() {
		if closeRaw {
			_ = raw.close()
		}
	}()
	opened, err := raw.stat()
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: stat opened lock sidecar %s: %w", name, err)
	}
	if !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: opened lock sidecar is not regular: %s", ErrUnsafeArchive, name)
	}
	if before != nil && !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%w: lock sidecar changed while opening: %s", ErrUnsafeArchive, name)
	}
	file := &rootedPlatformLockFile{file: raw, rootPath: rootPath, root: root, name: name, ownsRoot: ownsRoot}
	if err := file.validateIdentity(); err != nil {
		return nil, err
	}
	if err := raw.protect(); err != nil {
		return nil, fmt.Errorf("runtimebundle: protect lock sidecar %s: %w", name, err)
	}
	if err := file.validate(); err != nil {
		return nil, err
	}
	closeRaw = false
	return file, nil
}

func (f *rootedPlatformLockFile) tryLock(mode lockMode) (bool, error) {
	return f.file.tryLock(mode)
}

func (f *rootedPlatformLockFile) unlock() error { return f.file.unlock() }

func (f *rootedPlatformLockFile) close() error {
	fileErr := f.file.close()
	if !f.ownsRoot {
		return fileErr
	}
	return errors.Join(fileErr, f.root.Close())
}

func (f *rootedPlatformLockFile) validate() error {
	if err := f.validateIdentity(); err != nil {
		return err
	}
	pathInfo, err := f.root.Lstat(f.name)
	if err != nil {
		return fmt.Errorf("%w: lock sidecar pathname changed: %v", ErrUnsafeArchive, err)
	}
	if runtimeIsUnix() && pathInfo.Mode().Perm() != 0o600 {
		return fmt.Errorf("%w: lock sidecar is not private: %s", ErrUnsafeArchive, f.name)
	}
	return verifyPrivateRootPath(f.root, f.name)
}

func (f *rootedPlatformLockFile) validateIdentity() error {
	if err := checkPinnedRootPath(f.rootPath, f.root); err != nil {
		return err
	}
	pathInfo, err := f.root.Lstat(f.name)
	if err != nil {
		return fmt.Errorf("%w: lock sidecar pathname changed: %v", ErrUnsafeArchive, err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: lock sidecar is not a regular non-symlink file: %s", ErrUnsafeArchive, f.name)
	}
	opened, err := f.file.stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return fmt.Errorf("%w: lock sidecar pathname was replaced: %s", ErrUnsafeArchive, f.name)
	}
	return nil
}

func openOrCreateLockParent(parent string) (*os.Root, error) {
	if parent == "" || parent == "." {
		return nil, fmt.Errorf("runtimebundle: invalid lock parent")
	}
	if info, err := os.Lstat(parent); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("%w: lock parent is not a real directory: %s", ErrUnsafeArchive, parent)
		}
		if err := validateLockParentSecurity(parent); err != nil {
			return nil, err
		}
		root, err := openPinnedRoot(parent)
		if err != nil {
			return nil, err
		}
		if err := validateLockParentSecurity(parent); err != nil {
			_ = root.Close()
			return nil, err
		}
		return root, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// Cache materialization normally prepares this directory. Direct provider
	// users may securely create missing private components from the nearest
	// pinned existing ancestor.
	root, err := openOrCreateCacheRoot(parent, defaultPermissions())
	if err != nil {
		return nil, err
	}
	if err := validateLockParentSecurity(parent); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func waitForLock(ctx context.Context) error {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runtimeIsWindows() bool { return runtime.GOOS == "windows" }

// platformLockFile is implemented in lock_posix.go, lock_windows.go, or the
// explicit unsupported fallback for platforms without an OS lock primitive.
type rawPlatformLockFile interface {
	tryLock(lockMode) (bool, error)
	unlock() error
	close() error
	stat() (os.FileInfo, error)
	protect() error
}

type platformLockFile interface {
	tryLock(lockMode) (bool, error)
	unlock() error
	close() error
	validate() error
}
