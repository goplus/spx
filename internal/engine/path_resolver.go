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
	"runtime"
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/base/fileutil"
)

// assetResolver resolves references against one configuration snapshot. Its
// methods never consult or change the configuration installed in the engine.
type assetResolver struct {
	assetRoot            string
	projectRoot          string
	canonicalProjectRoot string
	explicitFSRoots      bool
	enforceCanonical     bool
}

func (r assetResolver) filesystemPath(relPath string) string {
	name, projectResource, ok := parseAssetReference(relPath, runtime.GOOS != "windows")
	if !ok {
		return ""
	}
	base := r.assetRoot
	if projectResource {
		base = r.projectRoot
	}

	// Godot converts backslashes to separators even on POSIX filesystems.
	if strings.ContainsRune(filepath.ToSlash(base), '\\') || strings.ContainsRune(filepath.ToSlash(r.canonicalProjectRoot), '\\') {
		return ""
	}
	resolvedPath := filepath.Join(filepath.FromSlash(base), filepath.FromSlash(name))
	if !fileutil.IsWithinRoot(resolvedPath, r.projectRoot) {
		return ""
	}
	if r.enforceCanonical {
		if r.canonicalProjectRoot == "" {
			return ""
		}
		info, err := os.Lstat(resolvedPath)
		if err != nil {
			return ""
		}
		canonicalPath, err := fileutil.CanonicalPath(resolvedPath)
		if err != nil || !fileutil.IsWithinRoot(canonicalPath, r.canonicalProjectRoot) {
			return ""
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if r.explicitFSRoots || projectResource || !fileutil.IsWithinRoot(resolvedPath, r.assetRoot) {
				return ""
			}
			assets, err := fileutil.CanonicalPath(r.assetRoot)
			if err != nil || !fileutil.IsWithinRoot(canonicalPath, assets) {
				return ""
			}
		}
	}
	return filepath.ToSlash(resolvedPath)
}

func (r assetResolver) packmodePath(reference string) string {
	relPath, projectResource, ok := parseAssetReference(reference, false)
	if !ok {
		return ""
	}
	if projectResource {
		return projectResourcePath(relPath)
	}
	root := strings.TrimPrefix(r.assetRoot, projectResourcePrefix)
	return projectResourcePath(pathpkg.Join(root, relPath))
}

func newAssetResolver(prefix, dir string) assetResolver {
	resolver := assetResolver{assetRoot: joinAssetRoot(prefix, defaultAssetDir(dir))}
	if prefix == projectResourcePrefix {
		resolver.projectRoot = projectResourcePrefix
	} else {
		resolver.projectRoot = cleanFilesystemPath(prefix)
	}
	return resolver
}

func newFilesystemAssetResolver(projectDir, assetDir string) (assetResolver, error) {
	project, err := canonicalFilesystemRoot("project", projectDir)
	if err != nil {
		return assetResolver{}, err
	}
	assets, err := canonicalFilesystemRoot("asset", assetDir)
	if err != nil {
		return assetResolver{}, err
	}
	if !fileutil.IsWithinRoot(assets, project) {
		return assetResolver{}, fmt.Errorf("engine: asset root %q must be within project root %q", assetDir, projectDir)
	}
	return assetResolver{
		assetRoot:            joinAssetRoot("", assetDir),
		projectRoot:          filepath.ToSlash(projectDir),
		canonicalProjectRoot: project,
		explicitFSRoots:      true,
		enforceCanonical:     true,
	}, nil
}

func nativeAssetResolver(projectDir, assetDir string) assetResolver {
	canonical, _ := fileutil.CanonicalPath(projectDir)
	return assetResolver{
		assetRoot:            joinAssetRoot("", assetDir),
		projectRoot:          filepath.ToSlash(projectDir),
		canonicalProjectRoot: canonical,
		enforceCanonical:     true,
	}
}

// parseAssetReference shares lexical validation across filesystem and archive paths.
func parseAssetReference(reference string, allowColon bool) (name string, projectResource, ok bool) {
	name, projectResource = strings.CutPrefix(reference, projectResourcePrefix)
	if !projectResource && strings.HasPrefix(reference, "res:") {
		return "", false, false
	}
	return name, projectResource, isRelativeResourcePath(name, allowColon)
}

func canonicalFilesystemRoot(name, root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("engine: %s root is empty", name)
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("engine: %s root %q is not absolute", name, root)
	}
	if clean := filepath.Clean(root); clean != root {
		return "", fmt.Errorf("engine: %s root %q is not clean (want %q)", name, root, clean)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("engine: canonicalize %s root %q: %w", name, root, err)
	}
	return canonical, nil
}

func isRelativeResourcePath(name string, allowColon bool) bool {
	if name == "" || strings.ContainsAny(name, "\\\x00") {
		return false
	}
	if strings.ContainsRune(name, ':') && (!allowColon || strings.Contains(name, "://")) {
		return false
	}
	return !spxfs.IsAbsResourcePath(name)
}

func projectResourcePath(name string) string {
	name = pathpkg.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || !isRelativeResourcePath(name, false) {
		return ""
	}
	return projectResourcePrefix + name
}

func defaultAssetDir(dir string) string {
	if dir == "" {
		return defaultAssetDirName
	}
	return dir
}

func joinAssetRoot(prefix, dir string) string {
	return filepath.ToSlash(prefix + dir + "/")
}

func cleanFilesystemPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
