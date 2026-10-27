//go:build !packmode

package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/goplus/spx/v4/internal/engine/platform"
)

func TestSetAssetDirPreservesExplicitRootsAndReloadsOnlyLegacyConfig(t *testing.T) {
	resources := setupAssetPathTest(t)
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		if err := os.WriteFile(filepath.Join(projectDir, projectConfigFile), []byte(`{"extasset":"initial"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := setFilesystemRoots(projectDir, assetDir, legacy); err != nil {
			t.Fatal(err)
		}
		configured := assetPaths
		if err := os.WriteFile(filepath.Join(projectDir, projectConfigFile), []byte(`{"extasset":"updated"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		SetAssetDir("ignored")
		if legacy {
			configured.extAssetDir = "updated"
		}
		if assetPaths != configured {
			t.Fatalf("legacy=%v: SetAssetDir changed explicit roots: %+v, want %+v", legacy, assetPaths, configured)
		}
	}
	if !slices.Equal(resources.modes, []bool{true, true}) {
		t.Fatalf("resource load modes = %v, want filesystem mode for both calls", resources.modes)
	}
}

func TestSetAssetDirConfiguresLegacyPathsAndDefaultDirectory(t *testing.T) {
	resources := setupAssetPathTest(t)
	t.Chdir(t.TempDir())
	prefix := defaultAssetPathPrefix
	if platform.IsWeb() {
		prefix = ""
	}
	resources.files[projectConfigPath(prefix)] = `{"extasset":"shared"}`
	for _, dir := range []string{"", "images"} {
		assetPaths = assetPathState{}
		SetAssetDir(dir)
		wantDir := dir
		if wantDir == "" {
			wantDir = "assets"
		}
		if assetPaths.root != prefix+wantDir+"/" || assetPaths.extAssetDir != "shared" || !assetPaths.legacyCompatibility || assetPaths.explicitFSRoots {
			t.Fatalf("SetAssetDir(%q) = %+v", dir, assetPaths)
		}
	}
	if !slices.Equal(resources.modes, []bool{true, true}) {
		t.Fatalf("resource load modes = %v, want filesystem mode for both calls", resources.modes)
	}
}
