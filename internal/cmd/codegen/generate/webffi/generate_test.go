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

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
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
	spxModulePath := filepath.Join(t.TempDir(), "spx")

	require.NoError(t, GenerateJsEngineJsFile("", spxModulePath, ast))
	body, err := os.ReadFile(filepath.Join(spxModulePath, "web", "js", "engine", "gdspx.js"))
	require.NoError(t, err)
	for _, line := range strings.Split(string(body), "\n") {
		require.Equal(t, strings.TrimRight(line, " \t"), line)
	}
	require.Contains(t, string(body), "GdspxFuncs.prototype['gdspx_test_do_thing'] = GdspxFuncs.prototype.gdspx_test_do_thing;")
	require.Contains(t, string(body), "globalThis['GdspxFuncs'] = GdspxFuncs;")
	require.Contains(t, string(body), "var _gdFuncPtr = Module['_gdspx_test_do_thing'];")
	require.NotContains(t, string(body), "Module._gdspx_test_do_thing")
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
	require.Contains(t, body, "Module['_gdspx_new_obj'](obj_high, obj_low)")
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
	require.Contains(t, body, "var _arg1 = out['count'];")
	require.NotContains(t, body, "GetFastArrayWasmPtr(out)")
}

func TestJsTemplateUsesHeapU32ForFlatReads(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "var _u32 = Module['HEAPU32'];")
	require.Contains(t, jsEngineJsFileText, "scratch['low'] = _u32[_word];")
	require.Contains(t, jsEngineJsFileText, "scratch['high'] = _u32[_word + 1];")
	require.NotContains(t, jsEngineJsFileText, "_getGdDataView()")
	require.NotContains(t, jsEngineJsFileText, "getUint32(")
	require.NotContains(t, jsEngineJsFileText, "Module.")
}

func TestJsTemplateDocumentsFlatCtorAbiOrder(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "_gdspx_new_int` / `_gdspx_new_obj` follow the C ABI and expect (high, low)")
}

func TestJsTemplateDeclaresInstanceScratchForMousePos(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "this._inputMousePosScratch = { 'x': 0, 'y': 0 };")
	require.Contains(t, jsEngineJsFileText, "this._gdIntScratch = { 'low': 0, 'high': 0 };")
	require.Contains(t, jsEngineJsFileText, "this._gdObjScratch = { 'low': 0, 'high': 0 };")
}

func TestGetJsFuncBodyUsesInstanceScratchForMousePos(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxInputGetGlobalMousePos",
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "var _scratch = this._inputMousePosScratch;")
	require.Contains(t, body, "_scratch['x'] = _heap[_floatIndex];")
	require.Contains(t, body, "_scratch['y'] = _heap[_floatIndex + 1];")
	require.NotContains(t, body, "if (_scratch == null) {")
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

func TestRepositoryWebBridgeKeepsCrossCompilationABIStable(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..", "..", "..")
	read := func(parts ...string) string {
		t.Helper()
		path := filepath.Join(append([]string{repositoryRoot}, parts...)...)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(body)
	}

	util := read("godot_modules", "spx", "web", "js", "engine", "gdspx.util.js")
	require.NotContains(t, util, "Module.")
	require.Contains(t, util, "module['_gdspx_alloc_array']")
	for _, name := range []string{
		"GdspxFlushDeferredFrees",
		"GdspxBorrowFastArray",
		"GdspxInputSnapshot",
		"GdspxInputActionEpoch",
		"GdspxInputActionID",
		"GdspxInputActionBool",
		"GdspxInputAxisByID",
		"GdspxBatchSpritePhysics",
	} {
		require.Contains(t, util, "globalThis['"+name+"']")
	}
	for _, unstable := range []string{
		"array.__gdspx_",
		"array.type",
		"array.count",
		"array.data",
		"value.low",
		"value.high",
	} {
		require.NotContains(t, util, unstable)
	}

	preloader := read("godot_modules", "spx", "web", "js", "engine", "preloader.js")
	require.Contains(t, preloader, "miniEngine['getFileSystemManager']()")
	require.Contains(t, preloader, "fs['readFile']({")
	require.Contains(t, preloader, "'filePath': file")

	library := read("godot_modules", "spx", "web", "js", "libs", "library_godot_gdspx.js")
	require.Contains(t, library, "globalThis['FFI']")
	require.Contains(t, library, "globalThis['GdspxFlushDeferredFrees']()")
	require.Contains(t, library, "self['initExtensionWasm']()")
	require.NotContains(t, library, "FFI.gdspx_dispatch")

	audioLibrary := read("godot_modules", "spx", "web", "js", "libs", "library_godot_audio.js")
	require.Contains(t, audioLibrary, "positionWorker['onMessage']")
	require.Contains(t, audioLibrary, "'inputLength': input.length")
	require.Contains(t, audioLibrary, "event['type']")
	require.Contains(t, audioLibrary, "if (index !== -1 && index !== newBus.getId())")
	require.Contains(t, audioLibrary, "if (toIndex === -1) {\n\t\t\tbuses.push(movedBus);")
	require.NotContains(t, audioLibrary, "positionWorker.onMessage")
}
