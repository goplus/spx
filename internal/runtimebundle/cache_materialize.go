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
	"fmt"
	"os"
	"path/filepath"
)

// materializationRequest contains the archive identity prepared by Materialize.
// Publication still reopens and verifies the archive under the exclusive lease.
type materializationRequest struct {
	zipPath  string
	expected Bundle
	limits   Limits
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
	target, err := c.materializePath(ctx, materializationRequest{zipPath: zipPath, expected: *expected, limits: limits})
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

// materializePath verifies zipPath and atomically materializes it under the
// namespace-specific content address. The caller must acquire a shared lease
// before exposing the returned path to a consumer.
func (c *Cache) materializePath(ctx context.Context, request materializationRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c.Permissions == nil {
		c.Permissions = defaultPermissions()
	}
	zipPath, expected, limits := request.zipPath, &request.expected, request.limits
	namespace, digest := expected.Namespace, expected.Digest

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
		source.bundle, err = source.bundle.WithDigestWithLimits(limits)
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
	if err := writeMetadataRoot(tempRoot, source.bundle, limits); err != nil {
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
		identity, err := namespaceLess.IdentityDigestWithLimits(limits)
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
	digest, err := want.IdentityDigestWithLimits(limits)
	if err != nil {
		return nil, err
	}
	if originalNamespace != "" && originalDigest != "" && originalDigest != digest {
		return nil, fmt.Errorf("%w: expected manifest digest %s, identity %s", ErrDigestMismatch, originalDigest, digest)
	}
	want.Digest = digest
	return &want, nil
}
