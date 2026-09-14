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
	"os/exec"
	"path/filepath"
	"strconv"
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
			Name: "GdInt",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
		},
	}

	require.Equal(t, []string{"obj_low", "obj_high"}, generation.jsArgs(function))
}

func TestJSFunctionArgsSkipsArrayLengthArgument(t *testing.T) {
	metadata := common.GenerationMetadata{}

	metadata.ArrayBridges = map[string]common.ArrayBridge{"GDExtensionSpxSpriteBatchUpdateTransforms": {
		FunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms",
		Buffers: []common.ArrayBuffer{{
			Data:   common.CParam{CType: "const float *", Name: "buffer_data"},
			Length: common.CParam{CType: "int", Name: "len"}, Type: 2,
		}},
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
			Name: "GdInt",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
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
			Name: "GdInt",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
		},
	}

	body := generation.jsBody(function)
	require.Contains(t, body, `ToJsInt(_resultPtr, this._reusableResults["GdInt"])`)
	require.NotContains(t, body, `"_gdIntResult"`)
}

func TestJSFunctionBodyUsesOrdinaryArrayReturn(t *testing.T) {
	ast, err := clang.ParseCString("typedef GdArray (*GDExtensionSpxExampleConvertArray)(GdArray objs);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	functions := ast.CollectGDExtensionInterfaceFunctions()
	body := generation.jsBody(&functions[0])
	require.Contains(t, body, "ToGdArray(objs)")
	require.Contains(t, body, "AllocGdArray()")
	require.Contains(t, body, "_call(_arg0, _resultPtr)")
	require.Contains(t, body, "ToJsArray(_resultPtr)")
	require.Contains(t, body, "FreeGdArray(_arg0)")
	require.Contains(t, body, "FreeGdArray(_resultPtr)")
	require.NotContains(t, body, "TryTransformArray")
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
			body := generation.jsBody(arrayFunction(t, test.name, headers.Metadata.ArrayBridges[test.name]))
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
 SPX_API void write_values(int64_t out[7]);
 SPX_API void update_values(const float *values, int count);
 };`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(header), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString(`typedef void (*GDExtensionSpxExampleWriteValues)(int64_t *out);
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
	require.Contains(t, string(output), "NativeArrayCount(out) < 7")
	require.Contains(t, string(output), "_call(_arg0);")
	functions := ast.CollectGDExtensionInterfaceFunctions()
	require.NotContains(t, generation.jsBody(&functions[0]), "_call(_arg0, _arg1);")
	body := generation.managerBody(&functions[0])
	require.Contains(t, body, "if out == nil")
	require.Contains(t, body, "JsFromNativeArray(out[:], 1)")
	require.Contains(t, body, "CopyNativeArrayOutput(out[:], arg0)")
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
		FunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms",
		Buffers: []common.ArrayBuffer{{
			Data:   common.CParam{CType: "const float *", Name: "buffer_data"},
			Length: common.CParam{CType: "int", Name: "len"}, Type: 2,
		}},
	}}
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, metadata)}

	body := generation.jsBody(arrayFunction(t, "GDExtensionSpxSpriteBatchUpdateTransforms", metadata.ArrayBridges["GDExtensionSpxSpriteBatchUpdateTransforms"]))
	require.Contains(t, body, `RequireNativeArray(buffer, "gdspx_sprite_batch_update_transforms", 2, false)`)
	require.Contains(t, body, "var _arg1 = NativeArrayCount(buffer);")
	require.NotContains(t, body, "buffer['count']")
}

