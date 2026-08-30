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
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var errAcquireSizeLimit = errors.New("runtimebundle: acquired asset exceeds size limit")

// ErrOfflineCacheMiss reports that offline acquisition has no cached file.
var ErrOfflineCacheMiss = errors.New("runtimebundle: offline cache miss")

// FetchFunc writes one URL response to dst and must honor ctx.
type FetchFunc func(ctx context.Context, url string, dst io.Writer) error

// FetchSpec pins one downloadable asset.
type FetchSpec struct {
	Name    string
	URL     string
	Size    int64
	SHA256  string
	Offline bool
	Fetch   FetchFunc
}

// AcquiredFile is a verified cache file held by a shared use lease.
type AcquiredFile struct {
	file      *os.File
	lease     LockLease
	closeOnce sync.Once
	closeErr  error
}

func (f *AcquiredFile) Read(p []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Read(p)
}

func (f *AcquiredFile) ReadAt(p []byte, off int64) (int, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.ReadAt(p, off)
}

func (f *AcquiredFile) Stat() (os.FileInfo, error) {
	if f == nil || f.file == nil {
		return nil, os.ErrInvalid
	}
	return f.file.Stat()
}

// Close is idempotent and safe for concurrent callers.
func (f *AcquiredFile) Close() error {
	if f == nil {
		return nil
	}
	f.closeOnce.Do(func() {
		var fileErr, leaseErr error
		if f.file != nil {
			fileErr = f.file.Close()
		}
		if f.lease != nil {
			leaseErr = f.lease.Close()
		}
		f.closeErr = errors.Join(fileErr, leaseErr)
	})
	return f.closeErr
}

// AcquireFile downloads or reuses one file, verifies it, and keeps a shared
// use lease while the returned descriptor is read.
func AcquireFile(ctx context.Context, root string, spec FetchSpec) (*AcquiredFile, error) {
	_, err := acquirePath(ctx, root, spec)
	if err != nil {
		return nil, err
	}
	cacheRoot, err := openOrCreateCacheRoot(root, defaultPermissions())
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: pin acquisition root for use: %w", err)
	}
	defer cacheRoot.Close()
	lease, err := acquirePlatformRootFileLock(ctx, root, cacheRoot, spec.Name, lockShared)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: acquire shared lock for asset %q: %w", spec.Name, err)
	}
	closeLease := true
	defer func() {
		if closeLease {
			_ = lease.Close()
		}
	}()
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		return nil, err
	}
	file, err := openVerifiedAcquireRootFile(cacheRoot, spec.Name, spec.Size, spec.SHA256)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: verify acquired asset %q for use: %w", spec.Name, err)
	}
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		_ = file.Close()
		return nil, err
	}
	closeLease = false
	return &AcquiredFile{file: file, lease: lease}, nil
}

func acquirePath(ctx context.Context, root string, spec FetchSpec) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateAcquireSpec(spec); err != nil {
		return "", err
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", fmt.Errorf("runtimebundle: acquisition root must be absolute and clean: %q", root)
	}
	cacheRoot, err := openOrCreateCacheRoot(root, defaultPermissions())
	if err != nil {
		return "", fmt.Errorf("runtimebundle: pin acquisition root: %w", err)
	}
	defer cacheRoot.Close()
	path := filepath.Join(root, spec.Name)
	lease, err := acquirePlatformRootFileLock(ctx, root, cacheRoot, spec.Name, lockExclusive)
	if err != nil {
		return "", fmt.Errorf("runtimebundle: acquire lock for asset %q: %w", spec.Name, err)
	}
	defer lease.Close()
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		return "", err
	}
	if info, err := cacheRoot.Lstat(spec.Name); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("%w: cached asset %q is not a regular non-symlink file", ErrUnsafeArchive, path)
		}
		if err := verifyAcquireRootFile(cacheRoot, spec.Name, spec.Size, spec.SHA256); err == nil {
			if err := checkPinnedRootPath(root, cacheRoot); err != nil {
				return "", err
			}
			return path, nil
		} else if spec.Offline {
			return "", fmt.Errorf("runtimebundle: offline cached asset %q failed verification: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("runtimebundle: inspect cached asset %q: %w", path, err)
	}
	if spec.Offline {
		return "", fmt.Errorf("%w for %q (URL: %s)", ErrOfflineCacheMiss, spec.Name, spec.URL)
	}
	if spec.Fetch == nil {
		return "", fmt.Errorf("runtimebundle: no fetcher for %q", spec.Name)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	tmpName, tmp, err := newAcquireRootTemp(cacheRoot)
	if err != nil {
		return "", fmt.Errorf("runtimebundle: create temporary asset %q: %w", spec.Name, err)
	}
	defer func() {
		_ = cacheRoot.Remove(tmpName)
	}()
	limiter := &acquireLimitWriter{destination: tmp, remaining: spec.Size}
	if err := spec.Fetch(ctx, spec.URL, limiter); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("runtimebundle: fetch %q: %w", spec.Name, err)
	}
	if limiter.exceeded {
		_ = tmp.Close()
		return "", errAcquireSizeLimit
	}
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := ctx.Err(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("runtimebundle: sync downloaded asset %q: %w", spec.Name, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("runtimebundle: close downloaded asset %q: %w", spec.Name, err)
	}
	if err := verifyAcquireRootFile(cacheRoot, tmpName, spec.Size, spec.SHA256); err != nil {
		return "", fmt.Errorf("runtimebundle: verify downloaded asset %q: %w", spec.Name, err)
	}
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		return "", err
	}
	if err := replaceAcquireRootFile(cacheRoot, tmpName, spec.Name); err != nil {
		return "", fmt.Errorf("runtimebundle: publish downloaded asset %q: %w", spec.Name, err)
	}
	if err := syncRootDir(cacheRoot); err != nil {
		return "", fmt.Errorf("runtimebundle: sync acquired asset directory %q: %w", spec.Name, err)
	}
	if err := verifyAcquireRootFile(cacheRoot, spec.Name, spec.Size, spec.SHA256); err != nil {
		return "", fmt.Errorf("runtimebundle: verify published asset %q: %w", spec.Name, err)
	}
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		return "", err
	}
	return path, nil
}

