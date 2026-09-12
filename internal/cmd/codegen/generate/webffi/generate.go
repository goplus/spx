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
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

const WebRelDir = "../../gdengine/binding/web"

var (
	//go:embed callbacks.go.tmpl
	callbacksFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_web.go.tmpl
	managerWebText string

	//go:embed gdspx.js.tmpl
	jsEngineJsFileText string

	//go:embed worker.js.tmpl
	workerWrapJsFileText string
)

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*common.GenerationContext
	caches map[string]cacheFunc
}

func (g *Generator) Generate(projectPath, spxModulePath string) error {
	ast := g.AST()
	generators := []struct {
		name string
		fn   func() error
	}{
		{"array ABI constants", func() error { return writeGoArrays(projectPath) }},
		{"JavaScript array ABI constants", func() error { return writeJSArrays(spxModulePath) }},
		{"callback Go source", func() error { return writeCallbacks(projectPath, ast) }},
		{"GDExtension interface", func() error { return writeFFI(projectPath, ast) }},
		{"manager wrapper", func() error { return g.writeManager(projectPath) }},
		{"JavaScript engine bridge", func() error { return g.writeEngineJS(spxModulePath) }},
		{"Web worker wrapper", func() error { return writeWorker(projectPath, ast) }},
	}
	for _, generator := range generators {
		if err := generator.fn(); err != nil {
			return fmt.Errorf("generate %s: %w", generator.name, err)
		}
	}
	return nil
}

func writeCallbacks(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"add":                   common.Add,
		"trimPrefix":            strings.TrimPrefix,
		"mustPrimitiveTypeName": common.MustPrimitiveTypeName,
	}

	return common.GenerateFile(funcs, "callbacks.gen.go", callbacksFileText, ast,
		filepath.Join(projectPath, WebRelDir, "callbacks.gen.go"))
}

func writeWorker(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"snakeCase":  strcase.ToSnake,
		"trimPrefix": strings.TrimPrefix,
	}

	return common.GenerateFile(funcs, "worker.wrap.gen.js", workerWrapJsFileText, ast,
		filepath.Join(projectPath, "../../../cmd/spx/template/platform/webworker/worker.wrap.gen.js"))
}

func writeFFI(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"trimPrefix":          strings.TrimPrefix,
		"loadProcAddressName": common.LoadProcAddressName,
	}

	return common.GenerateFile(funcs, "ffi.gen.go", ffiFileText, ast,
		filepath.Join(projectPath, WebRelDir, "ffi.gen.go"))
}

func (g *Generator) writeManager(projectPath string) error {
	caches, err := scanCaches(filepath.Join(projectPath, WebRelDir))
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
	renderer := *g
	renderer.caches = caches
	funcs := template.FuncMap{
		"camelCase":        strcase.ToCamel,
		"isManagerMethod":  g.IsManagerMethod,
		"managerSignature": g.ManagerMethodSignature,
		"managerBody":      renderer.managerBody,
	}

	return common.GenerateFile(funcs, "manager_web.gen.go", managerWebText, g.ManagerData(),
		filepath.Join(projectPath, common.GdengineImplRelDir, "manager_web.gen.go"))
}

func (g *Generator) writeEngineJS(spxModulePath string) error {
	funcs := template.FuncMap{
		"sub":       common.Sub,
		"jsArgs":    g.jsArgs,
		"jsBody":    g.jsBody,
		"jsResults": g.jsResults,
		"arrayBridge": func(name string) common.ArrayBridge {
			spec, _ := g.ArrayBridge(name)
			return spec
		},
		"loadProcAddressName": common.LoadProcAddressName,
	}

	output, err := common.RenderTemplate(funcs, "gdspx.js", jsEngineJsFileText, g.AST())
	if err != nil {
		return err
	}
	output = trimTrailingWhitespace(output)
	dstPath := filepath.Join(spxModulePath, "web", "js", "engine", "gdspx.js")
	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return err
	}
	return common.WriteGeneratedFile(dstPath, output, 0o666)
}

func trimTrailingWhitespace(src []byte) []byte {
	lines := bytes.Split(src, []byte("\n"))
	for i, line := range lines {
		lines[i] = bytes.TrimRight(line, " \t")
	}
	return bytes.Join(lines, []byte("\n"))
}
