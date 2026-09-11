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
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
)

func (g *Generator) getManagerImplPure(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := common.EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)
	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType)
		sb.WriteByte(' ')
	}
	sb.WriteString("{\n")
	if retType != "" {
		fmt.Fprintf(&sb, "\treturn %s\n", goZeroValue(retType))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (g *Generator) getManagerImpl(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := common.EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	hasObjArg := len(args) > 0 && args[0].Name == "obj"

	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType)
		sb.WriteByte(' ')
	}
	sb.WriteString("{\n\t")
	if retType != "" {
		sb.WriteString("return ")
	}
	fmt.Fprintf(&sb, "%sMgr.%s(", mgrName, funcName)
	wroteCallArg := false
	if hasObjArg {
		sb.WriteString("pself.Id")
		wroteCallArg = true
	}
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteCallArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		wroteCallArg = true
	}
	sb.WriteString(")\n}\n")
	return sb.String()
}
