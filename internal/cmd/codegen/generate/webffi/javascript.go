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

func (g *Generator) getJsFuncArgs(function *clang.TypedefFunction) []string {
	args := common.EffectiveArguments(function)
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		argName := g.EffectiveGoArgumentName(function, arg)
		if g.usesFlatJsGdIntArg(function, arg) {
			result = append(result, argName+"_low", argName+"_high")
			continue
		}
		result = append(result, argName)
	}
	return result
}

func (g *Generator) usesFlatJsGdIntArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	if g.IsNativeArrayDataArg(function, arg) {
		return false
	}
	return isFlatJsGdIntLikeType(common.MustPrimitiveTypeName(arg, function.Name))
}

func isFlatJsGdIntLikeType(typeName string) bool {
	switch typeName {
	case "GdInt", "GdObj":
		return true
	default:
		return false
	}
}

func flatJsCtor(typeName string) string {
	switch typeName {
	case "GdInt":
		return "Module['_gdspx_new_int']"
	case "GdObj":
		return "Module['_gdspx_new_obj']"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func flatJsSplitHelper(typeName string) string {
	switch typeName {
	case "GdInt":
		return "JsSplitGdInt"
	case "GdObj":
		return "JsSplitGdObj"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func flatJsScratchAccessor(typeName string) string {
	switch typeName {
	case "GdInt":
		return "this._getGdIntScratch()"
	case "GdObj":
		return "this._getGdObjScratch()"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func (g *Generator) getJsFuncBody(function *clang.TypedefFunction) string {
	if function.Name == "GDExtensionSpxResFreeStr" {
		// Web strings are values; the wrapper owns its storage.
		return "// Web strings are value-owned; there is no pointer to release.\n\treturn;"
	}
	if spec, ok := g.GetArrayTransformBridgeSpec(function.Name); ok {
		return "var _fastRetValue = TryArrayTransformFastPath(_gdFuncPtr, " + spec.ArrayArgName + ", " +
			strconv.Itoa(int(spec.InputArrayType)) + ", " + strconv.Itoa(int(spec.OutputArrayType)) + ", " +
			strconv.Itoa(spec.OutputCountScale) + ");\n" +
			"\tif (_fastRetValue == null) {\n" +
			"\t\tthrow new Error(\"gd" + common.LoadProcAddressName(function.Name) + " fast path unavailable\");\n" +
			"\t}\n" +
			"\treturn _fastRetValue"
	}
	if function.Name == "GDExtensionSpxInputWriteSnapshot" {
		return "var _arg0 = RequireWasmFastArray(out, \"gdspx_input_write_snapshot\", 2);\n" +
			"\tvar _arg1 = FastArrayCount(out);\n" +
			"\t_gdFuncPtr(_arg0, _arg1);"
	}
	if spec, ok := g.GetNativeArrayBridgeSpec(function.Name); ok {
		if common.HasEffectiveReturn(function) {
			panic(fmt.Sprintf("native-array webffi path does not support return values: %s", function.Name))
		}
		argName := spec.BaseArgName
		return "var _arg0 = RequireFastArray(" + argName + ", \"gd" + common.LoadProcAddressName(function.Name) + "\", " + strconv.Itoa(int(spec.FastArrayType)) + ");\n" +
			"\tvar _arg1 = FastArrayCount(" + argName + ");\n" +
			"\t_gdFuncPtr(_arg0, _arg1);"
	}
	if function.Name == "GDExtensionSpxInputGetGlobalMousePos" {
		return "var _retValue = AllocGdVec2();\n" +
			"\t_gdFuncPtr(_retValue);\n" +
			"\tvar _scratch = this._inputMousePosScratch;\n" +
			"\tvar _floatIndex = _retValue / 4;\n" +
			"\tvar _heap = Module['HEAPF32'];\n" +
			"\t_scratch['x'] = _heap[_floatIndex];\n" +
			"\t_scratch['y'] = _heap[_floatIndex + 1];\n" +
			"\tFreeGdVec2(_retValue);\n" +
			"\treturn _scratch"
	}
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := common.EffectiveArguments(function)
	rawRetType := common.EffectiveRawReturnType(function)

	// call the function
	if rawRetType != "" {
		fmt.Fprintf(&sb, "var _retValue = Alloc%s();", rawRetType)
	}
	sb.WriteString("\n")

	// convert arguments
	for i, arg := range args {
		sb.WriteString(prefixTab)
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		argName := "_arg" + strconv.Itoa(i)
		if g.usesFlatJsGdIntArg(function, arg) {
			fmt.Fprintf(&sb, "var %s = ", argName)
			sb.WriteString(flatJsCtor(typeName))
			sb.WriteString("(")
			fmt.Fprintf(&sb, "%s_high, %s_low", arg.Name, arg.Name)
			sb.WriteString(");")

			sb.WriteString("\n")
			params = append(params, argName)
			continue
		}
		fmt.Fprintf(&sb, "var %s = ", argName)
		sb.WriteString("To")
		sb.WriteString(typeName)
		sb.WriteString("(")
		sb.WriteString(arg.Name)
		sb.WriteString(");")

		sb.WriteString("\n")
		params = append(params, argName)
	}
	sb.WriteString(prefixTab)
	sb.WriteString("_gdFuncPtr(")
	sb.WriteString(strings.Join(params, ", "))
	if rawRetType != "" {
		if len(params) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("_retValue")
	}
	sb.WriteString(");")

	sb.WriteString("\n")
	// convert arguments
	for i, arg := range args {
		sb.WriteString(prefixTab)
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		argName := "_arg" + strconv.Itoa(i)
		fmt.Fprintf(&sb, "Free%s(%s); \n", typeName, argName)
	}

	if rawRetType != "" {
		sb.WriteString(prefixTab)
		sb.WriteString("var _finalRetValue = ")
		if isFlatJsGdIntLikeType(rawRetType) {
			sb.WriteString("this._readGdIntLike(_retValue, ")
			sb.WriteString(flatJsScratchAccessor(rawRetType))
			sb.WriteString(");\n")
		} else {
			typeName := rawRetType
			funcName := strcase.ToCamel(typeName)
			funcName = "ToJs" + strings.ReplaceAll(funcName, "Gd", "")
			sb.WriteString(funcName)
			sb.WriteString("(_retValue);\n")
		}
		fmt.Fprintf(&sb, "%sFree%s(_retValue); \n", prefixTab, rawRetType)
		sb.WriteString(prefixTab)
		sb.WriteString("return _finalRetValue")
	}
	return sb.String()
}
