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
	require.Contains(t, generateManagerHeader(merged, false), "GDExtensionSpxUiBindNode")
}

func TestGenerateManagerHeaderSkipsRawMethods(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_update_transforms(GdArray buffer);
	SPX_API void batch_update_transforms_raw(const float *buffer_data, int len);
	SPX_BIND GdBool destroy_sprite(GdObj obj);
	SPX_API void batch_update_visuals_raw(const float *buffer_data, int len);
	void helper_not_exported();
};
`)

	output := generateManagerHeader(input, false)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	require.Contains(t, output, "GDExtensionSpxSpriteDestroySprite")
	require.NotContains(t, output, "GDExtensionSpxSpriteHelperNotExported")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchUpdateTransformsRaw")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchUpdateVisualsRaw")

	spec, ok := common.GetNativeArrayBridgeSpec("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.BaseArgName)
	require.Equal(t, "[]float32", spec.GoArgType)
	require.EqualValues(t, 2, spec.FastArrayType)
	require.Equal(t, "GDExtensionSpxSpriteBatchUpdateTransformsRaw", spec.RawFunctionName)
}

func TestGodotJsTemplateUsesModuleRelativeIncludes(t *testing.T) {
	require.Contains(t, gdJsSpxCpp, `#include "../gdextension_spx_ext.h"`)
	require.Contains(t, gdJsSpxCpp, `#include "../spx_engine.h"`)
	require.Contains(t, gdJsSpxCpp, `#include "godot_js_spx_util.h"`)
	require.NotContains(t, gdJsSpxCpp, `#include "modules/spx/`)
}

func TestGodotJsTemplateKeepsResStringOwned(t *testing.T) {
	projectPath := t.TempDir()
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: &clang.TypedefFunction{
		ReturnType: clang.PrimativeType{Name: "void"},
		Name:       "GDExtensionSpxResFreeStr",
		Arguments: []clang.Argument{{
			Type: clang.Type{Primative: &clang.PrimativeType{Name: "GdString"}},
			Name: "str",
		}},
	}}}}

	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, common.NativeRelDir), 0o755))
	require.NoError(t, generateGdCppFile(projectPath, gdJsSpxCpp, ast, "godot_js_spx.cpp"))
	generated, err := os.ReadFile(filepath.Join(projectPath, common.NativeRelDir, "godot_js_spx.cpp"))
	require.NoError(t, err)
	body := string(generated)
	require.Contains(t, body, "void gdspx_res_free_str(GdString *str)")
	require.Contains(t, body, "(void)str;")
	require.NotContains(t, body, "resMgr->free_str(*str)")
}

func TestGodotJsTemplateValidatesAndBindsGdArrays(t *testing.T) {
	projectPath := t.TempDir()
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: &clang.TypedefFunction{
		ReturnType: clang.PrimativeType{Name: "GdArray"},
		Name:       "GDExtensionSpxPhysicsRaycastWithDetails",
		Arguments: []clang.Argument{{
			Type: clang.Type{Primative: &clang.PrimativeType{Name: "GdArray"}},
			Name: "ignore_sprites",
		}},
	}}}}

	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, common.NativeRelDir), 0o755))
	require.NoError(t, generateGdCppFile(projectPath, gdJsSpxCpp, ast, "godot_js_spx.cpp"))
	generated, err := os.ReadFile(filepath.Join(projectPath, common.NativeRelDir, "godot_js_spx.cpp"))
	require.NoError(t, err)
	body := string(generated)
	require.Contains(t, body, "gdspx_prepare_array_wrapper(ret_val)")
	require.Contains(t, body, "gdspx_validate_array_wrapper(ignore_sprites)")
	require.Contains(t, body, "*ret_val = result;")
	require.Contains(t, body, "gdspx_bind_array_wrapper(ret_val)")
	require.Contains(t, body, "gdspx_release_array_info(result)")
}

func TestGenerateManagerHeaderRegistersDirectNativeArrayBridge(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_update_transforms(const float *buffer_data, int len);
};
`)

	output := generateManagerHeader(input, false)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	spec, ok := common.GetNativeArrayBridgeSpec("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.BaseArgName)
	require.Equal(t, "buffer_data", spec.DataArgName)
	require.Equal(t, "len", spec.LenArgName)
	require.Equal(t, "[]float32", spec.DataArgGoType)
	require.Equal(t, "*float32", spec.DataArgPtrType)
	require.Equal(t, "int32", spec.LenArgGoType)
}

func TestGenerateManagerHeaderRegistersArrayTransformBridge(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len);
};
`)

	output := generateManagerHeader(input, false)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchRetrievePositions")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchRetrievePositionsRaw")

	specs := common.ListArrayTransformBridgeSpecs()
	require.Len(t, specs, 1)
	require.Equal(t, "GDExtensionSpxSpriteBatchRetrievePositions", specs[0].FunctionName)
	require.Equal(t, "objs", specs[0].ArrayArgName)
	require.Equal(t, "batch_retrieve_positions", specs[0].MethodName)
	require.EqualValues(t, 6, specs[0].InputArrayType)
	require.EqualValues(t, 2, specs[0].OutputArrayType)
	require.Equal(t, 2, specs[0].OutputCountScale)
	require.Len(t, specs[0].Params, 4)
	require.Equal(t, "const GdObj *", specs[0].Params[0].CType)
	require.Equal(t, "ids", specs[0].Params[0].Name)
	require.Equal(t, "float *", specs[0].Params[2].CType)
	require.Equal(t, "out", specs[0].Params[2].Name)
}

func TestGenerateManagerHeaderSynthesizesFastReturnTypedefForRawFormat(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	SPX_API void batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len);
};
`)

	output := generateManagerHeader(input, true)

	require.Contains(t, output, "typedef GdArray (*GDExtensionSpxSpriteBatchRetrievePositions)(GdArray objs);")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchRetrievePositionsRaw")
}
