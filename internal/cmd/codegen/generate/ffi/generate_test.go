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
	"github.com/stretchr/testify/require"
)

func TestGenerateManagerWrapperRunsNativeCallsOnMainThread(t *testing.T) {
	common.ClearKnownManagerNames()
	common.RegisterManagerName("sprite")
	common.RegisterManagerName("platform")
	t.Cleanup(common.ClearKnownManagerNames)

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

	projectPath := filepath.Join(t.TempDir(), "internal", "cmd", "codegen")
	require.NoError(t, GenerateManagerWrapperGoFile(projectPath, ast))
	generatedPath := filepath.Join(projectPath, common.GdengineImplRelDir, "manager_native.gen.go")
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

func managerFunction(name, returnType string, arguments ...clang.Argument) *clang.TypedefFunction {
	return &clang.TypedefFunction{
		Name:       name,
		ReturnType: clang.PrimativeType{Name: returnType},
		Arguments:  arguments,
	}
}

func managerArgument(name, typeName string) clang.Argument {
	return clang.Argument{
		Name: name,
		Type: clang.Type{Primative: &clang.PrimativeType{Name: typeName}},
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
