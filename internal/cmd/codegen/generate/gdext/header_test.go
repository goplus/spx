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
	require.Equal(t, "buffer", spec.ArgName)
	require.Equal(t, "[]float32", spec.CallerBuffer().GoType())
	require.EqualValues(t, 2, spec.CallerBuffer().Type)
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
	require.Equal(t, "buffer", spec.ArgName)
	require.Equal(t, "buffer_data", spec.CallerBuffer().Data.Name)
	require.Equal(t, "len", spec.CallerBuffer().Length.Name)
	require.Equal(t, "[]float32", spec.CallerBuffer().GoType())
	require.Equal(t, "*float32", spec.CallerBuffer().GoPointerType())
	require.Equal(t, "int32(len(buffer))", common.ArrayLengthExpr("buffer"))
}

func TestGenerateManagerHeaderRegistersReturnedArray(t *testing.T) {

	input := strings.TrimSpace(`
class SpxExampleMgr {
public:
	SPX_BINDING(array_arg=values, elements_per_input=3) SPX_API void expand_values(const GdObj *ids, int count, float *out, int out_len);
};
`)

	header := parseManagerHeader(input)
	output := header.render(false)
	generation := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)

	require.Contains(t, output, "GDExtensionSpxExampleExpandValues")

	specs := generation.ListArrayBridges()
	require.Len(t, specs, 1)
	require.Equal(t, "GDExtensionSpxExampleExpandValues", specs[0].FunctionName)
	require.Equal(t, "values", specs[0].ArgName)
	require.Equal(t, "expand_values", specs[0].MethodName)
	require.EqualValues(t, 6, specs[0].Input.Type)
	require.EqualValues(t, 2, specs[0].Output.Type)
	require.Equal(t, 3, specs[0].Output.ElementsPerInput)
	require.Len(t, specs[0].Params(), 4)
	require.Equal(t, "const GdObj *", specs[0].Params()[0].CType)
	require.Equal(t, "ids", specs[0].Params()[0].Name)
	require.Equal(t, "float *", specs[0].Params()[2].CType)
	require.Equal(t, "out", specs[0].Params()[2].Name)
}

func TestGenerateManagerHeaderSynthesizesArrayReturnTypedefForRawFormat(t *testing.T) {

	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_BINDING(array_arg=objs, elements_per_input=2) SPX_API void batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len);
};
`)

	output := parseManagerHeader(input).render(true)

	require.Contains(t, output, "typedef GdArray (*GDExtensionSpxSpriteBatchRetrievePositions)(GdArray objs);")
}

func TestArrayTransformRejectsInvalidDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"SPX_BINDING(array_arg=values, elements_per_input=0) SPX_API void expand_values(const float *in, int count, float *out, int capacity);",
		"SPX_BINDING(array_arg=values, elements_per_input=2) SPX_API void expand_values(const float *in, int count);",
		"SPX_BINDING(array_arg=values, elements_per_input=2) SPX_API void expand_values(const float *in, int count, const float *out, int capacity);",
		"SPX_BINDING(array_arg=values, elements_per_input=2) SPX_API GdBool expand_values(const float *in, int count, float *out, int capacity);",
	} {
		require.Panics(t, func() {
			parseManagerHeader("class SpxExampleMgr {\npublic:\n" + declaration + "\n};")
		}, declaration)
	}
}

func TestArrayBridgesKeepDistinctBufferOwnership(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_API void update_values(const float *values, int count);
 SPX_BINDING(output_count=7) SPX_API void read_bytes(uint8_t *out, int capacity);
 SPX_BINDING(array_arg=ids, elements_per_input=2) SPX_API void read_positions(const GdObj *objects, int count, float *out, int capacity);
 };`)
	generation := common.NewGenerationContext(clang.CHeaderFileAST{}, header.metadata)
	specs := generation.ListArrayBridges()
	require.Len(t, specs, 3)
	require.Equal(t, "GDExtensionSpxExampleReadBytes", specs[0].FunctionName)
	input, _ := generation.ArrayBridge("GDExtensionSpxExampleUpdateValues")
	require.NotNil(t, input.Input)
	require.Nil(t, input.Output)
	require.False(t, input.ReturnArray)
	require.Equal(t, "[]float32", input.CallerBuffer().GoType())
	output, _ := generation.ArrayBridge("GDExtensionSpxExampleReadBytes")
	require.Nil(t, output.Input)
	require.NotNil(t, output.Output)
	require.False(t, output.ReturnArray)
	require.Equal(t, 7, output.Output.Count)
	require.Equal(t, "[]byte", output.CallerBuffer().GoType())
	require.Equal(t, "*uint8", output.CallerBuffer().GoPointerType())
	result, _ := generation.ArrayBridge("GDExtensionSpxExampleReadPositions")
	require.True(t, result.ReturnArray)
	require.Nil(t, result.CallerBuffer())
	require.EqualValues(t, 6, result.Input.Type)
	require.EqualValues(t, 2, result.Output.Type)
	require.Equal(t, 2, result.Output.ElementsPerInput)
	require.Equal(t, []common.CParam{
		{CType: "const GdObj *", Name: "objects"}, {CType: "int", Name: "count"},
		{CType: "float *", Name: "out"}, {CType: "int", Name: "capacity"},
	}, result.Params())
	// Returned arrays retain GdArray conversion instead of being lowered to a raw Go slice.
	function := &clang.TypedefFunction{Name: result.FunctionName}
	require.False(t, generation.IsArrayBufferArgument(function, clang.Argument{Name: "ids"}))
}

