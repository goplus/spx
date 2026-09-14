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
	cType   string
	name    string
	goSlice string
	size    int
}

// Fixed-width arrays can also use caller-owned native buffers.
var arrayTypes = map[ArrayType]arrayBinding{
	ArrayUnknown: {name: "Unknown"},
	ArrayBool:    {name: "Bool", size: 1},
	ArrayString:  {name: "String"},
	ArrayInt64:   {name: "Int64", cType: "int64_t", goSlice: "[]int64", size: 8},
	ArrayFloat:   {name: "Float", cType: "float", goSlice: "[]float32", size: 4},
	ArrayByte:    {name: "Byte", cType: "uint8_t", goSlice: "[]byte", size: 1},
	ArrayObject:  {name: "GdObj", cType: "GdObj", goSlice: "[]int64", size: 8},
}

// ArrayBridge describes ordered, independently sized caller-owned buffers.
type ArrayBridge struct {
	FunctionName string
	MethodName   string
	Buffers      []ArrayBuffer
	Arguments    []CParam
}

type ArrayBuffer struct {
	Data       CParam
	Length     CParam
	Type       ArrayType
	Count      int  // Fixed element count; zero means a separate length parameter.
	OutputOnly bool // The callee writes the full range without reading caller storage.
}

type CParam struct {
	CType string
	Name  string
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

func (p CParam) Declaration() string {
	if strings.HasSuffix(p.CType, "*") {
		return p.CType + p.Name
	}
	return p.CType + " " + p.Name
}

func (s ArrayBridge) Clone() ArrayBridge {
	s.Buffers = slices.Clone(s.Buffers)
	s.Arguments = slices.Clone(s.Arguments)
	return s
}

func (s ArrayBridge) Params() []CParam {
	if s.Arguments != nil {
		return slices.Clone(s.Arguments)
	}
	var params []CParam
	for _, buffer := range s.Buffers {
		params = append(params, buffer.Data)
		if buffer.Length.Name != "" {
			params = append(params, buffer.Length)
		}
	}
	return params
}

func (s ArrayBridge) HasOutputOnly() bool {
	for _, buffer := range s.Buffers {
		if buffer.OutputOnly {
			return true
		}
	}
	return false
}

// FixedOutput supports the no-argument Web reader for a single fixed output.
func (s ArrayBridge) FixedOutput() *ArrayBuffer {
	if len(s.Params()) == 1 && len(s.Buffers) == 1 && s.Buffers[0].Count != 0 && s.Buffers[0].OutputOnly {
		return &s.Buffers[0]
	}
	return nil
}

func (b ArrayBuffer) ArgName() string { return strings.TrimSuffix(b.Data.Name, "_data") }

func (b ArrayBuffer) Writable() bool {
	return !slices.Contains(strings.Fields(strings.TrimSuffix(b.Data.CType, "*")), "const")
}

func (b ArrayBuffer) GoType() string {
	slice := b.Type.goBinding().goSlice
	if b.Count != 0 {
		return fmt.Sprintf("*[%d]%s", b.Count, strings.TrimPrefix(slice, "[]"))
	}
	return slice
}

// GoSliceExpr provides a slice view without allocating or copying storage.
func (b ArrayBuffer) GoSliceExpr() string {
	if b.Count != 0 {
		return b.ArgName() + "[:]"
	}
	return b.ArgName()
}

// GoArgumentCheck keeps Native and Web buffer preconditions consistent.
func (b ArrayBuffer) GoArgumentCheck(functionName string) string {
	name := b.ArgName()
	if b.Count != 0 {
		return fmt.Sprintf("if %s == nil { panic(%q) }", name, functionName+" requires a non-nil array: "+name)
	}
	return fmt.Sprintf("if len(%s) > math.MaxInt32 { panic(%q) }", name, functionName+" array length exceeds int32: "+name)
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

// ArrayTypes returns ABI types in numeric order for deterministic generation.
func ArrayTypes() []ArrayType {
	types := make([]ArrayType, 0, len(arrayTypes))
	for t := range arrayTypes {
		types = append(types, t)
	}
	slices.Sort(types)
	return types
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
