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
	returnType := g.EffectiveRawReturnType(function)
	for _, param := range args {
		if param.IsLength {
			continue
		}
		argName, source := param.LocalName("arg"), param.Name
		switch {
		case param.Buffer != nil:
			buffer := param.Buffer
			source = buffer.GoSliceExpr()
			fmt.Fprintf(&sb, "\t%s\n", buffer.GoArgumentCheck(function.Name))
			if buffer.OutputOnly {
				fmt.Fprintf(&sb, "\t%s := JsAllocNativeArray(%s, len(%s))\n", argName, buffer.Type.GoConstant(), source)
			} else {
				fmt.Fprintf(&sb, "\t%s := JsFromNativeArray(%s, %s)\n", argName, source, buffer.Type.GoConstant())
			}
			if buffer.Writable() {
				outputs = append(outputs, fmt.Sprintf("\n\tCopyNativeArrayOutput(%s, %s)", source, argName))
			}
		case param.DirectScalar():
			fmt.Fprintf(&sb, "\t%s := %s\n", argName, source)
		default:
			typeName := common.MustPrimitiveTypeName(param.Argument, function.Name)
			if binding, ok := jsInt64Types[typeName]; ok {
				low, high := argName+"Low", argName+"High"
				fmt.Fprintf(&sb, "\t%s, %s := %s(%s)\n", low, high, binding.split, source)
				params = append(params, low, high)
				continue
			}
			fmt.Fprintf(&sb, "\t%s := JsFrom%s(%s)\n", argName, typeName, source)
		}
		params = append(params, argName)
	}

	sb.WriteByte('\t')
	if returnType != "" {
		sb.WriteString("_result := ")
	}
	fmt.Fprintf(&sb, "API.Spx%s.Invoke(%s)", strings.TrimPrefix(function.Name, "GDExtensionSpx"), strings.Join(params, ", "))
	outputStatus := g.HasOutputStatus(function)
	if outputStatus {
		sb.WriteString("\n\tif !JsToGdBool(_result) { return false }")
	}
	sb.WriteString(strings.Join(outputs, ""))
	if outputStatus {
		sb.WriteString("\n\treturn true")
	} else if returnType != "" {
		name := strcase.ToCamel(returnType)
		if name == "GdObj" {
			name = "GdObject"
		}
		fmt.Fprintf(&sb, "\n\treturn JsTo%s(_result)", name)
	}
	return g.wrapCache(function, sb.String())
}
