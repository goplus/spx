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
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

func (g *Generator) managerBody(function *clang.TypedefFunction) string {
	var sb strings.Builder
	args := common.EffectiveArguments(function)
	params := make([]string, 0, len(args))
	for i, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		argName := "arg" + strconv.Itoa(i)
		source := g.EffectiveGoArgumentName(function, arg)
		typeName := "GdArray"
		if !g.IsArrayBufferArgument(function, arg) {
			typeName = common.MustPrimitiveTypeName(arg, function.Name)
		}
		if binding, ok := jsInt64Types[typeName]; ok {
			low, high := argName+"Low", argName+"High"
			fmt.Fprintf(&sb, "\t%s, %s := %s(%s)\n", low, high, binding.split, source)
			params = append(params, low, high)
		} else {
			fmt.Fprintf(&sb, "\t%s := JsFrom%s(%s)\n", argName, typeName, source)
			params = append(params, argName)
		}
	}

	sb.WriteByte('\t')
	if common.HasEffectiveReturn(function) {
		sb.WriteString("_result := ")
	}
	fmt.Fprintf(&sb, "API.Spx%s.Invoke(%s)", strings.TrimPrefix(function.Name, "GDExtensionSpx"), strings.Join(params, ", "))
	if common.HasEffectiveReturn(function) {
		name := strcase.ToCamel(common.EffectiveRawReturnType(function))
		if name == "GdObj" {
			name = "GdObject"
		}
		fmt.Fprintf(&sb, "\n\treturn JsTo%s(_result)", name)
	}
	return g.wrapCache(function, sb.String())
}
