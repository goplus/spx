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
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestCallbackDefaultsFollowABIFieldsAndTypes(t *testing.T) {
	ast, err := clang.ParseCString(`
typedef void (*GDExtensionSpxCallbackReady)();
typedef void (*GDExtensionSpxCallbackChanged)(GdObj obj, GdString value);
typedef struct {
 GDExtensionSpxCallbackChanged custom_field;
 GDExtensionSpxCallbackReady func_on_ready;
} SpxCallbackInfo;`)
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	dir := t.TempDir()
	require.NoError(t, generation.writeCallbackDefaults(dir))
	output, err := os.ReadFile(filepath.Join(dir, "spx_callback_defaults.gen.h"))
	require.NoError(t, err)
	require.Contains(t, string(output), "SpxCallbackInfo callbacks = {};")
	require.Contains(t, string(output), "callbacks.custom_field = [](GdObj, GdString) {};")
	require.Contains(t, string(output), "callbacks.func_on_ready = []() {};")
}

func TestCallbackDefaultsRejectUnknownAndNonVoidFields(t *testing.T) {
	for _, source := range []string{
		"typedef struct { Unknown field; } SpxCallbackInfo;",
		"typedef GdInt (*GDExtensionSpxCallbackValue)(); typedef struct { GDExtensionSpxCallbackValue field; } SpxCallbackInfo;",
		"typedef struct { GdInt count; } Other;",
	} {
		ast, err := clang.ParseCString(source)
		require.NoError(t, err)
		generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
		require.Error(t, generation.writeCallbackDefaults(t.TempDir()))
	}
}
