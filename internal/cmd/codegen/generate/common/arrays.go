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
	"slices"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

// ArrayTag identifies array descriptors across Go and JavaScript.
const ArrayTag = "__gdspx_array"

// ArrayType is the native/Web ABI type ID.
type ArrayType int32

const (
	ArrayUnknown ArrayType = 0
	ArrayInt64   ArrayType = 1
	ArrayFloat   ArrayType = 2
	ArrayBool    ArrayType = 3
	ArrayString  ArrayType = 4
	ArrayByte    ArrayType = 5
	ArrayObject  ArrayType = 6
)

type arrayBinding struct {
	cType     string
	name      string
	goSlice   string
	goPointer string
	size      int
}

// Object arrays are exposed through GdArray transforms, not direct Go slices.
var arrayTypes = map[ArrayType]arrayBinding{
	ArrayUnknown: {name: "Unknown"},
	ArrayBool:    {name: "Bool", size: 1},
	ArrayString:  {name: "String"},
	ArrayInt64:   {name: "Int64", cType: "int64_t", goSlice: "[]int64", goPointer: "*int64", size: 8},
	ArrayFloat:   {name: "Float", cType: "float", goSlice: "[]float32", goPointer: "*float32", size: 4},
	ArrayByte:    {name: "Byte", cType: "uint8_t", goSlice: "[]byte", goPointer: "*uint8", size: 1},
	ArrayObject:  {name: "GdObj", cType: "GdObj", size: 8},
}

// LookupArrayType resolves a C element type to its array ABI type.
func LookupArrayType(elementType string) (ArrayType, bool) {
	if elementType == "real_t" {
		elementType = "float"
	}
	for arrayType, binding := range arrayTypes {
		if binding.cType != "" && binding.cType == elementType {
			return arrayType, true
		}
	}
	return 0, false
}

func (t ArrayType) binding() arrayBinding {
	binding, ok := arrayTypes[t]
	if !ok {
		panic(fmt.Sprintf("unsupported array type: %d", t))
	}
	return binding
}

func (t ArrayType) goBinding() arrayBinding {
	binding := t.binding()
	if binding.goSlice == "" {
		panic(fmt.Sprintf("array type %d does not support direct Go slices", t))
	}
	return binding
}

func (t ArrayType) CType() string {
	name := t.binding().cType
	if name == "" {
		panic(fmt.Sprintf("array type %d does not support raw buffers", t))
	}
	return name
}

func (t ArrayType) CConstant() string { return "GD_ARRAY_TYPE_" + strings.ToUpper(t.binding().name) }

func (t ArrayType) JSConstant() string {
	return "GDSPX_ARRAY_TYPE_" + strings.ToUpper(t.binding().name)
}

func (t ArrayType) GoConstant() string { return "GdArrayType" + t.binding().name }

// ElementSize is the fixed ABI width in bytes; zero means no fixed-width layout.
func (t ArrayType) ElementSize() int { return t.binding().size }

// ArrayTypes returns ABI types in numeric order for deterministic generation.
func ArrayTypes() []ArrayType {
	types := make([]ArrayType, 0, len(arrayTypes))
	for t := range arrayTypes {
		types = append(types, t)
	}
	slices.Sort(types)
	return types
}

// ArrayBridge maps raw buffers to a high-level array argument or result.
type ArrayBridge struct {
	FunctionName string
	MethodName   string
	ArgName      string
	Input        *ArrayBuffer
	Output       *ArrayBuffer
	ReturnArray  bool // Allocate and return Output.
}

type ArrayBuffer struct {
	Data             CParam
	Length           CParam
	Type             ArrayType
	Count            int // Fixed element count; zero means unspecified.
	ElementsPerInput int // Output element count per input element.
}

type CParam struct {
	CType string
	Name  string
}

func (s ArrayBridge) Clone() ArrayBridge {
	if s.Input != nil {
		input := *s.Input
		s.Input = &input
	}
	if s.Output != nil {
		output := *s.Output
		s.Output = &output
	}
	return s
}

// CallerBuffer returns the caller-owned buffer, or nil for array transforms.
func (s ArrayBridge) CallerBuffer() *ArrayBuffer {
	if s.ReturnArray {
		return nil
	}
	if s.Input != nil {
		return s.Input
	}
	return s.Output
}

func (s ArrayBridge) Params() []CParam {
	var params []CParam
	for _, buffer := range []*ArrayBuffer{s.Input, s.Output} {
		if buffer != nil {
			params = append(params, buffer.Data, buffer.Length)
		}
	}
	return params
}

func (b ArrayBuffer) GoType() string {
	return b.Type.goBinding().goSlice
}

func (b ArrayBuffer) GoPointerType() string {
	return b.Type.goBinding().goPointer
}

func (c *GenerationContext) IsArrayBufferArgument(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.ArrayBridge(function.Name)
	buffer := spec.CallerBuffer()
	return ok && buffer != nil && (arg.Name == buffer.Data.Name || arg.Name == spec.ArgName)
}

func (c *GenerationContext) IsArrayLengthArgument(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	spec, ok := c.ArrayBridge(function.Name)
	buffer := spec.CallerBuffer()
	return ok && buffer != nil && buffer.Length.Name != "" && arg.Name == buffer.Length.Name
}

func ArrayLengthExpr(argName string) string {
	return "int32(len(" + argName + "))"
}
