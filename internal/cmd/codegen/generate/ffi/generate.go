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

// Package gdextensionwrapper generates C code to wrap all of the gdextension
// methods to call functions on the gdextension_api_structs to work
// around the Cgo C function pointer limitation.
package ffi

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
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

func Generate(projectPath string, ast clang.CHeaderFileAST) error {
	generators := []struct {
		name string
		fn   func(string, clang.CHeaderFileAST) error
	}{
		{"GDExtension wrapper header", GenerateGDExtensionWrapperHeaderFile},
		{"GDExtension wrapper Go source", GenerateGDExtensionWrapperGoFile},
		{"GDExtension interface", GenerateGDExtensionInterfaceGoFile},
		{"manager wrapper", GenerateManagerWrapperGoFile},
		{"manager interface", GenerateManagerInterfaceGoFile},
		{"synchronized API", GenerateSyncApiGoFile},
		{"pure synchronized API", GenerateSyncPureGoFile},
	}
	for _, generator := range generators {
		if err := generator.fn(projectPath, ast); err != nil {
			return fmt.Errorf("generate %s: %w", generator.name, err)
		}
	}

	clsNames := []string{"Sprite"} // add other classes if needed, Audio, Camera, Input, etc
	for _, clsName := range clsNames {
		if err := GenerateManagerImplGoFile(projectPath, ast, clsName); err != nil {
			return fmt.Errorf("generate %s manager implementation: %w", clsName, err)
		}
		if err := GenerateManagerImplPureGoFile(projectPath, ast, clsName); err != nil {
			return fmt.Errorf("generate pure %s manager implementation: %w", clsName, err)
		}
	}
	return nil
}

func GenerateGDExtensionWrapperHeaderFile(projectPath string, ast clang.CHeaderFileAST) error {
	tmpl, err := template.New("ffi_wrapper.gen.h").
		Funcs(template.FuncMap{
			"snakeCase": strcase.ToSnake,
		}).
		Parse(ffiWrapperHeaderFileText)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, ast)
	if err != nil {
		return err
	}

	filename := filepath.Join(projectPath, NativeRelDir, "ffi_wrapper.gen.h")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b.Bytes())
	if err != nil {
		return err
	}
	return nil
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

func GenerateManagerWrapperGoFile(projectPath string, ast clang.CHeaderFileAST) error {
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
		"isManagerMethod":     IsManagerMethod,
		"getManagerFuncName":  getManagerFuncName,
		"getManagerFuncBody":  getManagerFuncBody,
		"getManagerInterface": getManagerInterface,
	}

	return GenerateFile(funcs, "manager_native.gen.go", managerNativeText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, GdengineImplRelDir, "manager_native.gen.go"))

}

func GenerateManagerInterfaceGoFile(projectPath string, ast clang.CHeaderFileAST) error {
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
		"isManagerMethod":     IsManagerMethod,
		"getManagerFuncName":  getManagerFuncName,
		"getManagerFuncBody":  getManagerFuncBody,
		"getManagerInterface": getManagerInterface,
	}

	return GenerateFile(funcs, "interface.gen.go", interfaceGoFileText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, EnginePkgRelDir, "interface.gen.go"))
}

func GenerateSyncApiGoFile(projectPath string, ast clang.CHeaderFileAST) error {
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
		"isManagerMethod":            IsManagerMethod,
		"genSyncApiWrapFunction":     genSyncApiWrapFunction,
		"genSyncManagerWrapFunction": genSyncManagerWrapFunction,
	}

	return GenerateFile(funcs, "sync.gen.go", syncApiText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, EnginewrapRelDir, "sync.gen.go"))
}

func GenerateSyncPureGoFile(projectPath string, ast clang.CHeaderFileAST) error {
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
		"isManagerMethod":            IsManagerMethod,
		"genSyncPureApiWrapFunction": genSyncPureApiWrapFunction,
		"genSyncManagerWrapFunction": genSyncManagerWrapFunction,
	}

	return GenerateFile(funcs, "sync_pure.gen.go", syncPureApiText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, EnginewrapRelDir, "sync_pure.gen.go"))
}

type ImplData struct {
	Ast     clang.CHeaderFileAST
	Methods []clang.TypedefFunction
	ClsName string
}

