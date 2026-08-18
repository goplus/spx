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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	// CompleteMarkerName and CacheManifestName are metadata files owned by the
	// cache. Archives cannot use these names; this prevents an untrusted ZIP
	// entry from replacing the publication marker or cache manifest.
	CompleteMarkerName = ".runtimebundle.complete"
	CacheManifestName  = ".runtimebundle.manifest.json"
)

// CacheOptions controls a Cache. Zero Limits selects DefaultLimits. A custom
// LockProvider or Permissions implementation is useful for tests and for
// platforms with an explicitly verified integration.
type CacheOptions struct {
	Limits       Limits
	LockProvider LockProvider
	Permissions  Permissions
}

// Cache is a content-addressed runtime bundle cache. It creates
// <root>/<namespace>/<full-sha256> directories and never uses a shortened
// digest. NewCache uses an OS-backed cross-process provider; callers that
// explicitly accept process-local coordination must use NewProcessLocalCache.
type Cache struct {
	Root         string
	Limits       Limits
	LockProvider LockProvider
	Permissions  Permissions
}

// NewCache constructs a cache using OS-backed cross-process locks.
func NewCache(root string) *Cache {
	return newCache(root, CrossProcessLockProvider{})
}

// NewProcessLocalCache constructs a cache using the explicitly opt-in,
// process-local lock and platform permissions. It is suitable for tests and
// adapters that can prove they never share a cache root across processes.
func NewProcessLocalCache(root string) *Cache {
	return newCache(root, ProcessLockProvider{})
}

func newCache(root string, lockProvider LockProvider) *Cache {
	return &Cache{
		Root:         filepath.Clean(root),
		Limits:       DefaultLimits,
		LockProvider: lockProvider,
		Permissions:  defaultPermissions(),
	}
}

// NewCacheWithOptions validates options and constructs a cache.
func NewCacheWithOptions(root string, options CacheOptions) (*Cache, error) {
	if root == "" {
		return nil, fmt.Errorf("runtimebundle: cache root is empty")
	}
	limits, err := options.Limits.withDefaults()
	if err != nil {
		return nil, err
	}
	if options.LockProvider == nil {
		options.LockProvider = CrossProcessLockProvider{}
	}
	if options.Permissions == nil {
		options.Permissions = defaultPermissions()
	}
	return &Cache{
		Root:         filepath.Clean(root),
		Limits:       limits,
		LockProvider: options.LockProvider,
		Permissions:  options.Permissions,
	}, nil
}

// DefaultCacheRoot returns the user cache namespace used by runtimebundle
// when an embedding application does not choose an explicit root.
func DefaultCacheRoot() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "spx", "runtimebundle")
}

// Path returns the on-disk location for a namespace/digest pair. It does not
// create anything and rejects invalid identities.
func (c *Cache) Path(namespace Namespace, digest string) (string, error) {
	if c == nil || c.Root == "" {
		return "", fmt.Errorf("runtimebundle: nil or empty cache root")
	}
	if !filepath.IsAbs(c.Root) || filepath.Clean(c.Root) != c.Root {
		return "", fmt.Errorf("runtimebundle: cache root must be absolute and clean: %q", c.Root)
	}
	if err := validateCacheRoot(c.Root); err != nil {
		return "", err
	}
	if !namespace.valid() {
		return "", fmt.Errorf("runtimebundle: invalid cache namespace %q", namespace)
	}
	if err := validateSHA256(digest); err != nil {
		return "", fmt.Errorf("runtimebundle: invalid cache digest: %v", err)
	}
	return filepath.Join(c.Root, string(namespace), digest), nil
}

