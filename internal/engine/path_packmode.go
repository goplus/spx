//go:build packmode
// +build packmode

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
	"path"
	"strings"
)

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(false)
	// Packmode keeps the legacy archive and export policy.
	setAssetRoot(packmodeAssetPrefix, dir)
	assetPaths.explicitFSRoots = false
	assetPaths.legacyCompatibility = true
	assetPaths.extAssetDir = readExtAssetDirFromProjectConfig(packmodeAssetPrefix)
}

func ToAssetPath(relPath string) string {
	if strings.Contains(relPath, "\\") {
		return ""
	}
	relPath = normalizeSlashes(relPath)
	if relPath == "" || strings.HasPrefix(relPath, "/") {
		return ""
	}
	if strings.HasPrefix(relPath, packmodeAssetPrefix) {
		return projectResourcePath(strings.TrimPrefix(relPath, packmodeAssetPrefix))
	}
	if strings.Contains(relPath, ":") {
		return ""
	}
	if suffix, ok := extAssetSuffix(relPath); ok {
		return projectResourcePath(path.Join(engineExtAssetPath, suffix))
	}
	if suffix, ok := packmodeCompatibilitySuffix(relPath); ok {
		return projectResourcePath(suffix)
	}
	root := strings.TrimPrefix(assetPaths.root, packmodeAssetPrefix)
	return projectResourcePath(path.Join(root, relPath))
}

func packmodeCompatibilitySuffix(relPath string) (string, bool) {
	clean := path.Clean(relPath)
	if !strings.HasPrefix(clean, "../../") {
		return "", false
	}
	suffix := strings.TrimPrefix(clean, "../../")
	if suffix == "" || suffix == clean {
		return "", false
	}
	return suffix, true
}

func projectResourcePath(name string) string {
	name = path.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) || strings.Contains(name, ":") {
		return ""
	}
	return packmodeAssetPrefix + name
}
