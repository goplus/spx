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

// Package ffi generates native Go bindings and C function-pointer wrappers.
package ffi

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	//go:embed ffi_wrapper.h.tmpl
	ffiWrapperHeaderFileText string

	//go:embed ffi_wrapper.go.tmpl
	ffiWrapperGoFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_native.go.tmpl
	managerNativeText string
	//go:embed interface.go.tmpl
	interfaceGoFileText string
	//go:embed sprite.go.tmpl
	implGoFileText string
	//go:embed sprite_pure.go.tmpl
	implPureGoFileText string

	//go:embed sync.go.tmpl
	syncAPIText string
)

type implData struct {
	Methods   []clang.TypedefFunction
	ClassName string
}

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
}

func (g *Generator) Generate(codegenDir string) error {
	ast := g.AST()
	generators := []struct {
		name string
		fn   func() error
	}{
		{"GDExtension wrapper header", func() error { return writeWrapperHeader(codegenDir, ast) }},
		{"GDExtension wrapper Go source", func() error { return writeWrapperGo(codegenDir, ast) }},
		{"GDExtension interface", func() error { return writeFFI(codegenDir, ast) }},
		{"manager wrapper", func() error { return g.writeManager(codegenDir) }},
		{"manager interface", func() error { return g.writeManagerInterface(codegenDir) }},
		{"synchronized API", func() error { return g.writeSyncAPI(codegenDir) }},
	}
	for _, generator := range generators {
		if err := generator.fn(); err != nil {
			return fmt.Errorf("generate %s: %w", generator.name, err)
		}
	}

	if err := g.writeManagerImpl(codegenDir, "Sprite"); err != nil {
		return fmt.Errorf("generate Sprite manager implementation: %w", err)
	}
	return nil
}

func (g *Generator) writeManager(codegenDir string) error {
	funcs := template.FuncMap{
		"camelCase":        strcase.ToCamel,
		"isManagerMethod":  g.IsManagerMethod,
		"managerSignature": g.ManagerMethodSignature,
		"managerBody":      g.managerBody,
	}

	return common.GenerateFile(funcs, "manager_native.gen.go", managerNativeText, g.ManagerData(),
		filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_native.gen.go"))
}

func (g *Generator) writeManagerInterface(codegenDir string) error {
	funcs := template.FuncMap{
		"camelCase":           strcase.ToCamel,
		"getManagerInterface": g.ManagerInterfaceSignature,
	}

	return common.GenerateFile(funcs, "interface.gen.go", interfaceGoFileText, g.ManagerData(),
		filepath.Join(codegenDir, common.EnginePkgRelDir, "interface.gen.go"))
}

func (g *Generator) writeSyncAPI(codegenDir string) error {
	variants := []struct {
		filename     string
		buildTag     string
		wrapFunction func(*clang.TypedefFunction) string
	}{
		{"sync.gen.go", "!pure_engine", g.genSyncAPIWrapFunction},
		{"sync_pure.gen.go", "pure_engine", g.genSyncPureAPIWrapFunction},
	}
	for _, variant := range variants {
		funcs := template.FuncMap{
			"lowerCamelCase":         strcase.ToLowerCamel,
			"camelCase":              strcase.ToCamel,
			"genSyncAPIWrapFunction": variant.wrapFunction,
		}
		data := struct {
			common.ManagerData
			BuildTag string
		}{g.ManagerData(), variant.buildTag}
		if err := common.GenerateFile(funcs, variant.filename, syncAPIText, data,
			filepath.Join(codegenDir, common.EngineWrapRelDir, variant.filename)); err != nil {
			return fmt.Errorf("generate %s: %w", variant.filename, err)
		}
	}
	return nil
}

func (g *Generator) writeManagerImpl(codegenDir, className string) error {
	methods := g.AST().CollectFunctionsOfClass(className)
	slices.SortFunc(methods, func(a, b clang.TypedefFunction) int {
		return strings.Compare(a.Name, b.Name)
	})
	data := implData{Methods: methods, ClassName: className}
	funcs := template.FuncMap{"getManagerImpl": g.getManagerImpl}
	variants := []struct {
		suffix string
		text   string
	}{
		{".gen.go", implGoFileText},
		{"_pure.gen.go", implPureGoFileText},
	}
	for _, variant := range variants {
		filename := strings.ToLower(className) + variant.suffix
		if err := common.GenerateFile(funcs, filename, variant.text, data,
			filepath.Join(codegenDir, common.EnginePkgRelDir, filename)); err != nil {
			return fmt.Errorf("generate %s: %w", filename, err)
		}
	}
	return nil
}

func writeWrapperHeader(codegenDir string, ast clang.CHeaderFileAST) error {
	output, err := common.RenderTemplate(nil, "ffi_wrapper.gen.h", ffiWrapperHeaderFileText, ast)
	if err != nil {
		return err
	}
	return common.WriteGeneratedFile(filepath.Join(codegenDir, common.NativeRelDir, "ffi_wrapper.gen.h"), output, 0o666)
}

func writeWrapperGo(codegenDir string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"goReturnType":       common.GoReturnType,
		"goArgumentType":     common.GoArgumentType,
		"goEnumValue":        common.GoEnumValue,
		"add":                common.Add,
		"cgoCastArgument":    common.CgoCastArgument,
		"cgoCastReturnType":  common.CgoCastReturnType,
		"cgoCleanUpArgument": common.CgoCleanUpArgument,
		"trimPrefix":         strings.TrimPrefix,
	}

	return common.GenerateFile(funcs, "ffi_wrapper.gen.go", ffiWrapperGoFileText, ast,
		filepath.Join(codegenDir, common.NativeRelDir, "ffi_wrapper.gen.go"))
}

func writeFFI(codegenDir string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"trimPrefix":          strings.TrimPrefix,
		"loadProcAddressName": common.LoadProcAddressName,
	}

	return common.GenerateFile(funcs, "ffi.gen.go", ffiFileText, ast,
		filepath.Join(codegenDir, common.NativeRelDir, "ffi.gen.go"))
}
