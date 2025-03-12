//go:build packmode
// +build packmode

package engine

import (
	"strings"
)

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(false)
	enginePathPrefix = "res://"
	assetsDir = enginePathPrefix + dir + "/"
}

func ToAssetPath(relPath string) string {
	path := assetsDir + relPath
	finalPath := strings.ReplaceAll(path, "\\", "/")
	return finalPath
}
