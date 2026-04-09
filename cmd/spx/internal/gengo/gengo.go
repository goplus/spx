/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gengo

import (
	"fmt"
	"os"
	"regexp"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	"github.com/goplus/spx/v2/internal/base/licenseheader"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/xgo/parser"
)

// GenGoFromFS generates main.go from .spx files.
func GenGoFromFS(fsys parser.FileSystem, outputPath string) error {
	ctx := ixgo.NewContext(0)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		spxlog.Warn("failed to resolve package import %q", path)
		return
	}
	// Keep this in sync with gop.mod.
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
	})

	// Patch generic GetWidget support.
	if err := registerPackagePatches(ctx); err != nil {
		return fmt.Errorf("failed to register package patches: %w", err)
	}

	source, err := xgobuild.BuildFSDir(ctx, fsys, "")
	if err != nil {
		return fmt.Errorf("failed to build XGo source: %w", err)
	}

	// Strip ixgo-only @patch aliases.
	importPatchRegex := regexp.MustCompile(`(\w+)@patch`)
	sourceStr := importPatchRegex.ReplaceAllString(string(source), "$1")
	sourceStr = string(licenseheader.AddToGoSource([]byte(sourceStr)))

	if err := os.WriteFile(outputPath, []byte(sourceStr), 0644); err != nil {
		return fmt.Errorf("failed to write generated Go code to %s: %w", outputPath, err)
	}

	spxlog.Info("xgobuild generated Go code: %s len=%d", outputPath, len(source))
	return nil
}

// registerPackagePatches registers local ixgo patches.
func registerPackagePatches(ctx *ixgo.Context) error {
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
