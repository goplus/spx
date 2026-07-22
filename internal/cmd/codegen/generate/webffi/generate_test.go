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

package webffi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestGenerateJsEngineJsFileTrimsTrailingWhitespace(t *testing.T) {
	function := &clang.TypedefFunction{
		Name:       "GDExtensionSpxTestDoThing",
		ReturnType: clang.PrimativeType{Name: "void"},
	}
	ast := clang.CHeaderFileAST{
		Expr: []clang.Expr{{Function: function}},
	}
	godotPath := t.TempDir()

	require.NoError(t, GenerateJsEngineJsFile("", godotPath, ast))
	body, err := os.ReadFile(filepath.Join(godotPath, "platform", "web", "js", "engine", "gdspx.js"))
	require.NoError(t, err)
	for _, line := range strings.Split(string(body), "\n") {
		require.Equal(t, strings.TrimRight(line, " \t"), line)
	}
}

func TestGetJsFuncArgsFlattensGdObj(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int64_t"},
				},
			},
		},
	}

	require.Equal(t, []string{"obj_low", "obj_high"}, getJsFuncArgs(function))
}

func TestGetJsFuncArgsSkipsNativeArrayLenArg(t *testing.T) {
	common.ClearNativeArrayBridgeSpecs()
	t.Cleanup(common.ClearNativeArrayBridgeSpecs)
	common.RegisterNativeArrayBridgeSpec(common.NativeArrayBridgeSpec{
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

	require.Equal(t, []string{"buffer"}, getJsFuncArgs(function))
}

func TestGetJsFuncBodyUsesHighLowCtorOrderForFlatGdIntArgs(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int64_t"},
				},
			},
		},
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "Module._gdspx_new_obj(obj_high, obj_low)")
}

func TestGetJsFuncBodyUsesFixedScratchAccessorForFlatReturn(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdInt"},
				},
			},
		},
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "this._readGdIntLike(_retValue, this._getGdIntScratch())")
	require.NotContains(t, body, `"_gdIntScratch"`)
}

func TestGetJsFuncBodyUsesArrayTransformBridgeSpec(t *testing.T) {
	common.ClearArrayTransformBridgeSpecs()
	t.Cleanup(common.ClearArrayTransformBridgeSpecs)
	common.RegisterArrayTransformBridgeSpec(common.ArrayTransformBridgeSpec{
		FunctionName:     "GDExtensionSpxSpriteBatchRetrievePositions",
		ArrayArgName:     "objs",
		InputArrayType:   6,
		OutputArrayType:  2,
		OutputCountScale: 2,
	})

	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchRetrievePositions",
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "TryArrayTransformFastPath(_gdFuncPtr, objs, 6, 2, 2)")
	require.Contains(t, body, `throw new Error("gdspx_sprite_batch_retrieve_positions fast path unavailable")`)
}

func TestGetJsFuncBodyRequiresWasmArrayForInputSnapshot(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxInputWriteSnapshot",
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, `RequireWasmFastArray(out, "gdspx_input_write_snapshot")`)
	require.Contains(t, body, "var _arg1 = out.count;")
	require.NotContains(t, body, "GetFastArrayWasmPtr(out)")
}

func TestJsTemplateUsesHeapU32ForFlatReads(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "var _u32 = Module.HEAPU32;")
	require.Contains(t, jsEngineJsFileText, "scratch.low = _u32[_word];")
	require.Contains(t, jsEngineJsFileText, "scratch.high = _u32[_word + 1];")
	require.NotContains(t, jsEngineJsFileText, "_getGdDataView()")
	require.NotContains(t, jsEngineJsFileText, "getUint32(")
}

func TestJsTemplateDocumentsFlatCtorAbiOrder(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "_gdspx_new_int` / `_gdspx_new_obj` follow the C ABI and expect (high, low)")
}

func TestJsTemplateDeclaresInstanceScratchForMousePos(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "this._inputMousePosScratch = null;")
}

func TestJsTemplateUsesExplicitNullChecksForScratch(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "if (_scratch == null) {")
	require.NotContains(t, jsEngineJsFileText, "if (!_scratch) {")
}

func TestGetJsFuncBodyUsesInstanceScratchForMousePos(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxInputGetGlobalMousePos",
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "var _scratch = this._inputMousePosScratch;")
	require.Contains(t, body, "if (_scratch == null) {")
	require.Contains(t, body, "this._inputMousePosScratch = { x: 0, y: 0 };")
	require.NotContains(t, body, "GdspxFuncs._inputMousePosScratch")
}

func TestGetManagerFuncBodyUsesInputCacheOverride(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxInputIsActionPressed",
	}

	body := getManagerFuncBody(function)
	require.Contains(t, body, `return CachedActionBool("pressed", action, func() bool {`)
	require.Contains(t, body, "_retValue := API.SpxInputIsActionPressed.Invoke(arg0)")
	require.NotContains(t, body, "actionSuffix")
}

func TestGetManagerFuncBodyUsesActionAxisOverride(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxInputGetAxis",
	}

	body := getManagerFuncBody(function)
	require.Contains(t, body, "return CachedActionAxis(neg_action, pos_action, func() float64 {")
	require.Contains(t, body, "_retValue := API.SpxInputGetAxis.Invoke(arg0, arg1)")
}
