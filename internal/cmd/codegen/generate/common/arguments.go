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

func (c *GenerationContext) ShouldSkipHighLevelArgument(function *clang.TypedefFunction, arg clang.Argument) bool {
	return c.IsArrayLengthArgument(function, arg)
}

func (c *GenerationContext) EffectiveGoArgumentName(function *clang.TypedefFunction, arg clang.Argument) string {
	if c.IsArrayBufferArgument(function, arg) {
		spec, _ := c.ArrayBridge(function.Name)
		return spec.ArgName
	}
	return arg.Name
}

func (c *GenerationContext) EffectiveGoArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	if c.IsArrayBufferArgument(function, arg) {
		spec, _ := c.ArrayBridge(function.Name)
		return spec.CallerBuffer().GoType()
	}
	return c.MustGoTypeForCType(MustPrimitiveTypeName(arg, function.Name), function.Name)
}

func (c *GenerationContext) EffectiveGdxArgumentType(function *clang.TypedefFunction, arg clang.Argument) string {
	typeName := c.EffectiveGoArgumentType(function, arg)
	switch typeName {
	case "Object":
		return "gdx.Object"
	case "Array":
		return "gdx.Array"
	default:
		return typeName
	}
}

func EffectiveArguments(function *clang.TypedefFunction) []clang.Argument {
	args := function.Arguments
	if function.ReturnType.Name == "void" && len(args) > 0 && args[len(args)-1].Name == "ret_value" {
		return args[:len(args)-1]
	}
	return args
}

func (c *GenerationContext) HighLevelArguments(function *clang.TypedefFunction) []clang.Argument {
	args := EffectiveArguments(function)
	result := make([]clang.Argument, 0, len(args))
	for _, arg := range args {
		if c.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		cloned := arg
		cloned.Name = c.EffectiveGoArgumentName(function, arg)
		result = append(result, cloned)
	}
	return result
}

func EffectiveRawReturnType(function *clang.TypedefFunction) string {
	if function.ReturnType.Name != "void" {
		return function.ReturnType.Name
	}
	if len(function.Arguments) > 0 {
		last := function.Arguments[len(function.Arguments)-1]
		if last.Name == "ret_value" {
			if last.Type.Primative != nil {
				return last.Type.Primative.Name
			}
			panic(fmt.Sprintf("unsupported synthetic ret_value type in %s: %s", function.Name, last.Type.CStyleString()))
		}
	}
	return ""
}

func (c *GenerationContext) EffectiveGoReturnType(function *clang.TypedefFunction) string {
	rawType := EffectiveRawReturnType(function)
	if rawType == "" {
		return ""
	}
	return c.MustGoTypeForCType(rawType, function.Name)
}

func HasEffectiveReturn(function *clang.TypedefFunction) bool {
	return EffectiveRawReturnType(function) != ""
}

func (c *GenerationContext) GetFuncParamTypeString(typeName string) string {
	return c.cppType2Go[typeName]
}

func (c *GenerationContext) MustGoTypeForCType(typeName string, functionName string) string {
	goType := c.GetFuncParamTypeString(typeName)
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
