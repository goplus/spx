//go:build !packmode
// +build !packmode

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

	"github.com/goplus/spx/v3/internal/engine/platform"
)

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(true)
	if assetPaths.explicitFSRoots {
		if assetPaths.legacyCompatibility {
			assetPaths.extAssetDir = readExtAssetDirFromFilesystem(assetPaths.projectRoot, true)
		}
		return
	}
	setLegacyFilesystemAssetDir(dir)
	// Reading the project config uses the Engine resource manager. Keep that
	// call at the public Engine boundary so path-state tests can configure the
	// legacy filesystem roots without initializing enginewrap.
	prefix := defaultAssetPathPrefix
	if platform.IsWeb() {
		prefix = ""
	}
	assetPaths.extAssetDir = readExtAssetDirFromProjectConfig(prefix)
}

func setLegacyFilesystemAssetDir(dir string) {
	prefix := defaultAssetPathPrefix
	if platform.IsWeb() {
		prefix = ""
	}

	setAssetRoot(prefix, dir)
	assetPaths.explicitFSRoots = false
	assetPaths.legacyCompatibility = true
	assetPaths.compatibilityRoot = cleanFilesystemPath(filepath.Join(filepath.FromSlash(assetPaths.root), "..", ".."))
	assetPaths.canonicalCompatibilityRoot = ""
	assetPaths.extAssetDir = ""
	assetPaths.enforceCanonical = !platform.IsWeb()
	assetPaths.canonicalProjectRoot = ""
	if assetPaths.enforceCanonical {
		projectRoot, err := filepath.Abs(filepath.FromSlash(assetPaths.projectRoot))
		if err == nil {
			projectRoot, err = filepath.EvalSymlinks(projectRoot)
		}
		if err == nil {
			assetPaths.canonicalProjectRoot = cleanFilesystemPath(projectRoot)
		}
		compatibilityRootPath, compatErr := filepath.Abs(filepath.FromSlash(assetPaths.compatibilityRoot))
		if compatErr == nil {
			compatibilityRootPath, compatErr = filepath.EvalSymlinks(compatibilityRootPath)
		}
		if compatErr == nil {
			assetPaths.canonicalCompatibilityRoot = cleanFilesystemPath(compatibilityRootPath)
		}
	}
}

func ToAssetPath(relPath string) string {
	return buildFilesystemAssetPath(relPath)
}
