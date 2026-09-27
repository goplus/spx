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

package ffi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/gdext"
	"github.com/stretchr/testify/require"
)

func TestSpriteImplementationsPreservePublicParameters(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_sprite_mgr.h"), []byte(`class SpxSpriteMgr {
public:
 SPX_BIND void set_enabled(GdObj obj, GdBool enabled);
 SPX_BIND GdBool collect(GdObj obj, SPX_OUT float *values, int count);
 SPX_BIND void write(GdObj obj, float output[3]);
 SPX_BIND void collect_objects(const GdObj *obj, int count);
 SPX_BIND void select(GdBool enabled, GdObj obj);
 SPX_BIND GdString label();
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString(`
typedef void (*GDExtensionSpxSpriteSetEnabled)(GdObj obj, GdBool enabled);
typedef GdBool (*GDExtensionSpxSpriteCollect)(GdObj obj, float *values, int count);
typedef void (*GDExtensionSpxSpriteWrite)(GdObj obj, float *output);
typedef void (*GDExtensionSpxSpriteCollectObjects)(const GdObj *obj, int count);
typedef void (*GDExtensionSpxSpriteSelect)(GdBool enabled, GdObj obj);
typedef GdString (*GDExtensionSpxSpriteLabel)();`)
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.writeManagerImpl(codegenDir, "Sprite"))
	methods := []struct {
		name, signature, call, zero string
	}{
		{"SetEnabled", "SetEnabled(enabled bool)", "SpriteMgr.SetEnabled(pself.Id, enabled)", ""},
		{"Collect", "Collect(values []float32) bool", "return SpriteMgr.Collect(pself.Id, values)", "return false"},
		{"Write", "Write(output *[3]float32)", "SpriteMgr.Write(pself.Id, output)", ""},
		{"CollectObjects", "CollectObjects(obj []int64)", "SpriteMgr.CollectObjects(obj)", ""},
		{"Select", "Select(enabled bool, obj Object)", "SpriteMgr.Select(enabled, obj)", ""},
		{"Label", "Label() string", "return SpriteMgr.Label()", `return ""`},
	}
	for _, filename := range []string{"sprite.gen.go", "sprite_pure.gen.go"} {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(codegenDir, common.EnginePkgRelDir, filename)
			output, err := os.ReadFile(path)
			require.NoError(t, err)
			for _, method := range methods {
				body := method.call
				if filename == "sprite_pure.gen.go" {
					body = method.zero
				}
				if body != "" {
					body = "\t" + body + "\n"
				}
				require.Equal(t, "func (pself *Sprite) "+method.signature+" {\n"+body+"}",
					generatedMethod(t, path, output, method.name))
			}
		})
	}
}
