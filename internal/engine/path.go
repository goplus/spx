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

	spxfs "github.com/goplus/spx/v3/fs"
)

const (
	defaultAssetPathPrefix = "../"
	packmodeAssetPrefix    = "res://"
	defaultAssetDirName    = "assets"
)

type assetPathState struct {
	root                 string
	projectRoot          string
	canonicalProjectRoot string
	explicitFSRoots      bool
	enforceCanonical     bool
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
	assetPaths = assetPathState{
		root:                 joinAssetRoot("", assetRoot),
		projectRoot:          projectRoot,
		canonicalProjectRoot: cleanFilesystemPath(canonicalProjectRoot),
		explicitFSRoots:      true,
		enforceCanonical:     true,
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
	base, name, ok := filesystemAssetReference(relPath)
	if !ok {
		return ""
	}

	projectRoot := cleanFilesystemPath(assetPaths.projectRoot)
	base = cleanFilesystemPath(base)
	if projectRoot == "" || projectRoot == "." && filepath.IsAbs(filepath.FromSlash(base)) {
		return ""
	}
	resolvedPath := cleanFilesystemPath(filepath.Join(filepath.FromSlash(base), filepath.FromSlash(name)))
	if !isWithinRoot(resolvedPath, projectRoot) {
		return ""
	}
	if assetPaths.enforceCanonical {
		if assetPaths.canonicalProjectRoot == "" {
			return ""
		}
		info, err := os.Lstat(filepath.FromSlash(resolvedPath))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ""
		}
		canonicalPath, err := filepath.EvalSymlinks(filepath.FromSlash(resolvedPath))
		if err != nil || !isWithinRoot(cleanFilesystemPath(canonicalPath), assetPaths.canonicalProjectRoot) {
			return ""
		}
	}
	return resolvedPath
}

func filesystemAssetReference(reference string) (base, name string, ok bool) {
	if reference == "" {
		return "", "", false
	}
	schema, file := spxfs.SplitSchema(reference)
	switch schema {
	case "":
		base, name = assetPaths.root, file
	case "res":
		if !strings.HasPrefix(reference, packmodeAssetPrefix) {
			return "", "", false
		}
		base, name = assetPaths.projectRoot, file
	default:
		return "", "", false
	}
	if !isPortableRelativeResourcePath(name) {
		return "", "", false
	}
	return base, name, true
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
