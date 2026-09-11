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
	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

type NativeArrayBridgeSpec struct {
	BaseFunctionName string
	BaseArgName      string

	DataArgName    string
	DataArgGoType  string
	DataArgPtrType string
	LenArgName     string
	LenArgGoType   string

	RawFunctionName string
	RawMethodName   string
	RawDataArgName  string
	RawDataCType    string
	RawLenArgName   string
	RawLenCType     string

	GoArgType     string
	FastArrayType int32
}

type RawExportParam struct {
	CType string
	Name  string
}

type ArrayTransformBridgeSpec struct {
	FunctionName     string
	ArrayArgName     string
	MethodName       string
	Params           []RawExportParam
	InputArrayType   int32
	OutputArrayType  int32
	OutputCountScale int
}

func (c *GenerationContext) IsNativeArrayDataArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.DataArgName || arg.Name == spec.BaseArgName
}

func (c *GenerationContext) IsNativeArrayLenArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.GetNativeArrayBridgeSpec(function.Name)
	if !ok {
		return false
	}
	return arg.Name == spec.LenArgName && spec.LenArgName != ""
}

func (c *GenerationContext) NativeArrayLenExpr(function *clang.TypedefFunction, argName string) string {
	spec, _ := c.GetNativeArrayBridgeSpec(function.Name)
	return spec.LenArgGoType + "(len(" + argName + "))"
}