// materializePath verifies zipPath and atomically materializes it under the
// namespace-specific content address. The caller must acquire a shared lease
// before exposing the returned path to a consumer.
func (c *Cache) materializePath(ctx context.Context, namespace Namespace, zipPath string, expected *Bundle) (string, error) {
	if c == nil {
		return "", fmt.Errorf("runtimebundle: nil cache")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	limits, err := c.Limits.withDefaults()
	if err != nil {
		return "", err
	}
	if !namespace.valid() {
		return "", fmt.Errorf("runtimebundle: invalid cache namespace %q", namespace)
	}
	if c.LockProvider == nil {
		return "", ErrCrossProcessLockUnsupported
	}
	if c.Permissions == nil {
		c.Permissions = defaultPermissions()
	}
	expected, err = normalizeExpectedBundle(namespace, expected, limits)
	if err != nil {
		return "", err
	}

	var digest string
	if expected != nil {
		digest = expected.Digest
	}
	// A nil expected has no address until the archive has been read. It is
	// therefore intentionally verified before lock acquisition; callers that
	// already possess a manifest get the cheap cache-hit path first.
	if digest == "" {
		bundle, verifyErr := VerifyZip(zipPath, VerifyOptions{Limits: limits})
		if verifyErr != nil {
			return "", verifyErr
		}
		bundle.Namespace = namespace
		bundle, err = bundle.WithDigest()
		if err != nil {
			return "", err
		}
		digest = bundle.Digest
		expected = &bundle
	}

	target, err := c.Path(namespace, digest)
	if err != nil {
		return "", err
	}
	if err := c.prepareCacheNamespace(namespace); err != nil {
		return "", err
	}
	lease, err := c.LockProvider.AcquireExclusive(ctx, target)
	if err != nil {
		return "", err
	}
	if lease == nil {
		return "", fmt.Errorf("runtimebundle: lock provider returned a nil exclusive lease")
	}
	defer lease.Close()

	cacheRoot, err := openOrCreateCacheRoot(c.Root, c.Permissions)
	if err != nil {
		return "", fmt.Errorf("runtimebundle: pin cache root: %w", err)
	}
	defer cacheRoot.Close()
	if err := ensureRootDir(cacheRoot, filepath.FromSlash(string(namespace))); err != nil {
		return "", fmt.Errorf("runtimebundle: prepare cache namespace: %w", err)
	}
	namespaceRoot, err := openPinnedChildRoot(cacheRoot, string(namespace))
	if err != nil {
		return "", fmt.Errorf("runtimebundle: pin cache namespace: %w", err)
	}
	defer namespaceRoot.Close()
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return "", err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return "", err
	}

	if ok, hitErr := c.validCacheHitRoot(namespaceRoot, string(namespace), digest, expected); hitErr != nil {
		return "", hitErr
	} else if ok {
		if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
			return "", err
		}
		if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
			return "", err
		}
		return target, nil
	}

	// A corrupt/incomplete target is never reused. It is removed through the
	// pinned namespace root, never by resolving the namespace path again.
	if info, statErr := namespaceRoot.Lstat(filepath.FromSlash(digest)); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: refusing to remove symlink cache target %s", ErrUnsafeArchive, target)
		}
		if err := namespaceRoot.RemoveAll(filepath.FromSlash(digest)); err != nil {
			return "", fmt.Errorf("runtimebundle: remove corrupt cache %s: %w", target, err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}

	tempName, err := newRootTemp(namespaceRoot)
	if err != nil {
		return "", fmt.Errorf("runtimebundle: create sibling temp directory: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = namespaceRoot.RemoveAll(filepath.FromSlash(tempName))
		}
	}()
	tempRoot, err := openPinnedChildRoot(namespaceRoot, tempName)
	if err != nil {
		return "", fmt.Errorf("runtimebundle: pin sibling temp directory: %w", err)
	}
	tempOpen := true
	defer func() {
		if tempOpen {
			_ = tempRoot.Close()
		}
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Re-open and verify the source after the cache lock is held. This keeps
	// the source/hash/extraction transaction coherent if a caller replaces a
	// downloaded ZIP between the initial identity check and publication.
	source, closeSource, err := openAndVerify(zipPath, VerifyOptions{Limits: limits, Expected: expected})
	if err != nil {
		return "", err
	}
	defer closeSource.Close()
	if source.bundle.Namespace != namespace {
		source.bundle.Namespace = namespace
		source.bundle, err = source.bundle.WithDigest()
		if err != nil {
			return "", err
		}
	}
	if source.bundle.Digest != digest {
		return "", fmt.Errorf("%w: source %s, cache target %s", ErrDigestMismatch, source.bundle.Digest, digest)
	}
	if err := extractVerifiedRoot(source, tempRoot); err != nil {
		return "", fmt.Errorf("runtimebundle: materialize %s: %w", zipPath, err)
	}
	if err := writeMetadataRoot(tempRoot, source.bundle); err != nil {
		return "", err
	}
	if err := syncRootDir(tempRoot); err != nil {
		return "", err
	}
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return "", err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return "", err
	}
	if err := checkPinnedChildPath(namespaceRoot, tempName, tempRoot); err != nil {
		return "", err
	}
	if err := tempRoot.Close(); err != nil {
		return "", err
	}
	tempOpen = false
	if err := namespaceRoot.Rename(filepath.FromSlash(tempName), filepath.FromSlash(digest)); err != nil {
		// Revalidate a competing publisher's winner; never accept an
		// unverified target merely because rename lost.
		if ok, hitErr := c.validCacheHitRoot(namespaceRoot, string(namespace), digest, expected); hitErr == nil && ok {
			return target, nil
		}
		return "", fmt.Errorf("runtimebundle: atomically publish cache %s: %w", target, err)
	}
	published = true
	if err := syncRootDir(namespaceRoot); err != nil {
		return "", err
	}
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return "", err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return "", err
	}
	return target, nil
}

