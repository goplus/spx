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
	"encoding/json"

	"github.com/goplus/spx/v2/internal/engine/platform"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(true)

	prefix := defaultAssetPathPrefix
	if platform.IsWeb() {
		prefix = ""
	}

	setExtAssetDir(readExtAssetDirFromProjectConfig(prefix))
	setAssetRoot(prefix, dir)
}

func ToAssetPath(relPath string) string {
	return buildFilesystemAssetPath(relPath)
}

func readExtAssetDirFromProjectConfig(prefix string) string {
	configPath := projectConfigPath(prefix)
	if !resMgr.HasFile(configPath) {
		return ""
	}

	configJSON := resMgr.ReadAllText(configPath)
	var config assetProjectConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		spxlog.Warn("SetAssetDir: failed to parse %s: %v", configPath, err)
		return ""
	}
	return config.ExtAsset
}
