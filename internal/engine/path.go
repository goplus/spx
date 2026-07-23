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
	"path/filepath"
	"slices"
	"strings"

	spxlog "github.com/goplus/spx/v3/internal/log"
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
	if isWithinRoot(path, root) {
		return path
	}
	// Preserve legacy projects that referenced a shared sibling resource directory
	// with "../../...". The compatibility root is still bounded to two parent levels
	// above the configured asset root, so paths outside that legacy scope stay rejected.
	if leadingParentCount(relPath) >= 2 && isWithinCompatibilityRoot(path, root) {
		return path
	}
	return ""
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

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = normalizeSlashes(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func isWithinCompatibilityRoot(path, assetRoot string) bool {
	root := cleanFilesystemPath(filepath.Join(assetRoot, "../.."))
	return isWithinRoot(path, root)
}

func leadingParentCount(relPath string) int {
	normalized := cleanFilesystemPath(relPath)
	count := 0
	for segment := range strings.SplitSeq(normalized, "/") {
		if segment != ".." {
			break
		}
		count++
	}
	return count
}

func containsPathSegment(segments []string, target string) bool {
	return slices.Contains(segments, target)
}
