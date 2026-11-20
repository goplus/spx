//go:build !packmode

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
	"github.com/goplus/spx/v4/internal/base/fileutil"
	"github.com/goplus/spx/v4/internal/engine/platform"
)

// A virtual source can use the default directory before any physical project
// is configured. SetAssetDir enables canonical checks for native roots.
var assetPaths = func() assetResolver {
	project, assets := fileutil.AssetRoots(defaultAssetDirName)
	return assetResolver{assetRoot: joinAssetRoot("", assets), projectRoot: project}
}()

func SetAssetDir(dir string) {
	resolver := assetPaths
	if !resolver.explicitFSRoots {
		if platform.IsWeb() {
			resolver = newAssetResolver("", dir)
		} else {
			projectDir, assetDir := fileutil.AssetRoots(dir)
			resolver = nativeAssetResolver(projectDir, assetDir)
		}
	}
	Managers().ResMgr.SetLoadMode(true)
	assetPaths = resolver
}

func ToAssetPath(relPath string) string {
	return assetPaths.filesystemPath(relPath)
}
