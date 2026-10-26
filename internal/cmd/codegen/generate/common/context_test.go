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
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestGenerationContextSnapshotsMetadata(t *testing.T) {
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxSpriteShow)();")
	require.NoError(t, err)
	metadata := GenerationMetadata{
		ManagerNames:  []string{"sprite"},
		StaticMethods: map[string]string{"control": "SpxExtMgr::pause"},
		ArrayBridges: map[string]ArrayBridge{"first": {
			FunctionName: "first",
			Buffers:      []ArrayBuffer{{Type: 2, Data: CParam{Name: "input"}}, {Type: 2, Count: 3}},
		}},
	}
	first := NewGenerationContext(ast, metadata)
	metadata.ManagerNames[0] = "camera"
	metadata.StaticMethods["control"] = "SpxExtMgr::resume"
	metadata.ArrayBridges["first"].Buffers[0].Data.Name = "changed"
	metadata.ArrayBridges["first"].Buffers[1].Count = 9
	secondAST, err := clang.ParseCString("typedef void (*GDExtensionSpxCameraShow)();")
	require.NoError(t, err)
	second := NewGenerationContext(secondAST, metadata)
	require.Equal(t, "SpxExtMgr::pause", first.MethodTarget(&clang.TypedefFunction{Name: "control"}))
	require.Equal(t, "SpxExtMgr::resume", second.MethodTarget(&clang.TypedefFunction{Name: "control"}))
	require.Equal(t, "spriteMgr->show", first.MethodTarget(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.Equal(t, "sprite", first.GetManagerName("GDExtensionSpxSpriteShow"))
	require.True(t, first.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.False(t, second.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.Equal(t, []string{"sprite"}, first.ManagerData().Managers)
	require.Equal(t, []string{"camera"}, second.ManagerData().Managers)
	spec, ok := first.ArrayBridge("first")
	require.True(t, ok)
	require.Equal(t, "[]float32", spec.Buffers[0].GoType())
	require.Equal(t, "input", spec.Buffers[0].Data.Name)
	require.Equal(t, 3, spec.Buffers[1].Count)
	spec.Buffers[0].Data.Name = "caller mutation"
	spec.Buffers[1].Count = 8
	again, _ := first.ArrayBridge("first")
	require.Equal(t, "input", again.Buffers[0].Data.Name)
	require.Equal(t, 3, again.Buffers[1].Count)
	require.Equal(t, "int64", first.MustGoTypeForCType("GdInt", "test"))
}

func TestGenerationContextsRenderIndependently(t *testing.T) {
	for _, manager := range []string{"foo", "foobar"} {
		generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{ManagerNames: []string{manager}})
		// Render concurrent outputs using the same prepared context as well as distinct contexts.
		for _, output := range []string{"native", "web"} {
			t.Run(manager+"/"+output, func(t *testing.T) {
				t.Parallel()
				dst := filepath.Join(t.TempDir(), "manager.go")
				var plan OutputPlan
				err := plan.RenderFile(template.FuncMap{"manager": generation.GetManagerName},
					`package generated
const Manager = "{{manager .}}"
`, "GDExtensionSpxFooBar", dst)
				require.NoError(t, err)
				require.NoError(t, plan.Commit())
				data, err := os.ReadFile(dst)
				require.NoError(t, err)
				require.Contains(t, string(data), `const Manager = "`+manager+`"`)
			})
		}
	}
}

func TestCollectCallbacksPreservesABIFieldsAndSignatures(t *testing.T) {
	ast, err := clang.ParseCString(`
typedef void (*GDExtensionSpxCallbackReady)();
typedef void (*GDExtensionSpxCallbackChanged)(GdObj obj, GdString value);
typedef struct {
 GDExtensionSpxCallbackChanged custom_field;
 GDExtensionSpxCallbackReady func_on_ready;
} SpxCallbackInfo;`)
	require.NoError(t, err)
	callbacks, err := CollectCallbacks(ast)
	require.NoError(t, err)
	require.Len(t, callbacks, 2)
	require.Equal(t, "custom_field", callbacks[0].Field)
	require.Equal(t, "func_on_ready", callbacks[1].Field)
	functions := ast.CollectGDExtensionICallbackFunctions()
	require.Equal(t, functions[1], callbacks[0].Function)
	require.Equal(t, functions[0], callbacks[1].Function)
}

func TestCollectCallbacksRejectsUnsupportedABIFields(t *testing.T) {
	for _, source := range []string{
		"typedef struct { Unknown field; } SpxCallbackInfo;",
		"typedef void (*GDExtensionSpxCallbackValue)(); typedef struct { GDExtensionSpxCallbackValue *field; } SpxCallbackInfo;",
		"typedef GdInt (*GDExtensionSpxCallbackValue)(); typedef struct { GDExtensionSpxCallbackValue field; } SpxCallbackInfo;",
		"typedef struct { void (*field)(GdInt value); } SpxCallbackInfo;",
		"typedef struct { GdInt count; } Other;",
	} {
		t.Run(source, func(t *testing.T) {
			ast, err := clang.ParseCString(source)
			require.NoError(t, err)
			callbacks, err := CollectCallbacks(ast)
			require.Error(t, err)
			require.Nil(t, callbacks)
		})
	}
}
