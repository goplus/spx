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
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

func (g *Generator) managerBody(function *clang.TypedefFunction) string {
	var sb strings.Builder
	indent := "\t"
	args := g.Parameters(function)
	params := make([]string, 0, len(args)+1)
	returnType := g.EffectiveRawReturnType(function)
	goReturnType := g.EffectiveGoReturnType(function)
	hasSyntheticReturn := g.HasSyntheticReturn(function)
	dispatchToMainThread := function.Name != "GDExtensionSpxPlatformIsMainThread"
	if dispatchToMainThread {
		if returnType != "" {
			fmt.Fprintf(&sb, "\treturn enginewrap.CallInMainThreadValue(func() %s {\n", goReturnType)
		} else {
			sb.WriteString("\tenginewrap.CallInMainThread(func() {\n")
		}
		indent += "\t"
	}

	for _, param := range args {
		if param.IsLength {
			continue
		}
		argName := param.LocalName("arg")
		typeName := common.MustPrimitiveTypeName(param.Argument, function.Name)
		switch {
		case param.Buffer != nil:
			buffer := param.Buffer
			fmt.Fprintf(&sb, "%s%s\n", indent, buffer.GoArgumentCheck(function.Name))
			fmt.Fprintf(&sb, "%s%s := unsafe.SliceData(%s)\n", indent, argName, buffer.GoSliceExpr())
			pointer := argName
			if buffer.Type == common.ArrayObject {
				pointer = "(*GdObj)(unsafe.Pointer(" + pointer + "))"
			}
			params = append(params, pointer)
			if buffer.Count == 0 {
				length := param.LengthName("arg")
				fmt.Fprintf(&sb, "%s%s := int32(len(%s))\n", indent, length, param.Name)
				params = append(params, length)
			}
			continue
		case param.DirectScalar():
			fmt.Fprintf(&sb, "%s%s := %s\n", indent, argName, param.Name)
		case typeName == "GdString":
			fmt.Fprintf(&sb, "%s%sStr := C.CString(%s)\n", indent, argName, param.Name)
			fmt.Fprintf(&sb, "%s%s := (GdString)(%sStr)\n", indent, argName, argName)
			fmt.Fprintf(&sb, "%sdefer C.free(unsafe.Pointer(%sStr))\n", indent, argName)
		case typeName == "GdArray":
			fmt.Fprintf(&sb, "%s%sInfo := ToGdArrayInfo(%s)\n", indent, argName, param.Name)
			fmt.Fprintf(&sb, "%sif %sInfo != nil {\n%s\tdefer %sInfo.Free()\n%s}\n", indent, argName, indent, argName, indent)
			fmt.Fprintf(&sb, "%s%s := GdArray(nil)\n", indent, argName)
			fmt.Fprintf(&sb, "%sif %sInfo != nil {\n%s\t%s = %sInfo.Raw()\n%s}\n", indent, argName, indent, argName, argName, indent)
		default:
			fmt.Fprintf(&sb, "%s%s := To%s(%s)\n", indent, argName, typeName, param.Name)
		}
		params = append(params, argName)
	}

	if hasSyntheticReturn {
		fmt.Fprintf(&sb, "%svar retValue %s\n", indent, returnType)
		params = append(params, "&retValue")
	}
	sb.WriteString(indent)
	if function.ReturnType.Name != "void" {
		sb.WriteString("retValue := ")
	}
	fmt.Fprintf(&sb, "Call%s(%s)", strings.TrimPrefix(function.Name, "GDExtensionSpx"), strings.Join(params, ", "))
	if returnType != "" {
		fmt.Fprintf(&sb, "\n%sreturn To%s(retValue)", indent, strcase.ToCamel(goReturnType))
	}
	if dispatchToMainThread {
		sb.WriteString("\n\t})")
	}
	return sb.String()
}
