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

package common

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
)

// ManagerTemplate is the shared manager registration and method skeleton.
//
//go:embed manager.go.tmpl
var ManagerTemplate string

// ManagerMethodSignature renders a manager method with its receiver.
func (c *GenerationContext) ManagerMethodSignature(function *clang.TypedefFunction) string {
	mgrName := c.GetManagerName(function.Name)
	return "(" + c.ReceiverName(function, mgrName) + " *" + mgrName + "Mgr) " + c.managerSignature(function, mgrName)
}

// ManagerInterfaceSignature renders a manager method without its receiver.
func (c *GenerationContext) ManagerInterfaceSignature(function *clang.TypedefFunction) string {
	return c.managerSignature(function, c.GetManagerName(function.Name))
}

// ReceiverName uses the type's initial without shadowing a public parameter.
func (c *GenerationContext) ReceiverName(function *clang.TypedefFunction, typeName string) string {
	used := make(map[string]bool)
	for _, param := range c.Parameters(function) {
		if !param.IsLength {
			used[param.Name] = true
		}
	}
	initial := strings.ToLower(typeName[:1])
	name := initial
	for suffix := 1; used[name]; suffix++ {
		name = initial + strconv.Itoa(suffix)
	}
	return name
}

func (c *GenerationContext) managerSignature(function *clang.TypedefFunction, mgrName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := c.Parameters(function)
	sb.WriteString(funcName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
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
	sb.WriteString(")")

	if c.HasEffectiveReturn(function) {
		typeName := c.EffectiveGoReturnType(function)
		sb.WriteByte(' ')
		sb.WriteString(typeName)
		sb.WriteByte(' ')
	}
	return sb.String()
}