// Materialize verifies and publishes a bundle, then returns while holding a
// shared use lease for the materialized target. The caller must Close the
// returned value after its final read or execution.
func (c *Cache) Materialize(ctx context.Context, namespace Namespace, zipPath string, expected *Bundle) (*Materialized, error) {
	if c == nil {
		return nil, fmt.Errorf("runtimebundle: nil cache")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	limits, err := c.Limits.withDefaults()
	if err != nil {
		return nil, err
	}
	if expected == nil {
		bundle, err := VerifyZip(zipPath, VerifyOptions{Limits: limits})
		if err != nil {
			return nil, err
		}
		expected = &bundle
	}
	expected, err = normalizeExpectedBundle(namespace, expected, limits)
	if err != nil {
		return nil, err
	}
	if hit, ok, err := c.tryMaterializedHit(ctx, namespace, expected.Digest, expected); err != nil {
		return nil, err
	} else if ok {
		return hit, nil
	}
	target, err := c.materializePath(ctx, namespace, zipPath, expected)
	if err != nil {
		return nil, err
	}
	// Publication releases its exclusive lease before a shared use lease is
	// acquired. Revalidate under the shared lease so the returned path is never
	// exposed through an unprotected lock-transition window.
	hit, ok, err := c.tryMaterializedHit(ctx, namespace, expected.Digest, expected)
	if err != nil {
		return nil, err
	}
	if !ok || hit.Path != target {
		return nil, fmt.Errorf("runtimebundle: published cache target failed shared-lease revalidation: %s", target)
	}
	return hit, nil
}

func normalizeExpectedBundle(namespace Namespace, expected *Bundle, limits Limits) (*Bundle, error) {
	if expected == nil {
		return nil, nil
	}
	want := *expected
	originalNamespace := want.Namespace
	originalDigest := want.Digest
	if originalNamespace != "" && originalNamespace != namespace {
		return nil, fmt.Errorf("runtimebundle: expected namespace %q does not match %q", want.Namespace, namespace)
	}
	if originalNamespace == "" && originalDigest != "" {
		// An empty namespace identifies the archive manifest itself. Verify that
		// identity before deriving the namespace-specific cache identity.
		namespaceLess := want
		namespaceLess.Digest = ""
		identity, err := namespaceLess.IdentityDigest()
		if err != nil {
			return nil, err
		}
		if identity != originalDigest {
			return nil, fmt.Errorf("%w: namespace-empty expected digest %s, identity %s", ErrDigestMismatch, originalDigest, identity)
		}
	}
	want.Namespace = namespace
	want.Digest = ""
	if err := want.ValidateWithLimits(limits); err != nil {
		return nil, err
	}
	digest, err := want.IdentityDigest()
	if err != nil {
		return nil, err
	}
	if originalNamespace != "" && originalDigest != "" && originalDigest != digest {
		return nil, fmt.Errorf("%w: expected manifest digest %s, identity %s", ErrDigestMismatch, originalDigest, digest)
	}
	want.Digest = digest
	return &want, nil
}

func (c *Cache) tryMaterializedHit(ctx context.Context, namespace Namespace, digest string, expected *Bundle) (*Materialized, bool, error) {
	if c.LockProvider == nil {
		return nil, false, ErrCrossProcessLockUnsupported
	}
	target, err := c.Path(namespace, digest)
	if err != nil {
		return nil, false, err
	}
	if err := c.prepareCacheNamespace(namespace); err != nil {
		return nil, false, err
	}
	lease, err := c.LockProvider.AcquireShared(ctx, target)
	if err != nil {
		return nil, false, err
	}
	if lease == nil {
		return nil, false, fmt.Errorf("runtimebundle: lock provider returned a nil shared lease")
	}
	closeLease := true
	defer func() {
		if closeLease {
			_ = lease.Close()
		}
	}()
	cacheRoot, err := openOrCreateCacheRoot(c.Root, c.Permissions)
	if err != nil {
		return nil, false, fmt.Errorf("runtimebundle: pin cache root: %w", err)
	}
	defer cacheRoot.Close()
	namespaceRoot, err := openPinnedChildRoot(cacheRoot, string(namespace))
	if err != nil {
		return nil, false, fmt.Errorf("runtimebundle: pin cache namespace: %w", err)
	}
	defer namespaceRoot.Close()
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return nil, false, err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return nil, false, err
	}
	ok, err := c.validCacheHitRoot(namespaceRoot, string(namespace), digest, expected)
	if err != nil || !ok {
		return nil, false, err
	}
	closeLease = false
	return &Materialized{Path: target, lease: lease}, true, nil
}