func GenerateManagerImplGoFile(projectPath string, ast clang.CHeaderFileAST, clsName string) error {
	funcs := template.FuncMap{
		"getManagerImpl": getManagerImpl,
	}

	genFile := strings.ToLower(clsName) + ".gen.go"
	methods := ast.CollectFunctionsOfClass(clsName)
	sort.Sort(ByName(methods))
	data := ImplData{Ast: ast, Methods: methods, ClsName: clsName}

	return GenerateFile(funcs, genFile, implGoFileText, data,
		filepath.Join(projectPath, EnginePkgRelDir, genFile))
}
func GenerateManagerImplPureGoFile(projectPath string, ast clang.CHeaderFileAST, clsName string) error {
	funcs := template.FuncMap{
		"getManagerImplPure": getManagerImplPure,
	}
	methods := ast.CollectFunctionsOfClass(clsName)
	sort.Sort(ByName(methods))
	data := ImplData{Ast: ast, Methods: methods, ClsName: clsName}

	genFile := strings.ToLower(clsName) + "_pure.gen.go"
	return GenerateFile(funcs, genFile, implPureGoFileText, data,
		filepath.Join(projectPath, EnginePkgRelDir, genFile))
}

func getManagerFuncName(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := GetManagerName(function.Name)
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	sb.WriteString("(")
	sb.WriteString("pself *" + mgrName)
	sb.WriteString("Mgr) ")
	sb.WriteString(funcName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if HasEffectiveReturn(function) {
		typeName := EffectiveGoReturnType(function)
		sb.WriteString(" " + typeName + " ")
	}
	return sb.String()
}

func getManagerFuncBody(function *clang.TypedefFunction) string {
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := EffectiveArguments(function)
	hasSyntheticReturn := function.ReturnType.Name == "void" && HasEffectiveReturn(function)
	dispatchToMainThread := function.Name != "GDExtensionSpxPlatformIsMainThread"
	if dispatchToMainThread {
		if HasEffectiveReturn(function) {
			sb.WriteString("\treturn enginewrap.CallInMainThreadValue(func() " + EffectiveGoReturnType(function) + " {\n")
		} else {
			sb.WriteString("\tenginewrap.CallInMainThread(func() {\n")
		}
		prefixTab += "\t"
	}
	// convert arguments
	for i, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		sb.WriteString(prefixTab)
		typeName := MustPrimitiveTypeName(arg, function.Name)
		argName := "arg" + strconv.Itoa(i)
		if IsNativeArrayDataArg(function, arg) {
			spec, _ := GetNativeArrayBridgeSpec(function.Name)
			goArgName := EffectiveGoArgumentName(function, arg)
			sb.WriteString("var " + argName + " " + spec.DataArgPtrType + "\n")
			sb.WriteString(prefixTab)
			sb.WriteString("if len(" + goArgName + ") > 0 {\n")
			sb.WriteString(prefixTab + "\t" + argName + " = &" + goArgName + "[0]\n")
			sb.WriteString(prefixTab + "}\n")
			lenArgName := "arg" + strconv.Itoa(i+1)
			sb.WriteString(prefixTab)
			sb.WriteString(lenArgName + " := " + NativeArrayLenExpr(function, goArgName))
			params = append(params, argName, lenArgName)
			sb.WriteString("\n")
			continue
		}
		switch typeName {
		case "GdString":
			sb.WriteString(argName + "Str := ")
			sb.WriteString("C.CString(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteString("\n" + prefixTab)
			sb.WriteString(argName + " := " + "(GdString)(" + argName + "Str) \n")
			sb.WriteString(prefixTab + "defer " + "C.free(unsafe.Pointer(" + argName + "Str))")
		case "GdArray":
			sb.WriteString(argName + "Info := ")
			sb.WriteString("ToGdArrayInfo(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteString("\n" + prefixTab)
			sb.WriteString("if " + argName + "Info != nil {\n")
			sb.WriteString(prefixTab + "\tdefer " + argName + "Info.Free()\n")
			sb.WriteString(prefixTab + "}\n")
			sb.WriteString(prefixTab)
			sb.WriteString(argName + " := GdArray(nil)\n")
			sb.WriteString(prefixTab)
			sb.WriteString("if " + argName + "Info != nil {\n")
			sb.WriteString(prefixTab + "\t" + argName + " = " + argName + "Info.Raw()\n")
			sb.WriteString(prefixTab + "}")

		default:
			sb.WriteString(argName + " := ")
			sb.WriteString("To" + typeName)
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
		sb.WriteString("var retValue " + rawType + "\n")
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
		sb.WriteString("\n" + prefixTab)
		sb.WriteString("return ")
		typeName := EffectiveGoReturnType(function)
		sb.WriteString("To" + strcase.ToCamel(typeName) + "(retValue)")
	}
	if dispatchToMainThread {
		sb.WriteString("\n\t})")
	}
	return sb.String()
}

func getManagerInterface(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := GetManagerName(function.Name)
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	sb.WriteString(funcName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if HasEffectiveReturn(function) {
		typeName := EffectiveGoReturnType(function)
		sb.WriteString(" " + typeName + " ")
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

func MustGdxReturnType(function *clang.TypedefFunction) string {
	typeName := EffectiveGoReturnType(function)
	if typeName == "Object" {
		return "gdx.Object"
	}
	if typeName == "Array" {
		return "gdx.Array"
	}
	return typeName
}

func genSyncPureApiWrapFunction(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := strcase.ToCamel(GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	mgrTypeName := strcase.ToLowerCamel(GetManagerName(function.Name)) + "Mgr"
	args := EffectiveArguments(function)
	retType := EffectiveGoReturnType(function)

	sb.WriteString(fmt.Sprintf("func (*%s) ", mgrTypeName+"Impl"))
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteString(" " + MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if retType != "" {
		sb.WriteString("\n" + prefixStr + "return " + goZeroValue(MustGdxReturnType(function)) + "\n")
	}
	sb.WriteString("}")
	return sb.String()
}

func genSyncApiWrapFunction(function *clang.TypedefFunction) string {
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
	mgrName := strcase.ToCamel(GetManagerName(function.Name))
	pureFuncName := function.Name[len(prefix)+len(mgrName):]
	//funcName := function.Name[len(prefix):]
	gdxMgrName := "gdx." + mgrName + "Mgr"
	mgrTypeName := strcase.ToLowerCamel(GetManagerName(function.Name)) + "Mgr"
	args := EffectiveArguments(function)
	retType := EffectiveGoReturnType(function)

	sb.WriteString(fmt.Sprintf("func (*%s) ", mgrTypeName+"Impl"))
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	wroteArg := false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGdxArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(")")

	if retType != "" {
		sb.WriteString(" " + MustGdxReturnType(function))
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if retType != "" {
		sb.WriteString("\n" + prefixStr + "var _ret1 " + MustGdxReturnType(function) + "")
	}

	sb.WriteString(`	
	callInMainThread(func() {
`)
	if retType != "" {
		sb.WriteString(prefixStr + "\t_ret1 = ")
	} else {
		sb.WriteString(prefixStr + "\t")
	}
	sb.WriteString(gdxMgrName + "." + pureFuncName + "(")
	wroteArg = false
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		wroteArg = true
	}
	sb.WriteString(")")

	sb.WriteString(`
	})
`)

	if retType != "" {
		sb.WriteString(prefixStr + "return _ret1 \n")
	}
	sb.WriteString("}")
	return sb.String()
}

func genSyncManagerWrapFunction(function *clang.TypedefFunction) string {
	return ""
}

type ByName []clang.TypedefFunction

func (arr ByName) Len() int { return len(arr) }

func (arr ByName) Swap(i, j int) { arr[i], arr[j] = arr[j], arr[i] }

func (arr ByName) Less(i, j int) bool {
	return arr[i].Name < arr[j].Name
}

func getManagerImplPure(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	retType := EffectiveGoReturnType(function)
	sb.WriteString("func (pself *" + clsName + ") " + funcName + "(")
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType + " ")
	}
	sb.WriteString("{\n")
	if retType != "" {
		sb.WriteString("\treturn " + goZeroValue(retType) + "\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}
func getManagerImpl(function *clang.TypedefFunction, clsName string) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	lowcaseMgr := GetManagerName(function.Name)
	mgrName := string(unicode.ToUpper(rune(lowcaseMgr[0]))) + lowcaseMgr[1:]
	funcName := function.Name[len(prefix)+len(mgrName):]
	args := EffectiveArguments(function)
	retType := EffectiveGoReturnType(function)

	// Check if the first argument is "obj" to determine if this is an instance method
	hasObjArg := len(args) > 0 && args[0].Name == "obj"

	sb.WriteString("func (pself *" + clsName + ") " + funcName + "(")
	wroteArg := false
	for i, arg := range args {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(" ")
		typeName := EffectiveGoArgumentType(function, arg)
		sb.WriteString(typeName)
		wroteArg = true
	}
	sb.WriteString(") ")
	if retType != "" {
		sb.WriteString(retType + " ")
	}
	sb.WriteString("{\n")
	sb.WriteString("\t")
	if retType != "" {
		sb.WriteString("return ")
	}
	sb.WriteString(mgrName + "Mgr." + funcName + "(")
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
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		if wroteCallArg {
			sb.WriteString(", ")
		}
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		wroteCallArg = true
	}
	sb.WriteString(")\n")
	sb.WriteString("}\n")
	return sb.String()
}
