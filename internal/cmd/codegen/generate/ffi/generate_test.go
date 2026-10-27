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
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v4/internal/cmd/codegen/generate/gdext"
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
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, metadata)}
	require.NoError(t, generation.renderManager(codegenDir))
	require.NoError(t, generation.Output.Commit())
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

func TestGenerateWrapperPreservesCEnumValues(t *testing.T) {
	header, err := clang.ParseCString(`typedef enum {
 First,
 Second,
 Explicit = 7,
 AfterExplicit,
 Alias = Explicit,
 AfterAlias,
} Example;`)
	require.NoError(t, err)
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(header, common.GenerationMetadata{})}
	require.NoError(t, generation.renderWrapperGo(codegenDir))
	require.NoError(t, generation.Output.Commit())
	path := filepath.Join(codegenDir, common.NativeRelDir, "ffi_wrapper.gen.go")
	source, err := os.ReadFile(path)
	require.NoError(t, err)
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, source, 0)
	require.NoError(t, err)
	checked := &ast.File{Name: ast.NewIdent("ffi")}
	var values []*ast.ValueSpec
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch group.Tok {
		case token.TYPE:
			// Isolate C.Example from cgo headers while retaining the generated
			// named type and every constant's actual initializer.
			group.Specs[0].(*ast.TypeSpec).Type = ast.NewIdent("int")
			checked.Decls = append(checked.Decls, group)
		case token.CONST:
			checked.Decls = append(checked.Decls, group)
			for _, spec := range group.Specs {
				value := spec.(*ast.ValueSpec)
				// A typed Go constant without an initializer parses but does not
				// compile. C also increments after explicit values and aliases.
				require.Len(t, value.Values, 1, "constant %s needs an initializer", value.Names[0].Name)
				values = append(values, value)
			}
		}
	}
	checkedPackage, err := (&types.Config{}).Check("generated", positions, []*ast.File{checked}, nil)
	require.NoError(t, err)
	want := []string{"0", "1", "7", "8", "7", "8"}
	require.Len(t, values, len(want))
	for i, value := range values {
		name := value.Names[0].Name
		constant := checkedPackage.Scope().Lookup(name).(*types.Const)
		require.Equal(t, want[i], constant.Val().ExactString(), name)
	}
}

func TestGenerateSyncAPIVariants(t *testing.T) {
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{
		{Function: managerFunction("GDExtensionSpxSpriteSetTriggerEnabled", "void",
			managerArgument("obj", "GdObj"), managerArgument("trigger", "GdBool"))},
		{Function: managerFunction("GDExtensionSpxSpriteIsCollisionEnabled", "GdBool",
			managerArgument("obj", "GdObj"))},
	}}
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast,
		common.GenerationMetadata{ManagerNames: []string{"sprite"}})}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.renderSyncAPI(codegenDir))
	require.NoError(t, generation.Output.Commit())

	for _, variant := range []struct {
		filename string
		buildTag string
		pure     bool
	}{
		{"sync.gen.go", "!pure_engine", false},
		{"sync_pure.gen.go", "pure_engine", true},
	} {
		t.Run(variant.filename, func(t *testing.T) {
			path := filepath.Join(codegenDir, common.EngineWrapRelDir, variant.filename)
			generated, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Contains(t, string(generated), "//go:build "+variant.buildTag+"\n// +build "+variant.buildTag+"\n")
			setter := generatedMethod(t, path, generated, "SetTriggerEnabled")
			getter := generatedMethod(t, path, generated, "IsCollisionEnabled")
			require.Contains(t, setter, "func (*spriteMgrImpl) SetTriggerEnabled(obj gdx.Object, trigger bool)")
			require.Contains(t, getter, "func (*spriteMgrImpl) IsCollisionEnabled(obj gdx.Object) bool")
			if variant.pure {
				require.Contains(t, setter, "{}")
				require.Contains(t, getter, "return false")
				require.NotContains(t, string(generated), "callInMainThread")
			} else {
				require.Contains(t, setter, "callInMainThread(func() {\n\t\tgdx.SpriteMgr.SetTriggerEnabled(obj, trigger)\n\t})")
				require.Contains(t, getter, "callInMainThread(func() {\n\t\t_ret1 = gdx.SpriteMgr.IsCollisionEnabled(obj)\n\t})\n\treturn _ret1")
			}
		})
	}
}

func TestNativeStringArgumentRetainsTemporaryOwnership(t *testing.T) {
	function := managerFunction("GDExtensionSpxExampleAcceptText", "void", managerArgument("text", "GdString"))
	ast := clang.CHeaderFileAST{Expr: []clang.Expr{{Function: function}}}
	generation := &Generator{GenerationContext: common.NewGenerationContext(ast, common.GenerationMetadata{})}
	body := generation.managerBody(function)
	require.Contains(t, body, "C.CString(text)")
	require.Contains(t, body, "defer C.free")
}

