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

// GenGoFromFS generates Go code from .spx files in the provided filesystem
// Parameters:
//   - fsys: filesystem containing .spx files (should implement fsx.FileSystem interface)
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
	// NOTE(everyone): Keep sync with the config in spx [gop.mod](https://github.com/goplus/spx/blob/main/gop.mod)
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
		Import:   []*modfile.Import{{Name: "ai", Path: "github.com/goplus/builder/tools/ai"}},
	})

	// Register patch for spx to support functions with generic type like `Gopt_Game_Gopx_GetWidget`.
	// See details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805
	if err := registerPackagePatches(ctx); err != nil {
		return fmt.Errorf("failed to register package patches: %w", err)
	}

	// Build Go source code from .spx files
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

func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(any).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
`); err != nil {
		return fmt.Errorf("failed to register package patch for github.com/goplus/spx: %w", err)
	}

	// Patch for ai package - supports generic OnCmd function
	if err := xgobuild.RegisterPackagePatch(ctx, "github.com/goplus/builder/tools/ai", `
package ai

import . "github.com/goplus/builder/tools/ai"

func Gopt_Player_Gopx_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}
`); err != nil {
		return fmt.Errorf("failed to register package patch for github.com/goplus/builder/tools/ai: %w", err)
	}

	return nil
}
