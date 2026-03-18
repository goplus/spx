package gengo

import (
	"fmt"
	"os"

	"regexp"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	"github.com/goplus/xgo/parser"
)

const (
	preferredSourceExt = "_spx.gox"
	legacySourceExt    = ".spx"
	preferredMainFile  = "main" + preferredSourceExt
	legacyMainFile     = "main" + legacySourceExt
)

func registerProject(ext string) {
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ext,
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ext, Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
	})
}

// GenGoFromFS generates Go code from spx classfiles in the provided filesystem.
// Parameters:
//   - fsys: filesystem containing spx classfiles (should implement fsx.FileSystem interface)
//   - outputPath: absolute path where the generated main.go should be written
//
// Returns:
//   - error if generation fails
func GenGoFromFS(fsys parser.FileSystem, outputPath string) error {
	// Create a minimal context for code generation only
	ctx := ixgo.NewContext(0)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		fmt.Printf("Failed to resolve package import %q\n", path)
		return
	}
	// NOTE(everyone): Keep sync with the config in spx [gox.mod](https://github.com/goplus/spx/blob/main/gox.mod)
	registerProject(preferredSourceExt)
	registerProject(legacySourceExt)

	// Register patch for spx to support functions with generic type like `XGot_Game_XGox_GetWidget`.
	// See details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805
	if err := registerPackagePatches(ctx); err != nil {
		return fmt.Errorf("failed to register package patches: %w", err)
	}

	// Build Go source code from spx classfiles.
	source, err := xgobuild.BuildFSDir(ctx, fsys, "")
	if err != nil {
		return fmt.Errorf("failed to build XGo source: %w", err)
	}

	// Replace @patch suffix in import aliases only, these patch suffix only works on ixgo
	importPatchRegex := regexp.MustCompile(`(\w+)@patch`)
	sourceStr := importPatchRegex.ReplaceAllString(string(source), "$1")

	// Write generated source code to output file
	if err := os.WriteFile(outputPath, []byte(sourceStr), 0644); err != nil {
		return fmt.Errorf("failed to write generated Go code to %s: %w", outputPath, err)
	}

	println("xgobuild generated Go code: ", outputPath, "len = ", len(source))
	return nil
}

// registerPackagePatches registers necessary package patches for spx and ai packages
func registerPackagePatches(ctx *ixgo.Context) error {
	// Patch for spx package - supports generic GetWidget function
	if err := xgobuild.RegisterPackagePatch(ctx, "github.com/goplus/spx/v2", `
package spx

import . "github.com/goplus/spx/v2"

func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}
`); err != nil {
		return fmt.Errorf("failed to register package patch for github.com/goplus/spx: %w", err)
	}

	return nil
}
