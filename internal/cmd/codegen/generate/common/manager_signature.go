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
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

// ManagerMethodSignature renders a manager method with its receiver.
func (c *GenerationContext) ManagerMethodSignature(function *clang.TypedefFunction) string {
	mgrName := c.GetManagerName(function.Name)
	return "(pself *" + mgrName + "Mgr) " + c.managerSignature(function, mgrName)
}

// ManagerInterfaceSignature renders a manager method without its receiver.
func (c *GenerationContext) ManagerInterfaceSignature(function *clang.TypedefFunction) string {
	return c.managerSignature(function, c.GetManagerName(function.Name))
}

func (c *GenerationContext) managerSignature(function *clang.TypedefFunction, mgrName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	sb.WriteString(funcName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if c.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(c.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := c.EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if HasEffectiveReturn(function) {
		typeName := c.EffectiveGoReturnType(function)
		sb.WriteByte(' ')
		sb.WriteString(typeName)
		sb.WriteByte(' ')
	}
	return sb.String()
}
