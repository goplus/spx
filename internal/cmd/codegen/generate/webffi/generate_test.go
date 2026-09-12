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
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/gdext"
	"github.com/stretchr/testify/require"
)

func TestWriteEngineJSTrimsTrailingWhitespace(t *testing.T) {

	function := &clang.TypedefFunction{
		Name:       "GDExtensionSpxTestDoThing",
		ReturnType: clang.PrimativeType{Name: "void"},
	}
	ast := clang.CHeaderFileAST{
		Expr: []clang.Expr{{Function: function}},
	}
	spxModulePath := filepath.Join(t.TempDir(), "spx")

	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	require.NoError(t, generation.writeEngineJS(spxModulePath))
	body, err := os.ReadFile(filepath.Join(spxModulePath, "web", "js", "engine", "gdspx.js"))
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(body), "\n") {
		require.Equal(t, strings.TrimRight(line, " \t"), line)
	}
	require.Contains(t, string(body), "GdspxFuncs.prototype['gdspx_test_do_thing'] = GdspxFuncs.prototype.gdspx_test_do_thing;")
	require.Contains(t, string(body), "globalThis['GdspxFuncs'] = GdspxFuncs;")
	require.Contains(t, string(body), "var _call = Module['_gdspx_test_do_thing'];")
	require.NotContains(t, string(body), "Module._gdspx_test_do_thing")
}

func TestJSFunctionArgsFlattensGdObj(t *testing.T) {
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}

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

	require.Equal(t, []string{"obj_low", "obj_high"}, generation.jsArgs(function))
}

func TestJSFunctionArgsSkipsArrayLengthArgument(t *testing.T) {
	metadata := common.GenerationMetadata{}

	metadata.ArrayBridges = map[string]common.ArrayBridge{"GDExtensionSpxSpriteBatchUpdateTransforms": {
		FunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms", ArgName: "buffer",
		Input: &common.ArrayBuffer{
			Data:   common.CParam{CType: "const float *", Name: "buffer_data"},
			Length: common.CParam{CType: "int", Name: "len"}, Type: 2,
		},
	}}
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, metadata)}

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

	require.Equal(t, []string{"buffer"}, generation.jsArgs(function))
}

func TestJSFunctionBodyUsesHighLowCtorOrderForFlatGdIntArgs(t *testing.T) {
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}

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

	body := generation.jsBody(function)
	require.Contains(t, body, "Module['_gdspx_new_obj'](obj_high, obj_low)")
}

func TestJSFunctionBodyUsesInstanceResultForFlatReturn(t *testing.T) {
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})}

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

	body := generation.jsBody(function)
	require.Contains(t, body, `ToJsInt(_resultPtr, this._reusableResults["GdInt"])`)
	require.NotContains(t, body, `"_gdIntResult"`)
}

func TestJSFunctionBodyReturnsArrayFromBridgeSpec(t *testing.T) {
	metadata := common.GenerationMetadata{}

	metadata.ArrayBridges = map[string]common.ArrayBridge{"GDExtensionSpxSpriteBatchRetrievePositions": {
		FunctionName: "GDExtensionSpxSpriteBatchRetrievePositions", ArgName: "objs", ReturnArray: true,
		Input:  &common.ArrayBuffer{Type: 6},
		Output: &common.ArrayBuffer{Type: 2, ElementsPerInput: 2},
	}}
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, metadata)}

	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchRetrievePositions",
	}

	body := generation.jsBody(function)
	require.Contains(t, body, "TryTransformArray(_call, objs, 6, 2, 2)")
	require.Contains(t, body, `throw new Error("gdspx_sprite_batch_retrieve_positions array transform failed")`)
}

func TestJSFunctionBodyUsesArrayAccessSemantics(t *testing.T) {
	dir := t.TempDir()
	header := `
class SpxTestMgr : public SpxBaseMgr {
public:
	SPX_API void read_values(const float *values_data, int len);
	SPX_API void write_values(float *values_data, int len);
	SPX_API void write_bytes(uint8_t *out, int len);
};
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_test_mgr.h"), []byte(header), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, headers.Metadata)}

	for _, test := range []struct {
		name string
		want string
	}{
		{"GDExtensionSpxTestReadValues", `RequireNativeArray(values, "gdspx_test_read_values", 2, false)`},
		{"GDExtensionSpxTestWriteValues", `RequireNativeArray(values, "gdspx_test_write_values", 2, true)`},
		{"GDExtensionSpxTestWriteBytes", `RequireNativeArray(out, "gdspx_test_write_bytes", 5, true)`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Contains(t, headers.Metadata.ArrayBridges, test.name)
			body := generation.jsBody(&clang.TypedefFunction{
				Name:       test.name,
				ReturnType: clang.PrimativeType{Name: "void"},
			})
			require.Contains(t, body, test.want)
			require.Contains(t, body, "_call(_arg0, _arg1);")
			require.NotContains(t, body, "GetNativeArrayPointer(")
		})
	}
}

func TestGenerateFixedArrayOutputReader(t *testing.T) {
	dir := t.TempDir()
	header := `class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_BINDING(output_count=7) SPX_API void write_values(int64_t *out, int capacity);
 SPX_API void update_values(const float *values, int count);
 };`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(header), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString(`typedef void (*GDExtensionSpxExampleWriteValues)(int64_t *out, int capacity);
