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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
