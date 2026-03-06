//go:build !packmode
// +build !packmode

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
