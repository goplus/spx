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
	"go/format"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestGenerateFileFormatsGoSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "generated.go")
	templateText := "package generated\n\nfunc Answer( )int { return 42 }\n"
	require.NoError(t, GenerateFile(nil, "generated.go", templateText, nil, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	formatted, err := format.Source(got)
	require.NoError(t, err)
	require.Equal(t, got, formatted)
	require.Contains(t, string(got), "func Answer() int")
}

func TestGenerateFileRejectsInvalidGoBeforeWriting(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "nested", "generated.go")
	err := GenerateFile(nil, "generated.go", "package generated\nfunc {", nil, dst)
	require.ErrorContains(t, err, "format generated Go file")
	_, statErr := os.Stat(dst)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

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
	generation := NewGenerationContext()

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
			_ = generation.EffectiveGoReturnType(function)
		},
	)
}

func TestMustGoTypeForCTypePanicsOnMissingMapping(t *testing.T) {
	generation := NewGenerationContext()

	require.PanicsWithValue(
		t,
		`no Go mapping for C type "GdUnknown" in function GDExtensionSpxTestUnknownType`,
		func() {
			_ = generation.MustGoTypeForCType("GdUnknown", "GDExtensionSpxTestUnknownType")
		},
	)
}

func TestEffectiveGoArgumentTypeUsesNativeArrayBridgeSpec(t *testing.T) {
	generation := NewGenerationContext()

	generation.RegisterNativeArrayBridgeSpec(NativeArrayBridgeSpec{
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

	require.Equal(t, "[]float32", generation.EffectiveGoArgumentType(function, function.Arguments[0]))
	require.Equal(t, "[]float32", generation.EffectiveGdxArgumentType(function, function.Arguments[0]))
	require.Equal(t, "buffer", generation.EffectiveGoArgumentName(function, function.Arguments[0]))
	require.True(t, generation.ShouldSkipHighLevelArgument(function, function.Arguments[1]))
}

func TestGenerationContextsAreIndependent(t *testing.T) {
	first, second := NewGenerationContext(), NewGenerationContext()
	first.RegisterManagerName("sprite")
	first.RegisterNativeArrayBridgeSpec(NativeArrayBridgeSpec{BaseFunctionName: "first", GoArgType: "[]float32"})
	first.RegisterArrayTransformBridgeSpec(ArrayTransformBridgeSpec{FunctionName: "first"})
	second.RegisterManagerName("camera")
	require.Equal(t, []string{"sprite"}, first.KnownManagerNames)
	require.Equal(t, []string{"camera"}, second.KnownManagerNames)
	_, exists := second.GetNativeArrayBridgeSpec("first")
	require.False(t, exists)
	require.Empty(t, second.ListArrayTransformBridgeSpecs())
	second.ClearKnownManagerNames()
	require.Equal(t, []string{"sprite"}, first.KnownManagerNames)
	require.Equal(t, "int64", first.MustGoTypeForCType("GdInt", "test"))
	require.Equal(t, "int64", second.MustGoTypeForCType("GdInt", "test"))
}

func TestGetManagersDoesNotChangePreparedAST(t *testing.T) {
	generation := NewGenerationContext()
	generation.RegisterManagerName("sprite")
	generation.RegisterManagerName("camera")
	first, err := clang.ParseCString("typedef void (*GDExtensionSpxSpriteShow)();")
	require.NoError(t, err)
	second, err := clang.ParseCString("typedef void (*GDExtensionSpxCameraShow)();")
	require.NoError(t, err)
	generation.PrepareAST(first)
	require.Equal(t, []string{"camera"}, generation.GetManagers(second))
	require.True(t, generation.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.False(t, generation.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxCameraShow"}))
}

func TestGenerationContextsRenderIndependently(t *testing.T) {
	for _, manager := range []string{"foo", "foobar"} {
		t.Run(manager, func(t *testing.T) {
			t.Parallel()
			generation := NewGenerationContext()
			generation.RegisterManagerName(manager)
			dst := filepath.Join(t.TempDir(), "manager.go")
			err := GenerateFile(template.FuncMap{"manager": generation.GetManagerName}, "manager.go",
				`package generated
const Manager = "{{manager .}}"
`, "GDExtensionSpxFooBar", dst)
			require.NoError(t, err)
			data, err := os.ReadFile(dst)
			require.NoError(t, err)
			require.Contains(t, string(data), `const Manager = "`+manager+`"`)
		})
	}
}
