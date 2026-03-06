package engine

import (
	"path/filepath"
	"strings"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

const (
	defaultAssetPathPrefix = "../"
	packmodeAssetPrefix    = "res://"
	defaultAssetDirName    = "assets"
	projectConfigFile      = ".config"
	engineExtAssetPath     = "extasset"
)

type assetPathState struct {
	prefix      string
	root        string
	extAssetDir string
}

var (
	assetPaths = assetPathState{
		prefix: defaultAssetPathPrefix,
		root:   defaultAssetPathPrefix + defaultAssetDirName + "/",
	}
)

type assetProjectConfig struct {
	ExtAsset string `json:"extasset"`
}

func projectConfigPath(prefix string) string {
	return normalizeSlashes(prefix + projectConfigFile)
}

func setAssetRoot(prefix, dir string) {
	assetPaths.prefix = prefix
	assetPaths.root = joinAssetRoot(prefix, defaultAssetDir(dir))
}

func setExtAssetDir(dir string) {
	assetPaths.extAssetDir = dir
}

func defaultAssetDir(dir string) string {
	if dir == "" {
		return defaultAssetDirName
	}
	return dir
}

func joinAssetRoot(prefix, dir string) string {
	return normalizeSlashes(prefix + dir + "/")
}

func normalizeSlashes(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func cleanFilesystemPath(path string) string {
	return normalizeSlashes(filepath.Clean(path))
}

func buildFilesystemAssetPath(relPath string) string {
	if replacedPath := rewriteExtAssetPath(relPath); replacedPath != "" {
		return replacedPath
	}

	root := cleanFilesystemPath(assetPaths.root)
	path := cleanFilesystemPath(filepath.Join(root, relPath))
	if !isWithinAssetRoot(path, root) {
		return ""
	}
	return path
}

func buildPackmodeAssetPath(relPath string) string {
	return normalizeSlashes(assetPaths.root + relPath)
}

func rewriteExtAssetPath(relPath string) string {
	if assetPaths.extAssetDir == "" {
		return ""
	}

	path := cleanFilesystemPath(relPath)
	segments := strings.Split(path, "/")
	leadingParents := 0
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if segment == ".." {
			leadingParents++
			continue
		}
		if segment != assetPaths.extAssetDir {
			if containsPathSegment(segments[i+1:], assetPaths.extAssetDir) {
				spxlog.Warn("ToAssetPath: extassetDir must be in the root directory: %s", relPath)
			}
			if leadingParents == 0 {
				return ""
			}
			return ""
		}
		if leadingParents == 0 {
			return ""
		}

		suffix := filepath.Join(segments[i+1:]...)
		newPath := assetPaths.prefix + filepath.Join(engineExtAssetPath, suffix)
		return normalizeSlashes(newPath)
	}

	return ""
}

func isWithinAssetRoot(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+"/")
}

func containsPathSegment(segments []string, target string) bool {
	for _, segment := range segments {
		if segment == target {
			return true
		}
	}
	return false
}
