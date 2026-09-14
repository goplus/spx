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

func (g *Generator) getManagerImplPure(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := g.Parameters(function)
	retType := g.EffectiveGoReturnType(function)
	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" && arg.Buffer == nil {
			continue
		}
		if arg.IsLength {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := arg.MustGoType(function.Name)
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
	args := g.Parameters(function)
	retType := g.EffectiveGoReturnType(function)

	hasObjArg := len(args) > 0 && args[0].Name == "obj" && args[0].Buffer == nil

	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" && arg.Buffer == nil {
			continue
		}
		if arg.IsLength {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := arg.MustGoType(function.Name)
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
		if i == 0 && arg.Name == "obj" && arg.Buffer == nil {
			continue
		}
		if arg.IsLength {
			continue
		}
		if wroteCallArg {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name)
		wroteCallArg = true
	}
	sb.WriteString(")\n}\n")
	return sb.String()
}
