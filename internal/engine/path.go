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
	"encoding/json"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
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
	root                       string
	projectRoot                string
	compatibilityRoot          string
	canonicalProjectRoot       string
	canonicalCompatibilityRoot string
	extAssetDir                string
	explicitFSRoots            bool
	enforceCanonical           bool
	legacyCompatibility        bool
}

var (
	assetPaths = assetPathState{
		root:        defaultAssetPathPrefix + defaultAssetDirName + "/",
		projectRoot: cleanFilesystemPath(defaultAssetPathPrefix),
	}
)

// SetFilesystemRoots configures the physical project and asset roots used by
// interpreted desktop sessions. It only updates path state; the Engine resource
// manager is configured later, after the GDExtension has linked its callbacks.
func SetFilesystemRoots(projectDir, assetDir string) error {
	return setFilesystemRoots(projectDir, assetDir, false)
}

// SetLegacyFilesystemRoots enables bounded external-resource compatibility for
// existing SPX commands.
func SetLegacyFilesystemRoots(projectDir, assetDir string) error {
	return setFilesystemRoots(projectDir, assetDir, true)
}

func setFilesystemRoots(projectDir, assetDir string, legacy bool) error {
	projectRoot, err := validateFilesystemRoot("project", projectDir)
	if err != nil {
		return err
	}
	assetRoot, err := validateFilesystemRoot("asset", assetDir)
	if err != nil {
		return err
	}
	canonicalProjectRoot, err := filepath.EvalSymlinks(filepath.FromSlash(projectRoot))
	if err != nil {
		return fmt.Errorf("engine: canonicalize project root %q: %w", projectDir, err)
	}
	canonicalAssetRoot, err := filepath.EvalSymlinks(filepath.FromSlash(assetRoot))
	if err != nil {
		return fmt.Errorf("engine: canonicalize asset root %q: %w", assetDir, err)
	}
	if !isWithinRoot(cleanFilesystemPath(canonicalAssetRoot), cleanFilesystemPath(canonicalProjectRoot)) {
		return fmt.Errorf("engine: asset root %q must be within project root %q", assetDir, projectDir)
	}
	compatibilityRoot := ""
	canonicalCompatibilityRoot := ""
	if legacy {
		compatibilityRoot = cleanFilesystemPath(filepath.Join(filepath.FromSlash(assetRoot), "..", ".."))
		canonical, err := filepath.EvalSymlinks(filepath.FromSlash(compatibilityRoot))
		if err != nil {
			return fmt.Errorf("engine: canonicalize legacy compatibility root %q: %w", compatibilityRoot, err)
		}
		canonicalCompatibilityRoot = cleanFilesystemPath(canonical)
	}
	assetPaths = assetPathState{
		root:                       joinAssetRoot("", assetRoot),
		projectRoot:                projectRoot,
		compatibilityRoot:          compatibilityRoot,
		canonicalProjectRoot:       cleanFilesystemPath(canonicalProjectRoot),
		canonicalCompatibilityRoot: canonicalCompatibilityRoot,
		extAssetDir:                readExtAssetDirFromFilesystem(projectRoot, legacy),
		explicitFSRoots:            true,
		enforceCanonical:           true,
		legacyCompatibility:        legacy,
	}
	return nil
}

func validateFilesystemRoot(name, root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("engine: %s root is empty", name)
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("engine: %s root %q is not absolute", name, root)
	}
	if clean := filepath.Clean(root); clean != root {
		return "", fmt.Errorf("engine: %s root %q is not clean (want %q)", name, root, clean)
	}
	return cleanFilesystemPath(root), nil
}