typedef void (*GDExtensionSpxExampleUpdateValues)(const float *values, int count);`)
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	require.NoError(t, generation.writeEngineJS(dir))
	output, err := os.ReadFile(filepath.Join(dir, "web", "js", "engine", "gdspx.js"))
	require.NoError(t, err)
	require.Contains(t, string(output), "GdspxFuncs['arrayOutputs'] = {")
	require.Contains(t, string(output), "'gdspx_example_write_values': function() {")
	require.Contains(t, string(output), "ReadArrayOutput('_gdspx_example_write_values', 1, 7)")
	require.NotContains(t, string(output), "'gdspx_example_update_values':")
	require.Contains(t, string(output), `RequireNativeArray(out, "gdspx_example_write_values", 1, true)`)
	require.Contains(t, string(output), "NativeArrayCount(out)")
}

func TestJSFunctionBodyUsesDeclaredNoop(t *testing.T) {
	const name = "GDExtensionSpxExampleReleaseValue"
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{
		WebBindings: map[string]common.WebBindingMode{name: common.WebBindingNoop},
	})}
	body := generation.jsBody(&clang.TypedefFunction{Name: name})
	require.Equal(t, "return;", body)
}

func TestJSFunctionBodyValidatesNativeArray(t *testing.T) {
	metadata := common.GenerationMetadata{}

	metadata.ArrayBridges = map[string]common.ArrayBridge{"GDExtensionSpxSpriteBatchUpdateTransforms": {
		FunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms", ArgName: "buffer",
		Input: &common.ArrayBuffer{
			Data:   common.CParam{CType: "const float *", Name: "buffer_data"},
			Length: common.CParam{CType: "int", Name: "len"}, Type: 2,
		},
	}}
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, metadata)}

	body := generation.jsBody(&clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchUpdateTransforms",
	})
	require.Contains(t, body, `RequireNativeArray(buffer, "gdspx_sprite_batch_update_transforms", 2, false)`)
	require.Contains(t, body, "var _arg1 = NativeArrayCount(buffer);")
	require.NotContains(t, body, "buffer['count']")
}

func TestJSResultsFollowReturnTypes(t *testing.T) {
	ast, err := clang.ParseCString(`
	typedef void (*GDExtensionSpxExampleReadInt)(GdInt *ret_value);
	typedef void (*GDExtensionSpxExampleReadOtherInt)(GdInt *ret_value);
	typedef void (*GDExtensionSpxExampleReadObj)(GdObj *ret_value);
	typedef void (*GDExtensionSpxExampleReadVec)(GdVec2 *ret_value);
	`)
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	require.Equal(t, map[string]string{
		"GdInt": "{ 'low': 0, 'high': 0 }",
		"GdObj": "{ 'low': 0, 'high': 0 }",
	}, generation.jsResults())
	generation.GenerationContext = common.NewGenerationContext(clang.CHeaderFileAST{}, common.GenerationMetadata{})
	require.Empty(t, generation.jsResults())
}

func TestJSFunctionBodyReusesOnlyDeclaredResults(t *testing.T) {
	for _, typeName := range []string{"GdVec2", "GdVec3", "GdVec4", "GdColor", "GdRect2"} {
		t.Run(typeName, func(t *testing.T) {
			const name = "GDExtensionSpxExampleRead"
			ast, err := clang.ParseCString("typedef void (*" + name + ")(" + typeName + " *ret_value);")
			require.NoError(t, err)
			function := ast.CollectGDExtensionInterfaceFunctions()[0]
			for _, mode := range []common.WebBindingMode{common.WebBindingDefault, common.WebBindingReuseResult} {
				generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{
					WebBindings: map[string]common.WebBindingMode{name: mode},
				})}
				body := generation.jsBody(&function)
				key, initializer := generation.jsResult(&function)
				if mode == common.WebBindingReuseResult {
					require.Contains(t, body, `this._reusableResults["gdspx_example_read"]`)
					require.Equal(t, "gdspx_example_read", key)
					require.NotEmpty(t, initializer)
				} else {
					require.NotContains(t, body, "_reusableResults")
					require.Empty(t, key)
					require.Empty(t, initializer)
				}
				require.Contains(t, body, "finally")
				require.Contains(t, body, "Free"+typeName+"(_resultPtr)")
			}
		})
	}
}

func TestManagerInputCacheUsesGeneratedFallback(t *testing.T) {
	caches, err := scanCaches(filepath.Join("..", "..", "..", "..", "..", "internal", "gdengine", "binding", "web"))
	require.NoError(t, err)
	for _, test := range []struct {
		method, params, call, conversion string
	}{
		{"GetGlobalMousePos", "GdVec2 ret_value", "CachedInputGetGlobalMousePos(func() Vec2", "JsToGdVec2(_result)"},
		{"GetKey", "GdInt key, GdBool ret_value", "CachedInputGetKey(key, func() bool", "JsToGdBool(_result)"},
		{"GetMouseState", "GdInt mouse_id, GdBool ret_value", "CachedInputGetMouseState(mouse_id, func() bool", "JsToGdBool(_result)"},
		{"GetKeyState", "GdInt key, GdInt ret_value", "CachedInputGetKeyState(key, func() int64", "JsToGdInt(_result)"},
		{"GetAxis", "GdString neg_action, GdString pos_action, GdFloat ret_value", "CachedInputGetAxis(neg_action, pos_action, func() float64", "JsToGdFloat(_result)"},
		{"IsActionPressed", "GdString action, GdBool ret_value", `CachedInputIsActionPressed(action, func() bool`, "JsToGdBool(_result)"},
		{"IsActionJustPressed", "GdString action, GdBool ret_value", `CachedInputIsActionJustPressed(action, func() bool`, "JsToGdBool(_result)"},
		{"IsActionJustReleased", "GdString action, GdBool ret_value", `CachedInputIsActionJustReleased(action, func() bool`, "JsToGdBool(_result)"},
		{"IsActionPressed", "GdString renamed, GdBool ret_value", `CachedInputIsActionPressed(renamed, func() bool`, "JsToGdBool(_result)"},
	} {
		t.Run(test.method+"/"+test.params, func(t *testing.T) {
			declaration := "typedef void (*GDExtensionSpxInput" + test.method + ")(" + test.params + ");"
			ast, err := clang.ParseCString(declaration)
			require.NoError(t, err)
			generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{}), caches: caches}
			functions := ast.CollectGDExtensionInterfaceFunctions()
			require.Len(t, functions, 1)
			body := generation.managerBody(&functions[0])
			require.Contains(t, body, test.call+" {")
			require.Contains(t, body, "_result := API.SpxInput"+test.method+".Invoke(")
			require.Contains(t, body, "return "+test.conversion)
			if strings.Contains(test.params, "renamed") {
				require.Contains(t, body, "JsFromGdString(renamed)")
				require.NotContains(t, body, "JsFromGdString(action)")
			}
		})
	}
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
		"GdspxBorrowNativeArray",
	} {
		require.Contains(t, util, "globalThis['"+name+"']")
	}
	input := read("godot_modules", "spx", "web", "js", "engine", "gdspx.input.js")
	for _, name := range []string{"GdspxGetInputActionEpoch", "GdspxGetInputActionID"} {
		require.Contains(t, input, "globalThis['"+name+"']")
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

func TestRepositoryWebBridgeKeepsNativeArrayPointersPrivate(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..", "..", "..")
	utilPath := filepath.Join(repositoryRoot, "godot_modules", "spx", "web", "js", "engine", "gdspx.util.js")
	body, err := os.ReadFile(utilPath)
	require.NoError(t, err)
	util := string(body)

	// Raw pointers must come from bridge-created wrappers.
	require.Contains(t, util, "const [GdspxBorrowNativeArray, GetNativeArrayMetadata] = (() => {")
	require.Contains(t, util, "const registry = new WeakMap();")
	require.Contains(t, util, "registry.set(wrapper, metadata);")
	require.Contains(t, util, "return [borrow, get];")
	start := strings.Index(util, "const [GdspxBorrowNativeArray, GetNativeArrayMetadata] = (() => {")
	require.NotEqual(t, -1, start)
	endOffset := strings.Index(util[start:], "})();")
	require.NotEqual(t, -1, endOffset)
	closure := util[start : start+endOffset]
	require.NotContains(t, closure, "globalThis")
	require.NotContains(t, closure, "register")
	require.Contains(t, util, "return Object.freeze(wrapper);")
	require.Contains(t, util, "if (metadata.module !== Module)")
	require.Contains(t, util, "writable && GetNativeArrayMetadata(array) === null")
	require.NotContains(t, util, "return array['ptr'];")
}
