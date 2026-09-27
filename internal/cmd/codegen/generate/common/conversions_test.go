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

func TestNativeArgumentTypes(t *testing.T) {
	// A dash denotes a rejected spelling; void legitimately maps to an empty type.
	for _, tt := range []struct{ cType, value, pointer string }{
		{"void", "", "unsafe.Pointer"},
		{"float", "float32", "*float32"},
		{"real_t", "float32", "*float32"},
		{"size_t", "uint64", "-"},
		{"char", "-", "string"},
		{"int32_t", "int32", "-"},
		{"char16_t", "-", "*Char16T"},
		{"char32_t", "Char32T", "*Char32T"},
		{"wchar_t", "-", "*WcharT"},
		{"uint8_t", "Uint8T", "*Uint8T"},
		{"int", "int32", "*int32"},
		{"uint32_t", "Uint32T", "*Uint32T"},
		{"uint64_t", "Uint64T", "*Uint64T"},
		{"GdArray", "GdArray", "*GdArray"},
		{"GdObj", "GdObj", "*GdObj"},
		{"double", "double", "*double"},
		{"int64_t", "int64_t", "*int64_t"},
		{"CustomType", "CustomType", "*CustomType"},
	} {
		t.Run(tt.cType, func(t *testing.T) {
			for _, pointer := range []bool{false, true} {
				cType := clang.PrimitiveType{Name: " " + tt.cType + " ", IsPointer: pointer}
				want := tt.value
				if pointer {
					want = tt.pointer
				}
				if want == "-" {
					require.PanicsWithValue(t, "unhandled type: "+cType.CStyleString(), func() { GoArgumentType(cType, "value") })
				} else {
					require.Equal(t, want, GoArgumentType(cType, "value"), "pointer=%t", pointer)
				}
			}
		})
	}
}

func TestNativeStringArgumentOwnership(t *testing.T) {
	for _, tt := range []struct{ name, goType, cast, cleanup string }{
		{"text", "string", "C.CString(text)", "C.free(unsafe.Pointer(arg2))"},
		{"r_text", "*Char", "(*C.char)(r_text)", ""},
		// Ownership follows the declared name, even when the fallback has an r_ prefix.
		{"", "string", "C.CString(r_fallback)", "C.free(unsafe.Pointer(arg2))"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			primitive := clang.PrimitiveType{Name: " char ", IsPointer: true}
			arg := clang.Argument{Name: tt.name, Type: clang.Type{Primitive: &primitive}}
			require.Equal(t, tt.goType, GoArgumentType(primitive, tt.name))
			require.Equal(t, tt.cast, CgoCastArgument(arg, "r_fallback"))
			require.Equal(t, tt.cleanup, CgoCleanUpArgument(arg, 2))
		})
	}
}

func TestNativeArrayArgumentCasts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pointer bool
		cast    string
	}{
		{"array", false, "(C.GdArray)(array)"},
		{"array", true, "(*C.GdArray)(array)"},
		{"", false, "(C.GdArray)(inArg1)"},
		{"", true, "(*C.GdArray)(inArg1)"},
	} {
		arg := clang.Argument{Name: tt.name, Type: clang.Type{Primitive: &clang.PrimitiveType{Name: " GdArray ", IsPointer: tt.pointer}}}
		require.Equal(t, tt.cast, CgoCastArgument(arg, "inArg1"))
		require.Empty(t, CgoCleanUpArgument(arg, 1))
	}
}

func TestNativeReturnConversions(t *testing.T) {
	for _, tt := range []struct{ cType, goType, cast string }{
		{"float", "float32", "float32(result)"},
		{"real_t", "float32", "float32(result)"},
		{"double", "float64", "float64(result)"},
		{"int32_t", "int32", "int32(result)"},
		{"int64_t", "int64", "int64(result)"},
		{"uint8_t", "uint8", "uint8(result)"},
		{"uint32_t", "uint32", "uint32(result)"},
		{"uint64_t", "uint64", "uint64(result)"},
		{"GdArray", "GdArray", "GdArray(result)"},
		{"GdObj", "GdObj", "(GdObj)(result)"},
	} {
		t.Run(tt.cType, func(t *testing.T) {
			cType := clang.PrimitiveType{Name: " " + tt.cType + " "}
			require.Equal(t, tt.goType, GoReturnType(cType))
			require.Equal(t, tt.cast, CgoCastReturnType(cType, "result"))
			cType.IsPointer = true
			require.Equal(t, "*"+tt.goType, GoReturnType(cType))
			require.Equal(t, "(*"+tt.goType+")(result)", CgoCastReturnType(cType, "result"))
		})
	}
}

func TestNativeSpecialReturnConversions(t *testing.T) {
	void := clang.PrimitiveType{Name: "void"}
	require.Empty(t, GoReturnType(void))
	require.Panics(t, func() { CgoCastReturnType(void, "result") })
	void.IsPointer = true
	require.Equal(t, "unsafe.Pointer", GoReturnType(void))
	require.Equal(t, "unsafe.Pointer(result)", CgoCastReturnType(void, "result"))
	for _, tt := range []struct{ cType, goType string }{
		{"char16_t", "Char16T"}, {"char32_t", "Char32T"},
	} {
		cType := clang.PrimitiveType{Name: tt.cType, IsPointer: true}
		require.Equal(t, "*"+tt.goType, GoReturnType(cType))
		require.Equal(t, "(*"+tt.goType+")(result)", CgoCastReturnType(cType, "result"))
		cType.IsPointer = false
		require.Panics(t, func() { CgoCastReturnType(cType, "result") })
	}
}

func TestLoadProcAddressNamePreservesScalarWidths(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"GDExtensionSpxPackedFloat32ArrayGet", "spx_packed_float32_array_get"},
		{"GDExtensionSpxPackedFloat64ArrayGet", "spx_packed_float64_array_get"},
		{"GDExtensionSpxPackedInt64ArrayGet", "spx_packed_int64_array_get"},
	} {
		require.Equal(t, tt.want, LoadProcAddressName(tt.name))
	}
}