func setAssetRoot(prefix, dir string) {
	assetPaths.root = joinAssetRoot(prefix, defaultAssetDir(dir))
	if prefix == packmodeAssetPrefix {
		assetPaths.projectRoot = packmodeAssetPrefix
	} else {
		assetPaths.projectRoot = cleanFilesystemPath(prefix)
	}
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
	base, name, allowCompatibility, ok := filesystemAssetReference(relPath)
	if !ok {
		return ""
	}

	projectRoot := cleanFilesystemPath(assetPaths.projectRoot)
	base = cleanFilesystemPath(base)
	if projectRoot == "" || projectRoot == "." && filepath.IsAbs(filepath.FromSlash(base)) {
		return ""
	}
	resolvedPath := cleanFilesystemPath(filepath.Join(filepath.FromSlash(base), filepath.FromSlash(name)))
	canonicalRoot := assetPaths.canonicalProjectRoot
	if !isWithinRoot(resolvedPath, projectRoot) {
		if !assetPaths.legacyCompatibility || !allowCompatibility || leadingParentCount(name) < 2 ||
			assetPaths.compatibilityRoot == "" || !isWithinRoot(resolvedPath, assetPaths.compatibilityRoot) {
			return ""
		}
		canonicalRoot = assetPaths.canonicalCompatibilityRoot
	}
	if assetPaths.enforceCanonical {
		if canonicalRoot == "" {
			return ""
		}
		info, err := os.Lstat(filepath.FromSlash(resolvedPath))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ""
		}
		canonicalPath, err := filepath.EvalSymlinks(filepath.FromSlash(resolvedPath))
		if err != nil || !isWithinRoot(cleanFilesystemPath(canonicalPath), canonicalRoot) {
			return ""
		}
	}
	return resolvedPath
}

func filesystemAssetReference(reference string) (base, name string, allowCompatibility, ok bool) {
	if reference == "" {
		return "", "", false, false
	}
	schema, file := spxfs.SplitSchema(reference)
	switch schema {
	case "":
		base, name, allowCompatibility = assetPaths.root, file, true
	case "res":
		if !strings.HasPrefix(reference, packmodeAssetPrefix) {
			return "", "", false, false
		}
		base, name = assetPaths.projectRoot, file
	default:
		return "", "", false, false
	}
	if !isPortableRelativeResourcePath(name) {
		return "", "", false, false
	}
	return base, name, allowCompatibility, true
}

func isPortableRelativeResourcePath(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") {
		return false
	}
	return !pathpkg.IsAbs(name) && !filepath.IsAbs(filepath.FromSlash(name))
}

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = normalizeSlashes(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func leadingParentCount(name string) int {
	count := 0
	for _, segment := range strings.Split(cleanFilesystemPath(name), "/") {
		if segment != ".." {
			break
		}
		count++
	}
	return count
}

type assetProjectConfig struct {
	ExtAsset string `json:"extasset"`
}

func projectConfigPath(prefix string) string {
	return normalizeSlashes(prefix + projectConfigFile)
}

func readExtAssetDirFromFilesystem(projectRoot string, enabled bool) string {
	if !enabled {
		return ""
	}
	configPath := filepath.Join(filepath.FromSlash(projectRoot), projectConfigFile)
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		spxlog.Warn("SetAssetDir: failed to read %s: %v", configPath, err)
		return ""
	}
	return parseExtAssetDir(configPath, data)
}

func readExtAssetDirFromProjectConfig(prefix string) string {
	configPath := projectConfigPath(prefix)
	if !resMgr.HasFile(configPath) {
		return ""
	}
	return parseExtAssetDir(configPath, []byte(resMgr.ReadAllText(configPath)))
}

func parseExtAssetDir(configPath string, data []byte) string {
	var config assetProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		spxlog.Warn("SetAssetDir: failed to parse %s: %v", configPath, err)
		return ""
	}
	return config.ExtAsset
}

func extAssetSuffix(relPath string) (string, bool) {
	if assetPaths.extAssetDir == "" {
		return "", false
	}
	segments := strings.Split(cleanFilesystemPath(relPath), "/")
	leadingParents := 0
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if segment == ".." {
			leadingParents++
			continue
		}
		if leadingParents == 0 || segment != assetPaths.extAssetDir {
			return "", false
		}
		return normalizeSlashes(filepath.Join(segments[i+1:]...)), true
	}
	return "", false
}
