//go:build !packmode
// +build !packmode

package engine

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(true)
	// load config
	if resMgr.HasFile(configPath) {
		configJson := resMgr.ReadAllText(configPath)
		var config projectConfig
		json.Unmarshal([]byte(configJson), &config)
		extassetDir = config.ExtAsset
	}
	assetsDir = enginePathPrefix + dir + "/"
}
func ToAssetPath(relPath string) string {
	replacedPath := replacePathIfInExtAssetDir(relPath, extassetDir, engineExtAssetPath)
	if replacedPath != "" {
		return replacedPath
	}
	path := assetsDir + relPath
	finalPath := filepath.Clean(path)
	finalPath = strings.ReplaceAll(finalPath, "\\", "/")
	return finalPath
}
