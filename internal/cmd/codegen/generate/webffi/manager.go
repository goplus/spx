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

func (g *Generator) getManagerFuncBody(function *clang.TypedefFunction) string {
	if body, ok := getInputCacheManagerFuncBody(function.Name); ok {
		return body
	}

	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := common.EffectiveArguments(function)
	// convert arguments
	for i, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		sb.WriteString(prefixTab)
		if g.IsNativeArrayDataArg(function, arg) {
			argName := "arg" + strconv.Itoa(i)
			sb.WriteString(argName)
			sb.WriteString(" := JsFromGdArray(")
			sb.WriteString(g.EffectiveGoArgumentName(function, arg))
			sb.WriteString(")\n")
			params = append(params, argName)
			continue
		}
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		if g.usesFlatJsGdIntArg(function, arg) {
			argName := "arg" + strconv.Itoa(i)
			lowName := argName + "Low"
			highName := argName + "High"
			fmt.Fprintf(&sb, "%s, %s := ", lowName, highName)
			sb.WriteString(flatJsSplitHelper(typeName))
			sb.WriteString("(")
			sb.WriteString(g.EffectiveGoArgumentName(function, arg))
			sb.WriteString(")\n")
			params = append(params, lowName, highName)
			continue
		}
		argName := "arg" + strconv.Itoa(i)
		sb.WriteString(argName)
		sb.WriteString(" := JsFrom")
		sb.WriteString(typeName)
		sb.WriteString("(")
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(")")

		sb.WriteString("\n")
		params = append(params, argName)
	}

	// call the function
	sb.WriteString(prefixTab)
	if common.HasEffectiveReturn(function) {
		sb.WriteString("_retValue := ")
	}

	funcName := "API.Spx" + (strings.TrimPrefix(function.Name, "GDExtensionSpx"))
	sb.WriteString(funcName)
	sb.WriteString(".Invoke(")
	sb.WriteString(strings.Join(params, ", "))
	sb.WriteString(")")

	if common.HasEffectiveReturn(function) {
		sb.WriteByte('\n')
		sb.WriteString(prefixTab)
		sb.WriteString("return ")
		typeName := common.EffectiveRawReturnType(function)
		name := strcase.ToCamel(typeName)
		if name == "GdObj" {
			name = "GdObject"
		}
		fmt.Fprintf(&sb, "JsTo%s(_retValue)", name)
	}
	return sb.String()
}
