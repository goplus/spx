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

// Package ffi generates native Go bindings and C function-pointer wrappers.
package ffi

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	//go:embed ffi_wrapper.h.tmpl
	ffiWrapperHeaderFileText string

	//go:embed ffi_wrapper.go.tmpl
	ffiWrapperGoFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_native.go.tmpl
	managerNativeText string
	//go:embed interface.go.tmpl
	interfaceGoFileText string
	//go:embed sprite.go.tmpl
	implGoFileText string
	//go:embed sprite_pure.go.tmpl
	implPureGoFileText string

	//go:embed sync.gen.go.tmpl
	syncApiText string

	//go:embed sync_pure.gen.go.tmpl
	syncPureApiText string
)

type ImplData struct {
	Ast     clang.CHeaderFileAST
	Methods []clang.TypedefFunction
	ClsName string
}

type ByName []clang.TypedefFunction

// Generator renders bindings using metadata owned by one generation task.
type Generator struct {
	*GenerationContext
}

func (arr ByName) Len() int { return len(arr) }

func (arr ByName) Swap(i, j int) { arr[i], arr[j] = arr[j], arr[i] }

func (arr ByName) Less(i, j int) bool {
	return arr[i].Name < arr[j].Name
}

func (g *Generator) Generate(projectPath string) error {
	ast := g.AST()
	generators := []struct {
		name string
		fn   func() error
	}{
		{"GDExtension wrapper header", func() error { return GenerateGDExtensionWrapperHeaderFile(projectPath, ast) }},
		{"GDExtension wrapper Go source", func() error { return GenerateGDExtensionWrapperGoFile(projectPath, ast) }},
		{"GDExtension interface", func() error { return GenerateGDExtensionInterfaceGoFile(projectPath, ast) }},
		{"manager wrapper", func() error { return g.GenerateManagerWrapperGoFile(projectPath) }},
		{"manager interface", func() error { return g.GenerateManagerInterfaceGoFile(projectPath) }},
		{"synchronized API", func() error { return g.GenerateSyncApiGoFile(projectPath) }},
		{"pure synchronized API", func() error { return g.GenerateSyncPureGoFile(projectPath) }},
	}
	for _, generator := range generators {
		if err := generator.fn(); err != nil {
			return fmt.Errorf("generate %s: %w", generator.name, err)
		}
	}

	clsNames := []string{"Sprite"}
	for _, clsName := range clsNames {
		if err := g.GenerateManagerImplGoFile(projectPath, clsName); err != nil {
			return fmt.Errorf("generate %s manager implementation: %w", clsName, err)
		}
		if err := g.GenerateManagerImplPureGoFile(projectPath, clsName); err != nil {
			return fmt.Errorf("generate pure %s manager implementation: %w", clsName, err)
		}
	}
	return nil
}

func GenerateGDExtensionWrapperHeaderFile(projectPath string, ast clang.CHeaderFileAST) error {
	output, err := RenderTemplate(template.FuncMap{"snakeCase": strcase.ToSnake}, "ffi_wrapper.gen.h", ffiWrapperHeaderFileText, ast)
	if err != nil {
		return err
	}
	return WriteGeneratedFile(filepath.Join(projectPath, NativeRelDir, "ffi_wrapper.gen.h"), output, 0o666)
}

func GenerateGDExtensionWrapperGoFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":       GdiVariableName,
		"snakeCase":             strcase.ToSnake,
		"camelCase":             strcase.ToCamel,
		"goReturnType":          GoReturnType,
		"goArgumentType":        GoArgumentType,
		"goEnumValue":           GoEnumValue,
		"add":                   Add,
		"cgoCastArgument":       CgoCastArgument,
		"cgoCastReturnType":     CgoCastReturnType,
		"cgoCleanUpArgument":    CgoCleanUpArgument,
		"trimPrefix":            TrimPrefix,
		"mustPrimitiveTypeName": MustPrimitiveTypeName,
	}

	return GenerateFile(funcs, "ffi_wrapper.gen.go", ffiWrapperGoFileText, ast,
		filepath.Join(projectPath, NativeRelDir, "ffi_wrapper.gen.go"))

}

func GenerateGDExtensionInterfaceGoFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":     GdiVariableName,
		"snakeCase":           strcase.ToSnake,
		"camelCase":           strcase.ToCamel,
		"goReturnType":        GoReturnType,
		"goArgumentType":      GoArgumentType,
		"goEnumValue":         GoEnumValue,
		"add":                 Add,
		"cgoCastArgument":     CgoCastArgument,
		"cgoCastReturnType":   CgoCastReturnType,
		"cgoCleanUpArgument":  CgoCleanUpArgument,
		"trimPrefix":          TrimPrefix,
		"loadProcAddressName": LoadProcAddressName,
	}

	return GenerateFile(funcs, "ffi.gen.go", ffiFileText, ast,
		filepath.Join(projectPath, NativeRelDir, "ffi.gen.go"))
}

func (g *Generator) GenerateManagerWrapperGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"gdiVariableName":     GdiVariableName,
		"snakeCase":           strcase.ToSnake,
		"camelCase":           strcase.ToCamel,
		"goReturnType":        GoReturnType,
		"goArgumentType":      GoArgumentType,
		"goEnumValue":         GoEnumValue,
		"add":                 Add,
		"cgoCastArgument":     CgoCastArgument,
		"cgoCastReturnType":   CgoCastReturnType,
		"cgoCleanUpArgument":  CgoCleanUpArgument,
		"trimPrefix":          TrimPrefix,
		"isManagerMethod":     g.IsManagerMethod,
		"getManagerFuncName":  g.ManagerMethodSignature,
		"getManagerFuncBody":  g.getManagerFuncBody,
		"getManagerInterface": g.ManagerInterfaceSignature,
	}

	return GenerateFile(funcs, "manager_native.gen.go", managerNativeText, g.ManagerData(),
		filepath.Join(projectPath, GdengineImplRelDir, "manager_native.gen.go"))

}

func (g *Generator) GenerateManagerInterfaceGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"gdiVariableName":     GdiVariableName,
		"snakeCase":           strcase.ToSnake,
		"camelCase":           strcase.ToCamel,
		"goReturnType":        GoReturnType,
		"goArgumentType":      GoArgumentType,
		"goEnumValue":         GoEnumValue,
		"add":                 Add,
		"cgoCastArgument":     CgoCastArgument,
		"cgoCastReturnType":   CgoCastReturnType,
		"cgoCleanUpArgument":  CgoCleanUpArgument,
		"trimPrefix":          TrimPrefix,
		"isManagerMethod":     g.IsManagerMethod,
		"getManagerFuncName":  g.ManagerMethodSignature,
		"getManagerFuncBody":  g.getManagerFuncBody,
		"getManagerInterface": g.ManagerInterfaceSignature,
	}

	return GenerateFile(funcs, "interface.gen.go", interfaceGoFileText, g.ManagerData(),
		filepath.Join(projectPath, EnginePkgRelDir, "interface.gen.go"))
}

func (g *Generator) GenerateSyncApiGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"gdiVariableName":            GdiVariableName,
		"snakeCase":                  strcase.ToSnake,
		"lowerCamelCase":             strcase.ToLowerCamel,
		"camelCase":                  strcase.ToCamel,
		"goReturnType":               GoReturnType,
		"goArgumentType":             GoArgumentType,
		"goEnumValue":                GoEnumValue,
		"add":                        Add,
		"cgoCastArgument":            CgoCastArgument,
		"cgoCastReturnType":          CgoCastReturnType,
		"cgoCleanUpArgument":         CgoCleanUpArgument,
		"trimPrefix":                 TrimPrefix,
		"isManagerMethod":            g.IsManagerMethod,
		"genSyncApiWrapFunction":     g.genSyncApiWrapFunction,
		"genSyncManagerWrapFunction": genSyncManagerWrapFunction,
	}

	return GenerateFile(funcs, "sync.gen.go", syncApiText, g.ManagerData(),
		filepath.Join(projectPath, EnginewrapRelDir, "sync.gen.go"))
}

func (g *Generator) GenerateSyncPureGoFile(projectPath string) error {
	funcs := template.FuncMap{
		"gdiVariableName":            GdiVariableName,
		"snakeCase":                  strcase.ToSnake,
		"lowerCamelCase":             strcase.ToLowerCamel,
		"camelCase":                  strcase.ToCamel,
		"goReturnType":               GoReturnType,
		"goArgumentType":             GoArgumentType,
		"goEnumValue":                GoEnumValue,
		"add":                        Add,
		"cgoCastArgument":            CgoCastArgument,
		"cgoCastReturnType":          CgoCastReturnType,
		"cgoCleanUpArgument":         CgoCleanUpArgument,
		"trimPrefix":                 TrimPrefix,
		"isManagerMethod":            g.IsManagerMethod,
		"genSyncPureApiWrapFunction": g.genSyncPureApiWrapFunction,
		"genSyncManagerWrapFunction": genSyncManagerWrapFunction,
	}

	return GenerateFile(funcs, "sync_pure.gen.go", syncPureApiText, g.ManagerData(),
		filepath.Join(projectPath, EnginewrapRelDir, "sync_pure.gen.go"))
}

