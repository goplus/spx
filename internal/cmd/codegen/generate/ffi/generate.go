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

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	//go:embed callbacks.go.tmpl
	callbacksGoTemplate string

	//go:embed callbacks.h.tmpl
	callbacksHeaderTemplate string

	//go:embed ffi_wrapper.h.tmpl
	wrapperHeaderTemplate string

	//go:embed ffi_wrapper.go.tmpl
	wrapperGoTemplate string

	//go:embed ffi.go.tmpl
	ffiTemplate string

	//go:embed manager_native.go.tmpl
	managerTemplate string

	//go:embed interface.go.tmpl
	interfaceTemplate string

	//go:embed sprite.go.tmpl
	spriteTemplate string

	//go:embed sprite_pure.go.tmpl
	spritePureTemplate string

	//go:embed sync.go.tmpl
	syncTemplate string
)

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
	Output *common.OutputPlan
}

type implData struct {
	Methods   []clang.TypedefFunction
	ClassName string
}

func (g *Generator) Generate(codegenDir string) error {
	steps := []struct {
		name   string
		render func(string) error
	}{
		{"GDExtension wrapper header", g.renderWrapperHeader},
		{"GDExtension wrapper Go source", g.renderWrapperGo},
		{"GDExtension interface", g.renderFFI},
		{"native callbacks", g.renderCallbacks},
		{"manager wrapper", g.renderManager},
		{"manager interface", g.renderManagerInterface},
		{"synchronized API", g.renderSyncAPI},
	}
	for _, step := range steps {
		if err := step.render(codegenDir); err != nil {
			return fmt.Errorf("generate %s: %w", step.name, err)
		}
	}

	if err := g.renderManagerImpl(codegenDir, "Sprite"); err != nil {
		return fmt.Errorf("generate Sprite manager implementation: %w", err)
	}
	return nil
}

func (g *Generator) renderWrapperHeader(codegenDir string) error {
	return g.Output.RenderFile(nil, wrapperHeaderTemplate, g.AST(),
		filepath.Join(codegenDir, common.NativeRelDir, "ffi_wrapper.gen.h"))
}

func (g *Generator) renderWrapperGo(codegenDir string) error {
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

	return g.Output.RenderFile(funcs, wrapperGoTemplate, g.AST(),
		filepath.Join(codegenDir, common.NativeRelDir, "ffi_wrapper.gen.go"))
}

func (g *Generator) renderFFI(codegenDir string) error {
	funcs := template.FuncMap{
		"trimPrefix":          strings.TrimPrefix,
		"loadProcAddressName": common.LoadProcAddressName,
	}

	return g.Output.RenderFile(funcs, ffiTemplate, g.AST(),
		filepath.Join(codegenDir, common.NativeRelDir, "ffi.gen.go"))
}

func (g *Generator) renderManager(codegenDir string) error {
	funcs := template.FuncMap{
		"camelCase":        strcase.ToCamel,
		"isManagerMethod":  g.IsManagerMethod,
		"managerSignature": g.ManagerMethodSignature,
		"managerBody":      g.managerBody,
	}

	return g.Output.RenderFile(funcs, managerTemplate+common.ManagerTemplate, g.ManagerData(),
		filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_native.gen.go"))
}

func (g *Generator) renderManagerInterface(codegenDir string) error {
	funcs := template.FuncMap{
		"camelCase":           strcase.ToCamel,
		"getManagerInterface": g.ManagerInterfaceSignature,
	}

	return g.Output.RenderFile(funcs, interfaceTemplate, g.ManagerData(),
		filepath.Join(codegenDir, common.EnginePkgRelDir, "interface.gen.go"))
}

func (g *Generator) renderSyncAPI(codegenDir string) error {
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
		if err := g.Output.RenderFile(funcs, syncTemplate, data,
			filepath.Join(codegenDir, common.EngineWrapRelDir, variant.filename)); err != nil {
			return fmt.Errorf("generate %s: %w", variant.filename, err)
		}
	}
	return nil
}

func (g *Generator) renderManagerImpl(codegenDir, className string) error {
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
		{".gen.go", spriteTemplate},
		{"_pure.gen.go", spritePureTemplate},
	}
	for _, variant := range variants {
		filename := strings.ToLower(className) + variant.suffix
		if err := g.Output.RenderFile(funcs, variant.text, data,
			filepath.Join(codegenDir, common.EnginePkgRelDir, filename)); err != nil {
			return fmt.Errorf("generate %s: %w", filename, err)
		}
	}
	return nil
}
