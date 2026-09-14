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
	"fmt"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

func (c *GenerationContext) EffectiveArguments(function *clang.TypedefFunction) []clang.Argument {
	args := function.Arguments
	if c.HasSyntheticReturn(function) {
		c.EffectiveRawReturnType(function) // Validate the recorded ABI parameter before hiding it.
		return args[:len(args)-1]
	}
	return args
}

func (c *GenerationContext) HasSyntheticReturn(function *clang.TypedefFunction) bool {
	return function.ReturnType.Name == "void" && c.returnParameters[function.Name].Name != ""
}

func (c *GenerationContext) EffectiveRawReturnType(function *clang.TypedefFunction) string {
	if function.ReturnType.Name != "void" {
		return function.ReturnType.Name
	}
	if result, ok := c.returnParameters[function.Name]; ok {
		args := function.Arguments
		if len(args) == 0 || args[len(args)-1].Name != result.Name || args[len(args)-1].Type.Primative == nil || !args[len(args)-1].Type.Primative.IsPointer || args[len(args)-1].Type.Primative.Name != result.CType {
			panic("invalid synthetic return parameter in " + function.Name)
		}
		return result.CType
	}
	return ""
}

func (c *GenerationContext) EffectiveGoReturnType(function *clang.TypedefFunction) string {
	rawType := c.EffectiveRawReturnType(function)
	if rawType == "" {
		return ""
	}
	return c.MustGoTypeForCType(rawType, function.Name)
}

func (c *GenerationContext) HasEffectiveReturn(function *clang.TypedefFunction) bool {
	return c.EffectiveRawReturnType(function) != ""
}

func (c *GenerationContext) MustGoTypeForCType(typeName string, functionName string) string {
	goType := c.cppType2Go[typeName]
	if goType != "" {
		return goType
	}
	panic(fmt.Sprintf("no Go mapping for C type %q in function %s", typeName, functionName))
}

func MustPrimitiveTypeName(arg clang.Argument, functionName string) string {
	if arg.Type.Primative != nil {
		return arg.Type.Primative.Name
	}
	panic(fmt.Sprintf("unsupported function-pointer argument %q in %s: %s", arg.Name, functionName, arg.Type.CStyleString()))
}
