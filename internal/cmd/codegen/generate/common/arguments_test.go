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
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestEffectiveRawReturnTypeWithPrimitiveRetValue(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestPrimitiveReturn",
		ReturnType: clang.PrimitiveType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Primitive: &clang.PrimitiveType{Name: "GdInt", IsPointer: true},
				},
				Name: "ret_value",
			},
		},
	}

	generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{ReturnParameters: map[string]CParam{function.Name: {CType: "GdInt", Name: "ret_value"}}})
	require.Equal(t, "GdInt", generation.EffectiveRawReturnType(function))
}

func TestEffectiveRawReturnTypePanicsOnFunctionPointerRetValue(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestFunctionReturn",
		ReturnType: clang.PrimitiveType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Function: &clang.FunctionType{
						ReturnType: clang.PrimitiveType{Name: "void"},
						Name:       "ret_value",
						Arguments: []clang.Argument{
							{
								Type: clang.Type{
									Primitive: &clang.PrimitiveType{Name: "void", IsPointer: true},
								},
							},
						},
					},
				},
				Name: "ret_value",
			},
		},
	}

	require.PanicsWithValue(
		t,
		"invalid synthetic return parameter in GDExtensionSpxTestFunctionReturn",
		func() {
			generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{ReturnParameters: map[string]CParam{function.Name: {CType: "GdInt", Name: "ret_value"}}})
			_ = generation.EffectiveRawReturnType(function)
		},
	)
}

func TestMustPrimitiveTypeName(t *testing.T) {
	arg := clang.Argument{
		Name: "value",
		Type: clang.Type{
			Primitive: &clang.PrimitiveType{Name: "GdString"},
		},
	}

	require.Equal(t, "GdString", MustPrimitiveTypeName(arg, "GDExtensionSpxTest"))
}

func TestMustPrimitiveTypeNamePanicsOnFunctionPointer(t *testing.T) {
	arg := clang.Argument{
		Name: "callback",
		Type: clang.Type{
			Function: &clang.FunctionType{
				ReturnType: clang.PrimitiveType{Name: "void"},
				Name:       "callback",
				Arguments: []clang.Argument{
					{
						Type: clang.Type{
							Primitive: &clang.PrimitiveType{Name: "uint32_t"},
						},
					},
				},
			},
		},
	}

	require.PanicsWithValue(
		t,
		`unsupported function-pointer argument "callback" in GDExtensionSpxTest: void(*callback)(uint32_t)`,
		func() {
			_ = MustPrimitiveTypeName(arg, "GDExtensionSpxTest")
		},
	)
}

func TestEffectiveGoReturnTypePanicsOnMissingTypeMapping(t *testing.T) {
	generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{ReturnParameters: map[string]CParam{"GDExtensionSpxTestUnknownReturn": {CType: "GdUnknown", Name: "ret_value"}}})

	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestUnknownReturn",
		ReturnType: clang.PrimitiveType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Primitive: &clang.PrimitiveType{Name: "GdUnknown", IsPointer: true},
				},
				Name: "ret_value",
			},
		},
	}

	require.PanicsWithValue(
		t,
		`no Go mapping for C type "GdUnknown" in function GDExtensionSpxTestUnknownReturn`,
		func() {
			_ = generation.EffectiveGoReturnType(function)
		},
	)
}

func TestMustGoTypeForCTypePanicsOnMissingMapping(t *testing.T) {
	generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{})

	require.PanicsWithValue(
		t,
		`no Go mapping for C type "GdUnknown" in function GDExtensionSpxTestUnknownType`,
		func() {
			_ = generation.MustGoTypeForCType("GdUnknown", "GDExtensionSpxTestUnknownType")
		},
	)
}

func TestManagerSignatureRejectsUnmappedArgumentTypes(t *testing.T) {
	for _, tt := range []struct {
		cType        string
		functionName string
	}{
		{"double", "GDExtensionSpxSpriteSetPrecision"},
		{"uint64_t", "GDExtensionSpxSpriteSetToken"},
	} {
		t.Run(tt.cType, func(t *testing.T) {
			header, err := clang.ParseCString(fmt.Sprintf("typedef void (*%s)(%s value);", tt.functionName, tt.cType))
			require.NoError(t, err)
			functions := header.CollectGDExtensionInterfaceFunctions()
			require.Len(t, functions, 1)
			generation := NewGenerationContext(header, GenerationMetadata{})
			require.PanicsWithValue(t,
				fmt.Sprintf("no Go mapping for C type %q in function %s", tt.cType, tt.functionName),
				func() { generation.ManagerInterfaceSignature(&functions[0]) },
			)
		})
	}
}

func TestEffectiveGoArgumentTypeUsesArrayBridge(t *testing.T) {
	metadata := GenerationMetadata{}

	metadata.ArrayBridges = map[string]ArrayBridge{"GDExtensionSpxSpriteBatchUpdateTransforms": {
		Arguments: []CParam{
			{CType: "const float *", Name: "buffer_data"},
			{CType: "int", Name: "len"},
		},
		Buffers: []ArrayBuffer{{
			Data:   CParam{CType: "const float *", Name: "buffer_data"},
			Length: CParam{CType: "int", Name: "len"}, Type: 2,
		}},
	}}
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchUpdateTransforms",
		Arguments: []clang.Argument{
			{
				Name: "buffer_data",
				Type: clang.Type{
					Primitive: &clang.PrimitiveType{Name: "float", IsPointer: true},
				},
			},
			{
				Name: "len",
				Type: clang.Type{
					Primitive: &clang.PrimitiveType{Name: "int"},
				},
			},
		},
	}

	generation := NewGenerationContext(clang.CHeaderFileAST{Expr: []clang.Expr{{Function: function}}}, metadata)
	require.Equal(t, "[]float32", generation.Parameter(function, "buffer_data").MustGoType(function.Name))
	require.Equal(t, "[]float32", generation.Parameter(function, "buffer_data").GDXType(function.Name))
	require.Equal(t, "buffer", generation.Parameter(function, "buffer_data").Name)
	require.True(t, generation.Parameter(function, "len").IsLength)
	// Renderers look up ABI argument names, even when the public name differs.
	require.Equal(t, Parameter{}, generation.Parameter(function, "buffer"))
}

func TestArrayBufferRejectsUnsupportedElementTypes(t *testing.T) {
	for _, tt := range []struct {
		name    string
		typeID  ArrayType
		unknown bool
	}{
		{"negative ABI ID", ArrayType(-1), true},
		{"unassigned ABI ID", ArrayType(1<<31 - 1), true},
		{"unknown array", ArrayUnknown, false},
		{"boolean array", ArrayBool, false},
		{"string array", ArrayString, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Descriptor-only types must not generate raw buffers or direct Go
			// slices; both backends need an explicit storage representation.
			cError := fmt.Sprintf("array type %d does not support raw buffers", tt.typeID)
			goError := fmt.Sprintf("array type %d does not support direct Go slices", tt.typeID)
			if tt.unknown {
				cError = fmt.Sprintf("unsupported array type: %d", tt.typeID)
				goError = cError
			}
			require.PanicsWithValue(t, cError, func() { tt.typeID.CType() })
			buffer := ArrayBuffer{Type: tt.typeID}
			require.PanicsWithValue(t, goError, func() { buffer.GoType() })
		})
	}
}
