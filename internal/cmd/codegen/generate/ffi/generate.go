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

	//go:embed sync.gen.go.tmpl
	syncAPIText string

	//go:embed sync_pure.gen.go.tmpl
	syncPureAPIText string
)

type implData struct {
	Methods []clang.TypedefFunction
	ClsName string
}

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
}

func (g *Generator) Generate(projectPath string) error {
	ast := g.AST()
	generators := []struct {
		name string
		fn   func() error
	}{
		{"GDExtension wrapper header", func() error { return GenerateGDExtensionWrapperHeaderFile(projectPath, ast) }},
		{"GDExtension wrapper Go source", func() error { return GenerateGDExtensionWrapperGoFile(projectPath, ast) }},
		{"GDExtension interface", func() error { return GenerateGDExtensionInterfaceGoFile(projectPath, ast) }},
		{"manager wrapper", func() error { return g.GenerateManagerWrapperGoFile(projectPath) }},
		{"manager interface", func() error { return g.GenerateManagerInterfaceGoFile(projectPath) }},
		{"synchronized API", func() error { return g.GenerateSyncAPIGoFile(projectPath) }},
		{"pure synchronized API", func() error { return g.GenerateSyncPureGoFile(projectPath) }},
	}
	for _, generator := range generators {
		if err := generator.fn(); err != nil {
			return fmt.Errorf("generate %s: %w", generator.name, err)
		}
	}

	clsNames := []string{"Sprite"}
	for _, clsName := range clsNames {
		if err := g.GenerateManagerImplGoFile(projectPath, clsName); err != nil {
			return fmt.Errorf("generate %s manager implementation: %w", clsName, err)
		}
		if err := g.GenerateManagerImplPureGoFile(projectPath, clsName); err != nil {
			return fmt.Errorf("generate pure %s manager implementation: %w", clsName, err)
		}
	}
	return nil
}

func GenerateGDExtensionWrapperHeaderFile(projectPath string, ast clang.CHeaderFileAST) error {
	output, err := common.RenderTemplate(nil, "ffi_wrapper.gen.h", ffiWrapperHeaderFileText, ast)
	if err != nil {
		return err
	}
	return common.WriteGeneratedFile(filepath.Join(projectPath, common.NativeRelDir, "ffi_wrapper.gen.h"), output, 0o666)
}

func GenerateGDExtensionWrapperGoFile(projectPath string, ast clang.CHeaderFileAST) error {
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
		filepath.Join(projectPath, common.NativeRelDir, "ffi_wrapper.gen.go"))
}

func GenerateGDExtensionInterfaceGoFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"trimPrefix":          strings.TrimPrefix,
		"loadProcAddressName": common.LoadProcAddressName,
	}

	return common.GenerateFile(funcs, "ffi.gen.go", ffiFileText, ast,
		filepath.Join(projectPath, common.NativeRelDir, "ffi.gen.go"))
}

func (g *Generator) GenerateManagerWrapperGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"camelCase":          strcase.ToCamel,
		"isManagerMethod":    g.IsManagerMethod,
		"getManagerFuncName": g.ManagerMethodSignature,
		"getManagerFuncBody": g.getManagerFuncBody,
	}

	return common.GenerateFile(funcs, "manager_native.gen.go", managerNativeText, g.ManagerData(),
		filepath.Join(projectPath, common.GdengineImplRelDir, "manager_native.gen.go"))
}

func (g *Generator) GenerateManagerInterfaceGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"camelCase":           strcase.ToCamel,
		"getManagerInterface": g.ManagerInterfaceSignature,
	}

	return common.GenerateFile(funcs, "interface.gen.go", interfaceGoFileText, g.ManagerData(),
		filepath.Join(projectPath, common.EnginePkgRelDir, "interface.gen.go"))
}

func (g *Generator) GenerateSyncAPIGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"lowerCamelCase":         strcase.ToLowerCamel,
		"camelCase":              strcase.ToCamel,
		"genSyncAPIWrapFunction": g.genSyncAPIWrapFunction,
	}

	return common.GenerateFile(funcs, "sync.gen.go", syncAPIText, g.ManagerData(),
		filepath.Join(projectPath, common.EnginewrapRelDir, "sync.gen.go"))
}

func (g *Generator) GenerateSyncPureGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"lowerCamelCase":             strcase.ToLowerCamel,
		"camelCase":                  strcase.ToCamel,
		"genSyncPureAPIWrapFunction": g.genSyncPureAPIWrapFunction,
	}

	return common.GenerateFile(funcs, "sync_pure.gen.go", syncPureAPIText, g.ManagerData(),
		filepath.Join(projectPath, common.EnginewrapRelDir, "sync_pure.gen.go"))
}

func (g *Generator) GenerateManagerImplGoFile(projectPath string, clsName string) error {
	ast := g.AST()
	funcs := template.FuncMap{
		"getManagerImpl": g.getManagerImpl,
	}

	genFile := strings.ToLower(clsName) + ".gen.go"
	methods := ast.CollectFunctionsOfClass(clsName)
	slices.SortFunc(methods, func(a, b clang.TypedefFunction) int {
		return strings.Compare(a.Name, b.Name)
	})
	data := implData{Methods: methods, ClsName: clsName}

	return common.GenerateFile(funcs, genFile, implGoFileText, data,
		filepath.Join(projectPath, common.EnginePkgRelDir, genFile))
}

func (g *Generator) GenerateManagerImplPureGoFile(projectPath string, clsName string) error {
	ast := g.AST()
	funcs := template.FuncMap{
		"getManagerImplPure": g.getManagerImplPure,
	}
	methods := ast.CollectFunctionsOfClass(clsName)
	slices.SortFunc(methods, func(a, b clang.TypedefFunction) int {
		return strings.Compare(a.Name, b.Name)
	})
	data := implData{Methods: methods, ClsName: clsName}

	genFile := strings.ToLower(clsName) + "_pure.gen.go"
	return common.GenerateFile(funcs, genFile, implPureGoFileText, data,
		filepath.Join(projectPath, common.EnginePkgRelDir, genFile))
}
