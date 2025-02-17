package engine

import (
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
