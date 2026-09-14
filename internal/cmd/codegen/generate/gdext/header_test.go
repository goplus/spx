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

package gdext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestMergeManagerHeaderSupportsAdditionalBaseClasses(t *testing.T) {

	dir := t.TempDir()
	header := strings.TrimSpace(`
class SpxUiMgr : public SpxBaseMgr, private SpxUiBindingListener {
public:
	SPX_API GdObj bind_node(GdObj obj, GdString rel_path);
};
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_ui_mgr.h"), []byte(header), 0o600))

	merged, err := mergeManagerHeader(dir)
	require.NoError(t, err)
	require.Contains(t, merged, "class SpxUiMgr")
	require.Contains(t, parseManagerHeader(merged).render(false), "GDExtensionSpxUiBindNode")
}

func TestGenerateManagerHeaderUsesExplicitExports(t *testing.T) {

	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_update_transforms(const float *buffer_data, int len);
	SPX_BIND GdBool destroy_sprite(GdObj obj);
	void helper_not_exported();
};
`)

	header := parseManagerHeader(input)
	output := header.render(false)
	generation := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	require.Contains(t, output, "GDExtensionSpxSpriteDestroySprite")
	require.NotContains(t, output, "GDExtensionSpxSpriteHelperNotExported")

	spec, ok := generation.ArrayBridge("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.Buffers[0].ArgName())
	require.Equal(t, "[]float32", spec.Buffers[0].GoType())
	require.EqualValues(t, 2, spec.Buffers[0].Type)
	require.Equal(t, "batch_update_transforms", spec.MethodName)
}

func TestGodotJsTemplateUsesModuleRelativeIncludes(t *testing.T) {
	require.Contains(t, gdJsSpxCpp, `#include "../gdextension_spx_ext.h"`)
	require.Contains(t, gdJsSpxCpp, `#include "../spx_engine.h"`)
	require.Contains(t, gdJsSpxCpp, `#include "godot_js_spx_util.h"`)
	require.NotContains(t, gdJsSpxCpp, `#include "modules/spx/`)
}

func TestGodotJsTemplateKeepsResStringOwned(t *testing.T) {

	outputPath := filepath.Join(t.TempDir(), "godot_js_spx.cpp")
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: &clang.TypedefFunction{
		ReturnType: clang.PrimativeType{Name: "void"},
		Name:       "GDExtensionSpxResFreeStr",
		Arguments: []clang.Argument{{
			Type: clang.Type{Primative: &clang.PrimativeType{Name: "GdString"}},
			Name: "str",
		}},
	}}}}

	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	require.NoError(t, generation.writeCPP(outputPath, gdJsSpxCpp))
	generated, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(generated)
	require.Contains(t, body, "void gdspx_res_free_str(GdString *str)")
	require.Contains(t, body, "gdspx_get_string_value(str, &gdspx_string_arg_0)")
	require.Contains(t, body, "(void)gdspx_string_arg_0;")
	require.NotContains(t, body, "resMgr->free_str(*str)")
}

func TestGodotJsTemplateValidatesAndBindsGdStrings(t *testing.T) {

	outputPath := filepath.Join(t.TempDir(), "godot_js_spx.cpp")
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: &clang.TypedefFunction{
		ReturnType: clang.PrimativeType{Name: "GdString"},
		Name:       "GDExtensionSpxResReadAllText",
		Arguments: []clang.Argument{{
			Type: clang.Type{Primative: &clang.PrimativeType{Name: "GdString"}},
			Name: "p_path",
		}},
	}}}}

	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	require.NoError(t, generation.writeCPP(outputPath, gdJsSpxCpp))
	generated, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(generated)
	require.Contains(t, body, "gdspx_prepare_string_wrapper(ret_val)")
	require.Contains(t, body, "GdString gdspx_string_arg_0 = nullptr;")
	require.Contains(t, body, "gdspx_get_string_value(p_path, &gdspx_string_arg_0)")
	require.Contains(t, body, "GdString result = resMgr->read_all_text(gdspx_string_arg_0);")
	require.Contains(t, body, "gdspx_bind_string_wrapper(ret_val, result)")
	require.NotContains(t, body, "resMgr->read_all_text(*p_path)")
}

