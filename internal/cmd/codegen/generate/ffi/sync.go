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

	"github.com/iancoleman/strcase"
)

func (g *Generator) MustGdxReturnType(function *clang.TypedefFunction) string {
	typeName := g.EffectiveGoReturnType(function)
	if typeName == "Object" {
		return "gdx.Object"
	}
	if typeName == "Array" {
		return "gdx.Array"
	}
	return typeName
}

// syncSignature is shared by the forwarding and pure-engine implementations.
func (g *Generator) syncSignature(function *clang.TypedefFunction) string {
	mgrName := g.GetManagerName(function.Name)
	methodName := function.Name[len("GDExtensionSpx")+len(strcase.ToCamel(mgrName)):]
	var params []string
	for _, param := range g.Parameters(function) {
		if !param.IsLength {
			params = append(params, param.Name+" "+param.GDXType(function.Name))
		}
	}
	signature := fmt.Sprintf("func (*%sMgrImpl) %s(%s)", strcase.ToLowerCamel(mgrName), methodName, strings.Join(params, ", "))
	if returnType := g.MustGdxReturnType(function); returnType != "" {
		signature += " " + returnType
	}
	return signature
}

func (g *Generator) genSyncPureAPIWrapFunction(function *clang.TypedefFunction) string {
	body := g.syncSignature(function) + " {"
	if returnType := g.MustGdxReturnType(function); returnType != "" {
		body += "\n\treturn " + goZeroValue(returnType) + "\n"
	}
	return body + "}"
}

func (g *Generator) genSyncAPIWrapFunction(function *clang.TypedefFunction) string {
	if g.IsStringRelease(function) {
		return g.syncSignature(function) + " {}"
	}
	var sb strings.Builder
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	methodName := function.Name[len("GDExtensionSpx")+len(mgrName):]
	returnType := g.MustGdxReturnType(function)
	var args []string
	for _, param := range g.Parameters(function) {
		if !param.IsLength {
			args = append(args, param.Name)
		}
	}

	sb.WriteString(g.syncSignature(function))
	sb.WriteString(" {")
	if returnType != "" {
		fmt.Fprintf(&sb, "\n\tvar _ret1 %s", returnType)
	}
	sb.WriteString("\t\n\tcallInMainThread(func() {\n\t\t")
	if returnType != "" {
		sb.WriteString("_ret1 = ")
	}
	fmt.Fprintf(&sb, "gdx.%sMgr.%s(%s)\n\t})\n", mgrName, methodName, strings.Join(args, ", "))
	if returnType != "" {
		sb.WriteString("\treturn _ret1\n")
	}
	sb.WriteString("}")
	return sb.String()
}

func goZeroValue(typeName string) string {
	switch typeName {
	case "bool":
		return "false"
	case "int64", "float64", "Object", "gdx.Object":
		return "0"
	case "string":
		return `""`
	case "Array", "gdx.Array":
		return "nil"
	default:
		return typeName + "{}"
	}
}
