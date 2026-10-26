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

// Package webffi generates Go, JavaScript, and worker bindings for Web runtimes.
package webffi

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

const WebRelDir = "../../gdengine/binding/web"

var (
	//go:embed callbacks.go.tmpl
	callbacksTemplate string

	//go:embed ffi.go.tmpl
	ffiTemplate string

	//go:embed manager_web.go.tmpl
	managerTemplate string

	//go:embed gdspx.js.tmpl
	engineTemplate string

	//go:embed worker.js.tmpl
	workerTemplate string
)

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
	Output *common.OutputPlan
}

func (g *Generator) Generate(codegenDir, spxModulePath string) error {
	steps := []struct {
		name   string
		render func(string) error
		dir    string
	}{
		{"array ABI constants", g.renderGoArrays, codegenDir},
		{"JavaScript array ABI constants", g.renderJSArrays, spxModulePath},
		{"callback Go source", g.renderCallbacks, codegenDir},
		{"GDExtension interface", g.renderFFI, codegenDir},
		{"manager wrapper", g.renderManager, codegenDir},
		{"JavaScript engine bridge", g.renderEngineJS, spxModulePath},
		{"Web worker wrapper", g.renderWorker, codegenDir},
	}
	for _, step := range steps {
		if err := step.render(step.dir); err != nil {
			return fmt.Errorf("generate %s: %w", step.name, err)
		}
	}
	return nil
}

func (g *Generator) renderCallbacks(codegenDir string) error {
	funcs := template.FuncMap{
		"add":                   common.Add,
		"trimPrefix":            strings.TrimPrefix,
		"mustPrimitiveTypeName": common.MustPrimitiveTypeName,
	}

	return g.Output.RenderFile(funcs, callbacksTemplate, g.AST(),
		filepath.Join(codegenDir, WebRelDir, "callbacks.gen.go"))
}

func (g *Generator) renderFFI(codegenDir string) error {
	funcs := template.FuncMap{
		"trimPrefix":          strings.TrimPrefix,
		"loadProcAddressName": common.LoadProcAddressName,
	}

	return g.Output.RenderFile(funcs, ffiTemplate, g.AST(),
		filepath.Join(codegenDir, WebRelDir, "ffi.gen.go"))
}

func (g *Generator) renderManager(codegenDir string) error {
	caches, err := scanCaches(filepath.Join(codegenDir, WebRelDir))
	if err != nil {
		return err
	}
	functions := make(map[string]bool)
	for _, function := range g.AST().CollectGDExtensionInterfaceFunctions() {
		if g.IsManagerMethod(&function) {
			functions[function.Name] = true
		}
	}
	for name := range caches {
		if !functions[name] {
			return fmt.Errorf("cache function has no matching API: %s", name)
		}
	}
	funcs := template.FuncMap{
		"camelCase":        strcase.ToCamel,
		"isManagerMethod":  g.IsManagerMethod,
		"managerSignature": g.ManagerMethodSignature,
		"managerBody": func(function *clang.TypedefFunction) string {
			return g.cachedManagerBody(function, caches)
		},
	}

	return g.Output.RenderFile(funcs, managerTemplate+common.ManagerTemplate, g.ManagerData(),
		filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_web.gen.go"))
}

func (g *Generator) renderEngineJS(spxModulePath string) error {
	funcs := template.FuncMap{
		"sub":                common.Sub,
		"jsArgs":             g.jsArgs,
		"jsBody":             g.jsBody,
		"jsResults":          g.jsResults,
		"hasEffectiveReturn": g.HasEffectiveReturn,
		"arrayBridge": func(name string) common.ArrayBridge {
			spec, _ := g.ArrayBridge(name)
			return spec
		},
		"loadProcAddressName": common.LoadProcAddressName,
	}

	output, err := common.RenderTemplate(funcs, "gdspx.js", engineTemplate, g.AST())
	if err != nil {
		return err
	}
	output = trimTrailingWhitespace(output)
	dstPath := filepath.Join(spxModulePath, "web", "js", "engine", "gdspx.js")
	return g.Output.AddFile(dstPath, output)
}

func (g *Generator) renderWorker(codegenDir string) error {
	return g.Output.RenderFile(nil, workerTemplate, nil,
		filepath.Join(codegenDir, "../../../cmd/spx/template/platform/webworker/worker.wrap.gen.js"))
}

func trimTrailingWhitespace(src []byte) []byte {
	lines := bytes.Split(src, []byte("\n"))
	for i, line := range lines {
		lines[i] = bytes.TrimRight(line, " \t")
	}
	return bytes.Join(lines, []byte("\n"))
}