func TestFixedOutputManagerUsesArrayPointerWithoutLength(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxManager {
public:
 SPX_BIND void write_values(float out[3]);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleWriteValues)(float *out);")
	require.NoError(t, err)
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.renderManager(codegenDir))
	require.NoError(t, generation.Output.Commit())
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxManager {
public:
 SPX_BIND void collect(`+params+`);
};`), 0o600))
	headers, err := gdext.PrepareHeaders(dir)
	require.NoError(t, err)
	ast, err := clang.ParseCString("typedef void (*GDExtensionSpxExampleCollect)(" + params + ");")
	require.NoError(t, err)
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(ast, headers.Metadata)}
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, generation.renderManager(codegenDir))
	require.NoError(t, generation.Output.Commit())
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxManager {
public:
 SPX_BIND void collect(const GdObj objects[2], const float *values, int count, GdObj selected[3]);
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxManager {
public:
 SPX_BIND void collect(GdString label, int mode, const GdObj *objects, int count, float *out, int out_len, float ret_value[3]);
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spx_example_mgr.h"), []byte(`class SpxExampleMgr : public SpxManager {
public:
 SPX_BIND GdBool collect(const GdObj *ids, int count, SPX_OUT float *out, int out_len);
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

func TestNativeCallbacksMatchABI(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	source, err := os.ReadFile(filepath.Join(root, "godot_modules", "spx", "gdextension_spx_ext.h"))
	require.NoError(t, err)
	start := strings.Index(string(source), "// Callbacks run synchronously.")
	end := strings.Index(string(source), "} SpxCallbackInfo;") + len("} SpxCallbackInfo;")
	require.GreaterOrEqual(t, start, 0)
	require.Greater(t, end, start)
	callbackAST, err := clang.ParseCString(string(source[start:end]))
	require.NoError(t, err)
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(callbackAST, common.GenerationMetadata{})}
	require.NoError(t, generation.renderCallbacks(codegenDir))
	require.NoError(t, generation.Output.Commit())

	output, err := os.ReadFile(filepath.Join(codegenDir, common.NativeRelDir, "callbacks.gen.h"))
	require.NoError(t, err)
	header := string(output)
	declarations := regexp.MustCompile(`(?m)^extern void (\w+)\(`).FindAllStringSubmatch(header, -1)
	registrations := regexp.MustCompile(`info\.(\w+) = (\w+);`).FindAllStringSubmatch(header, -1)
	fields := callbackAST.CollectStructs()[0].Fields
	require.NotEmpty(t, fields)
	require.Len(t, declarations, len(fields))
	require.Len(t, registrations, len(fields))
	for i, field := range fields {
		symbol := field.Variable.Name
		require.Equal(t, symbol, declarations[i][1])
		require.Equal(t, []string{symbol, symbol}, registrations[i][1:])
	}
}

func TestNativeCallbacksUseABIFieldNamesAndRejectUnsupportedTypes(t *testing.T) {
	callbackAST, err := clang.ParseCString(`
typedef void (*GDExtensionSpxCallbackOnChanged)(GdObj obj, GdInt key, GdFloat delta, GdBool flag, GdString text);
typedef void (*GDExtensionSpxCallbackOnReady)();
typedef struct {
 GDExtensionSpxCallbackOnReady custom_ready;
 GDExtensionSpxCallbackOnChanged custom_changed;
} SpxCallbackInfo;`)
	require.NoError(t, err)
	codegenDir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(callbackAST, common.GenerationMetadata{})}
	require.NoError(t, generation.renderCallbacks(codegenDir))
	require.NoError(t, generation.Output.Commit())
	generated, err := os.ReadFile(filepath.Join(codegenDir, common.NativeRelDir, "callbacks.gen.go"))
	require.NoError(t, err)
	method := generatedMethod(t, "callbacks.gen.go", generated, "custom_changed")
	require.Contains(t, method, "func custom_changed(arg0 C.GDExtensionInt, arg1 C.GDExtensionInt, arg2 C.GDReal, arg3 C.GDExtensionBool, arg4 C.GdString)")
	require.Contains(t, method, "callbacks.OnChanged(int64(arg0), int64(arg1), float64(arg2), arg3 != 0, copyString(GdString(arg4)))")
	for _, source := range []string{
		"typedef struct { GdInt count; } Other;",
		"typedef void (*GDExtensionSpxCallbackValue)(Unknown value); typedef struct { GDExtensionSpxCallbackValue field; } SpxCallbackInfo;",
		"typedef void (*GDExtensionSpxCallbackValue)(GdInt *value); typedef struct { GDExtensionSpxCallbackValue field; } SpxCallbackInfo;",
		"typedef void (*GDExtensionSpxCallbackValue)(void (*value)(GdInt arg)); typedef struct { GDExtensionSpxCallbackValue field; } SpxCallbackInfo;",
	} {
		t.Run(source, func(t *testing.T) {
			callbackAST, err := clang.ParseCString(source)
			require.NoError(t, err)
			dir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
			generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(callbackAST, common.GenerationMetadata{})}
			require.Error(t, generation.renderCallbacks(dir))
			_, err = os.Stat(filepath.Join(dir, common.NativeRelDir))
			require.True(t, os.IsNotExist(err), "invalid input must not write output")
		})
	}
	for _, name := range []string{"callbacks.gen.go", "callbacks.gen.h"} {
		t.Run(name+" write error", func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
			path := filepath.Join(dir, common.NativeRelDir, name)
			require.NoError(t, os.MkdirAll(path, 0o755))
			generation := &Generator{Output: &common.OutputPlan{}, GenerationContext: common.NewGenerationContext(callbackAST, common.GenerationMetadata{})}
			require.NoError(t, generation.renderCallbacks(dir))
			require.ErrorContains(t, generation.Output.Commit(), path)
		})
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