func TestJSResultsFollowReturnTypes(t *testing.T) {
	ast, err := clang.ParseCString(`
	typedef GdInt (*GDExtensionSpxExampleReadInt)();
	typedef GdInt (*GDExtensionSpxExampleReadOtherInt)();
	typedef GdObj (*GDExtensionSpxExampleReadObj)();
	typedef GdVec2 (*GDExtensionSpxExampleReadVec)();
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
			ast, err := clang.ParseCString("typedef " + typeName + " (*" + name + ")();")
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
			parts := strings.Split(test.params, ", ")
			ret := strings.TrimSuffix(parts[len(parts)-1], " ret_value")
			declaration := "typedef " + ret + " (*GDExtensionSpxInput" + test.method + ")(" + strings.Join(parts[:len(parts)-1], ", ") + ");"
			ast, err := clang.ParseCString(declaration)
			require.NoError(t, err)
			generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{}), caches: caches}
			functions := ast.CollectGDExtensionInterfaceFunctions()
			require.Len(t, functions, 1)
			require.NotContains(t, generation.jsBody(&functions[0]), "_call(_arg0, _arg1);")
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

func TestIndependentNativeBuffersPreserveOrderingAndAccess(t *testing.T) {
	dir := t.TempDir()
	const params = "const GdObj *objs, int count, const uint8_t *mask_data, int mask_len, float *out, int out_len, int64_t *indices, int indices_len"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void collect(`+params+`);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleCollect)(" + params + ");")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	functions := ast.CollectGDExtensionInterfaceFunctions()
	function := &functions[0]
	require.Equal(t, []string{"objs", "mask", "out", "indices"}, generation.jsArgs(function))
	body := generation.jsBody(function)
	require.Contains(t, body, `RequireNativeArray(objs, "gdspx_example_collect", 6, false)`)
	require.Contains(t, body, `RequireNativeArray(mask, "gdspx_example_collect", 5, false)`)
	require.Contains(t, body, `RequireNativeArray(out, "gdspx_example_collect", 2, true)`)
	require.Contains(t, body, `RequireNativeArray(indices, "gdspx_example_collect", 1, true)`)
	require.Contains(t, body, "var _arg1 = NativeArrayCount(objs);")
	require.Contains(t, body, "var _arg3 = NativeArrayCount(mask);")
	require.Contains(t, body, "var _arg5 = NativeArrayCount(out);")
	require.Contains(t, body, "var _arg7 = NativeArrayCount(indices);")
	require.Contains(t, body, "_call(_arg0, _arg1, _arg2, _arg3, _arg4, _arg5, _arg6, _arg7);")
	manager := generation.managerBody(function)
	require.Contains(t, manager, "JsFromNativeArray(objs, 6)")
	require.Contains(t, manager, "JsFromNativeArray(mask, 5)")
	require.Contains(t, manager, "API.SpxExampleCollect.Invoke(arg0, arg2, arg4, arg6)")
	require.Contains(t, manager, "CopyNativeArrayOutput(out, arg4)")
	require.Contains(t, manager, "CopyNativeArrayOutput(indices, arg6)")
	require.NotContains(t, manager, "CopyNativeArrayOutput(objs")
	require.NotContains(t, manager, "CopyNativeArrayOutput(mask")
}

func TestMixedBuffersShareWebConversion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void collect(const GdObj objects[2], const float *values, int count, GdObj selected[3]);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleCollect)(const GdObj *objects, const float *values, int count, GdObj *selected);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	functions := ast.CollectGDExtensionInterfaceFunctions()
	function := &functions[0]
	require.Equal(t, []string{"objects", "values", "selected"}, generation.jsArgs(function))
	body := generation.jsBody(function)
	require.Contains(t, body, `RequireNativeArray(objects, "gdspx_example_collect", 6, false)`)
	require.Contains(t, body, `RequireNativeArray(selected, "gdspx_example_collect", 6, true)`)
	require.Contains(t, body, "NativeArrayCount(objects) < 2")
	require.Contains(t, body, "NativeArrayCount(selected) < 3")
	require.Contains(t, body, "var _arg2 = NativeArrayCount(values);")
	require.Contains(t, body, "_call(_arg0, _arg1, _arg2, _arg3);")
	manager := generation.managerBody(function)
	require.Contains(t, manager, "if objects == nil")
	require.Contains(t, manager, "if selected == nil")
	require.Contains(t, manager, "if len(values) > 2147483647")
	require.Contains(t, manager, "JsFromNativeArray(objects[:], 6)")
	require.Contains(t, manager, "API.SpxExampleCollect.Invoke(arg0, arg1, arg3)")
	require.Contains(t, manager, "CopyNativeArrayOutput(selected[:], arg3)")
	require.NotContains(t, manager, "CopyNativeArrayOutput(objects")
	require.NotContains(t, manager, "CopyNativeArrayOutput(values")
}

func TestMixedArrayCallPreservesArgumentsAndReleasesOwnedValues(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to execute the generated wrapper")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void collect(GdString label, int mode, const GdObj *objects, int count, float *out, int out_len, float ret_value[3]);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	const name = "GDExtensionSpxExampleCollect"
	function := arrayFunction(t, name, headers.Metadata.ArrayBridges[name])
	generation := &Generator{GenerationContext: common.NewGenerationContext(clang.CHeaderFileAST{}, headers.Metadata)}
	require.Equal(t, []string{"label", "mode", "objects", "out", "ret_value"}, generation.jsArgs(function))
	body := generation.jsBody(function)
	manager := generation.managerBody(function)
	require.Contains(t, manager, "arg1 := mode")
	require.Contains(t, manager, "Invoke(arg0, arg1, arg2, arg4, arg6)")
	require.Contains(t, manager, "CopyNativeArrayOutput(out, arg4)")
	require.Contains(t, manager, "CopyNativeArrayOutput(ret_value[:], arg6)")
	require.NotContains(t, manager, "return JsTo")
	script := `const assert = require('node:assert/strict');
const invoke = new Function('_call', 'ToGdString', 'FreeGdString', 'RequireNativeArray', 'NativeArrayCount', 'label', 'mode', 'objects', 'out', 'ret_value', ` + strconv.Quote(body) + `);
for (const failure of [null, 'buffer', 'call']) {
 const freed = [];
 let calls = 0;
 const nativeCall = (...args) => {
  calls++;
  assert.deepEqual(args, [77, 9, 16, 2, 128, 7, 256]);
  if (failure === 'call') throw new Error('call');
 };
 const requireArray = (a, op, type, writable) => {
  assert.equal(a.type, type);
  assert.equal(a.writable, writable);
  if (failure === 'buffer' && a.ptr === 128) throw new Error('buffer');
  return a.ptr;
 };
 const run = () => invoke(nativeCall, label => { assert.equal(label, 'query'); return 77; }, p => freed.push(p), requireArray, a => a.count, 'query', 9,
  {ptr:16, count:2, type:6, writable:false}, {ptr:128, count:7, type:2, writable:true}, {ptr:256, count:3, type:2, writable:true});
 if (failure) assert.throws(run, new RegExp(failure)); else run();
 assert.deepEqual(freed, [77]);
 assert.equal(calls, failure === 'buffer' ? 0 : 1);
}`
	output, err := exec.Command(node, "-e", script).CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func arrayFunction(t *testing.T, name string, spec common.ArrayBridge) *clang.TypedefFunction {
	t.Helper()
	var params []string
	for _, param := range spec.Params() {
		params = append(params, param.Declaration())
	}
	ast, err := clang.ParseCString("typedef void (*" + name + ")(" + strings.Join(params, ", ") + ");")
	require.NoError(t, err)
	function := ast.CollectGDExtensionInterfaceFunctions()[0]
	return &function
}
