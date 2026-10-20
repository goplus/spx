//go:build packmode && !pure_engine

package engine

import (
	"slices"
	"testing"
)

func TestSetAssetDirConfiguresPackmodeRoots(t *testing.T) {
	resources := setupAssetPathTest(t)
	resources.files["res://.config"] = `{"extasset":"shared"}`
	for _, dir := range []string{"", "images"} {
		assetPaths = assetPathState{explicitFSRoots: true}
		SetAssetDir(dir)
		wantDir := dir
		if wantDir == "" {
			wantDir = "assets"
		}
		if assetPaths.root != "res://"+wantDir+"/" || assetPaths.projectRoot != "res://" || assetPaths.extAssetDir != "shared" || !assetPaths.legacyCompatibility || assetPaths.explicitFSRoots {
			t.Fatalf("SetAssetDir(%q) = %+v", dir, assetPaths)
		}
	}
	if !slices.Equal(resources.modes, []bool{false, false}) {
		t.Fatalf("resource load modes = %v, want archive mode for both calls", resources.modes)
	}
}