func validateAcquireSpec(spec FetchSpec) error {
	if spec.Name == "" || spec.Name == "." || spec.Name == ".." || filepath.Base(spec.Name) != spec.Name || filepath.IsAbs(spec.Name) {
		return fmt.Errorf("runtimebundle: invalid acquired asset name %q", spec.Name)
	}
	if strings.ContainsAny(spec.Name, `/\\`) {
		return fmt.Errorf("runtimebundle: acquired asset name must be a basename: %q", spec.Name)
	}
	if spec.Size <= 0 {
		return fmt.Errorf("runtimebundle: asset size must be positive for %q", spec.Name)
	}
	if err := validateSHA256(spec.SHA256); err != nil {
		return fmt.Errorf("runtimebundle: invalid asset SHA-256 for %q: %w", spec.Name, err)
	}
	return nil
}

type acquireLimitWriter struct {
	destination io.Writer
	remaining   int64
	exceeded    bool
}

func (w *acquireLimitWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if w.remaining <= 0 {
		w.exceeded = true
		return 0, errAcquireSizeLimit
	}
	writable := data
	limited := int64(len(data)) > w.remaining
	if limited {
		w.exceeded = true
		writable = data[:w.remaining]
	}
	n, err := w.destination.Write(writable)
	w.remaining -= int64(n)
	if err != nil {
		return n, err
	}
	if n != len(writable) {
		return n, io.ErrShortWrite
	}
	if limited {
		return n, errAcquireSizeLimit
	}
	return n, nil
}

func newAcquireRootTemp(root *os.Root) (string, *os.File, error) {
	var random [12]byte
	for attempt := 0; attempt < 32; attempt++ {
		if _, err := cryptorand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := ".asset-" + hex.EncodeToString(random[:])
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", nil, err
		}
		if runtimeIsUnix() {
			if err := file.Chmod(0o600); err != nil {
				_ = file.Close()
				_ = root.Remove(name)
				return "", nil, err
			}
		}
		if err := verifyPrivateRootPath(root, name); err != nil {
			_ = file.Close()
			_ = root.Remove(name)
			return "", nil, err
		}
		return name, file, nil
	}
	return "", nil, fmt.Errorf("runtimebundle: unable to allocate sibling temporary asset")
}

func verifyAcquireRootFile(root *os.Root, name string, expectedSize int64, expectedDigest string) error {
	file, err := openVerifiedAcquireRootFile(root, name, expectedSize, expectedDigest)
	if err != nil {
		return err
	}
	return file.Close()
}

func openVerifiedAcquireRootFile(root *os.Root, name string, expectedSize int64, expectedDigest string) (file *os.File, err error) {
	name = filepath.FromSlash(name)
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: acquired asset is not a regular non-symlink file", ErrUnsafeArchive)
	}
	file, err = root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%w: acquired asset changed while opening", ErrUnsafeArchive)
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(hasher, file)
	if copyErr != nil {
		return nil, copyErr
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || after.Size() != size {
		return nil, fmt.Errorf("%w: acquired asset changed while reading", ErrUnsafeArchive)
	}
	if err := verifyPrivateRootPath(root, name); err != nil {
		return nil, err
	}
	if expectedSize > 0 && size != expectedSize {
		return nil, fmt.Errorf("%w: size = %d, want %d", ErrDigestMismatch, size, expectedSize)
	}
	if expectedDigest != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedDigest {
			return nil, fmt.Errorf("%w: SHA-256 = %s, want %s", ErrDigestMismatch, got, expectedDigest)
		}
	}
	return file, nil
}

func replaceAcquireRootFile(root *os.Root, src, dst string) error {
	src = filepath.FromSlash(src)
	dst = filepath.FromSlash(dst)
	if info, err := root.Lstat(src); err != nil {
		return err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: source is not a regular non-symlink file: %s", ErrUnsafeArchive, src)
	}
	if info, err := root.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%w: destination is not a regular non-symlink file: %s", ErrUnsafeArchive, dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.Rename(src, dst); err != nil {
		return err
	}
	return verifyPrivateRootPath(root, dst)
}