// MaterializeZip is a compatibility adapter for explicit process-local test
// callers. Production callers must use Materialize so the shared lease stays
// alive for the duration of use.
func (c *Cache) MaterializeZip(ctx context.Context, namespace Namespace, zipPath string, expected *Bundle) (string, error) {
	if c == nil {
		return "", fmt.Errorf("runtimebundle: nil cache")
	}
	switch c.LockProvider.(type) {
	case ProcessLockProvider, *ProcessLockProvider:
	default:
		return "", fmt.Errorf("runtimebundle: MaterializeZip compatibility requires ProcessLockProvider")
	}
	materialized, err := c.Materialize(ctx, namespace, zipPath, expected)
	if err != nil {
		return "", err
	}
	path := materialized.Path
	if err := materialized.Close(); err != nil {
		return "", err
	}
	return path, nil
}

// prepareCacheNamespace creates the lock-file parent through the same pinned
// root path used by publication. It runs before lock acquisition because the
// sidecar lives beside the target, not inside it.
func (c *Cache) prepareCacheNamespace(namespace Namespace) error {
	if c.Permissions == nil {
		c.Permissions = defaultPermissions()
	}
	cacheRoot, err := openOrCreateCacheRoot(c.Root, c.Permissions)
	if err != nil {
		return fmt.Errorf("runtimebundle: pin cache root for lock: %w", err)
	}
	defer cacheRoot.Close()
	if runtimeIsWindows() {
		if err := c.Permissions.EnsureDir(filepath.Join(c.Root, string(namespace))); err != nil {
			return fmt.Errorf("runtimebundle: protect cache namespace: %w", err)
		}
	}
	if err := ensureRootDir(cacheRoot, filepath.FromSlash(string(namespace))); err != nil {
		return fmt.Errorf("runtimebundle: prepare cache namespace for lock: %w", err)
	}
	namespaceRoot, err := openPinnedChildRoot(cacheRoot, string(namespace))
	if err != nil {
		return fmt.Errorf("runtimebundle: pin cache namespace for lock: %w", err)
	}
	defer namespaceRoot.Close()
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return err
	}
	return checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot)
}

