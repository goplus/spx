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
	"os"
	"path/filepath"

	spxlog "github.com/goplus/spx/v4/internal/log"
)

const (
	defaultAssetPathPrefix = "../"
	packmodeAssetPrefix    = "res://"
	defaultAssetDirName    = "assets"
	projectConfigFile      = ".config"
	engineExtAssetPath     = "extasset"
)

type assetProjectConfig struct {
	ExtAsset string `json:"extasset"`
}

var assetPaths = assetResolver{
	root:        defaultAssetPathPrefix + defaultAssetDirName + "/",
	projectRoot: cleanFilesystemPath(defaultAssetPathPrefix),
}

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
	resolver, err := newFilesystemAssetResolver(projectDir, assetDir, legacy)
	if err != nil {
		return err
	}
	resolver.extAssetDir = readExtAssetDirFromFilesystem(resolver.projectRoot, legacy)
	assetPaths = resolver
	return nil
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
	if !Managers().ResMgr.HasFile(configPath) {
		return ""
	}
	return parseExtAssetDir(configPath, []byte(Managers().ResMgr.ReadAllText(configPath)))
}

func parseExtAssetDir(configPath string, data []byte) string {
	var config assetProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		spxlog.Warn("SetAssetDir: failed to parse %s: %v", configPath, err)
		return ""
	}
	return config.ExtAsset
}

func projectConfigPath(prefix string) string {
	return normalizeSlashes(prefix + projectConfigFile)
}
