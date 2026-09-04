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
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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

func writeMetadataRoot(root *os.Root, bundle Bundle, limits Limits) error {
	manifest, err := bundle.WithDigestWithLimits(limits)
	if err != nil {
		return err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("runtimebundle: encode cache manifest: %w", err)
	}
	if err := writeRootPrivateFile(root, cacheManifestName, data, false); err != nil {
		return fmt.Errorf("runtimebundle: write cache manifest: %w", err)
	}
	if err := writeRootPrivateFile(root, completeMarkerName, []byte(manifest.Digest+"\n"), false); err != nil {
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
	manifestData, err := readRootRegularFile(targetRoot, cacheManifestName)
	if err != nil {
		return false, nil
	}
	manifest, err := ParseManifestWithLimits(manifestData, c.Limits)
	if err != nil {
		return false, nil
	}
	if manifest.Namespace != Namespace(namespace) || manifest.Digest != digest {
		return false, nil
	}
	marker, err := readRootRegularFile(targetRoot, completeMarkerName)
	if err != nil || strings.TrimSpace(string(marker)) != digest {
		return false, nil
	}
	if expected != nil {
		if err := manifestEntriesEqualWithLimits(manifest, *expected, c.Limits); err != nil {
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
		if name == cacheManifestName || name == completeMarkerName {
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
