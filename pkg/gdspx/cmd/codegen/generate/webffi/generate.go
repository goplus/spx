// Package gdextensionwrapper generates C code to wrap all of the gdextension
// methods to call functions on the gdextension_api_structs to work
// around the Cgo C function pointer limitation.
package webffi

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/goplus/spx/pkg/gdspx/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/pkg/gdspx/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	WebRelDir = "../../internal/webffi"
)
var (

	//go:embed callback.go.tmpl
	callbacksFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_wrapper.go.tmpl
	wrapManagerGoFileText string

	//go:embed gdspx.js.tmpl
	jsEngineJsFileText string
)

func Generate(projectPath string, ast clang.CHeaderFileAST) {
	err := GenerateCallbackGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateGDExtensionInterfaceGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateManagerWrapperGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateJsEngineJsFile(projectPath, ast)
	if err != nil {
		panic(err)
	}

}

func GenerateCallbackGoFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":    GdiVariableName,
		"snakeCase":          strcase.ToSnake,
		"camelCase":          strcase.ToCamel,
		"goReturnType":       GoReturnType,
		"goArgumentType":     GoArgumentType,
		"goEnumValue":        GoEnumValue,
		"add":                Add,
		"cgoCastArgument":    CgoCastArgument,
		"cgoCastReturnType":  CgoCastReturnType,
		"cgoCleanUpArgument": CgoCleanUpArgument,
		"trimPrefix":         TrimPrefix,
	}

	return GenerateFile(funcs, "callbacks.gen.go", callbacksFileText, ast,
		filepath.Join(projectPath, WebRelDir, "callbacks.gen.go"))
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
		filepath.Join(projectPath, WebRelDir, "ffi.gen.go"))
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

	return GenerateFile(funcs, "manager_wrapper.gen.go", wrapManagerGoFileText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, WebRelDir, "../wrap/manager_wrapper_web.gen.go"))
}

func GenerateJsEngineJsFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":     GdiVariableName,
		"snakeCase":           strcase.ToSnake,
		"camelCase":           strcase.ToCamel,
		"goReturnType":        GoReturnType,
		"goArgumentType":      GoArgumentType,
		"goEnumValue":         GoEnumValue,
		"add":                 Add,
		"sub":                 Sub,
		"cgoCastArgument":     CgoCastArgument,
		"cgoCastReturnType":   CgoCastReturnType,
		"cgoCleanUpArgument":  CgoCleanUpArgument,
		"getJsFuncBody":       getJsFuncBody,
		"trimPrefix":          TrimPrefix,
		"loadProcAddressName": LoadProcAddressName,
	}

	tmpl, err := template.New("gdspx.js").
		Funcs(funcs).
		Parse(jsEngineJsFileText)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, ast)
	if err != nil {
		return err
	}

	headerFileName := filepath.Join(filepath.Join(projectPath, "../../godot/platform/web/js/engine"), "gdspx.js")
	os.MkdirAll(filepath.Dir(headerFileName), os.ModePerm)
	f, err := os.Create(headerFileName)
	f.Write(b.Bytes())
	f.Close()
	return err
}

func getManagerFuncName(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := GetManagerName(function.Name)
	funcName := function.Name[len(prefix)+len(mgrName):]
	sb.WriteString("(")
	sb.WriteString("pself *" + mgrName)
	sb.WriteString("Mgr) ")
	sb.WriteString(funcName)
	sb.WriteString("(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	if function.ReturnType.Name != "void" {
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(" " + typeName + " ")
	}
	return sb.String()
}

func getManagerFuncBody(function *clang.TypedefFunction) string {
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	// convert arguments
	for i, arg := range function.Arguments {
		sb.WriteString(prefixTab)
		typeName := arg.Type.Primative.Name
		argName := "arg" + strconv.Itoa(i)
		sb.WriteString(argName + " := ")
		sb.WriteString("JsFrom" + typeName)
		sb.WriteString("(")
		sb.WriteString(arg.Name)
		sb.WriteString(")")

		sb.WriteString("\n")
		params = append(params, argName)
	}

	// call the function
	sb.WriteString(prefixTab)
	if function.ReturnType.Name != "void" {
		sb.WriteString("_retValue := ")
	}

	funcName := "API.Spx" + (TrimPrefix(function.Name, "GDExtensionSpx"))
	sb.WriteString(funcName)
	sb.WriteString(".Invoke(")
	for i, param := range params {
		sb.WriteString(param)
		if i != len(params)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	if function.ReturnType.Name != "void" {
		sb.WriteString("\n" + prefixTab)
		sb.WriteString("return ")
		typeName := function.ReturnType.Name
		name := strcase.ToCamel(typeName)
		if name == "GdObj" {
			name = "GdObject"
		}
		sb.WriteString("JsTo" + name + "(_retValue)")
	}
	return sb.String()
}
func getManagerInterface(function *clang.TypedefFunction) string {
	prefix := "GDExtensionSpx"
	sb := strings.Builder{}
	mgrName := GetManagerName(function.Name)
	funcName := function.Name[len(prefix)+len(mgrName):]
	sb.WriteString(funcName)
	sb.WriteString("(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	if function.ReturnType.Name != "void" {
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(" " + typeName + " ")
	}
	return sb.String()
}
func getJsFuncBody(function *clang.TypedefFunction) string {
	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}

	// call the function
	if function.ReturnType.Name != "void" {
		sb.WriteString("_retValue = Alloc" + function.ReturnType.Name + "();")
	}
	sb.WriteString("\n")

	// convert arguments
	for i, arg := range function.Arguments {
		sb.WriteString(prefixTab)
		typeName := arg.Type.Primative.Name
		argName := "_arg" + strconv.Itoa(i)
		sb.WriteString(argName + " = ")
		sb.WriteString("To" + typeName)
		sb.WriteString("(")
		sb.WriteString(arg.Name)
		sb.WriteString(");")

		sb.WriteString("\n")
		params = append(params, argName)
	}
	sb.WriteString(prefixTab)
	sb.WriteString("_gdFuncPtr")
	sb.WriteString("(")
	for i, param := range params {
		sb.WriteString(param)
		if i != len(params)-1 {
			sb.WriteString(", ")
		}
	}
	if function.ReturnType.Name != "void" {
		if len(params) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("_retValue")
	}
	sb.WriteString(");")

	sb.WriteString("\n")
	// convert arguments
	for i, arg := range function.Arguments {
		sb.WriteString(prefixTab)
		typeName := arg.Type.Primative.Name
		argName := "_arg" + strconv.Itoa(i)
		sb.WriteString("Free" + typeName + "(" + argName + "); \n")
	}

	if function.ReturnType.Name != "void" {
		sb.WriteString(prefixTab + "_finalRetValue = ")
		typeName := function.ReturnType.Name
		funcName := strcase.ToCamel(typeName)
		funcName = "ToJs" + strings.ReplaceAll(funcName, "Gd", "")
		sb.WriteString(funcName + "(_retValue);\n")
		sb.WriteString(prefixTab + "Free" + typeName + "(_retValue); \n")
		sb.WriteString(prefixTab + "return _finalRetValue")
	}
	return sb.String()
}
