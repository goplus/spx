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
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// assetResolver resolves references against one configuration snapshot. Its
// methods never consult or change the configuration installed in the engine.
type assetResolver struct {
	root                       string
	projectRoot                string
	compatibilityRoot          string
	canonicalProjectRoot       string
	canonicalCompatibilityRoot string
	extAssetDir                string
	explicitFSRoots            bool
	enforceCanonical           bool
}

func newAssetResolver(prefix, dir string) assetResolver {
	resolver := assetResolver{root: joinAssetRoot(prefix, defaultAssetDir(dir))}
	if prefix == packmodeAssetPrefix {
		resolver.projectRoot = packmodeAssetPrefix
	} else {
		resolver.projectRoot = cleanFilesystemPath(prefix)
	}
	return resolver
}

func newFilesystemAssetResolver(projectDir, assetDir string, legacy bool) (assetResolver, error) {
	projectRoot, err := validateFilesystemRoot("project", projectDir)
	if err != nil {
		return assetResolver{}, err
	}
	assetRoot, err := validateFilesystemRoot("asset", assetDir)
	if err != nil {
		return assetResolver{}, err
	}
	canonicalProjectRoot, err := filepath.EvalSymlinks(filepath.FromSlash(projectRoot))
	if err != nil {
		return assetResolver{}, fmt.Errorf("engine: canonicalize project root %q: %w", projectDir, err)
	}
	canonicalAssetRoot, err := filepath.EvalSymlinks(filepath.FromSlash(assetRoot))
	if err != nil {
		return assetResolver{}, fmt.Errorf("engine: canonicalize asset root %q: %w", assetDir, err)
	}
	if !isWithinRoot(cleanFilesystemPath(canonicalAssetRoot), cleanFilesystemPath(canonicalProjectRoot)) {
		return assetResolver{}, fmt.Errorf("engine: asset root %q must be within project root %q", assetDir, projectDir)
	}
	compatibilityRoot := ""
	canonicalCompatibilityRoot := ""
	if legacy {
		compatibilityRoot = cleanFilesystemPath(filepath.Join(filepath.FromSlash(assetRoot), "..", ".."))
		canonical, err := filepath.EvalSymlinks(filepath.FromSlash(compatibilityRoot))
		if err != nil {
			return assetResolver{}, fmt.Errorf("engine: canonicalize legacy compatibility root %q: %w", compatibilityRoot, err)
		}
		canonicalCompatibilityRoot = cleanFilesystemPath(canonical)
	}
	return assetResolver{
		root:                       joinAssetRoot("", assetRoot),
		projectRoot:                projectRoot,
		compatibilityRoot:          compatibilityRoot,
		canonicalProjectRoot:       cleanFilesystemPath(canonicalProjectRoot),
		canonicalCompatibilityRoot: canonicalCompatibilityRoot,
		explicitFSRoots:            true,
		enforceCanonical:           true,
	}, nil
}

func newLegacyFilesystemAssetResolver(prefix, dir string, enforceCanonical bool) assetResolver {
	resolver := newAssetResolver(prefix, dir)
	resolver.compatibilityRoot = cleanFilesystemPath(filepath.Join(filepath.FromSlash(resolver.root), "..", ".."))
	resolver.enforceCanonical = enforceCanonical
	if resolver.enforceCanonical {
		projectRoot, err := filepath.Abs(filepath.FromSlash(resolver.projectRoot))
		if err == nil {
			projectRoot, err = filepath.EvalSymlinks(projectRoot)
		}
		if err == nil {
			resolver.canonicalProjectRoot = cleanFilesystemPath(projectRoot)
		}
		compatibilityRootPath, compatErr := filepath.Abs(filepath.FromSlash(resolver.compatibilityRoot))
		if compatErr == nil {
			compatibilityRootPath, compatErr = filepath.EvalSymlinks(compatibilityRootPath)
		}
		if compatErr == nil {
			resolver.canonicalCompatibilityRoot = cleanFilesystemPath(compatibilityRootPath)
		}
	}
	return resolver
}

func (r assetResolver) filesystemPath(relPath string) string {
	name, projectResource, ok := parseAssetReference(relPath)
	if !ok {
		return ""
	}
	base := r.root
	if projectResource {
		base = r.projectRoot
	}

	projectRoot := cleanFilesystemPath(r.projectRoot)
	base = cleanFilesystemPath(base)
	if projectRoot == "" || projectRoot == "." && filepath.IsAbs(filepath.FromSlash(base)) {
		return ""
	}
	resolvedPath := cleanFilesystemPath(filepath.Join(filepath.FromSlash(base), filepath.FromSlash(name)))
	canonicalRoot := r.canonicalProjectRoot
	if !isWithinRoot(resolvedPath, projectRoot) {
		if projectResource || leadingParentCount(name) < 2 ||
			r.compatibilityRoot == "" || !isWithinRoot(resolvedPath, r.compatibilityRoot) {
			return ""
		}
		canonicalRoot = r.canonicalCompatibilityRoot
	}
	if r.enforceCanonical {
		if canonicalRoot == "" {
			return ""
		}
		info, err := os.Lstat(filepath.FromSlash(resolvedPath))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ""
		}
		absolutePath, err := filepath.Abs(filepath.FromSlash(resolvedPath))
		if err != nil {
			return ""
		}
		canonicalPath, err := filepath.EvalSymlinks(absolutePath)
		if err != nil || !isWithinRoot(cleanFilesystemPath(canonicalPath), canonicalRoot) {
			return ""
		}
	}
	return resolvedPath
}

func (r assetResolver) packmodePath(reference string) string {
	relPath, projectResource, ok := parseAssetReference(reference)
	if !ok {
		return ""
	}
	if projectResource {
		return projectResourcePath(relPath)
	}
	if suffix, ok := r.extAssetSuffix(relPath); ok {
		return projectResourcePath(pathpkg.Join(engineExtAssetPath, suffix))
	}
	if suffix, ok := strings.CutPrefix(pathpkg.Clean(relPath), "../../"); ok {
		return projectResourcePath(suffix)
	}
	root := strings.TrimPrefix(r.root, packmodeAssetPrefix)
	return projectResourcePath(pathpkg.Join(root, relPath))
}

func (r assetResolver) extAssetSuffix(relPath string) (string, bool) {
	if r.extAssetDir == "" {
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
		if leadingParents == 0 || segment != r.extAssetDir {
			return "", false
		}
		return normalizeSlashes(filepath.Join(segments[i+1:]...)), true
	}
	return "", false
}

// parseAssetReference shares lexical validation across filesystem and archive paths.
func parseAssetReference(reference string) (name string, projectResource, ok bool) {
	projectResource = strings.HasPrefix(reference, packmodeAssetPrefix)
	name = reference
	if projectResource {
		name = strings.TrimPrefix(reference, packmodeAssetPrefix)
	}
	return name, projectResource, isPortableRelativeResourcePath(name)
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

func projectResourcePath(name string) string {
	name = pathpkg.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || !isPortableRelativeResourcePath(name) {
		return ""
	}
	return packmodeAssetPrefix + name
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

func cleanFilesystemPath(path string) string {
	return normalizeSlashes(filepath.Clean(path))
}

func normalizeSlashes(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
