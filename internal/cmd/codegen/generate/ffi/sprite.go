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
	"unicode"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

func (g *Generator) getManagerImpl(function *clang.TypedefFunction, className string, pure bool) string {
	lowerManagerName := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowerManagerName[0]))) + lowerManagerName[1:]
	funcName := function.Name[len("GDExtensionSpx")+len(mgrName):]
	retType := g.EffectiveGoReturnType(function)
	var params, callArgs []string
	for i, arg := range g.Parameters(function) {
		if arg.IsLength {
			continue
		}
		if i == 0 && arg.Name == "obj" && arg.Buffer == nil {
			callArgs = append(callArgs, "pself.Id")
			continue
		}
		params = append(params, arg.Name+" "+arg.MustGoType(function.Name))
		callArgs = append(callArgs, arg.Name)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "func (pself *%s) %s(%s)", className, funcName, strings.Join(params, ", "))
	if retType != "" {
		fmt.Fprintf(&sb, " %s", retType)
	}
	sb.WriteString(" {\n")
	if pure {
		if retType != "" {
			fmt.Fprintf(&sb, "\treturn %s\n", goZeroValue(retType))
		}
	} else {
		sb.WriteByte('\t')
		if retType != "" {
			sb.WriteString("return ")
		}
		fmt.Fprintf(&sb, "%sMgr.%s(%s)\n", mgrName, funcName, strings.Join(callArgs, ", "))
	}
	sb.WriteString("}\n")
	return sb.String()
}
