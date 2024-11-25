package engine

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

var (
	enginePathPrefix   = "../"
	extassetDir        = ""
	assetsDir          = enginePathPrefix + "assets/"
	configPath         = enginePathPrefix + ".config"
	engineExtAssetPath = "extasset"
)

type projectConfig struct {
	ExtAsset string `json:"extasset"`
}

func SetAssetDir(dir string) {
	// load config
	if SyncResHasFile(configPath) {
		configJson := SyncResReadAllText(configPath)
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
	return assetsDir + relPath
}

func replacePathIfInExtAssetDir(rpath string, extassetDir string, newAssetDir string) string {
	if extassetDir == "" {
		return ""
	}
	path := filepath.Clean(rpath)
	path = strings.ReplaceAll(path, "\\", "/")
	prefix := "../" + extassetDir
	if strings.Contains(path, prefix) {
		idx := strings.Index(path, prefix)
		directDir := path[:idx]
		directDir = strings.ReplaceAll(directDir, "../", "")
		if len(directDir) <= 0 {
			newPath := enginePathPrefix + filepath.Join(newAssetDir, path[:idx]+path[idx+len(prefix)+1:])
			newPath = strings.ReplaceAll(newPath, "\\", "/")
			return newPath
		} else {
			panic("extassetDir must be in the root directory" + rpath)
		}
	}

	return ""
}
