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

package pack

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
)

const (
	projectConfigName      = ".config"
	packDirName            = "assets"
	sourceIndexName        = "index.json"
	packedIndexName        = "index_pack.json"
	engineExtAssetDir      = "extasset"
	sharedAssetEscapeDepth = 2
)

type assetPathRef struct {
	configDir string
	path      string
}

func collectExternalAssetPathsWithConfig(baseFolder string, existingZipPaths map[string]struct{}, configuredExtAssetDir *string) (extraPaths []dirInfo, err error) {
	assetRoot := filepath.Join(baseFolder, packDirName)
	info, err := os.Lstat(assetRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("projectassets: PackDir %q must be a real directory", packDirName)
	}

	refs, err := collectAssetPathRefs(assetRoot)
	if err != nil {
		return nil, fmt.Errorf("projectassets: validate asset indexes: %w", err)
	}
	extAssetDir := ""
	if configuredExtAssetDir != nil {
		extAssetDir = *configuredExtAssetDir
	} else {
		extAssetDir, err = readExtAssetDir(baseFolder)
		if err != nil {
			configPath := filepath.Join(baseFolder, projectConfigName)
			return nil, fmt.Errorf("projectpolicy: parse project config %q: %w", configPath, err)
		}
	}

	seen := make(map[string]struct{}, len(existingZipPaths))
	for zipPath := range existingZipPaths {
		seen[zipPath] = struct{}{}
	}

	assetRoot = cleanFilesystemPath(assetRoot)
	compatibilityRoot := sharedAssetCompatibilityRoot(assetRoot)
	var externalRoot *os.Root
	defer func() {
		if err != nil && externalRoot != nil {
			_ = externalRoot.Close()
		}
	}()

	for _, ref := range refs {
		normalized := normalizeConfigPath(ref.configDir, ref.path)
		sourcePath, zipPath, ok := resolveExternalAssetPath(assetRoot, compatibilityRoot, extAssetDir, normalized)
		if !ok {
			continue
		}
		if _, exists := seen[zipPath]; exists {
			continue
		}

		if externalRoot == nil {
			externalRoot, err = openPackRoot(compatibilityRoot)
			if err != nil {
				return nil, err
			}
		}
		info, rootPath, err := inspectExternalAssetPath(externalRoot, compatibilityRoot, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat external asset %s referenced by %q: %w", sourcePath, normalized, err)
		}

		seen[zipPath] = struct{}{}
		extraPaths = append(extraPaths, dirInfo{
			path: sourcePath, info: info, zipPath: zipPath,
			root: externalRoot, rootPath: rootPath,
		})
	}
	return extraPaths, nil
}

func inspectExternalAssetPath(root *os.Root, rootPath, name string) (os.FileInfo, string, error) {
	rel, err := filepath.Rel(rootPath, name)
	if err != nil {
		return nil, "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("path is outside the compatibility root")
	}

	parts := strings.Split(rel, string(filepath.Separator))
	for i := range parts {
		candidate := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(candidate)
		if err != nil {
			return nil, "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("must be a regular non-symlink path (symlink at %q)", candidate)
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, "", fmt.Errorf("path component %q is not a directory", candidate)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("must be a regular non-symlink file")
		}
		return info, rel, nil
	}
	return nil, "", fmt.Errorf("empty external asset path")
}

func appendAssetPathRef(refs []assetPathRef, configDir, relPath string) []assetPathRef {
	if relPath == "" {
		return refs
	}
	return append(refs, assetPathRef{configDir: configDir, path: relPath})
}

func resolveExternalAssetPath(assetRoot, compatibilityRoot, extAssetDir, relPath string) (string, string, bool) {
	if relPath == "" || strings.HasPrefix(relPath, "/") {
		return "", "", false
	}
	if schema, _ := spxfs.SplitSchema(relPath); schema != "" {
		return "", "", false
	}

	sourcePath := cleanFilesystemPath(filepath.Join(assetRoot, filepath.FromSlash(relPath)))
	if zipPath := rewriteExtAssetZipPath(relPath, extAssetDir); zipPath != "" {
		if isWithinRoot(sourcePath, compatibilityRoot) {
			return sourcePath, zipPath, true
		}
		return "", "", false
	}

	if isWithinRoot(sourcePath, assetRoot) {
		return "", "", false
	}
	if leadingParentCount(relPath) < sharedAssetEscapeDepth || !isWithinRoot(sourcePath, compatibilityRoot) {
		return "", "", false
	}

	zipPath, err := filepath.Rel(compatibilityRoot, sourcePath)
	if err != nil {
		return "", "", false
	}
	zipPath = normalizeZipPath(zipPath)
	if zipPath == "." || strings.HasPrefix(zipPath, "../") {
		return "", "", false
	}
	return sourcePath, zipPath, true
}

func rewriteExtAssetZipPath(relPath, extAssetDir string) string {
	if extAssetDir == "" {
		return ""
	}

	segments := strings.Split(cleanFilesystemPath(relPath), "/")
	leadingParents := 0
	for i, segment := range segments {
		switch {
		case segment == "":
			continue
		case segment == "..":
			leadingParents++
		case segment != extAssetDir || leadingParents == 0:
			return ""
		default:
			suffix := filepath.Join(segments[i+1:]...)
			return normalizeZipPath(filepath.Join(engineExtAssetDir, suffix))
		}
	}
	return ""
}

func relConfigDir(assetRoot, configDir string) (string, error) {
	rel, err := filepath.Rel(assetRoot, configDir)
	if err != nil {
		return "", err
	}
	return normalizeZipPath(rel), nil
}

// normalizeConfigPath matches runtime path rules.
func normalizeConfigPath(configDir, relPath string) string {
	if relPath == "" {
		return ""
	}
	if strings.HasPrefix(relPath, "/") {
		return relPath
	}
	if schema, _ := spxfs.SplitSchema(relPath); schema != "" {
		return relPath
	}
	return path.Clean(path.Join(configDir, relPath))
}

func cleanFilesystemPath(name string) string {
	return normalizeZipPath(filepath.Clean(name))
}

func sharedAssetCompatibilityRoot(assetRoot string) string {
	return cleanFilesystemPath(filepath.Join(assetRoot, "..", ".."))
}

func isWithinRoot(name, root string) bool {
	rel, err := filepath.Rel(root, name)
	if err != nil {
		return false
	}
	rel = normalizeZipPath(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func leadingParentCount(relPath string) int {
	segments := strings.Split(cleanFilesystemPath(relPath), "/")
	count := 0
	for _, segment := range segments {
		if segment != ".." {
			break
		}
		count++
	}
	return count
}

func normalizeZipPath(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}
