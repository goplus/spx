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

// FetchFunc writes one URL response to dst. Callers should make the function
// honor ctx; the acquisition layer also checks ctx before publishing a file.
type FetchFunc func(ctx context.Context, url string, dst io.Writer) error

// FetchSpec describes one release asset. ExpectedSize and ExpectedSHA256 are
// optional for metadata files, but both must be supplied for content assets.
// Name is a single cache-file component and is never interpreted as a path.
type FetchSpec struct {
	Name           string
	URL            string
	ExpectedSize   int64
	ExpectedSHA256 string
	MaxSize        int64
	Offline        bool
	Force          bool
	Fetch          FetchFunc
}

// AcquiredFile is a verified cache file together with a shared use lease.
// Reads use the file descriptor opened during verification, so callers do not
// need to resolve Path again. Close releases both the descriptor and lease.
type AcquiredFile struct {
	Path string

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

func (f *AcquiredFile) Seek(offset int64, whence int) (int64, error) {
	if f == nil || f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Seek(offset, whence)
}

func (f *AcquiredFile) Stat() (os.FileInfo, error) {
	if f == nil || f.file == nil {
		return nil, os.ErrInvalid
	}
	return f.file.Stat()
}

// Close is idempotent and safe to call from concurrent goroutines.
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

// Acquire downloads or reuses one file below root. Existing files are accepted
// only after an exact size and SHA-256 check when expectations are supplied.
// A successful download is written to a sibling temporary file and published
// with rename, so callers never observe a partial response. The returned path
// is private-cache material and must not be treated as a trusted release unless
// the caller supplied and checked its expected digest.
func Acquire(ctx context.Context, root string, spec FetchSpec) (string, error) {
	return acquirePath(ctx, root, spec)
}

// AcquireFile downloads or reuses one verified file, then reopens it while
// holding a shared use lease. Callers should prefer this API whenever the file
// will be read after acquisition and must Close the result after its final use.
func AcquireFile(ctx context.Context, root string, spec FetchSpec) (*AcquiredFile, error) {
	path, err := acquirePath(ctx, root, spec)
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
	file, err := openVerifiedAcquireRootFile(cacheRoot, spec.Name, spec.ExpectedSize, spec.ExpectedSHA256)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: verify acquired asset %q for use: %w", spec.Name, err)
	}
	if err := checkPinnedRootPath(root, cacheRoot); err != nil {
		_ = file.Close()
		return nil, err
	}
	closeLease = false
	return &AcquiredFile{Path: path, file: file, lease: lease}, nil
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
	if info, err := cacheRoot.Lstat(spec.Name); err == nil && !spec.Force {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("runtimebundle: cached asset %q is not a regular non-symlink file", path)
		}
		if err := verifyAcquireRootFile(cacheRoot, spec.Name, spec.ExpectedSize, spec.ExpectedSHA256); err == nil {
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
		return "", fmt.Errorf("runtimebundle: offline cache miss for %q (URL: %s)", spec.Name, spec.URL)
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
	var destination io.Writer = tmp
	if spec.MaxSize > 0 {
		destination = &acquireLimitWriter{destination: tmp, remaining: spec.MaxSize}
	}
	if err := spec.Fetch(ctx, spec.URL, destination); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("runtimebundle: fetch %q: %w", spec.Name, err)
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
	if err := verifyAcquireRootFile(cacheRoot, tmpName, spec.ExpectedSize, spec.ExpectedSHA256); err != nil {
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
	if err := verifyAcquireRootFile(cacheRoot, spec.Name, spec.ExpectedSize, spec.ExpectedSHA256); err != nil {
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
	if spec.ExpectedSize < 0 {
		return fmt.Errorf("runtimebundle: negative expected asset size for %q", spec.Name)
	}
	if spec.MaxSize < 0 {
		return fmt.Errorf("runtimebundle: negative acquired asset size limit for %q", spec.Name)
	}
	if spec.MaxSize > 0 && spec.ExpectedSize > spec.MaxSize {
		return fmt.Errorf("runtimebundle: expected size exceeds acquired asset size limit for %q", spec.Name)
	}
	if spec.ExpectedSHA256 != "" {
		if len(spec.ExpectedSHA256) != sha256.Size*2 {
			return fmt.Errorf("runtimebundle: invalid expected SHA-256 for %q", spec.Name)
		}
		decoded, err := hex.DecodeString(spec.ExpectedSHA256)
		if err != nil || len(decoded) != sha256.Size || spec.ExpectedSHA256 != strings.ToLower(spec.ExpectedSHA256) {
			return fmt.Errorf("runtimebundle: invalid expected SHA-256 for %q", spec.Name)
		}
		if spec.ExpectedSize <= 0 {
			return fmt.Errorf("runtimebundle: expected size is required with SHA-256 for %q", spec.Name)
		}
	}
	return nil
}

type acquireLimitWriter struct {
	destination io.Writer
	remaining   int64
}

func (w *acquireLimitWriter) Write(data []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errAcquireSizeLimit
	}
	writable := data
	limited := int64(len(data)) > w.remaining
	if limited {
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

func openVerifiedAcquireRootFile(root *os.Root, name string, expectedSize int64, expectedDigest string) (*os.File, error) {
	name = filepath.FromSlash(name)
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, errors.New("not a regular non-symlink file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, errors.New("file changed while opening")
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(hasher, file)
	if copyErr != nil {
		_ = file.Close()
		return nil, copyErr
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}
	after, err := root.Lstat(name)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || after.Size() != size {
		_ = file.Close()
		return nil, errors.New("file changed while reading")
	}
	if err := verifyPrivateRootPath(root, name); err != nil {
		_ = file.Close()
		return nil, err
	}
	if expectedSize > 0 && size != expectedSize {
		_ = file.Close()
		return nil, fmt.Errorf("size = %d, want %d", size, expectedSize)
	}
	if expectedDigest != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedDigest {
			_ = file.Close()
			return nil, fmt.Errorf("SHA-256 = %s, want %s", got, expectedDigest)
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
		return fmt.Errorf("source is not a regular non-symlink file: %s", src)
	}
	if info, err := root.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("destination is not a regular non-symlink file: %s", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.Rename(src, dst); err != nil {
		return err
	}
	return verifyPrivateRootPath(root, dst)
}