func TestArrayOutputPreservesInPlaceSignature(t *testing.T) {
	header := parseManagerHeader(`class SpxExampleMgr {
 SPX_BINDING(output_count=7) SPX_API void write_values(int64_t *out, int capacity);
 };`)
	spec := header.metadata.ArrayBridges["GDExtensionSpxExampleWriteValues"]
	require.Equal(t, 7, spec.Output.Count)
	require.EqualValues(t, 1, spec.CallerBuffer().Type)
	require.Contains(t, header.render(true), "typedef void (*GDExtensionSpxExampleWriteValues)(int64_t *out, int capacity);")
	require.False(t, spec.ReturnArray)
}

func TestArrayOutputRejectsInvalidDeclarations(t *testing.T) {
	for _, declaration := range []string{
		"SPX_BINDING(output_count=0) SPX_API void write_values(float *out, int len);",
		"SPX_BINDING(output_count=-1) SPX_API void write_values(float *out, int len);",
		"SPX_BINDING(output_count=2147483648) SPX_API void write_values(float *out, int len);",
		"SPX_BINDING(output_count=3) SPX_API void write_values(const float *out, int len);",
		"SPX_BINDING(output_count=3) SPX_API GdBool write_values(float *out, int len);",
		"SPX_BINDING(output_count=3) SPX_API void write_values(GdArray out);",
		"SPX_BINDING(output_count=3) SPX_API void write_values(float *out, int len, int extra);",
		"SPX_BINDING(output_count=3, array_arg=values, elements_per_input=2) SPX_API void write_values(float *out, int len);",
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
		"SPX_BINDING(web=noop, output_count=3) SPX_API void read(float *out, int len);",
		"SPX_BINDING(web=noop, array_arg=objs, elements_per_input=2) SPX_API void read(const GdObj *objs, int count, float *out, int capacity);",
	} {
		require.Panics(t, func() {
			parseManagerHeader("class SpxExampleMgr {\n" + declaration + "\n};")
		}, declaration)
	}
}

func TestBindingAnnotationPlacementAndOptionOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spx_example_mgr.h")
	var reference Headers
	for i, declaration := range []string{
		"SPX_BINDING(array_arg=class_ids, elements_per_input=2) SPX_API void positions(const GdObj *ids, int count, float *out, int len);",
		"SPX_BINDING ( elements_per_input = 2 , array_arg = class_ids )\n\n// Position pairs.\nSPX_API void positions(const GdObj *ids, int count, float *out, int len);",
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
		"SPX_BINDING(web=noop, web=noop) SPX_API void release();",
		"SPX_BINDING(web=noop) SPX_BINDING(web=noop) SPX_API void release();",
		"SPX_BINDING(web=noop)\nSPX_BINDING(web=noop)\nSPX_API void release();",
		"SPX_BINDING(web=noop SPX_API void release();",
		"SPX_BINDING(web=noop)",
		"SPX_BINDING(web=noop)\nvoid release();",
		"SPX_BINDING(array_arg=values) SPX_API void read(float *out, int len);",
		"SPX_BINDING(elements_per_input=2) SPX_API void read(float *out, int len);",
		"SPX_BINDING(array_arg=bad-name, elements_per_input=2) SPX_API void read(float *out, int len);",
		"SPX_BINDING(array_arg=values, elements_per_input=2147483648) SPX_API void read(float *out, int len);",
		"SPX_BINDING(array_arg=values, elements_per_input=2, web=noop) SPX_API void read(const float *in, int count, float *out, int len);",
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
 SPX_BINDING(array_arg=objs, elements_per_input=2) SPX_API void batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len);
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
		goPointer   string
	}{
		{"int64_t *", 1, "int64_t", "GD_ARRAY_TYPE_INT64", "[]int64", "*int64"},
		{"float *", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32", "*float32"},
		{"float*", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32", "*float32"},
		{"real_t *", 2, "float", "GD_ARRAY_TYPE_FLOAT", "[]float32", "*float32"},
		{"uint8_t *", 5, "uint8_t", "GD_ARRAY_TYPE_BYTE", "[]byte", "*uint8"},
		{"GdObj *", 6, "GdObj", "GD_ARRAY_TYPE_GDOBJ", "", ""},
	} {
		t.Run(tt.declaration, func(t *testing.T) {
			header := parseManagerHeader(fmt.Sprintf(`class SpxExampleMgr {
 SPX_API void update_values(const %svalues_data, int32_t count);
 SPX_BINDING(array_arg=values, elements_per_input=2) SPX_API void expand_values(const %sinput, int count, %soutput, int capacity);
 };`, tt.declaration, tt.declaration, tt.declaration))
			spec := header.metadata.ArrayBridges["GDExtensionSpxExampleExpandValues"]
			require.NotNil(t, spec.Input)
			require.NotNil(t, spec.Output)
			require.EqualValues(t, tt.abi, spec.Input.Type)
			require.EqualValues(t, tt.abi, spec.Output.Type)
			caller, ok := header.metadata.ArrayBridges["GDExtensionSpxExampleUpdateValues"]
			if tt.goSlice == "" {
				require.False(t, ok, "object arrays must use GdArray transforms")
				require.Panics(t, func() { spec.Input.GoType() })
			} else {
				require.True(t, ok)
				require.Equal(t, tt.goSlice, caller.CallerBuffer().GoType())
				require.Equal(t, tt.goPointer, caller.CallerBuffer().GoPointerType())
			}

			ast, err := clang.ParseCString(header.render(true))
			require.NoError(t, err)
			generation := &Generator{GenerationContext: common.NewGenerationContext(ast, header.metadata)}
			outputPath := filepath.Join(t.TempDir(), "gdextension_spx_ext.cpp")
			require.NoError(t, generation.writeCPP(outputPath, gdSpxExtCpp))
			output, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			require.Contains(t, string(output), "SpxBaseMgr::create_array("+tt.cConstant+", out_len)")
			require.Contains(t, string(output), "SpxBaseMgr::get_array<"+tt.cType+">(values, 0)")
			require.Contains(t, string(output), "SpxBaseMgr::get_array<"+tt.cType+">(result, 0)")
		})
	}
}