// openOrCreateCacheRoot creates missing cache-root components beneath the
// nearest pinned existing ancestor. This avoids MkdirAll/Chmod path traversal
// races on POSIX. Windows uses the creation-time DACL primitive in
// permissions_windows.go.
func openOrCreateCacheRoot(path string, permissions Permissions) (*os.Root, error) {
	if runtime.GOOS == "windows" {
		if err := permissions.EnsureDir(path); err != nil {
			return nil, err
		}
		return openPinnedRoot(path)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("%w: cache root is not a real directory: %s", ErrUnsafeArchive, path)
		}
		root, err := openPinnedRoot(path)
		if err != nil {
			return nil, err
		}
		if err := root.Chmod(".", 0o700); err != nil {
			_ = root.Close()
			return nil, err
		}
		return root, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	currentPath := path
	missing := make([]string, 0, 4)
	for {
		info, err := os.Lstat(currentPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return nil, fmt.Errorf("%w: cache ancestor is not a real directory: %s", ErrUnsafeArchive, currentPath)
			}
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		parentPath := filepath.Dir(currentPath)
		base := filepath.Base(currentPath)
		if parentPath == currentPath || base == "" || base == "." || base == string(filepath.Separator) {
			return nil, fmt.Errorf("%w: cache root has no existing directory ancestor", ErrUnsafeArchive)
		}
		missing = append(missing, base)
		currentPath = parentPath
	}
	root, err := openPinnedRoot(currentPath)
	if err != nil {
		return nil, fmt.Errorf("%w: pin existing cache ancestor: %v", ErrUnsafeArchive, err)
	}
	for index := len(missing) - 1; index >= 0; index-- {
		name := missing[index]
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			_ = root.Close()
			return nil, err
		}
		child, err := openPinnedChildRoot(root, name)
		if err != nil {
			_ = root.Close()
			return nil, err
		}
		if err := child.Chmod(".", 0o700); err != nil {
			_ = child.Close()
			_ = root.Close()
			return nil, err
		}
		currentPath = filepath.Join(currentPath, name)
		if err := checkPinnedRootPath(currentPath, child); err != nil {
			_ = child.Close()
			_ = root.Close()
			return nil, err
		}
		if err := root.Close(); err != nil {
			_ = child.Close()
			return nil, err
		}
		root = child
	}
	return root, nil
}

// Materialize is the package-level convenience wrapper using the default
// cross-process cache provider.
func Materialize(ctx context.Context, cacheRoot string, namespace Namespace, zipPath string, expected *Bundle) (*Materialized, error) {
	return NewCache(cacheRoot).Materialize(ctx, namespace, zipPath, expected)
}

// MaterializeZip is retained only for old process-local test adapters.
func MaterializeZip(ctx context.Context, cacheRoot string, namespace Namespace, zipPath string, expected *Bundle) (string, error) {
	return NewCache(cacheRoot).MaterializeZip(ctx, namespace, zipPath, expected)
}

