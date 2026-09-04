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

// Package gdextensionwrapper generates C code to wrap all of the gdextension
// methods to call functions on the gdextension_api_structs to work
// around the Cgo C function pointer limitation.
package gdext

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/iancoleman/strcase"
)

var (
	//go:embed gdextension_spx_ext.cpp.tmpl
	gdSpxExtCpp string

	//go:embed godot_js_spx.cpp.tmpl
	gdJsSpxCpp string

	//go:embed gdextension_spx_ext.h.tmpl
	gdSpxExtH string
)

func fileCopy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return dstFile.Sync()
}
func GenerateHeader(projectPath, spxModulePath string) error {
	outputFile := filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.h")
	return generateSpxExtHeader(spxModulePath, outputFile, true)
}

func Generate(projectPath, spxModulePath string, ast clang.CHeaderFileAST) error {
	if err := generateGdCppFile(projectPath, gdSpxExtCpp, ast, "gdextension_spx_ext.cpp"); err != nil {
		return err
	}
	outputFile := filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.cpp")
	if err := fileCopy(outputFile, filepath.Join(spxModulePath, "gdextension_spx_ext.cpp")); err != nil {
		return fmt.Errorf("copy gdextension_spx_ext.cpp: %w", err)
	}
	if err := os.Remove(outputFile); err != nil {
		return fmt.Errorf("remove temporary gdextension_spx_ext.cpp: %w", err)
	}

	// use the new format header
	outputFile = filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.h")
	if err := generateSpxExtHeader(spxModulePath, outputFile, false); err != nil {
		return err
	}
	if err := fileCopy(outputFile, filepath.Join(spxModulePath, "gdextension_spx_ext.h")); err != nil {
		return fmt.Errorf("copy gdextension_spx_ext.h: %w", err)
	}

	if err := generateGdCppFile(projectPath, gdJsSpxCpp, ast, "godot_js_spx.cpp"); err != nil {
		return err
	}
	outputFile = filepath.Join(projectPath, NativeRelDir, "godot_js_spx.cpp")
	if err := fileCopy(outputFile, filepath.Join(spxModulePath, "web", "godot_js_spx.cpp")); err != nil {
		return fmt.Errorf("copy godot_js_spx.cpp: %w", err)
	}
	if err := os.Remove(outputFile); err != nil {
		return fmt.Errorf("remove temporary godot_js_spx.cpp: %w", err)
	}
	return nil
}

func generateGdCppFile(projectPath string, templateStr string, ast clang.CHeaderFileAST, outputFileName string) error {
	funcs := template.FuncMap{
		"gdiVariableName":             GdiVariableName,
		"snakeCase":                   strcase.ToSnake,
		"camelCase":                   strcase.ToCamel,
		"goReturnType":                GoReturnType,
		"goArgumentType":              GoArgumentType,
		"goEnumValue":                 GoEnumValue,
		"add":                         Add,
		"sub":                         Sub,
		"cgoCastArgument":             CgoCastArgument,
		"cgoCastReturnType":           CgoCastReturnType,
		"cgoCleanUpArgument":          CgoCleanUpArgument,
		"trimPrefix":                  TrimPrefix,
		"loadProcAddressName":         LoadProcAddressName,
		"isManagerMethod":             IsManagerMethod,
		"getManagerName":              GetManagerName,
		"isWebOwnedStringFree":        isWebOwnedStringFree,
		"isWebGdStringReturn":         isWebGdStringReturn,
		"isWebGdArrayReturn":          isWebGdArrayReturn,
		"isGdStringArgument":          isGdStringArgument,
		"isGdArrayArgument":           isGdArrayArgument,
		"webManagerArgument":          webManagerArgument,
		"hasArrayTransformBridgeSpec": HasArrayTransformBridgeSpec,
		"hasNativeArrayBridgeSpec":    HasNativeArrayBridgeSpec,
		"getArrayTransformBridgeSpec": func(function *clang.TypedefFunction) ArrayTransformBridgeSpec {
			spec, _ := GetArrayTransformBridgeSpec(function.Name)
			return spec
		},
		"getNativeArrayBridgeSpec": func(function *clang.TypedefFunction) NativeArrayBridgeSpec {
			spec, _ := GetNativeArrayBridgeSpec(function.Name)
			return spec
		},
		"listArrayTransformBridgeSpecs": ListArrayTransformBridgeSpecs,
		"fastArrayElemCppType": func(arrayType int32) string {
			switch arrayType {
			case 1:
				return "int64_t"
			case 2:
				return "float"
			case 5:
				return "uint8_t"
			case 6:
				return "GdObj"
			default:
				panic("unsupported fast array element type")
			}
		},
		"fastArrayTypeConst": func(arrayType int32) string {
			switch arrayType {
			case 1:
				return "GD_ARRAY_TYPE_INT64"
			case 2:
				return "GD_ARRAY_TYPE_FLOAT"
			case 5:
				return "GD_ARRAY_TYPE_BYTE"
			case 6:
				return "GD_ARRAY_TYPE_GDOBJ"
			default:
				panic("unsupported fast array type constant")
			}
		},
		"cDecl": func(typeName, name string) string {
			typeName = strings.TrimSpace(typeName)
			if strings.HasSuffix(typeName, "*") {
				return typeName + name
			}
			return typeName + " " + name
		},
	}

	tmpl, err := template.New(outputFileName).
		Funcs(funcs).
		Parse(templateStr)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, ManagerData{Ast: ast, Mangers: GetManagers(ast)})
	if err != nil {
		return err
	}

	headerFileName := filepath.Join(projectPath, NativeRelDir, outputFileName)
	if err := os.WriteFile(headerFileName, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputFileName, err)
	}
	return nil
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
