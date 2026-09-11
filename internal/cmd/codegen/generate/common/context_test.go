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

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/stretchr/testify/require"
)

func TestGenerationContextSnapshotsMetadata(t *testing.T) {
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxSpriteShow)();")
	require.NoError(t, err)
	metadata := GenerationMetadata{
		ManagerNames:          []string{"sprite"},
		NativeArrayBridges:    map[string]NativeArrayBridgeSpec{"first": {GoArgType: "[]float32"}},
		ArrayTransformBridges: map[string]ArrayTransformBridgeSpec{"first": {FunctionName: "first", Params: []RawExportParam{{Name: "input"}}}},
	}
	first := NewGenerationContext(ast, metadata)
	metadata.ManagerNames[0] = "camera"
	metadata.NativeArrayBridges["first"] = NativeArrayBridgeSpec{GoArgType: "changed"}
	metadata.ArrayTransformBridges["first"].Params[0].Name = "changed"
	second := NewGenerationContext(clang.CHeaderFileAST{}, metadata)
	require.Equal(t, "sprite", first.GetManagerName("GDExtensionSpxSpriteShow"))
	require.True(t, first.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.False(t, second.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	spec, ok := first.GetNativeArrayBridgeSpec("first")
	require.True(t, ok)
	require.Equal(t, "[]float32", spec.GoArgType)
	transform, ok := first.GetArrayTransformBridgeSpec("first")
	require.True(t, ok)
	require.Equal(t, "input", transform.Params[0].Name)
	transform.Params[0].Name = "caller mutation"
	first.ListArrayTransformBridgeSpecs()[0].Params[0].Name = "another mutation"
	again, _ := first.GetArrayTransformBridgeSpec("first")
	require.Equal(t, "input", again.Params[0].Name)
	require.Equal(t, "int64", first.MustGoTypeForCType("GdInt", "test"))
}

func TestGetManagersDoesNotChangePreparedAST(t *testing.T) {
	first, err := clang.ParseCString("typedef void (*GDExtensionSpxSpriteShow)();")
	require.NoError(t, err)
	second, err := clang.ParseCString("typedef void (*GDExtensionSpxCameraShow)();")
	require.NoError(t, err)
	generation := NewGenerationContext(first, GenerationMetadata{ManagerNames: []string{"sprite", "camera"}})
	require.Equal(t, []string{"camera"}, generation.GetManagers(second))
	require.True(t, generation.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxSpriteShow"}))
	require.False(t, generation.IsManagerMethod(&clang.TypedefFunction{Name: "GDExtensionSpxCameraShow"}))
}

func TestGenerationContextsRenderIndependently(t *testing.T) {
	for _, manager := range []string{"foo", "foobar"} {
		generation := NewGenerationContext(clang.CHeaderFileAST{}, GenerationMetadata{ManagerNames: []string{manager}})
		// Render concurrent outputs using the same prepared context as well as distinct contexts.
		for _, output := range []string{"native", "web"} {
			t.Run(manager+"/"+output, func(t *testing.T) {
				t.Parallel()
				dst := filepath.Join(t.TempDir(), "manager.go")
				err := GenerateFile(template.FuncMap{"manager": generation.GetManagerName}, "manager.go",
					`package generated
const Manager = "{{manager .}}"
`, "GDExtensionSpxFooBar", dst)
				require.NoError(t, err)
				data, err := os.ReadFile(dst)
				require.NoError(t, err)
				require.Contains(t, string(data), `const Manager = "`+manager+`"`)
			})
		}
	}
}