func openAndVerify(zipPath string, options VerifyOptions) (verifiedArchive, io.Closer, error) {
	file, err := openSourceZip(zipPath)
	if err != nil {
		return verifiedArchive{}, nil, fmt.Errorf("runtimebundle: open ZIP %s: %w", zipPath, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return verifiedArchive{}, nil, fmt.Errorf("runtimebundle: stat ZIP %s: %w", zipPath, err)
	}
	archive, err := verifyReaderAt(file, info.Size(), options)
	if err != nil {
		_ = file.Close()
		return verifiedArchive{}, nil, err
	}
	return archive, file, nil
}

func openPinnedChildRoot(parent *os.Root, name string) (*os.Root, error) {
	name = filepath.FromSlash(name)
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("%w: cache component is not a real directory: %s", ErrUnsafeArchive, name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = child.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: cache component changed while opening: %s", ErrUnsafeArchive, name)
	}
	if err := checkPinnedChildPath(parent, name, child); err != nil {
		_ = child.Close()
		return nil, err
	}
	return child, nil
}

// checkPinnedChildPath verifies that a child root still occupies the same
// pathname beneath its pinned parent. This is the post-open half of the
// identity check; without it a concurrent rename could publish into an old
// directory while the caller observes a replacement at the namespace path.
func checkPinnedChildPath(parent *os.Root, name string, child *os.Root) error {
	info, err := parent.Lstat(name)
	if err != nil {
		return fmt.Errorf("%w: cache component pathname changed: %v", ErrUnsafeArchive, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: cache component pathname is not a real directory: %s", ErrUnsafeArchive, name)
	}
	pinned, err := child.Stat(".")
	if err != nil {
		return fmt.Errorf("%w: stat pinned cache component: %v", ErrUnsafeArchive, err)
	}
	if !os.SameFile(info, pinned) {
		return fmt.Errorf("%w: cache component pathname was replaced: %s", ErrUnsafeArchive, name)
	}
	return nil
}

func newRootTemp(root *os.Root) (string, error) {
	var random [12]byte
	for attempt := 0; attempt < 32; attempt++ {
		if _, err := cryptorand.Read(random[:]); err != nil {
			return "", err
		}
		name := ".runtimebundle-" + fmt.Sprintf("%x", random[:])
		if err := mkdirPrivateRootChild(root, name); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("runtimebundle: unable to allocate sibling temp directory")
}

func writeRootPrivateFile(root *os.Root, name string, data []byte, executable bool) error {
	file, err := root.OpenFile(filepath.FromSlash(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode(func() uint32 {
		if executable {
			return 0o700
		}
		return 0o600
	}()))
	if err != nil {
		return err
	}
	closeErr := func() error {
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			_ = root.Remove(filepath.FromSlash(name))
			return err
		}
		if runtimeIsUnix() {
			if err := file.Chmod(privateFileMode(func() uint32 {
				if executable {
					return 0o700
				}
				return 0o600
			}())); err != nil {
				_ = file.Close()
				_ = root.Remove(filepath.FromSlash(name))
				return err
			}
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			_ = root.Remove(filepath.FromSlash(name))
			return err
		}
		return file.Close()
	}()
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func writeMetadataRoot(root *os.Root, bundle Bundle) error {
	manifest, err := bundle.WithDigest()
	if err != nil {
		return err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("runtimebundle: encode cache manifest: %w", err)
	}
	if err := writeRootPrivateFile(root, CacheManifestName, data, false); err != nil {
		return fmt.Errorf("runtimebundle: write cache manifest: %w", err)
	}
	if err := writeRootPrivateFile(root, CompleteMarkerName, []byte(manifest.Digest+"\n"), false); err != nil {
		return fmt.Errorf("runtimebundle: write cache complete marker: %w", err)
	}
	return nil
}

func syncRootDir(root *os.Root) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func readRootRegularFile(root *os.Root, name string) ([]byte, error) {
	name = filepath.FromSlash(name)
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: metadata path is not a regular file: %s", ErrUnsafeArchive, name)
	}
	if runtimeIsUnix() && info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("%w: metadata path is not private: %s", ErrUnsafeArchive, name)
	}
	if err := verifyPrivateRootPath(root, name); err != nil {
		return nil, err
	}
	return root.ReadFile(name)
}

func (c *Cache) validCacheHitRoot(namespaceRoot *os.Root, namespace, digest string, expected *Bundle) (bool, error) {
	info, err := namespaceRoot.Lstat(filepath.FromSlash(digest))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%w: cache target is a symlink: %s", ErrUnsafeArchive, digest)
	}
	if !info.IsDir() {
		return false, nil
	}
	if runtimeIsUnix() && info.Mode().Perm() != 0o700 {
		return false, nil
	}
	targetRoot, err := openPinnedChildRoot(namespaceRoot, digest)
	if err != nil {
		return false, err
	}
	defer targetRoot.Close()
	if err := verifyPrivateRootPath(targetRoot, "."); err != nil {
		return false, err
	}
	manifestData, err := readRootRegularFile(targetRoot, CacheManifestName)
	if err != nil {
		return false, nil
	}
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return false, nil
	}
	if manifest.Namespace != Namespace(namespace) || manifest.Digest != digest {
		return false, nil
	}
	marker, err := readRootRegularFile(targetRoot, CompleteMarkerName)
	if err != nil || strings.TrimSpace(string(marker)) != digest {
		return false, nil
	}
	if expected != nil {
		if err := manifestEntriesEqual(manifest, *expected); err != nil {
			return false, nil
		}
	}
	if err := verifyMaterializedTreeRoot(targetRoot, manifest); err != nil {
		return false, nil
	}
	if err := checkPinnedChildPath(namespaceRoot, digest, targetRoot); err != nil {
		return false, err
	}
	return true, nil
}

