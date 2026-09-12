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

// Package gdext generates the SPX GDExtension headers and engine bindings.
package gdext

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

var (
	//go:embed gdextension_spx_ext.cpp.tmpl
	gdSpxExtCpp string

	//go:embed godot_js_spx.cpp.tmpl
	gdJsSpxCpp string

	//go:embed gdextension_spx_ext.h.tmpl
	gdSpxExtH string
)

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
}

func (g *Generator) Generate(projectPath, spxModulePath string, headers Headers) error {
	if err := g.writeCPP(filepath.Join(spxModulePath, "gdextension_spx_ext.cpp"), gdSpxExtCpp); err != nil {
		return err
	}
	for _, path := range []string{
		filepath.Join(projectPath, common.NativeRelDir, "gdextension_spx_ext.h"),
		filepath.Join(spxModulePath, "gdextension_spx_ext.h"),
	} {
		if err := common.WriteGeneratedFile(path, []byte(headers.Standard), 0o644); err != nil {
			return err
		}
	}
	return g.writeCPP(filepath.Join(spxModulePath, "web", "godot_js_spx.cpp"), gdJsSpxCpp)
}

func (g *Generator) writeCPP(outputPath, templateStr string) error {
	funcs := template.FuncMap{
		"sub":                  common.Sub,
		"trimPrefix":           strings.TrimPrefix,
		"loadProcAddressName":  common.LoadProcAddressName,
		"isManagerMethod":      g.IsManagerMethod,
		"getManagerName":       g.GetManagerName,
		"isWebOwnedStringFree": isWebOwnedStringFree,
		"isWebGdStringReturn":  isWebGdStringReturn,
		"isWebGdArrayReturn":   isWebGdArrayReturn,
		"isGdStringArgument":   isGdStringArgument,
		"isGdArrayArgument":    isGdArrayArgument,
		"webManagerArgument":   webManagerArgument,
		"arrayBridge": func(name string) *common.ArrayBridge {
			spec, ok := g.ArrayBridge(name)
			if !ok {
				return nil
			}
			return &spec
		},
		"listArrayBridges": g.ListArrayBridges,
		"cDecl": func(typeName, name string) string {
			typeName = strings.TrimSpace(typeName)
			if strings.HasSuffix(typeName, "*") {
				return typeName + name
			}
			return typeName + " " + name
		},
	}

	output, err := common.RenderTemplate(funcs, filepath.Base(outputPath), templateStr, g.ManagerData())
	if err != nil {
		return err
	}
	return common.WriteGeneratedFile(outputPath, output, 0o644)
}

// Web owns the value returned by this legacy method.
func isWebOwnedStringFree(function *clang.TypedefFunction) bool {
	return function != nil && function.Name == "GDExtensionSpxResFreeStr"
}

func isWebGdArrayReturn(function *clang.TypedefFunction) bool {
	return function != nil && function.ReturnType.Name == "GdArray" && !function.ReturnType.IsPointer
}

func isWebGdStringReturn(function *clang.TypedefFunction) bool {
	return function != nil && function.ReturnType.Name == "GdString" && !function.ReturnType.IsPointer
}

func isGdArrayArgument(argument clang.Argument) bool {
	return argument.Type.Primative != nil && argument.Type.Primative.Name == "GdArray"
}

func isGdStringArgument(argument clang.Argument) bool {
	return argument.Type.Primative != nil && argument.Type.Primative.Name == "GdString" &&
		!argument.Type.Primative.IsPointer
}

func webManagerArgument(argument clang.Argument, index int) string {
	if isGdStringArgument(argument) {
		return fmt.Sprintf("gdspx_string_arg_%d", index)
	}
	return argument.ResolvedPtrName(index)
}