func (g *Generator) GenerateManagerImplGoFile(projectPath string, clsName string) error {
	ast := g.AST()
	funcs := template.FuncMap{
		"getManagerImpl": g.getManagerImpl,
	}

	genFile := strings.ToLower(clsName) + ".gen.go"
	methods := ast.CollectFunctionsOfClass(clsName)
	sort.Sort(ByName(methods))
	data := ImplData{Ast: ast, Methods: methods, ClsName: clsName}

	return GenerateFile(funcs, genFile, implGoFileText, data,
		filepath.Join(projectPath, EnginePkgRelDir, genFile))
}

func (g *Generator) GenerateManagerImplPureGoFile(projectPath string, clsName string) error {
	ast := g.AST()
	funcs := template.FuncMap{
		"getManagerImplPure": g.getManagerImplPure,
	}
	methods := ast.CollectFunctionsOfClass(clsName)
	sort.Sort(ByName(methods))
	data := ImplData{Ast: ast, Methods: methods, ClsName: clsName}

	genFile := strings.ToLower(clsName) + "_pure.gen.go"
	return GenerateFile(funcs, genFile, implPureGoFileText, data,
		filepath.Join(projectPath, EnginePkgRelDir, genFile))
}

func (g *Generator) MustGdxReturnType(function *clang.TypedefFunction) string {
	typeName := g.EffectiveGoReturnType(function)
	if typeName == "Object" {
		return "gdx.Object"
	}
	if typeName == "Array" {
		return "gdx.Array"
	}
	return typeName
}

