package runtimeasset

import (
	"embed"
	"fmt"
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

//go:embed assets/*
var embeddedAssets embed.FS

var (
	assetsFS      fs.FS = embeddedAssets
	cacheBaseDirFn      = defaultCacheBaseDir
	extractMu     sync.Mutex
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

	cacheDir := filepath.Join(cacheBaseDirFn(), "spx", "embedded-runtime", version, runtime.GOOS+"-"+runtime.GOARCH)
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

	tmpPath := dstPath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create runtime cache parent dir for %s: %w", dstPath, err)
	}
	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create extracted runtime asset %s: %w", tmpPath, err)
	}
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

	if err := os.RemoveAll(dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace runtime cache asset %s: %w", dstPath, err)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move runtime cache asset %s into place: %w", dstPath, err)
	}
	if chmodErr := os.Chmod(dstPath, mode); chmodErr != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("chmod runtime cache asset %s: %w", dstPath, chmodErr)
	}
	return nil
}
