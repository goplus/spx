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

package webffi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

type cacheFunc struct {
	name     string
	argCount int
}

// scanCaches matches Cached<Manager><Method> functions in *_cache.go to APIs.
// Each takes the API arguments followed by a func() T fallback.
func scanCaches(dir string) (map[string]cacheFunc, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*_cache.go"))
	if err != nil {
		return nil, err
	}
	functions := make(map[string]cacheFunc)
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			api, ok := strings.CutPrefix(function.Name.Name, "Cached")
			if !ok || !ast.IsExported(api) {
				continue
			}
			cacheName := function.Name.Name
			params := function.Type.Params
			if params.NumFields() == 0 || function.Type.Results.NumFields() != 1 {
				return nil, fmt.Errorf("%s: %s requires arguments ending in a fallback and one result", path, cacheName)
			}
			fallback := params.List[len(params.List)-1]
			closure, ok := fallback.Type.(*ast.FuncType)
			if !ok || len(fallback.Names) > 1 || closure.Params.NumFields() != 0 || closure.Results.NumFields() != 1 {
				return nil, fmt.Errorf("%s: %s requires a final func() T fallback", path, cacheName)
			}
			apiName := "GDExtensionSpx" + api
			if _, exists := functions[apiName]; exists {
				return nil, fmt.Errorf("duplicate cache function for %s", apiName)
			}
			functions[apiName] = cacheFunc{name: cacheName, argCount: params.NumFields() - 1}
		}
	}
	return functions, nil
}

func (g *Generator) wrapCache(function *clang.TypedefFunction, body string) string {
	cache, ok := g.caches[function.Name]
	if !ok {
		return body
	}
	var args []string
	for _, arg := range g.HighLevelArguments(function) {
		args = append(args, arg.Name)
	}
	if len(args) != cache.argCount || !common.HasEffectiveReturn(function) {
		panic("cache signature does not match " + function.Name)
	}
	args = append(args, fmt.Sprintf("func() %s {\n\t%s\n\t}", g.EffectiveGoReturnType(function), strings.ReplaceAll(body, "\n", "\n\t")))
	return "return " + cache.name + "(" + strings.Join(args, ", ") + ")"
}