func TestGodotJsTemplateValidatesAndBindsGdArrays(t *testing.T) {

	outputPath := filepath.Join(t.TempDir(), "godot_js_spx.cpp")
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: &clang.TypedefFunction{
		ReturnType: clang.PrimativeType{Name: "GdArray"},
		Name:       "GDExtensionSpxPhysicsRaycastWithDetails",
		Arguments: []clang.Argument{{
			Type: clang.Type{Primative: &clang.PrimativeType{Name: "GdArray"}},
			Name: "ignore_sprites",
		}},
	}}}}

	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	require.NoError(t, generation.writeCPP(outputPath, gdJsSpxCpp))
	generated, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(generated)
	require.Contains(t, body, "gdspx_prepare_array_wrapper(ret_val)")
	require.Contains(t, body, "gdspx_validate_array_wrapper(ignore_sprites)")
	require.Contains(t, body, "*ret_val = result;")
	require.Contains(t, body, "gdspx_bind_array_wrapper(ret_val)")
	require.Contains(t, body, "gdspx_release_array_info(result)")
}

func TestGenerateManagerHeaderRegistersCallerArrayBuffer(t *testing.T) {

	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_update_transforms(const float *buffer_data, int len);
};
`)

	header := parseManagerHeader(input)
	output := header.render(false)
	generation := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	spec, ok := generation.ArrayBridge("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.Buffers[0].ArgName())
	require.Equal(t, "buffer_data", spec.Buffers[0].Data.Name)
	require.Equal(t, "len", spec.Buffers[0].Length.Name)
	require.Equal(t, "[]float32", spec.Buffers[0].GoType())
}

func TestArrayTransformUsesOrdinaryGdArraySignature(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API GdArray expand_values(GdArray input);
 };`)
	require.Empty(t, header.metadata.ArrayBridges)
	require.Contains(t, header.render(true), "typedef GdArray (*GDExtensionSpxExampleExpandValues)(GdArray input);")
	require.Contains(t, header.render(false), "typedef void (*GDExtensionSpxExampleExpandValues)(GdArray input, GdArray *ret_value);")
	ast, err := clang.ParseCString(header.render(true))
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, header.metadata)}
	for _, tmpl := range []string{gdSpxExtCpp, gdJsSpxCpp} {
		path := filepath.Join(t.TempDir(), "wrapper.cpp")
		require.NoError(t, generation.writeCPP(path, tmpl))
		output, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(output), "exampleMgr->expand_values(")
		require.NotContains(t, string(output), "SpxBaseMgr::create_array")
		if tmpl == gdJsSpxCpp {
			require.Contains(t, string(output), "gdspx_validate_array_wrapper(input)")
			require.Contains(t, string(output), "gdspx_bind_array_wrapper(ret_val)")
			require.Contains(t, string(output), "gdspx_release_array_info(result)")
		}
	}
}

