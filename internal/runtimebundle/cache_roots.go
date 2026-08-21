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
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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
