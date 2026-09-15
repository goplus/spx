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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/gdext"
	"github.com/stretchr/testify/require"
)

func TestGenerateManagerWrapperRunsNativeCallsOnMainThread(t *testing.T) {
	metadata := common.GenerationMetadata{}

	metadata.ManagerNames = append(metadata.ManagerNames, "sprite")
	metadata.ManagerNames = append(metadata.ManagerNames, "platform")

	ast := clang.CHeaderFileAST{Expr: []clang.Expr{
		{Function: managerFunction(
			"GDExtensionSpxSpriteSetTriggerEnabled",
			"void",
			managerArgument("obj", "GdObj"),
			managerArgument("trigger", "GdBool"),
		)},
		{Function: managerFunction(
			"GDExtensionSpxSpriteIsCollisionEnabled",
			"GdBool",
			managerArgument("obj", "GdObj"),
		)},
		{Function: managerFunction(
			"GDExtensionSpxPlatformIsMainThread",
			"GdBool",
		)},
	}}

	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, metadata)}
	require.NoError(t, generation.writeManager(codegenDir))
	generatedPath := filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_native.gen.go")
	generated, err := os.ReadFile(generatedPath)
	require.NoError(t, err)

	setTriggerEnabled := generatedMethod(t, generatedPath, generated, "SetTriggerEnabled")
	require.Contains(t, setTriggerEnabled, `enginewrap.CallInMainThread(func() {
		arg0 := ToGdObj(obj)
		arg1 := ToGdBool(trigger)
		CallSpriteSetTriggerEnabled(arg0, arg1)
	})`)

	isCollisionEnabled := generatedMethod(t, generatedPath, generated, "IsCollisionEnabled")
	require.Contains(t, isCollisionEnabled, `return enginewrap.CallInMainThreadValue(func() bool {
		arg0 := ToGdObj(obj)
		retValue := CallSpriteIsCollisionEnabled(arg0)
		return ToBool(retValue)
	})`)

	isMainThread := generatedMethod(t, generatedPath, generated, "IsMainThread")
	require.NotContains(t, isMainThread, "enginewrap.CallInMainThread")
	require.Contains(t, isMainThread, "retValue := CallPlatformIsMainThread()")
	require.Contains(t, isMainThread, "return ToBool(retValue)")
}

func TestFixedOutputManagerUsesArrayPointerWithoutLength(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void write_values(float out[3]);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleWriteValues)(float *out);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.writeManager(codegenDir))
	path := filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_native.gen.go")
	output, err := os.ReadFile(path)
	require.NoError(t, err)
	method := generatedMethod(t, path, output, "WriteValues")
	require.Contains(t, method, "WriteValues(out *[3]float32)")
	require.Contains(t, method, "if out == nil")
	require.Contains(t, method, "arg0 := unsafe.SliceData(out[:])")
	require.Contains(t, method, "CallExampleWriteValues(arg0)")
	require.NotContains(t, method, "len(out)")
}

func TestNativeBuffersPassIndependentLengths(t *testing.T) {
	dir := t.TempDir()
	const params = "const GdObj *objs, int count, float *out, int out_len"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void collect(`+params+`);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleCollect)(" + params + ");")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.writeManager(codegenDir))
	path := filepath.Join(codegenDir, common.GDEngineImplRelDir, "manager_native.gen.go")
	output, err := os.ReadFile(path)
	require.NoError(t, err)
	method := generatedMethod(t, path, output, "Collect")
	require.Contains(t, method, "Collect(objs []int64, out []float32)")
	require.Contains(t, method, "arg0 := unsafe.SliceData(objs)")
	require.Contains(t, method, "arg1 := int32(len(objs))")
	require.Contains(t, method, "arg2 := unsafe.SliceData(out)")
	require.Contains(t, method, "arg3 := int32(len(out))")
	require.Contains(t, method, "CallExampleCollect((*GdObj)(unsafe.Pointer(arg0)), arg1, arg2, arg3)")
}

func TestMixedBuffersShareNativePointerConversion(t *testing.T) {
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
	body := generation.managerBody(&functions[0])
	for _, source := range []string{"objects[:]", "values", "selected[:]"} {
		require.Contains(t, body, "unsafe.SliceData("+source+")")
	}
	require.Contains(t, body, "if objects == nil")
	require.Contains(t, body, "if selected == nil")
	require.Contains(t, body, "if len(values) > math.MaxInt32")
	require.Contains(t, body, "arg2 := int32(len(values))")
	require.Contains(t, body, "CallExampleCollect((*GdObj)(unsafe.Pointer(arg0)), arg1, arg2, (*GdObj)(unsafe.Pointer(arg3)))")
}

func TestMixedScalarsAndArraysPreserveNativeArguments(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API void collect(GdString label, int mode, const GdObj *objects, int count, float *out, int out_len, float ret_value[3]);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleCollect)(GdString label, int mode, const GdObj *objects, int count, float *out, int out_len, float *ret_value);")
	require.NoError(t, err)
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	function := ast.CollectGDExtensionInterfaceFunctions()[0]
	body := generation.managerBody(&function)
	require.Contains(t, body, "arg1 := mode")
	require.Contains(t, body, "arg3 := int32(len(objects))")
	require.Contains(t, body, "arg5 := int32(len(out))")
	require.Contains(t, body, "unsafe.SliceData(ret_value[:])")
	require.Contains(t, body, "defer C.free(unsafe.Pointer(arg0Str))")
	require.Contains(t, body, "CallExampleCollect(arg0, arg1, (*GdObj)(unsafe.Pointer(arg2)), arg3, arg4, arg5, arg6)")
	require.NotContains(t, body, "var retValue")
}

func TestOutputOnlyBuffersKeepNativeStatusAndCallerStorage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxBaseMgr {
public:
 SPX_API GdBool collect(const GdObj *ids, int count, SPX_OUT float *out, int out_len);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	for _, declaration := range []string{
		"typedef GdBool (*GDExtensionSpxExampleCollect)(const GdObj *ids, int count, float *out, int out_len);",
		"typedef void (*GDExtensionSpxExampleCollect)(const GdObj *ids, int count, float *out, int out_len, GdBool *ret_value);",
	} {
		ast, err := clang.ParseCString(declaration)
		require.NoError(t, err)
		generation := &Generator{GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
		function := ast.CollectGDExtensionInterfaceFunctions()[0]
		body := generation.managerBody(&function)
		require.Contains(t, body, "return enginewrap.CallInMainThreadValue(func() bool")
		require.Contains(t, body, "unsafe.SliceData(out)")
		require.Contains(t, body, "int32(len(out))")
		require.Contains(t, body, "return ToBool(retValue)")
		require.NotContains(t, body, "make(")
	}
}

func managerFunction(name, returnType string, arguments ...clang.Argument) *clang.TypedefFunction {
	return &clang.TypedefFunction{
		Name:       name,
		ReturnType: clang.PrimitiveType{Name: returnType},
		Arguments:  arguments,
	}
}

func managerArgument(name, typeName string) clang.Argument {
	return clang.Argument{
		Name: name,
		Type: clang.Type{Primitive: &clang.PrimitiveType{Name: typeName}},
	}
}

func generatedMethod(t *testing.T, filename string, source []byte, name string) string {
	t.Helper()

	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, filename, source, 0)
	require.NoError(t, err)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name {
			continue
		}
		start := files.Position(function.Pos()).Offset
		end := files.Position(function.End()).Offset
		return string(source[start:end])
	}
	t.Fatalf("generated method %s not found", name)
	return ""
}
