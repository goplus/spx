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
	"syscall"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

func fetchEngineAsset(env engineDownloadEnv, name, url, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".verified-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if env.assetDir == "" {
		if err := engineDownloadFetcher(url, tmpPath); err != nil {
			return err
		}
	} else {
		src, err := findLocalEngineAsset(env.assetDir, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Installing local engine asset %s -> %s\n", src, dst)
		if err := copyEngineAssetAtomically(src, tmpPath); err != nil {
			return err
		}
	}

	if env.manifest != nil {
		if err := env.manifest.VerifyAsset(name, tmpPath); err != nil {
			return err
		}
	}
	return replaceDownloadedFile(tmpPath, dst)
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
		if err := engineDownloadFetcher(env.urlPrefix+lock.Manifest, manifestPath); err != nil {
			var statusErr *downloadHTTPStatusError
			if errors.As(err, &statusErr) && statusErr.statusCode == http.StatusNotFound {
				return fmt.Errorf(
					"locked runtime %s is unavailable: %s returned %s\n"+
						"Published-asset setup requires a complete runtime release.\n"+
						"Publish the locked runtime, or build from source with %q.",
					lock.RuntimeReleaseTag(), lock.Manifest, statusErr.status, "make dev MODE=normal",
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

	data, err := os.ReadFile(manifestPath)
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

func copyEngineAssetAtomically(src, dst string) (err error) {
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

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := output.Name()
	defer func() {
		if output != nil {
			_ = output.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	output = nil
	return replaceDownloadedFile(tmpPath, dst)
}

func replaceDownloadedFile(src, dst string) error {
	return os.Rename(src, dst)
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
	} else if !errors.Is(err, fs.ErrExist) && !isLinkFallbackError(err) {
		return err
	}
	return shared.CopyFile(src, dst)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func isLinkFallbackError(err error) bool {
	switch {
	case errors.Is(err, fs.ErrExist):
		return true
	case errors.Is(err, fs.ErrPermission):
		return true
	case errors.Is(err, syscall.EXDEV):
		return true
	case errors.Is(err, syscall.ENOTSUP):
		return true
	case errors.Is(err, syscall.EPERM):
		return true
	case errors.Is(err, syscall.EACCES):
		return true
	default:
		return false
	}
}
