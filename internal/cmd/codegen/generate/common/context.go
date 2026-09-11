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
	"sort"
	"strings"
	"unicode"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

// GenerationContext owns the metadata for one generation task.
// Create a fresh context for each independent input; no state is shared across tasks.
type GenerationContext struct {
	managerSet            map[string]bool
	cppType2Go            map[string]string
	KnownManagerNames     []string
	nativeArrayBridges    map[string]NativeArrayBridgeSpec
	arrayTransformBridges map[string]ArrayTransformBridgeSpec
}

type ManagerData struct {
	Ast               clang.CHeaderFileAST
	Managers          []string
	KnownManagerNames []string
}

func NewGenerationContext() *GenerationContext {
	return &GenerationContext{
		managerSet:            make(map[string]bool),
		nativeArrayBridges:    make(map[string]NativeArrayBridgeSpec),
		arrayTransformBridges: make(map[string]ArrayTransformBridgeSpec),
		cppType2Go: map[string]string{
			"GdInt": "int64", "GdFloat": "float64", "GdObj": "Object",
			"GdVec2": "Vec2", "GdVec3": "Vec3", "GdVec4": "Vec4",
			"GdRect2": "Rect2", "GdString": "string", "GdBool": "bool",
			"GdColor": "Color", "GdArray": "Array",
		},
	}
}

// PrepareAST replaces the manager membership used by template predicates.
func (c *GenerationContext) PrepareAST(ast clang.CHeaderFileAST) {
	c.managerSet = make(map[string]bool)
	for _, name := range c.GetManagers(ast) {
		c.managerSet[name] = true
	}
}

// RegisterManagerName registers a known manager name (obtained from header parsing).
func (c *GenerationContext) RegisterManagerName(name string) {
	name = strings.ToLower(name)
	// Avoid duplicate entries
	for _, n := range c.KnownManagerNames {
		if n == name {
			return
		}
	}
	c.KnownManagerNames = append(c.KnownManagerNames, name)
}

// ClearKnownManagerNames clears the list of known manager names.
func (c *GenerationContext) ClearKnownManagerNames() {
	c.KnownManagerNames = []string{}
}

func (c *GenerationContext) ClearNativeArrayBridgeSpecs() {
	c.nativeArrayBridges = map[string]NativeArrayBridgeSpec{}
}

func (c *GenerationContext) ClearArrayTransformBridgeSpecs() {
	c.arrayTransformBridges = map[string]ArrayTransformBridgeSpec{}
}

func (c *GenerationContext) RegisterNativeArrayBridgeSpec(spec NativeArrayBridgeSpec) {
	c.nativeArrayBridges[spec.BaseFunctionName] = spec
}

func (c *GenerationContext) RegisterArrayTransformBridgeSpec(spec ArrayTransformBridgeSpec) {
	c.arrayTransformBridges[spec.FunctionName] = spec
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
	return spec, ok
}

func (c *GenerationContext) ListArrayTransformBridgeSpecs() []ArrayTransformBridgeSpec {
	specs := make([]ArrayTransformBridgeSpec, 0, len(c.arrayTransformBridges))
	for _, spec := range c.arrayTransformBridges {
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

func (c *GenerationContext) GetManagerName(str string) string {
	prefix := "GDExtensionSpx"
	str = str[len(prefix):]
	lowerStr := strings.ToLower(str)

	// Match the longest known name without reordering KnownManagerNames.
	if len(c.KnownManagerNames) > 0 {
		sortedNames := make([]string, len(c.KnownManagerNames))
		copy(sortedNames, c.KnownManagerNames)
		sort.Slice(sortedNames, func(i, j int) bool {
			return len(sortedNames[i]) > len(sortedNames[j])
		})

		for _, mgr := range sortedNames {
			if strings.HasPrefix(lowerStr, mgr) {
				return mgr
			}
		}
	}

	// Otherwise, keep the first two bytes and stop at the next uppercase rune.
	chs := []rune{rune(str[0]), rune(str[1])}
	for _, ch := range str[2:] {
		if unicode.IsUpper(ch) {
			break
		}
		chs = append(chs, ch)
	}
	return strings.ToLower(string(chs))
}

func (c *GenerationContext) IsManagerMethod(function *clang.TypedefFunction) bool {
	return c.managerSet[c.GetManagerName(function.Name)]
}

func (c *GenerationContext) GetManagers(ast clang.CHeaderFileAST) []string {
	items := []string{}
	for _, item := range ast.CollectGDExtensionInterfaceFunctions() {
		items = append(items, item.Name)
	}
	managerSet := make(map[string]bool)
	managers := []string{}
	for _, str := range items {
		managerSet[c.GetManagerName(str)] = true
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
