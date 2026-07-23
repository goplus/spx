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
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestEffectiveRawReturnTypeWithPrimitiveRetValue(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestPrimitiveReturn",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdInt"},
				},
				Name: "ret_value",
			},
		},
	}

	require.Equal(t, "GdInt", EffectiveRawReturnType(function))
}

func TestEffectiveRawReturnTypePanicsOnFunctionPointerRetValue(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestFunctionReturn",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Function: &clang.FunctionType{
						ReturnType: clang.PrimativeType{Name: "void"},
						Name:       "ret_value",
						Arguments: []clang.Argument{
							{
								Type: clang.Type{
									Primative: &clang.PrimativeType{Name: "void", IsPointer: true},
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
		"unsupported synthetic ret_value type in GDExtensionSpxTestFunctionReturn: void(*ret_value)(void * )",
		func() {
			_ = EffectiveRawReturnType(function)
		},
	)
}

func TestMustPrimitiveTypeName(t *testing.T) {
	arg := clang.Argument{
		Name: "value",
		Type: clang.Type{
			Primative: &clang.PrimativeType{Name: "GdString"},
		},
	}

	require.Equal(t, "GdString", MustPrimitiveTypeName(arg, "GDExtensionSpxTest"))
}

func TestMustPrimitiveTypeNamePanicsOnFunctionPointer(t *testing.T) {
	arg := clang.Argument{
		Name: "callback",
		Type: clang.Type{
			Function: &clang.FunctionType{
				ReturnType: clang.PrimativeType{Name: "void"},
				Name:       "callback",
				Arguments: []clang.Argument{
					{
						Type: clang.Type{
							Primative: &clang.PrimativeType{Name: "uint32_t"},
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
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxTestUnknownReturn",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdUnknown"},
				},
				Name: "ret_value",
			},
		},
	}

	require.PanicsWithValue(
		t,
		`no Go mapping for C type "GdUnknown" in function GDExtensionSpxTestUnknownReturn`,
		func() {
			_ = EffectiveGoReturnType(function)
		},
	)
}

func TestMustGoTypeForCTypePanicsOnMissingMapping(t *testing.T) {
	require.PanicsWithValue(
		t,
		`no Go mapping for C type "GdUnknown" in function GDExtensionSpxTestUnknownType`,
		func() {
			_ = MustGoTypeForCType("GdUnknown", "GDExtensionSpxTestUnknownType")
		},
	)
}

func TestEffectiveGoArgumentTypeUsesNativeArrayBridgeSpec(t *testing.T) {
	ClearNativeArrayBridgeSpecs()
	RegisterNativeArrayBridgeSpec(NativeArrayBridgeSpec{
		BaseFunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms",
		BaseArgName:      "buffer",
		DataArgName:      "buffer_data",
		DataArgGoType:    "[]float32",
		DataArgPtrType:   "*float32",
		LenArgName:       "len",
		LenArgGoType:     "int32",
		GoArgType:        "[]float32",
	})

	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchUpdateTransforms",
		Arguments: []clang.Argument{
			{
				Name: "buffer_data",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "float", IsPointer: true},
				},
			},
			{
				Name: "len",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int"},
				},
			},
		},
	}

	require.Equal(t, "[]float32", EffectiveGoArgumentType(function, function.Arguments[0]))
	require.Equal(t, "[]float32", EffectiveGdxArgumentType(function, function.Arguments[0]))
	require.Equal(t, "buffer", EffectiveGoArgumentName(function, function.Arguments[0]))
	require.True(t, ShouldSkipHighLevelArgument(function, function.Arguments[1]))
}
