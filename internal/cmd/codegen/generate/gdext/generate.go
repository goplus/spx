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
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"
	spxlog "github.com/goplus/spx/v2/internal/cmd/codegen/internal/log"

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
func GenerateHeader(projectPath, godotPath string) {
	dir := filepath.Join(godotPath, "modules", "spx")
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		spxlog.Warn("Directory does not exist: %s", dir)
		return
	}
	outputFile := filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.h")
	generateSpxExtHeader(dir, outputFile, true)
}
func Generate(projectPath, godotPath string, ast clang.CHeaderFileAST) {
	dir := filepath.Join(godotPath, "modules", "spx")
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		spxlog.Warn("Directory does not exist: %s", dir)
		return
	}
	err = generateGdCppFile(projectPath, gdSpxExtCpp, ast, "gdextension_spx_ext.cpp")
	if err != nil {
		panic(err)
	}
	outputFile := filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.cpp")
	fileCopy(outputFile, filepath.Join(dir, "gdextension_spx_ext.cpp"))
	os.Remove(outputFile)

	// use the new format header
	outputFile = filepath.Join(projectPath, NativeRelDir, "gdextension_spx_ext.h")
	generateSpxExtHeader(dir, outputFile, false)
	fileCopy(outputFile, filepath.Join(dir, "gdextension_spx_ext.h"))

	err = generateGdCppFile(projectPath, gdJsSpxCpp, ast, "godot_js_spx.cpp")
	if err != nil {
		panic(err)
	}
	outputFile = filepath.Join(projectPath, NativeRelDir, "godot_js_spx.cpp")
	fileCopy(outputFile, filepath.Join(godotPath, "platform", "web", "godot_js_spx.cpp"))
	os.Remove(outputFile)
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
	f, err := os.Create(headerFileName)
	f.Write(b.Bytes())
	f.Close()
	return err
}
