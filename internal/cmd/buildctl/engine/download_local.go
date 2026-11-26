/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package engine

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v4/internal/base/fileutil"
	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
	"github.com/goplus/spx/v4/internal/runtimebundle"
)

const maxRuntimeManifestBytes int64 = 1 << 20

func fetchEngineAsset(env engineDownloadEnv, name, dst string) error {
	var verify func(string) error
	if env.manifest != nil {
		verify = func(path string) error { return env.manifest.VerifyAsset(name, path) }
	}
	if env.assetDir != "" {
		src, err := findLocalEngineAsset(env.assetDir, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Installing local engine asset %s -> %s\n", src, dst)
		return copyEngineAssetAtomically(src, dst, verify)
	}

	// The HTTP downloader can report a response close error after writing its
	// output. Wait for it to finish before publishing even without a manifest.
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, ".verified-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "download")
	if err := fetchURLToFile(env.urlPrefix+name, path); err != nil {
		return err
	}
	if verify != nil {
		if err := verify(path); err != nil {
			return err
		}
	}
	return fileutil.ReplaceFile(path, dst)
}

func setLocalAssetDir(env *engineDownloadEnv, repoRoot, assetDir string, allowMissingManifest bool) error {
	env.assetDir = assetDir
	if !filepath.IsAbs(env.assetDir) {
		env.assetDir = filepath.Join(repoRoot, env.assetDir)
	}
	env.assetDir = filepath.Clean(env.assetDir)
	info, err := os.Stat(env.assetDir)
	if err != nil {
		return fmt.Errorf("open engine asset directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("engine asset path is not a directory: %s", env.assetDir)
	}
	env.allowMissingManifest = allowMissingManifest
	return nil
}

func loadEngineAssetManifest(env *engineDownloadEnv) error {
	lock, err := release.RuntimeLockForVersion(env.version)
	if err != nil {
		return fmt.Errorf("resolve runtime lock for %s: %w", env.version, err)
	}
	manifestPath := filepath.Join(env.cacheDir, lock.Manifest)
	if env.assetDir == "" {
		if err := fetchURLToFileWithLimit(env.urlPrefix+lock.Manifest, manifestPath, maxRuntimeManifestBytes); err != nil {
			var statusErr *shared.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
				return fmt.Errorf(
					"locked runtime %s is unavailable: %s returned %s\n"+
						"Published-asset setup requires a complete runtime release.\n"+
						"Publish the locked runtime, or build from source with %q",
					lock.RuntimeReleaseTag(), lock.Manifest, statusErr.Status, "make dev MODE=normal",
				)
			}
			return fmt.Errorf("download runtime manifest: %w", err)
		}
		defer os.Remove(manifestPath)
	} else {
		src, err := findLocalEngineAsset(env.assetDir, lock.Manifest)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && env.allowMissingManifest {
				fmt.Fprintf(os.Stdout, "Runtime manifest is not present in same-run artifacts; final release assembly will verify them.\n")
				return nil
			}
			return err
		}
		manifestPath = src
	}

	data, err := readRuntimeManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("read runtime manifest: %w", err)
	}
	manifest, err := release.ParseRuntimeManifestForRelease(data, lock.RuntimeVersion, lock.RequiredAssets)
	if err != nil {
		return err
	}
	env.manifest = &manifest
	return nil
}

func readRuntimeManifest(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime manifest is not a regular file: %s", path)
	}
	if info.Size() > maxRuntimeManifestBytes {
		return nil, fmt.Errorf("%w: runtime manifest size %d exceeds limit %d", runtimebundle.ErrArchiveLimit, info.Size(), maxRuntimeManifestBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRuntimeManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxRuntimeManifestBytes {
		return nil, fmt.Errorf("%w: runtime manifest exceeds limit %d", runtimebundle.ErrArchiveLimit, maxRuntimeManifestBytes)
	}
	return data, nil
}

func findLocalEngineAsset(assetDir, name string) (string, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid engine asset name: %q", name)
	}

	direct := filepath.Join(assetDir, name)
	if info, err := os.Stat(direct); err == nil && info.Mode().IsRegular() {
		return direct, nil
	}

	var matches []string
	err := filepath.WalkDir(assetDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && entry.Name() == name {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan engine asset directory: %w", err)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("engine asset %q not found under %s: %w", name, assetDir, fs.ErrNotExist)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("engine asset %q is ambiguous under %s: %v", name, assetDir, matches)
	}
}

func copyEngineAssetAtomically(src, dst string, verify func(string) error) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("engine asset is not a regular file: %s", src)
	}

	return fileutil.WriteFileAtomic(dst, func(output *os.File) error {
		if _, err := io.Copy(output, input); err != nil {
			return err
		}
		return output.Chmod(info.Mode().Perm())
	}, verify)
}

func linkOrCopyFile(src, dst string) error {
	if filepath.Clean(src) == filepath.Clean(dst) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := removeIfExists(dst); err != nil {
		return err
	}
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return fileutil.CopyFile(src, dst)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	} else {
		return err
	}
}
