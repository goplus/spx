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
func ManagerMethodSignature(function *clang.TypedefFunction) string {
	mgrName := GetManagerName(function.Name)
	return "(pself *" + mgrName + "Mgr) " + managerSignature(function, mgrName)
}

// ManagerInterfaceSignature renders a manager method without its receiver.
func ManagerInterfaceSignature(function *clang.TypedefFunction) string {
	return managerSignature(function, GetManagerName(function.Name))
}

func managerSignature(function *clang.TypedefFunction, mgrName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	sb.WriteString(funcName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if HasEffectiveReturn(function) {
		typeName := EffectiveGoReturnType(function)
		sb.WriteString(" " + typeName + " ")
	}
	return sb.String()
}
