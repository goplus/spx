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

package common

import (
	"slices"
	"sort"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

// GenerationMetadata is collected from headers before binding templates run.
type GenerationMetadata struct {
	ManagerNames          []string
	NativeArrayBridges    map[string]NativeArrayBridgeSpec
	ArrayTransformBridges map[string]ArrayTransformBridgeSpec
}

// GenerationContext snapshots metadata and manager membership for one AST.
// Rendering only reads this context; independent outputs can share it.
type GenerationContext struct {
	ast                   clang.CHeaderFileAST
	managerSet            map[string]bool
	managers              []string
	managerNames          clang.ManagerNames
	cppType2Go            map[string]string
	nativeArrayBridges    map[string]NativeArrayBridgeSpec
	arrayTransformBridges map[string]ArrayTransformBridgeSpec
}

type ManagerData struct {
	Ast          clang.CHeaderFileAST
	Managers     []string
	ManagerNames clang.ManagerNames
}

func NewGenerationContext(ast clang.CHeaderFileAST, metadata GenerationMetadata) *GenerationContext {
	c := &GenerationContext{
		ast:                   ast,
		managerSet:            make(map[string]bool),
		managerNames:          clang.NewManagerNames(metadata.ManagerNames),
		nativeArrayBridges:    make(map[string]NativeArrayBridgeSpec),
		arrayTransformBridges: make(map[string]ArrayTransformBridgeSpec),
		cppType2Go: map[string]string{
			"GdInt": "int64", "GdFloat": "float64", "GdObj": "Object",
			"GdVec2": "Vec2", "GdVec3": "Vec3", "GdVec4": "Vec4",
			"GdRect2": "Rect2", "GdString": "string", "GdBool": "bool",
			"GdColor": "Color", "GdArray": "Array",
		},
	}
	for name, spec := range metadata.NativeArrayBridges {
		c.nativeArrayBridges[name] = spec
	}
	for name, spec := range metadata.ArrayTransformBridges {
		spec.Params = slices.Clone(spec.Params)
		c.arrayTransformBridges[name] = spec
	}
	c.managers = c.GetManagers(ast)
	for _, name := range c.managers {
		c.managerSet[name] = true
	}
	return c
}

// AST returns the input used to prepare this context. Treat it as read-only.
func (c *GenerationContext) AST() clang.CHeaderFileAST { return c.ast }

func (c *GenerationContext) ManagerData() ManagerData {
	return ManagerData{Ast: c.ast, Managers: slices.Clone(c.managers), ManagerNames: c.managerNames}
}

func (c *GenerationContext) GetManagerName(name string) string {
	return c.managerNames.Resolve(name)
}

func (c *GenerationContext) IsManagerMethod(function *clang.TypedefFunction) bool {
	return c.managerSet[c.GetManagerName(function.Name)]
}

func (c *GenerationContext) HasArrayTransformBridgeSpec(function *clang.TypedefFunction) bool {
	if function == nil {
		return false
	}
	_, ok := c.arrayTransformBridges[function.Name]
	return ok
}

func (c *GenerationContext) GetNativeArrayBridgeSpec(functionName string) (NativeArrayBridgeSpec, bool) {
	spec, ok := c.nativeArrayBridges[functionName]
	return spec, ok
}

func (c *GenerationContext) GetArrayTransformBridgeSpec(functionName string) (ArrayTransformBridgeSpec, bool) {
	spec, ok := c.arrayTransformBridges[functionName]
	spec.Params = slices.Clone(spec.Params)
	return spec, ok
}

func (c *GenerationContext) ListArrayTransformBridgeSpecs() []ArrayTransformBridgeSpec {
	specs := make([]ArrayTransformBridgeSpec, 0, len(c.arrayTransformBridges))
	for _, spec := range c.arrayTransformBridges {
		spec.Params = slices.Clone(spec.Params)
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].FunctionName < specs[j].FunctionName
	})
	return specs
}

func (c *GenerationContext) HasNativeArrayBridgeSpec(function *clang.TypedefFunction) bool {
	if function == nil {
		return false
	}
	_, ok := c.GetNativeArrayBridgeSpec(function.Name)
	return ok
}

func (c *GenerationContext) GetManagers(ast clang.CHeaderFileAST) []string {
	managerSet := make(map[string]bool)
	managers := []string{}
	for _, fn := range ast.CollectGDExtensionInterfaceFunctions() {
		managerSet[c.GetManagerName(fn.Name)] = true
	}
	delete(managerSet, "")
	delete(managerSet, "string")
	delete(managerSet, "variant")
	delete(managerSet, "global")
	for item := range managerSet {
		managers = append(managers, item)
	}
	sort.Strings(managers)
	return managers
}
