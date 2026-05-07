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

package runtimeasset

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const assetsDir = "assets"
const manifestFileName = "manifest.json"

type assetManifest struct {
	CacheKey string   `json:"cache_key"`
	Names    []string `json:"names"`
}

//go:embed assets/*
var embeddedAssets embed.FS

var (
	assetsFS       fs.FS = embeddedAssets
	cacheBaseDirFn       = defaultCacheBaseDir
	extractMu      sync.Mutex
)

// Prepare extracts the named embedded runtime assets into a stable cache dir.
// It returns ok=false when this spx binary was built without the requested assets.
func Prepare(version string, names ...string) (dir string, ok bool, err error) {
	if len(names) == 0 {
		return "", false, nil
	}

	extractMu.Lock()
	defer extractMu.Unlock()

	for _, name := range names {
		if !hasAsset(name) {
			return "", false, nil
		}
	}

	cacheKey, ok, err := manifestCacheKey(names)
	if err != nil {
		return "", false, fmt.Errorf("load runtime asset manifest: %w", err)
	}
	if !ok {
		cacheKey, err = assetCacheKey(names)
		if err != nil {
			return "", false, fmt.Errorf("compute runtime cache key: %w", err)
		}
	}

	cacheDir := filepath.Join(cacheBaseDirFn(), "spx", "embedded-runtime", version, runtime.GOOS+"-"+runtime.GOARCH, cacheKey)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create runtime cache dir %s: %w", cacheDir, err)
	}

	for _, name := range names {
		if err := extractAsset(cacheDir, name); err != nil {
			return "", false, err
		}
	}

	return cacheDir, true, nil
}

func defaultCacheBaseDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "spx-cache")
}

func hasAsset(name string) bool {
	if name == "" {
		return false
	}
	_, err := fs.Stat(assetsFS, assetPath(name))
	return err == nil
}

func assetPath(name string) string {
	return path.Join(assetsDir, filepath.ToSlash(name))
}

func assetMode(name string) os.FileMode {
	if strings.HasPrefix(name, "gdspxrt") && !strings.HasSuffix(name, ".pck") {
		return 0o755
	}
	if strings.HasPrefix(name, "gdspx-") {
		return 0o755
	}
	return 0o644
}

func manifestCacheKey(names []string) (string, bool, error) {
	data, err := fs.ReadFile(assetsFS, assetPath(manifestFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	var manifest assetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", false, err
	}
	if manifest.CacheKey == "" || len(manifest.Names) != len(names) {
		return "", false, nil
	}
	for i, name := range names {
		if manifest.Names[i] != name {
			return "", false, nil
		}
	}
	return manifest.CacheKey, true, nil
}

func assetCacheKey(names []string) (string, error) {
	hasher := sha256.New()
	for _, name := range names {
		if err := addAssetHash(hasher, name); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16], nil
}

func addAssetHash(hasher hash.Hash, name string) (err error) {
	srcPath := assetPath(name)
	srcFile, err := assetsFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open embedded runtime asset %s for hashing: %w", name, err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err := io.WriteString(hasher, name+"\x00"); err != nil {
		return fmt.Errorf("hash embedded runtime asset name %s: %w", name, err)
	}
	if _, err := io.Copy(hasher, srcFile); err != nil {
		return fmt.Errorf("hash embedded runtime asset %s: %w", name, err)
	}
	if _, err := io.WriteString(hasher, "\x00"); err != nil {
		return fmt.Errorf("finalize embedded runtime asset hash %s: %w", name, err)
	}
	return nil
}

func extractAsset(cacheDir, name string) (err error) {
	srcPath := assetPath(name)
	info, err := fs.Stat(assetsFS, srcPath)
	if err != nil {
		return fmt.Errorf("stat embedded runtime asset %s: %w", name, err)
	}

	dstPath := filepath.Join(cacheDir, name)
	mode := assetMode(name)
	if dstInfo, err := os.Stat(dstPath); err == nil && dstInfo.Size() == info.Size() {
		if chmodErr := os.Chmod(dstPath, mode); chmodErr == nil || runtime.GOOS == "windows" {
			return nil
		}
	}

	srcFile, err := assetsFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open embedded runtime asset %s: %w", name, err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create runtime cache parent dir for %s: %w", dstPath, err)
	}
	outFile, err := os.CreateTemp(filepath.Dir(dstPath), filepath.Base(dstPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create extracted runtime asset for %s: %w", dstPath, err)
	}
	tmpPath := outFile.Name()
	defer func() {
		if outFile == nil {
			return
		}
		if closeErr := outFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(outFile, srcFile); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy embedded runtime asset %s to %s: %w", name, tmpPath, err)
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close extracted runtime asset %s: %w", tmpPath, err)
	}
	outFile = nil

	if err := os.Rename(tmpPath, dstPath); err != nil {
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(dstPath); removeErr != nil && !os.IsNotExist(removeErr) {
				_ = os.Remove(tmpPath)
				return fmt.Errorf("replace runtime cache asset %s: %w", dstPath, removeErr)
			}
			if retryErr := os.Rename(tmpPath, dstPath); retryErr == nil {
				goto chmod
			}
		}
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move runtime cache asset %s into place: %w", dstPath, err)
	}
chmod:
	if chmodErr := os.Chmod(dstPath, mode); chmodErr != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("chmod runtime cache asset %s: %w", dstPath, chmodErr)
	}
	return nil
}
