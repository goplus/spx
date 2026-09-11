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

func (g *Generator) genSyncPureAPIWrapFunction(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	mgrTypeName := strcase.ToLowerCamel(g.GetManagerName(function.Name)) + "Mgr"
	args := common.EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	fmt.Fprintf(&sb, "func (*%sImpl) ", mgrTypeName)
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteByte(' ')
		sb.WriteString(g.MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	if retType != "" {
		fmt.Fprintf(&sb, "\n%sreturn %s\n", prefixStr, goZeroValue(g.MustGdxReturnType(function)))
	}
	sb.WriteString("}")
	return sb.String()
}

func (g *Generator) genSyncAPIWrapFunction(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	gdxMgrName := "gdx." + mgrName + "Mgr"
	mgrTypeName := strcase.ToLowerCamel(g.GetManagerName(function.Name)) + "Mgr"
	args := common.EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	fmt.Fprintf(&sb, "func (*%sImpl) ", mgrTypeName)
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteByte(' ')
		sb.WriteString(g.MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	if retType != "" {
		fmt.Fprintf(&sb, "\n%svar _ret1 %s", prefixStr, g.MustGdxReturnType(function))
	}

	sb.WriteString("\t\n\tcallInMainThread(func() {\n")
	if retType != "" {
		sb.WriteString(prefixStr)
		sb.WriteString("\t_ret1 = ")
	} else {
		sb.WriteString(prefixStr)
		sb.WriteByte('\t')
	}
	fmt.Fprintf(&sb, "%s.%s(", gdxMgrName, pureFuncName)
	wroteArg = false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		wroteArg = true
	}
	sb.WriteString(")")

	sb.WriteString(`
	})
`)

	if retType != "" {
		sb.WriteString(prefixStr)
		sb.WriteString("return _ret1 \n")
	}
	sb.WriteString("}")
	return sb.String()
}