func verifyMaterializedTreeRoot(root *os.Root, manifest Bundle) error {
	expected := make(map[string]Entry, len(manifest.Entries))
	allowedDirs := map[string]struct{}{".": {}}
	for _, original := range manifest.Entries {
		entry, _, err := original.normalized()
		if err != nil {
			return err
		}
		key := strings.TrimSuffix(entry.Name, "/")
		expected[key] = entry
		if entry.isDir() {
			allowedDirs[key] = struct{}{}
		}
		parts := strings.Split(key, "/")
		for i := 1; i < len(parts); i++ {
			allowedDirs[strings.Join(parts[:i], "/")] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(expected))
	err := fs.WalkDir(root.FS(), ".", func(name string, dirent fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if name == CacheManifestName || name == CompleteMarkerName {
			if dirent.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if dirent.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: cache contains symlink %s", ErrUnsafeArchive, name)
		}
		key := filepath.ToSlash(name)
		if dirent.IsDir() {
			if _, ok := allowedDirs[key]; !ok {
				return fmt.Errorf("%w: cache contains unexpected directory %s", ErrDigestMismatch, name)
			}
			if entry, ok := expected[key]; ok && !entry.isDir() {
				return fmt.Errorf("%w: cache file/directory collision at %s", ErrDigestMismatch, name)
			}
			if runtimeIsUnix() {
				info, err := root.Lstat(filepath.FromSlash(key))
				if err != nil {
					return err
				}
				if info.Mode().Perm() != 0o700 {
					return fmt.Errorf("%w: cache directory %s is not private", ErrDigestMismatch, name)
				}
			}
			if err := verifyPrivateRootPath(root, key); err != nil {
				return err
			}
			if _, explicit := expected[key]; explicit {
				seen[key] = struct{}{}
			}
			return nil
		}
		entry, ok := expected[key]
		if !ok || entry.isDir() {
			return fmt.Errorf("%w: cache contains unexpected file %s", ErrDigestMismatch, name)
		}
		file, err := root.Open(filepath.FromSlash(key))
		if err != nil {
			return err
		}
		info, statErr := file.Stat()
		if statErr == nil && (!info.Mode().IsRegular() || info.Size() != entry.Size) {
			statErr = fmt.Errorf("%w: cache entry %s size/type mismatch", ErrDigestMismatch, name)
		}
		if statErr == nil && runtimeIsUnix() && info.Mode().Perm() != privateFileMode(entry.Mode).Perm() {
			statErr = fmt.Errorf("%w: cache entry %s is not private", ErrDigestMismatch, name)
		}
		if statErr == nil {
			statErr = verifyPrivateRootPath(root, key)
		}
		var digest string
		if statErr == nil {
			digest, _, statErr = hashReader(file, entry.Size)
		}
		closeErr := file.Close()
		if statErr == nil {
			statErr = closeErr
		}
		if statErr != nil {
			return statErr
		}
		if digest != entry.SHA256 {
			return fmt.Errorf("%w: cache entry %s digest mismatch", ErrDigestMismatch, name)
		}
		seen[key] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("%w: cache is missing entries", ErrDigestMismatch)
	}
	return nil
}

func hashReader(reader io.Reader, maxSize int64) (string, int64, error) {
	hasher := sha256.New()
	count, err := io.Copy(hasher, io.LimitReader(reader, limitWithOverflow(maxSize)))
	if err != nil {
		return "", count, err
	}
	if count > maxSize {
		return "", count, fmt.Errorf("%w: file exceeds expected size %d", ErrDigestMismatch, maxSize)
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), count, nil
}

// Remove removes a cache target only when it is absent or fails complete
// verification. It refuses to remove a valid target; callers needing quota or
// age-based deletion must wait for the lease-aware GC implementation.
func (c *Cache) Remove(namespace Namespace, digest string) error {
	if c == nil {
		return fmt.Errorf("runtimebundle: nil cache")
	}
	if c.LockProvider == nil {
		return ErrCrossProcessLockUnsupported
	}
	if c.Permissions == nil {
		c.Permissions = defaultPermissions()
	}
	target, err := c.Path(namespace, digest)
	if err != nil {
		return err
	}
	namespacePath := filepath.Join(c.Root, string(namespace))
	if err := validateCacheComponent(namespacePath); err != nil {
		return err
	}
	if _, err := os.Stat(namespacePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := c.prepareCacheNamespace(namespace); err != nil {
		return err
	}
	lease, err := c.LockProvider.AcquireExclusive(context.Background(), target)
	if err != nil {
		return err
	}
	if lease == nil {
		return fmt.Errorf("runtimebundle: lock provider returned a nil exclusive lease")
	}
	defer lease.Close()
	cacheRoot, err := openPinnedRoot(c.Root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("runtimebundle: pin cache root: %w", err)
	}
	defer cacheRoot.Close()
	namespaceRoot, err := openPinnedChildRoot(cacheRoot, string(namespace))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("runtimebundle: pin cache namespace: %w", err)
	}
	defer namespaceRoot.Close()
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return err
	}
	name := filepath.FromSlash(digest)
	if info, statErr := namespaceRoot.Lstat(name); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing to remove symlink cache target %s", ErrUnsafeArchive, target)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	if ok, verifyErr := c.validCacheHitRoot(namespaceRoot, string(namespace), digest, nil); verifyErr == nil && ok {
		return fmt.Errorf("runtimebundle: refusing to remove valid cache target %s", target)
	}
	// RemoveAll is scoped to the already pinned namespace root. The root and
	// namespace path checks above are deliberately repeated at this point so a
	// caller cannot turn this destructive operation into a path-swap escape.
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return err
	}
	if err := namespaceRoot.RemoveAll(name); err != nil {
		return err
	}
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return err
	}
	return checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot)
}

