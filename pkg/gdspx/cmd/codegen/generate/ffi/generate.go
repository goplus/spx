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

	"github.com/goplus/spx/pkg/gdspx/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/pkg/gdspx/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	//go:embed ffi_wrapper.h.tmpl
	ffiWrapperHeaderFileText string

	//go:embed ffi_wrapper.go.tmpl
	ffiWrapperGoFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_wrapper.go.tmpl
	wrapManagerGoFileText string
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

func Generate(projectPath string, ast clang.CHeaderFileAST) {
	err := GenerateGDExtensionWrapperHeaderFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateGDExtensionWrapperGoFile(projectPath, ast)
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
	err = GenerateManagerInterfaceGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateSyncApiGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}

	err = GenerateSyncPureGoFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
	/**/
	clsNames := []string{"Sprite"} // add other classes if needed, Audio, Camera, Input, etc
	for _, clsName := range clsNames {
		err = GenerateManagerImplGoFile(projectPath, ast, clsName)
		if err != nil {
			panic(err)
		}
		err = GenerateManagerImplPureGoFile(projectPath, ast, clsName)
		if err != nil {
			panic(err)
		}
	}
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

	filename := filepath.Join(projectPath, RelDir, "ffi_wrapper.gen.h")
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

	return GenerateFile(funcs, "ffi_wrapper.gen.go", ffiWrapperGoFileText, ast,
		filepath.Join(projectPath, RelDir, "ffi_wrapper.gen.go"))

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
		filepath.Join(projectPath, RelDir, "ffi.gen.go"))
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
		filepath.Join(projectPath, RelDir, "../wrap/manager_wrapper.gen.go"))

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
		filepath.Join(projectPath, RelDir, "../../pkg/engine/interface.gen.go"))
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
		filepath.Join(projectPath, RelDir, "../../../../internal/enginewrap/sync.gen.go"))
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
		filepath.Join(projectPath, RelDir, "../../../../internal/enginewrap/sync_pure.gen.go"))
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
		filepath.Join(projectPath, RelDir, "../../pkg/engine/"+genFile))
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
		filepath.Join(projectPath, RelDir, "../../pkg/engine/"+genFile))
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
		switch typeName {
		case "GdString":
			sb.WriteString(argName + "Str := ")
			sb.WriteString("C.CString(")
			sb.WriteString(arg.Name)
			sb.WriteString(")")
			sb.WriteString("\n" + prefixTab)
			sb.WriteString(argName + " := " + "(GdString)(" + argName + "Str) \n")
			sb.WriteString("\tdefer " + "C.free(unsafe.Pointer(" + argName + "Str))")

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
	sb.WriteString(prefixTab)
	if function.ReturnType.Name != "void" {
		sb.WriteString("retValue := ")
	}

	funcName := "Call" + TrimPrefix(function.Name, "GDExtensionSpx")
	sb.WriteString(funcName)
	sb.WriteString("(")
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
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString("To" + strcase.ToCamel(typeName) + "(retValue)")
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
func GetGdxFuncParamTypeString(typeName string) string {
	name := GetFuncParamTypeString(typeName)
	if name == "Object" {
		return "gdx." + name
	}
	return name
}
func genSyncPureApiWrapFunction(function *clang.TypedefFunction) string {
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
	mgrTypeName := strcase.ToLowerCamel(GetManagerName(function.Name)) + "Mgr"

	sb.WriteString(fmt.Sprintf("func (pself *%s) ", mgrTypeName+"Impl"))
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetGdxFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	if function.ReturnType.Name != "void" {
		typeName := GetGdxFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(" " + typeName)
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if function.ReturnType.Name != "void" {
		typeName := GetGdxFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString("\n" + prefixStr + "var _ret1 " + typeName + "\n")
	}

	if function.ReturnType.Name != "void" {
		sb.WriteString(prefixStr + "return _ret1 \n")
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

	sb.WriteString(fmt.Sprintf("func (pself *%s) ", mgrTypeName+"Impl"))
	sb.WriteString(pureFuncName)
	sb.WriteString("(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetGdxFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	if function.ReturnType.Name != "void" {
		typeName := GetGdxFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(" " + typeName)
	}
	sb.WriteString(" {")
	prefixStr := "\t"
	// body
	if function.ReturnType.Name != "void" {
		typeName := GetGdxFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString("\n" + prefixStr + "var _ret1 " + typeName + "")
	}

	sb.WriteString(`	
	callInMainThread(func() {
`)
	if function.ReturnType.Name != "void" {
		sb.WriteString(prefixStr + "\t_ret1 = ")
	} else {
		sb.WriteString(prefixStr + "\t")
	}
	sb.WriteString(gdxMgrName + "." + pureFuncName + "(")
	for i, arg := range function.Arguments {
		sb.WriteString(arg.Name)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")

	sb.WriteString(`
	})
`)

	if function.ReturnType.Name != "void" {
		sb.WriteString(prefixStr + "return _ret1 \n")
	}
	sb.WriteString("}")
	return sb.String()
}

func genSyncManagerWrapFunction(function *clang.TypedefFunction) string {
	return ""
}

type ByName []clang.TypedefFunction

func (arr ByName) Len() int      { return len(arr) }
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
	sb.WriteString("func (pself *" + clsName + ") " + funcName + "(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(") ")
	anyRet := function.ReturnType.Name != "void"
	if anyRet {
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(typeName + " ")
	}
	sb.WriteString("{\n")
	if anyRet {
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString("\tvar _val " + typeName + " \n")
		sb.WriteString("\treturn _val;\n")
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
	sb.WriteString("func (pself *" + clsName + ") " + funcName + "(")
	count := len(function.Arguments)
	for i, arg := range function.Arguments {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		sb.WriteString(arg.Name)
		sb.WriteString(" ")
		typeName := GetFuncParamTypeString(arg.Type.Primative.Name)
		sb.WriteString(typeName)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(") ")
	anyRet := function.ReturnType.Name != "void"
	if anyRet {
		typeName := GetFuncParamTypeString(function.ReturnType.Name)
		sb.WriteString(typeName + " ")
	}
	sb.WriteString("{\n")
	sb.WriteString("\t")
	if anyRet {
		sb.WriteString("return ")
	}
	sb.WriteString(mgrName + "Mgr." + funcName + "(")
	if !strings.HasSuffix(function.Name, "CreateSprite") && !strings.HasSuffix(function.Name, "CreateBackdrop") {
		sb.WriteString("pself.Id, ")
	}
	for i, arg := range function.Arguments {
		if i == 0 && arg.Name == "obj" {
			continue
		}
		sb.WriteString(arg.Name)
		if i != count-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")\n")
	sb.WriteString("}\n")
	return sb.String()
}