func TestArrayBridgesKeepDistinctBufferOwnership(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API void update_values(const float *values, int count);
 SPX_API void read_bytes(uint8_t out[7]);
 SPX_API GdArray read_positions(GdArray objects);
 };`)
	generation := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)
	specs := generation.ListArrayBridges()
	require.Len(t, specs, 2)
	require.Equal(t, "GDExtensionSpxExampleReadBytes", specs[0].FunctionName)
	input, _ := generation.ArrayBridge("GDExtensionSpxExampleUpdateValues")
	require.Len(t, input.Buffers, 1)
	require.False(t, input.Buffers[0].Writable())
	require.Equal(t, "[]float32", input.Buffers[0].GoType())
	output, _ := generation.ArrayBridge("GDExtensionSpxExampleReadBytes")
	require.Len(t, output.Buffers, 1)
	require.True(t, output.Buffers[0].Writable())
	require.Equal(t, 7, output.Buffers[0].Count)
	require.Equal(t, "*[7]byte", output.Buffers[0].GoType())
	_, ok := generation.ArrayBridge("GDExtensionSpxExampleReadPositions")
	require.False(t, ok)
	function := &clang.TypedefFunction{Name: "GDExtensionSpxExampleReadPositions"}
	require.Nil(t, generation.Parameter(function, "objects").Buffer)
}

func TestFixedArrayOutputTypesAndPointerABI(t *testing.T) {
	for _, tt := range []struct {
		cType, goType string
	}{
		{"float", "*[7]float32"},
		{"real_t", "*[7]float32"},
		{"int64_t", "*[7]int64"},
		{"uint8_t", "*[7]byte"},
		{"GdObj", "*[7]int64"},
	} {
		t.Run(tt.cType, func(t *testing.T) {
			header := parseManagerHeader(fmt.Sprintf(`class SpxExampleMgr {
 SPX_API void write_values(%s out_data [ 7 ]);
 };`, tt.cType))
			spec := header.metadata.ArrayBridges["GDExtensionSpxExampleWriteValues"]
			require.Equal(t, 7, spec.Buffers[0].Count)
			require.Equal(t, "out", spec.Buffers[0].ArgName())
			require.Equal(t, tt.goType, spec.Buffers[0].GoType())
			require.Empty(t, spec.Buffers[0].Length)
			for _, raw := range []bool{true, false} {
				require.Contains(t, header.render(raw), "typedef void (*GDExtensionSpxExampleWriteValues)("+tt.cType+" *out_data);")
			}
			ast, err := clang.ParseCString(header.render(true))
			require.NoError(t, err)
			generation := &Generator{GenerationContext: common.NewGenerationContext(ast, header.metadata)}
			for _, tmpl := range []string{gdSpxExtCpp, gdJsSpxCpp} {
				path := filepath.Join(t.TempDir(), "wrapper.cpp")
				require.NoError(t, generation.writeCPP(path, tmpl))
				output, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Contains(t, string(output), "exampleMgr->write_values(out_data);")
				require.NotContains(t, string(output), "write_values(out_data,")
			}
		})
	}
}

func TestArrayOutputRejectsInvalidDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"SPX_API void write_values(float out[0]);",
		"SPX_API void write_values(float out[-1]);",
		"SPX_API void write_values(float out[2147483648]);",
		"SPX_API void write_values(float out[]);",
		"SPX_API void write_values(float out[COUNT]);",
		"SPX_API void write_values(float out[2][3]);",
		"SPX_API void write_values(float *out[3]);",
		"SPX_API GdBool write_values(float out[3]);",
		"SPX_BINDING(elements_per_input=2) SPX_API void write_values(float out[3]);",
	} {
		require.Panics(t, func() {
			parseManagerHeader("class SpxExampleMgr {\n" + declaration + "\n};")
		}, declaration)
	}
}

func TestWebBindingPreservesNativeSignatures(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_BINDING(web=noop) SPX_API void release_value(GdString text);
 SPX_BINDING(web=reuse_result) SPX_API GdVec3 get_direction();
 SPX_API GdVec3 get_owned_direction();
 };`)
	require.Equal(t, map[string]common.WebBindingMode{
		"GDExtensionSpxExampleReleaseValue": common.WebBindingNoop,
		"GDExtensionSpxExampleGetDirection": common.WebBindingReuseResult,
	}, header.metadata.WebBindings)
	require.Contains(t, header.render(true), "typedef void (*GDExtensionSpxExampleReleaseValue)(GdString text);")
	require.Contains(t, header.render(true), "typedef GdVec3 (*GDExtensionSpxExampleGetDirection)();")
}

func TestWebBindingRejectsInvalidDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"SPX_BINDING(web=) SPX_API void release();",
		"SPX_BINDING(web=unknown) SPX_API void release();",
		"SPX_BINDING(web=noop) SPX_API GdVec2 read();",
		"SPX_BINDING(web=reuse_result) SPX_API void read();",
		"SPX_BINDING(web=reuse_result) SPX_API GdString read();",
		"SPX_BINDING(web=reuse_result) SPX_API GdArray read();",
		"SPX_BINDING(web=noop) SPX_API void read(float out[3]);",
		"SPX_BINDING(web=noop, elements_per_input=2) SPX_API void read(const GdObj *objs, int count, float *out, int capacity);",
	} {
		require.Panics(t, func() {
			parseManagerHeader("class SpxExampleMgr {\n" + declaration + "\n};")
		}, declaration)
	}
}

func TestBindingAnnotationPlacementAndWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spx_example_mgr.h")
	var reference Headers
	for i, declaration := range []string{
		"SPX_BINDING(web=noop) SPX_API void release(GdString text);",
		"SPX_BINDING ( web = noop )\n\n// Release text.\nSPX_API void release(GdString text);",
	} {
		header := "class SpxExampleMgr : public SpxBaseMgr {\npublic:\n" + declaration + "\n};"
		require.NoError(t, os.WriteFile(path, []byte(header), 0o600))
		generated, err := PrepareHeaders(dir)
		require.NoError(t, err)
		if i == 0 {
			reference = generated
		} else {
			require.Equal(t, reference, generated)
		}
	}
}

func TestBindingRejectsMalformedAndConflictingOptions(t *testing.T) {
	for _, declaration := range []string{
		"SPX_BINDING() SPX_API void release();",
		"SPX_BINDING(web) SPX_API void release();",
		"SPX_BINDING(web=noop,) SPX_API void release();",
		"SPX_BINDING(unknown=1) SPX_API void release();",
		"SPX_BINDING(output_count=3) SPX_API void read(float *out, int len);",
		"SPX_BINDING(array_arg=values, elements_per_input=2) SPX_API void read(const float *in, int count, float *out, int len);",
		"SPX_BINDING(web=noop, web=noop) SPX_API void release();",
		"SPX_BINDING(web=noop) SPX_BINDING(web=noop) SPX_API void release();",
		"SPX_BINDING(web=noop)\nSPX_BINDING(web=noop)\nSPX_API void release();",
		"SPX_BINDING(web=noop SPX_API void release();",
		"SPX_BINDING(web=noop)",
		"SPX_BINDING(web=noop)\nvoid release();",
		"SPX_BINDING(array_arg=values) SPX_API void read(float *out, int len);",
		"SPX_BINDING(elements_per_input=2) SPX_API void read(const GdObj *objs, int count, float *out, int len);",
		"SPX_BINDING(array_arg=bad-name, elements_per_input=2) SPX_API void read(float *out, int len);",
		"SPX_BINDING(elements_per_input=2147483648) SPX_API void read(float *out, int len);",
		"SPX_BINDING(elements_per_input=2, web=noop) SPX_API void read(const float *in, int count, float *out, int len);",
	} {
		t.Run(declaration, func(t *testing.T) {
			require.Panics(t, func() {
				parseManagerHeader("class SpxExampleMgr {\n" + declaration + "\n};")
			})
		})
	}
}

func TestHeaderRenderingDoesNotChangeMetadata(t *testing.T) {
	input := `class SpxSpriteMgr {
 SPX_API GdBool destroy_sprite(GdObj obj);
 SPX_API void batch_retrieve_positions(const GdObj *objs, int count, float *out, int out_len);
 };`
	header := parseManagerHeader(input)
	before := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)
	standard, raw := header.render(false), header.render(true)
	require.Equal(t, standard, header.render(false))
	require.Equal(t, raw, header.render(true))
	require.Contains(t, raw, "typedef GdBool (*GDExtensionSpxSpriteDestroySprite)(GdObj obj);")
	require.Contains(t, standard, "typedef void (*GDExtensionSpxSpriteDestroySprite)(GdObj obj, GdBool *ret_value);")
	after := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)
	require.Equal(t, before.ListArrayBridges(), after.ListArrayBridges())
}