// CacheStats is reserved for the future quota/age collector. It is provided
// as a stable shape so callers do not need to inspect cache internals.
type CacheStats struct {
	Bytes        int64
	Entries      int
	Oldest       time.Time
	Namespaces   map[Namespace]int
	CompleteOnly bool
}

// List returns only complete, digest-addressed cache directories. Every
// candidate is revalidated with the same marker, manifest, tree and digest
// checks used by a cache hit; incomplete/corrupt candidates are skipped. It
// does not recursively repair or delete entries.
func (c *Cache) List(namespace Namespace) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("runtimebundle: nil cache")
	}
	if c.LockProvider == nil {
		return nil, ErrCrossProcessLockUnsupported
	}
	if c.Permissions == nil {
		c.Permissions = defaultPermissions()
	}
	if err := validateCacheRoot(c.Root); err != nil {
		return nil, err
	}
	if !namespace.valid() {
		return nil, fmt.Errorf("runtimebundle: invalid cache namespace %q", namespace)
	}
	cacheRoot, err := openPinnedRoot(c.Root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer cacheRoot.Close()
	if err := verifyPrivateRootPath(cacheRoot, "."); err != nil {
		return nil, err
	}
	namespaceRoot, err := openPinnedChildRoot(cacheRoot, string(namespace))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer namespaceRoot.Close()
	if err := verifyPrivateRootPath(namespaceRoot, "."); err != nil {
		return nil, err
	}
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return nil, err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return nil, err
	}
	dir := filepath.Join(c.Root, string(namespace))
	items, err := fs.ReadDir(namespaceRoot.FS(), ".")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, item := range items {
		if !item.IsDir() || strings.HasPrefix(item.Name(), ".") {
			continue
		}
		if err := validateSHA256(item.Name()); err != nil {
			continue
		}
		target := filepath.Join(dir, item.Name())
		lease, err := c.LockProvider.AcquireShared(context.Background(), target)
		if err != nil {
			return nil, err
		}
		if lease == nil {
			return nil, fmt.Errorf("runtimebundle: lock provider returned a nil shared lease")
		}
		ok, verifyErr := c.validCacheHitRoot(namespaceRoot, string(namespace), item.Name(), nil)
		closeErr := lease.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		if verifyErr != nil || !ok {
			continue
		}
		result = append(result, target)
	}
	if err := checkPinnedRootPath(c.Root, cacheRoot); err != nil {
		return nil, err
	}
	if err := checkPinnedChildPath(cacheRoot, string(namespace), namespaceRoot); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

// validateCacheRoot rejects relative/unclean roots and an existing root
// symlink. Ancestors are intentionally not rejected: common system temp/cache
// paths such as /tmp may themselves be symlinks, while the cache root and its
// namespace are private objects controlled by this package.
func validateCacheRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return fmt.Errorf("runtimebundle: cache root must be absolute and clean: %q", root)
	}
	if isFilesystemRoot(root) {
		return fmt.Errorf("%w: cache root must not be a filesystem root", ErrUnsafeArchive)
	}
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: cache root is not a real directory: %s", ErrUnsafeArchive, root)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isFilesystemRoot(path string) bool {
	clean := filepath.Clean(path)
	if volume := filepath.VolumeName(clean); volume != "" {
		return clean == volume+string(filepath.Separator)
	}
	return clean == string(filepath.Separator)
}

func validateCacheComponent(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: cache namespace is not a real directory: %s", ErrUnsafeArchive, path)
		}
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
