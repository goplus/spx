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

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"

	"github.com/iancoleman/strcase"
)

func (g *Generator) genManagerWrapFunction(function *clang.TypedefFunction) string {
	var sb strings.Builder
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	methodName := function.Name[len("GDExtensionSpx")+len(mgrName):]
	receiver := g.ReceiverName(function, mgrName)
	returnType := g.EffectiveGoReturnType(function)
	if returnType == "Object" || returnType == "Array" {
		returnType = "gdx." + returnType
	}
	var params, args []string
	for _, param := range g.Parameters(function) {
		if !param.IsLength {
			params = append(params, param.Name+" "+param.GDXType(function.Name))
			args = append(args, param.Name)
		}
	}

	fmt.Fprintf(&sb, "func (%s *%sMgrImpl) %s(%s)", receiver, mgrName, methodName, strings.Join(params, ", "))
	if returnType != "" {
		fmt.Fprintf(&sb, " %s", returnType)
	}
	sb.WriteString(" {\n")
	binding := g.LocalName(function, "binding")
	call := fmt.Sprintf("%s.target().%s(%s)", binding, methodName, strings.Join(args, ", "))
	// Capture and validate before dispatch, then recheck the same binding inside.
	fmt.Fprintf(&sb, "\t%s := %s.resolve()\n", binding, receiver)
	if function.Name == "GDExtensionSpxPlatformIsMainThread" {
		fmt.Fprintf(&sb, "\treturn %s\n}", call)
		return sb.String()
	}
	if returnType == "" {
		fmt.Fprintf(&sb, "\tCallInMainThread(func() {\n\t\t%s\n\t})\n}", call)
	} else {
		fmt.Fprintf(&sb, "\treturn CallInMainThreadValue(func() %s {\n\t\treturn %s\n\t})\n}", returnType, call)
	}
	return sb.String()
}
