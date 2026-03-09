//go:build packmode
// +build packmode

package engine

func SetAssetDir(dir string) {
	resMgr.SetLoadMode(false)
	setExtAssetDir("")
	setAssetRoot(packmodeAssetPrefix, dir)
}

func ToAssetPath(relPath string) string {
	return normalizeSlashes(assetPaths.root + relPath)
}
