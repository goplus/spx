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
