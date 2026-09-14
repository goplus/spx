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
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

func (g *Generator) managerBody(function *clang.TypedefFunction) string {
	var sb strings.Builder
	args := g.Parameters(function)
	params := make([]string, 0, len(args))
	var outputs []string
	for _, param := range args {
		arg := param.Argument
		if param.IsLength {
			continue
		}
		argName := param.LocalName("arg")
		source := param.Name
		if buffer := param.Buffer; buffer != nil {
			fmt.Fprintf(&sb, "\t%s\n", buffer.GoArgumentCheck(function.Name))
			source = buffer.GoSliceExpr()
			if buffer.Writable() {
				outputs = append(outputs, fmt.Sprintf("\n\tCopyNativeArrayOutput(%s, %s)", source, argName))
			}
			fmt.Fprintf(&sb, "\t%s := JsFromNativeArray(%s, %d)\n", argName, source, buffer.Type)
			params = append(params, argName)
			continue
		}
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		if param.DirectScalar() {
			fmt.Fprintf(&sb, "\t%s := %s\n", argName, source)
			params = append(params, argName)
		} else if binding, ok := jsInt64Types[typeName]; ok {
			low, high := argName+"Low", argName+"High"
			fmt.Fprintf(&sb, "\t%s, %s := %s(%s)\n", low, high, binding.split, source)
			params = append(params, low, high)
		} else {
			fmt.Fprintf(&sb, "\t%s := JsFrom%s(%s)\n", argName, typeName, source)
			params = append(params, argName)
		}
	}

	sb.WriteByte('\t')
	if g.HasEffectiveReturn(function) {
		sb.WriteString("_result := ")
	}
	fmt.Fprintf(&sb, "API.Spx%s.Invoke(%s)", strings.TrimPrefix(function.Name, "GDExtensionSpx"), strings.Join(params, ", "))
	sb.WriteString(strings.Join(outputs, ""))
	if g.HasEffectiveReturn(function) {
		name := strcase.ToCamel(g.EffectiveRawReturnType(function))
		if name == "GdObj" {
			name = "GdObject"
		}
		fmt.Fprintf(&sb, "\n\treturn JsTo%s(_result)", name)
	}
	return g.wrapCache(function, sb.String())
}
