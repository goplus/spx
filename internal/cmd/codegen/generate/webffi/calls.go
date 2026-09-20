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

type jsInt64Binding struct {
	constructor string
	split       string
}

var jsInt64Types = map[string]jsInt64Binding{
	"GdInt": {constructor: "Module['_gdspx_new_int']", split: "JsSplitGdInt"},
	"GdObj": {constructor: "Module['_gdspx_new_obj']", split: "JsSplitGdObj"},
}

func (g *Generator) jsArgs(function *clang.TypedefFunction) []string {
	var result []string
	for _, param := range g.Parameters(function) {
		if param.IsLength {
			continue
		}
		if param.Buffer == nil {
			if _, ok := jsInt64Types[common.MustPrimitiveTypeName(param.Argument, function.Name)]; ok {
				result = append(result, param.Name+"_low", param.Name+"_high")
				continue
			}
		}
		result = append(result, param.Name)
	}
	return result
}

func (g *Generator) jsBody(function *clang.TypedefFunction) string {
	params := g.Parameters(function)
	callArgs := make([]string, len(params))
	var declarations, statements, cleanup []string
	rawRetType := g.EffectiveRawReturnType(function)
	if rawRetType != "" {
		statements = append(statements, fmt.Sprintf("_resultPtr = Alloc%s();", rawRetType))
	}
	for _, param := range params {
		local := param.LocalName("_arg")
		callArgs[param.Index] = local
		if param.IsLength {
			continue
		}
		if buffer := param.Buffer; buffer != nil {
			op := "gd" + common.LoadProcAddressName(function.Name)
			statements = append(statements, fmt.Sprintf("var %s = RequireNativeArray(%s, %q, %d, %t);", local, param.Name, op, buffer.Type, buffer.Writable()))
			if param.LengthIndex >= 0 {
				statements = append(statements, fmt.Sprintf("var %s = NativeArrayCount(%s);", param.LengthName("_arg"), param.Name))
			} else {
				comparison, message := "<", " array is too small: "
				if buffer.OutputOnly {
					comparison, message = "!==", " output array length must match its declaration: "
				}
				statements = append(statements, fmt.Sprintf("if (NativeArrayCount(%s) %s %d) {\n\tthrow new Error(%q);\n}", param.Name, comparison, buffer.Count, op+message+param.Name))
			}
			continue
		}
		typeName := common.MustPrimitiveTypeName(param.Argument, function.Name)
		if param.WebScalarByValue() {
			callArgs[param.Index] = param.Name
			if typeName == "GdBool" {
				callArgs[param.Index] += " ? 1 : 0"
			}
			continue
		}
		declarations = append(declarations, "var "+local+";")
		if binding, ok := jsInt64Types[typeName]; ok {
			statements = append(statements, fmt.Sprintf("%s = %s(%s_high, %s_low);", local, binding.constructor, param.Name, param.Name))
		} else {
			statements = append(statements, fmt.Sprintf("%s = To%s(%s);", local, typeName, param.Name))
		}
		cleanup = append(cleanup, fmt.Sprintf("if (%s) Free%s(%s);", local, typeName, local))
	}
	if rawRetType != "" {
		declarations = append(declarations, "var _resultPtr;")
		callArgs = append(callArgs, "_resultPtr")
	}
	statements = append(statements, fmt.Sprintf("_call(%s);", strings.Join(callArgs, ", ")))
	if rawRetType != "" {
		name := strings.TrimPrefix(strcase.ToCamel(rawRetType), "Gd")
		if key, _ := g.jsResult(function); key != "" {
			statements = append(statements, fmt.Sprintf("return ToJs%s(_resultPtr, this._reusableResults[%q]);", name, key))
		} else {
			statements = append(statements, fmt.Sprintf("return ToJs%s(_resultPtr);", name))
		}
		cleanup = append(cleanup, fmt.Sprintf("if (_resultPtr) Free%s(_resultPtr);", rawRetType))
	}
	body := strings.Join(statements, "\n")
	if len(cleanup) > 0 {
		body = strings.Join(declarations, "\n") + "\ntry {\n\t" + strings.ReplaceAll(body, "\n", "\n\t") + "\n} finally {\n\t" + strings.Join(cleanup, "\n\t") + "\n}"
	}
	return strings.ReplaceAll(body, "\n", "\n\t")
}
