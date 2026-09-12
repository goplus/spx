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

package ffi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

func (g *Generator) managerBody(function *clang.TypedefFunction) string {
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := common.EffectiveArguments(function)
	hasSyntheticReturn := function.ReturnType.Name == "void" && common.HasEffectiveReturn(function)
	dispatchToMainThread := function.Name != "GDExtensionSpxPlatformIsMainThread"
	if dispatchToMainThread {
		if common.HasEffectiveReturn(function) {
			fmt.Fprintf(&sb, "\treturn enginewrap.CallInMainThreadValue(func() %s {\n", g.EffectiveGoReturnType(function))
		} else {
			sb.WriteString("\tenginewrap.CallInMainThread(func() {\n")
		}
		prefixTab += "\t"
	}
	// convert arguments
	for i, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		sb.WriteString(prefixTab)
		typeName := common.MustPrimitiveTypeName(arg, function.Name)
		argName := "arg" + strconv.Itoa(i)
		if g.IsArrayBufferArgument(function, arg) {
			spec, _ := g.ArrayBridge(function.Name)
			goArgName := g.EffectiveGoArgumentName(function, arg)
			fmt.Fprintf(&sb, "var %s %s\n", argName, spec.CallerBuffer().GoPointerType())
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if len(%s) > 0 {\n", goArgName)
			fmt.Fprintf(&sb, "%s\t%s = &%s[0]\n", prefixTab, argName, goArgName)
			sb.WriteString(prefixTab)
			sb.WriteString("}\n")
			lenArgName := "arg" + strconv.Itoa(i+1)
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "%s := %s", lenArgName, common.ArrayLengthExpr(goArgName))
			params = append(params, argName, lenArgName)
			sb.WriteString("\n")
			continue
		}
		switch typeName {
		case "GdString":
			sb.WriteString(argName)
			sb.WriteString("Str := C.CString(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteByte('\n')
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "%s := (GdString)(%sStr) \n", argName, argName)
			fmt.Fprintf(&sb, "%sdefer C.free(unsafe.Pointer(%sStr))", prefixTab, argName)
		case "GdArray":
			sb.WriteString(argName)
			sb.WriteString("Info := ToGdArrayInfo(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteByte('\n')
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if %sInfo != nil {\n", argName)
			fmt.Fprintf(&sb, "%s\tdefer %sInfo.Free()\n", prefixTab, argName)
			sb.WriteString(prefixTab)
			sb.WriteString("}\n")
			sb.WriteString(prefixTab)
			sb.WriteString(argName)
			sb.WriteString(" := GdArray(nil)\n")
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if %sInfo != nil {\n", argName)
			fmt.Fprintf(&sb, "%s\t%s = %sInfo.Raw()\n", prefixTab, argName, argName)
			sb.WriteString(prefixTab)
			sb.WriteByte('}')

		default:
			sb.WriteString(argName)
			sb.WriteString(" := To")
			sb.WriteString(typeName)
			sb.WriteString("(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
		}
		sb.WriteString("\n")
		params = append(params, argName)
	}

	// call the function
	funcName := "Call" + strings.TrimPrefix(function.Name, "GDExtensionSpx")
	if hasSyntheticReturn {
		rawType := common.EffectiveRawReturnType(function)
		sb.WriteString(prefixTab)
		fmt.Fprintf(&sb, "var retValue %s\n", rawType)
	}

	sb.WriteString(prefixTab)
	if function.ReturnType.Name != "void" {
		sb.WriteString("retValue := ")
	}
	sb.WriteString(funcName)
	sb.WriteString("(")
	sb.WriteString(strings.Join(params, ", "))
	if hasSyntheticReturn {
		if len(params) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("&retValue")
	}
	sb.WriteString(")")

	if common.HasEffectiveReturn(function) {
		sb.WriteByte('\n')
		sb.WriteString(prefixTab)
		sb.WriteString("return ")
		typeName := g.EffectiveGoReturnType(function)
		fmt.Fprintf(&sb, "To%s(retValue)", strcase.ToCamel(typeName))
	}
	if dispatchToMainThread {
		sb.WriteString("\n\t})")
	}
	return sb.String()
}