func TestArrayBridgeTypeMappings(t *testing.T) {
	for _, tt := range []struct {
		declaration string
		abi         int32
		cType       string
		cConstant   string
		goSlice     string
	}{
		{"int64_t *", 1, "int64_t", "GD_ARRAY_TYPE_INT64", "[]int64"},
		{"float *", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32"},
		{"float*", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32"},
		{"real_t *", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32"},
		{"uint8_t *", 5, "uint8_t", "GD_ARRAY_TYPE_BYTE", "[]byte"},
		{"GdObj *", 6, "GdObj", "GD_ARRAY_TYPE_GDOBJ", "[]int64"},
	} {
		t.Run(tt.declaration, func(t *testing.T) {
			header := parseManagerHeader(fmt.Sprintf(`class SpxExampleMgr {
 SPX_API void update_values(const %svalues_data, int32_t count);
 };`, tt.declaration))
			caller, ok := header.metadata.ArrayBridges["GDExtensionSpxExampleUpdateValues"]
			if tt.goSlice == "" {
				require.False(t, ok, "object arrays must use GdArray parameters")
			} else {
				require.True(t, ok)
				require.Equal(t, tt.goSlice, caller.Buffers[0].GoType())
			}

			arrayType, ok := parseArrayType(tt.declaration)
			require.True(t, ok)
			require.EqualValues(t, tt.abi, arrayType)
			require.Equal(t, tt.cType, arrayType.CType())
			require.Equal(t, tt.cConstant, arrayType.CConstant())
		})
	}
}

func TestMixedFixedAndDynamicArraysPreservePointerABI(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API void collect(GdObj const objects[2], float const *values, int count, GdObj selected[3], uint8_t *flags, int flags_len);
 };`)
	spec := header.metadata.ArrayBridges["GDExtensionSpxExampleCollect"]
	require.Len(t, spec.Buffers, 4)
	require.Nil(t, spec.FixedOutput(), "a multi-buffer method has no standalone output reader")
	for i, want := range []struct {
		goType   string
		count    int
		writable bool
	}{
		{"*[2]int64", 2, false}, {"[]float32", 0, false},
		{"*[3]int64", 3, true}, {"[]byte", 0, true},
	} {
		require.Equal(t, want.goType, spec.Buffers[i].GoType())
		require.Equal(t, want.count, spec.Buffers[i].Count)
		require.Equal(t, want.writable, spec.Buffers[i].Writable())
	}
	require.Contains(t, header.render(true), "typedef void (*GDExtensionSpxExampleCollect)(const GdObj *objects, const float *values, int count, GdObj *selected, uint8_t *flags, int flags_len);")
	ast, err := clang.ParseCString(header.render(true))
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, header.metadata)}
	for _, tmpl := range []string{gdSpxExtCpp, gdJsSpxCpp} {
		path := filepath.Join(t.TempDir(), "wrapper.cpp")
		require.NoError(t, generation.writeCPP(path, tmpl))
		output, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(output), "exampleMgr->collect(objects, values, count, selected, flags, flags_len);")
	}
}

func TestReturnParametersHaveExplicitIdentity(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API void output(float ret_value[3]);
 SPX_API void update(float *values, int ret_value);
 SPX_API GdInt read(GdInt ret_value);
 };`)
	require.NotContains(t, header.metadata.ReturnParameters, "GDExtensionSpxExampleOutput")
	require.Equal(t, "ret_value_2", header.metadata.ReturnParameters["GDExtensionSpxExampleRead"].Name)
	require.Contains(t, header.render(false), "GdInt ret_value, GdInt *ret_value_2")
	for _, raw := range []bool{false, true} {
		ast, err := clang.ParseCString(header.render(raw))
		require.NoError(t, err)
		context := common.NewGenerationContext(ast, header.metadata)
		functions := ast.CollectGDExtensionInterfaceFunctions()
		require.Equal(t, "Output(ret_value *[3]float32)", context.ManagerInterfaceSignature(&functions[0]))
		require.False(t, context.HasEffectiveReturn(&functions[0]))
		require.Equal(t, "Update(values []float32)", context.ManagerInterfaceSignature(&functions[1]))
		require.False(t, context.HasEffectiveReturn(&functions[1]))
		require.Equal(t, "Read(ret_value int64) int64 ", context.ManagerInterfaceSignature(&functions[2]))
	}
}

func TestMixedScalarsAndArraysUseOneWebABI(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API void collect(GdString label, int mode, const GdObj *objects, int count, float *out, int out_len, float ret_value[3]);
 };`)
	ast, err := clang.ParseCString(header.render(true))
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, header.metadata)}
	function := ast.CollectGDExtensionInterfaceFunctions()[0]
	require.Equal(t, "Collect(label string, mode int32, objects []int64, out []float32, ret_value *[3]float32)", generation.ManagerInterfaceSignature(&function))
	path := filepath.Join(t.TempDir(), "wrapper.cpp")
	require.NoError(t, generation.writeCPP(path, gdJsSpxCpp))
	output, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(output), "int mode, const GdObj *objects, int count, float *out, int out_len, float *ret_value")
	require.Contains(t, string(output), "exampleMgr->collect(gdspx_string_arg_0, mode, objects, count, out, out_len, ret_value);")
}
