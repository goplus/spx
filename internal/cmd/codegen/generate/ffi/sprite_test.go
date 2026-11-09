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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/gdext"
	"github.com/stretchr/testify/require"
)

func TestSpriteImplementationPreservesPublicParameters(t *testing.T) {
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
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.renderManagerImpl(codegenDir, "Sprite"))
	require.NoError(t, generation.Output.Commit())
	methods := []struct {
		name, signature, call string
	}{
		{"SetEnabled", "SetEnabled(enabled bool)", "SpriteMgr.SetEnabled(s.Id, enabled)"},
		{"Collect", "Collect(values []float32) bool", "return SpriteMgr.Collect(s.Id, values)"},
		{"Write", "Write(output *[3]float32)", "SpriteMgr.Write(s.Id, output)"},
		{"CollectObjects", "CollectObjects(obj []int64)", "SpriteMgr.CollectObjects(obj)"},
		{"Select", "Select(enabled bool, obj Object)", "SpriteMgr.Select(enabled, obj)"},
		{"Label", "Label() string", "return SpriteMgr.Label()"},
	}
	path := filepath.Join(codegenDir, common.EnginePkgRelDir, "sprite.gen.go")
	output, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, method := range methods {
		require.Equal(t, "func (s *Sprite) "+method.signature+" {\n\t"+method.call+"\n}",
			generatedMethod(t, path, output, method.name))
	}
}

func TestManagerAndObjectReceiversAvoidPublicParameters(t *testing.T) {
	for _, tt := range []struct {
		name, className, managerName string
		cArgs, managerArgs, args     string
		receiver, callArgs           string
		buffer                       *common.ArrayBuffer
	}{
		{
			name: "sprite initial", className: "Sprite", managerName: "Sprite",
			cArgs: "GdObj obj, GdString value", managerArgs: "obj Object, value string", args: "value string",
			receiver: "s", callArgs: "s.Id, value",
		},
		{
			name: "UI initial", className: "UiNode", managerName: "Ui",
			cArgs: "GdObj obj, GdString value", managerArgs: "obj Object, value string", args: "value string",
			receiver: "u", callArgs: "u.Id, value",
		},
		{
			name: "single collision", className: "Sprite", managerName: "Sprite",
			cArgs: "GdObj obj, GdString s", managerArgs: "obj Object, s string", args: "s string",
			receiver: "s1", callArgs: "s1.Id, s",
		},
		{
			name: "successive collisions", className: "Sprite", managerName: "Sprite",
			cArgs:       "GdObj obj, GdString s2, GdString s, GdString s1",
			managerArgs: "obj Object, s2 string, s string, s1 string", args: "s2 string, s string, s1 string",
			receiver: "s3", callArgs: "s3.Id, s2, s, s1",
		},
		{
			name: "first free suffix", className: "UiNode", managerName: "Ui",
			cArgs: "GdObj obj, GdString u2, GdString u", managerArgs: "obj Object, u2 string, u string", args: "u2 string, u string",
			receiver: "u1", callArgs: "u1.Id, u2, u",
		},
		{
			name: "public buffer name", className: "Sprite", managerName: "Sprite",
			cArgs: "GdObj obj, const float *s_data, int count", managerArgs: "obj Object, s []float32", args: "s []float32",
			receiver: "s1", callArgs: "s1.Id, s",
			buffer: &common.ArrayBuffer{
				Data:   common.CParam{CType: "const float *", Name: "s_data"},
				Length: common.CParam{CType: "int", Name: "count"}, Type: common.ArrayFloat,
			},
		},
		{
			name: "binding local collisions", className: "Sprite", managerName: "Sprite",
			cArgs:       "GdObj obj, GdString binding, GdString binding1, GdString binding2",
			managerArgs: "obj Object, binding string, binding1 string, binding2 string",
			args:        "binding string, binding1 string, binding2 string",
			receiver:    "s", callArgs: "s.Id, binding, binding1, binding2",
		},
		{
			name: "hidden length does not collide", className: "Sprite", managerName: "Sprite",
			cArgs: "GdObj obj, const float *values, int s", managerArgs: "obj Object, values []float32", args: "values []float32",
			receiver: "s", callArgs: "s.Id, values",
			buffer: &common.ArrayBuffer{
				Data:   common.CParam{CType: "const float *", Name: "values"},
				Length: common.CParam{CType: "int", Name: "s"}, Type: common.ArrayFloat,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			functionName := "GDExtensionSpx" + tt.managerName + "Forward"
			header, err := clang.ParseCString(fmt.Sprintf("typedef void (*%s)(%s);", functionName, tt.cArgs))
			require.NoError(t, err)
			metadata := common.GenerationMetadata{}
			if tt.buffer != nil {
				metadata.ArrayBridges = map[string]common.ArrayBridge{functionName: {Buffers: []common.ArrayBuffer{*tt.buffer}}}
			}
			generation := &Generator{GenerationContext: common.NewGenerationContext(header, metadata)}
			function := header.CollectGDExtensionInterfaceFunctions()[0]
			signature := generation.ManagerMethodSignature(&function)
			method := generation.getManagerImpl(&function, tt.className)
			managerType := strings.ToLower(tt.managerName) + "Mgr"
			wrapperType := tt.managerName + "MgrImpl"
			wrapperMethod := generation.genManagerWrapFunction(&function)
			// Parsing/formatting alone accepts a receiver and parameter with the
			// same name. Type-check both the shared native/Web signature and the
			// object/wrapper forwarding bodies, without cgo or engine dependencies.
			source := fmt.Sprintf("package generated\ntype Object int64\ntype %s struct{ Id Object }\ntype %s struct{}\nvar %sMgr %s\nfunc %s {}\n%s",
				tt.className, managerType, tt.managerName, managerType, signature, method)
			source += fmt.Sprintf("\ntype %s struct{}\nfunc (*%s) resolve() boundManager { return boundManager{} }\ntype boundManager struct{}\nfunc (boundManager) target() *%s { return &%sMgr }\nfunc CallInMainThread(call func()) { call() }\n%s",
				wrapperType, wrapperType, managerType, tt.managerName, strings.ReplaceAll(wrapperMethod, "gdx.Object", "Object"))
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, "receiver.go", source, 0)
			require.NoError(t, err)
			_, err = (&types.Config{}).Check("generated", positions, []*ast.File{file}, nil)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("(%s *%s) Forward(%s)", tt.receiver, managerType, tt.managerArgs), signature)
			require.Equal(t, fmt.Sprintf("func (%s *%s) Forward(%s) {\n\t%sMgr.Forward(%s)\n}\n",
				tt.receiver, tt.className, tt.args, tt.managerName, tt.callArgs), method)
		})
	}
}
