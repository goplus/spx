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
	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

// jsResult returns a reuse key and initializer, or empty strings for fresh results.
// Int64 values share a slot per type; annotated values use one per API.
func (g *Generator) jsResult(function *clang.TypedefFunction) (key, initializer string) {
	typeName := common.EffectiveRawReturnType(function)
	if _, ok := jsInt64Types[typeName]; ok {
		return typeName, "{ 'low': 0, 'high': 0 }"
	}
	if g.WebBinding(function.Name) != common.WebBindingReuseResult {
		return "", ""
	}
	key = "gd" + common.LoadProcAddressName(function.Name)
	if typeName == "GdRect2" {
		return key, "{ 'position': {}, 'size': {} }"
	}
	return key, "{}"
}

func (g *Generator) jsResults() map[string]string {
	results := make(map[string]string)
	for _, function := range g.AST().CollectGDExtensionInterfaceFunctions() {
		if key, initializer := g.jsResult(&function); key != "" {
			results[key] = initializer
		}
	}
	return results
}