func (g *Generator) getManagerFuncBody(function *clang.TypedefFunction) string {
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := EffectiveArguments(function)
	hasSyntheticReturn := function.ReturnType.Name == "void" && HasEffectiveReturn(function)
	dispatchToMainThread := function.Name != "GDExtensionSpxPlatformIsMainThread"
	if dispatchToMainThread {
		if HasEffectiveReturn(function) {
			fmt.Fprintf(&sb, "\treturn enginewrap.CallInMainThreadValue(func() %s {\n", g.EffectiveGoReturnType(function))
		} else {
			sb.WriteString("\tenginewrap.CallInMainThread(func() {\n")
		}
		prefixTab += "\t"
	}
	// convert arguments
	for i, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		sb.WriteString(prefixTab)
		typeName := MustPrimitiveTypeName(arg, function.Name)
		argName := "arg" + strconv.Itoa(i)
		if g.IsNativeArrayDataArg(function, arg) {
			spec, _ := g.GetNativeArrayBridgeSpec(function.Name)
			goArgName := g.EffectiveGoArgumentName(function, arg)
			fmt.Fprintf(&sb, "var %s %s\n", argName, spec.DataArgPtrType)
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if len(%s) > 0 {\n", goArgName)
			fmt.Fprintf(&sb, "%s\t%s = &%s[0]\n", prefixTab, argName, goArgName)
			sb.WriteString(prefixTab)
			sb.WriteString("}\n")
			lenArgName := "arg" + strconv.Itoa(i+1)
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "%s := %s", lenArgName, g.NativeArrayLenExpr(function, goArgName))
			params = append(params, argName, lenArgName)
			sb.WriteString("\n")
			continue
		}
		switch typeName {
		case "GdString":
			sb.WriteString(argName)
			sb.WriteString("Str := ")
			sb.WriteString("C.CString(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteByte('\n')
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "%s := (GdString)(%sStr) \n", argName, argName)
			fmt.Fprintf(&sb, "%sdefer C.free(unsafe.Pointer(%sStr))", prefixTab, argName)
		case "GdArray":
			sb.WriteString(argName)
			sb.WriteString("Info := ")
			sb.WriteString("ToGdArrayInfo(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteByte('\n')
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if %sInfo != nil {\n", argName)
			fmt.Fprintf(&sb, "%s\tdefer %sInfo.Free()\n", prefixTab, argName)
			sb.WriteString(prefixTab)
			sb.WriteString("}\n")
			sb.WriteString(prefixTab)
			sb.WriteString(argName)
			sb.WriteString(" := GdArray(nil)\n")
			sb.WriteString(prefixTab)
			fmt.Fprintf(&sb, "if %sInfo != nil {\n", argName)
			fmt.Fprintf(&sb, "%s\t%s = %sInfo.Raw()\n", prefixTab, argName, argName)
			sb.WriteString(prefixTab)
			sb.WriteByte('}')

		default:
			sb.WriteString(argName)
			sb.WriteString(" := ")
			sb.WriteString("To")
			sb.WriteString(typeName)
			sb.WriteString("(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
		}
		sb.WriteString("\n")
		params = append(params, argName)
	}

	// call the function
	funcName := "Call" + TrimPrefix(function.Name, "GDExtensionSpx")
	if hasSyntheticReturn {
		rawType := EffectiveRawReturnType(function)
		sb.WriteString(prefixTab)
		fmt.Fprintf(&sb, "var retValue %s\n", rawType)
	}

	sb.WriteString(prefixTab)
	if function.ReturnType.Name != "void" {
		sb.WriteString("retValue := ")
	}
	sb.WriteString(funcName)
	sb.WriteString("(")
	for i, param := range params {
		sb.WriteString(param)
		if i != len(params)-1 {
			sb.WriteString(", ")
		}
	}
	if hasSyntheticReturn {
		if len(params) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("&retValue")
	}
	sb.WriteString(")")

	if HasEffectiveReturn(function) {
		sb.WriteByte('\n')
		sb.WriteString(prefixTab)
		sb.WriteString("return ")
		typeName := g.EffectiveGoReturnType(function)
		fmt.Fprintf(&sb, "To%s(retValue)", strcase.ToCamel(typeName))
	}
	if dispatchToMainThread {
		sb.WriteString("\n\t})")
	}
	return sb.String()
}

func goZeroValue(typeName string) string {
	switch typeName {
	case "bool":
		return "false"
	case "int64", "float64", "Object", "gdx.Object":
		return "0"
	case "string":
		return `""`
	case "Array", "gdx.Array":
		return "nil"
	default:
		return typeName + "{}"
	}
}

func (g *Generator) genSyncPureApiWrapFunction(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	mgrTypeName := strcase.ToLowerCamel(g.GetManagerName(function.Name)) + "Mgr"
	args := EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	fmt.Fprintf(&sb, "func (*%sImpl) ", mgrTypeName)
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteByte(' ')
		sb.WriteString(g.MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if retType != "" {
		fmt.Fprintf(&sb, "\n%sreturn %s\n", prefixStr, goZeroValue(g.MustGdxReturnType(function)))
	}
	sb.WriteString("}")
	return sb.String()
}

func (g *Generator) genSyncApiWrapFunction(function *clang.TypedefFunction) string {
	/*
		func syncUiGetFlip(obj Object, horizontal bool) bool {
			var _ret1 bool
			WaitMainThread(func() {
				_ret1 = UiMgr.GetFlip(obj, horizontal)
			})
			return _ret1
		}
	*/

	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := strcase.ToCamel(g.GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	//funcName := function.Name[len(prefix):]
	gdxMgrName := "gdx." + mgrName + "Mgr"
	mgrTypeName := strcase.ToLowerCamel(g.GetManagerName(function.Name)) + "Mgr"
	args := EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	fmt.Fprintf(&sb, "func (*%sImpl) ", mgrTypeName)
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteByte(' ')
		sb.WriteString(g.MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if retType != "" {
		fmt.Fprintf(&sb, "\n%svar _ret1 %s", prefixStr, g.MustGdxReturnType(function))
	}

	sb.WriteString(`	
	callInMainThread(func() {
`)
	if retType != "" {
		sb.WriteString(prefixStr)
		sb.WriteString("\t_ret1 = ")
	} else {
		sb.WriteString(prefixStr)
		sb.WriteByte('\t')
	}
	fmt.Fprintf(&sb, "%s.%s(", gdxMgrName, pureFuncName)
	wroteArg = false
	for _, arg := range args {
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		wroteArg = true
	}
	sb.WriteString(")")

	sb.WriteString(`
	})
`)

	if retType != "" {
		sb.WriteString(prefixStr)
		sb.WriteString("return _ret1 \n")
	}
	sb.WriteString("}")
	return sb.String()
}

func genSyncManagerWrapFunction(function *clang.TypedefFunction) string {
	return ""
}

func (g *Generator) getManagerImplPure(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)
	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType)
		sb.WriteByte(' ')
	}
	sb.WriteString("{\n")
	if retType != "" {
		fmt.Fprintf(&sb, "\treturn %s\n", goZeroValue(retType))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (g *Generator) getManagerImpl(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := g.GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	retType := g.EffectiveGoReturnType(function)

	// Check if the first argument is "obj" to determine if this is an instance method
	hasObjArg := len(args) > 0 && args[0].Name == "obj"

	fmt.Fprintf(&sb, "func (pself *%s) %s(", clsName, funcName)
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := g.EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType)
		sb.WriteByte(' ')
	}
	sb.WriteString("{\n")
	sb.WriteString("\t")
	if retType != "" {
		sb.WriteString("return ")
	}
	fmt.Fprintf(&sb, "%sMgr.%s(", mgrName, funcName)
	// Only add pself.Id if the first argument is "obj" (instance method)
	wroteCallArg := false
	if hasObjArg {
		sb.WriteString("pself.Id")
		wroteCallArg = true
	}
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if g.ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteCallArg {
			sb.WriteString(", ")
		}
		sb.WriteString(g.EffectiveGoArgumentName(function, arg))
		wroteCallArg = true
	}
	sb.WriteString(")\n")
	sb.WriteString("}\n")
	return sb.String()
}
