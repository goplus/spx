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
	"fmt"
	"os"
	"path/filepath"
)

// CacheOptions controls a Cache. Zero Limits selects the verifier defaults. A custom
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
		Limits:       Limits{},
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
