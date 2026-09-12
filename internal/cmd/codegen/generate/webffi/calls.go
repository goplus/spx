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

func (g *Generator) jsArgs(function *clang.TypedefFunction) []string {
	args := g.HighLevelArguments(function)
	result := make([]string, 0, len(args))
	for _, arg := range args {
		argName := arg.Name
		if g.isJSInt64Arg(function, arg) {
			result = append(result, argName+"_low", argName+"_high")
			continue
		}
		result = append(result, argName)
	}
	return result
}

func (g *Generator) isJSInt64Arg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	if g.IsArrayBufferArgument(function, arg) {
		return false
	}
	_, ok := jsInt64Types[common.MustPrimitiveTypeName(arg, function.Name)]
	return ok
}

type jsInt64Binding struct {
	constructor string
	split       string
}

var jsInt64Types = map[string]jsInt64Binding{
	"GdInt": {constructor: "Module['_gdspx_new_int']", split: "JsSplitGdInt"},
	"GdObj": {constructor: "Module['_gdspx_new_obj']", split: "JsSplitGdObj"},
}

func (g *Generator) jsBody(function *clang.TypedefFunction) string {
	if g.WebBinding(function.Name) == common.WebBindingNoop {
		return "return;"
	}
	if spec, ok := g.ArrayBridge(function.Name); ok {
		return jsArrayBody(function, spec)
	}
	args := common.EffectiveArguments(function)
	rawRetType := common.EffectiveRawReturnType(function)
	if len(args) == 0 && rawRetType == "" {
		return "_call();"
	}

	var sb strings.Builder
	params := make([]string, 0, len(args)+1)
	for i := range args {
		name := "_arg" + strconv.Itoa(i)
		fmt.Fprintf(&sb, "var %s;\n\t", name)
		params = append(params, name)
	}
	if rawRetType != "" {
		sb.WriteString("var _resultPtr;\n\t")
	}
	sb.WriteString("try {\n")
	if rawRetType != "" {
		fmt.Fprintf(&sb, "\t\t_resultPtr = Alloc%s();\n", rawRetType)
	}
	for i, arg := range args {
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		if g.isJSInt64Arg(function, arg) {
			fmt.Fprintf(&sb, "\t\t%s = %s(%s_high, %s_low);\n", params[i], jsInt64Types[typeName].constructor, arg.Name, arg.Name)
		} else {
			fmt.Fprintf(&sb, "\t\t%s = To%s(%s);\n", params[i], typeName, arg.Name)
		}
	}
	if rawRetType != "" {
		params = append(params, "_resultPtr")
	}
	fmt.Fprintf(&sb, "\t\t_call(%s);\n", strings.Join(params, ", "))
	if rawRetType != "" {
		name := strings.ReplaceAll(strcase.ToCamel(rawRetType), "Gd", "")
		if key, _ := g.jsResult(function); key != "" {
			fmt.Fprintf(&sb, "\t\treturn ToJs%s(_resultPtr, this._reusableResults[%q]);\n", name, key)
		} else {
			fmt.Fprintf(&sb, "\t\treturn ToJs%s(_resultPtr);\n", name)
		}
	}
	sb.WriteString("\t} finally {\n")
	for i, arg := range args {
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		fmt.Fprintf(&sb, "\t\tif (%s) Free%s(%s);\n", params[i], typeName, params[i])
	}
	if rawRetType != "" {
		fmt.Fprintf(&sb, "\t\tif (_resultPtr) Free%s(_resultPtr);\n", rawRetType)
	}
	sb.WriteString("\t}")
	return sb.String()
}

func jsArrayBody(function *clang.TypedefFunction, spec common.ArrayBridge) string {
	if spec.ReturnArray {
		return fmt.Sprintf(`var _result = TryTransformArray(_call, %s, %d, %d, %d);
	if (_result == null) {
		throw new Error(%q);
	}
	return _result;`, spec.ArgName, spec.Input.Type, spec.Output.Type, spec.Output.ElementsPerInput,
			"gd"+common.LoadProcAddressName(function.Name)+" array transform failed")
	}
	if common.HasEffectiveReturn(function) {
		panic(fmt.Sprintf("array-buffer webffi path does not support return values: %s", function.Name))
	}
	buffer := spec.CallerBuffer()
	return fmt.Sprintf(`var _arg0 = RequireNativeArray(%s, %q, %d, %t);
	var _arg1 = NativeArrayCount(%s);
	_call(_arg0, _arg1);`,
		spec.ArgName, "gd"+common.LoadProcAddressName(function.Name), buffer.Type, spec.Output != nil, spec.ArgName)
}
