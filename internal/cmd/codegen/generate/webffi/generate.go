// Package gdextensionwrapper generates C code to wrap all of the gdextension
// methods to call functions on the gdextension_api_structs to work
// around the Cgo C function pointer limitation.
package webffi

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	WebRelDir = "../../gdengine/binding/web"
)
var (

	//go:embed callbacks.go.tmpl
	callbacksFileText string

	//go:embed ffi.go.tmpl
	ffiFileText string

	//go:embed manager_web.go.tmpl
	managerWebText string

	//go:embed gdspx.js.tmpl
	jsEngineJsFileText string

	//go:embed worker.wrap.gen.js.tmpl
	workerWrapJsFileText string
)

func Generate(projectPath, godotPath string, ast clang.CHeaderFileAST) {
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
	err = GenerateJsEngineJsFile(projectPath, godotPath, ast)
	if err != nil {
		panic(err)
	}
	err = GenerateWorkerWrapJsFile(projectPath, ast)
	if err != nil {
		panic(err)
	}
}

func GenerateCallbackGoFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":       GdiVariableName,
		"snakeCase":             strcase.ToSnake,
		"camelCase":             strcase.ToCamel,
		"upper":                 strings.ToUpper,
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

	return GenerateFile(funcs, "callbacks.gen.go", callbacksFileText, ast,
		filepath.Join(projectPath, WebRelDir, "callbacks.gen.go"))
}

func GenerateWorkerWrapJsFile(projectPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":    GdiVariableName,
		"snakeCase":          strcase.ToSnake,
		"camelCase":          strcase.ToCamel,
		"upper":              strings.ToUpper,
		"goReturnType":       GoReturnType,
		"goArgumentType":     GoArgumentType,
		"goEnumValue":        GoEnumValue,
		"add":                Add,
		"cgoCastArgument":    CgoCastArgument,
		"cgoCastReturnType":  CgoCastReturnType,
		"cgoCleanUpArgument": CgoCleanUpArgument,
		"trimPrefix":         TrimPrefix,
	}

	return GenerateFile(funcs, "worker.wrap.gen.js", workerWrapJsFileText, ast,
		filepath.Join(projectPath, "../../../cmd/spx/template/platform/webworker/worker.wrap.gen.js"))
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

	return GenerateFile(funcs, "manager_web.gen.go", managerWebText, ManagerData{Ast: ast, Mangers: GetManagers(ast)},
		filepath.Join(projectPath, GdengineImplRelDir, "manager_web.gen.go"))
}

func GenerateJsEngineJsFile(projectPath, godotPath string, ast clang.CHeaderFileAST) error {
	funcs := template.FuncMap{
		"gdiVariableName":     GdiVariableName,
		"snakeCase":           strcase.ToSnake,
		"camelCase":           strcase.ToCamel,
		"goReturnType":        GoReturnType,
		"goArgumentType":      GoArgumentType,
		"goEnumValue":         GoEnumValue,
		"add":                 Add,
		"sub":                 Sub,
		"getJsFuncArgs":       getJsFuncArgs,
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

	headerFileName := filepath.Join(godotPath, "platform", "web", "js", "engine", "gdspx.js")
	err = os.MkdirAll(filepath.Dir(headerFileName), os.ModePerm)
	if err != nil {
		return err
	}
	f, err := os.Create(headerFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b.Bytes())
	return err
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
	// convert arguments
	for i, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		sb.WriteString(prefixTab)
		if IsNativeArrayDataArg(function, arg) {
			argName := "arg" + strconv.Itoa(i)
			sb.WriteString(argName + " := JsFromGdArray(")
			sb.WriteString(EffectiveGoArgumentName(function, arg))
			sb.WriteString(")\n")
			params = append(params, argName)
			continue
		}
		typeName := MustPrimitiveTypeName(arg, function.Name)
		if usesFlatJsGdIntArg(function, arg) {
			argName := "arg" + strconv.Itoa(i)
			lowName := argName + "Low"
			highName := argName + "High"
			sb.WriteString(lowName + ", " + highName + " := ")
			sb.WriteString(flatJsSplitHelper(typeName))
			sb.WriteString("(")
			sb.WriteString(EffectiveGoArgumentName(function, arg))
			sb.WriteString(")\n")
			params = append(params, lowName, highName)
			continue
		}
		argName := "arg" + strconv.Itoa(i)
		sb.WriteString(argName + " := ")
		sb.WriteString("JsFrom" + typeName)
		sb.WriteString("(")
		sb.WriteString(EffectiveGoArgumentName(function, arg))
		sb.WriteString(")")

		sb.WriteString("\n")
		params = append(params, argName)
	}

	// call the function
	sb.WriteString(prefixTab)
	if HasEffectiveReturn(function) {
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

	if HasEffectiveReturn(function) {
		sb.WriteString("\n" + prefixTab)
		sb.WriteString("return ")
		typeName := EffectiveRawReturnType(function)
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

func getJsFuncArgs(function *clang.TypedefFunction) []string {
	args := EffectiveArguments(function)
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if ShouldSkipHighLevelArgument(function, arg) {
			continue
		}
		argName := EffectiveGoArgumentName(function, arg)
		if usesFlatJsGdIntArg(function, arg) {
			result = append(result, argName+"_low", argName+"_high")
			continue
		}
		result = append(result, argName)
	}
	return result
}

func usesFlatJsGdIntArg(function *clang.TypedefFunction, arg clang.Argument) bool {
	if function == nil {
		return false
	}
	if IsNativeArrayDataArg(function, arg) {
		return false
	}
	return isFlatJsGdIntLikeType(MustPrimitiveTypeName(arg, function.Name))
}

func isFlatJsGdIntLikeType(typeName string) bool {
	switch typeName {
	case "GdInt", "GdObj":
		return true
	default:
		return false
	}
}

func flatJsCtor(typeName string) string {
	switch typeName {
	case "GdInt":
		return "Module._gdspx_new_int"
	case "GdObj":
		return "Module._gdspx_new_obj"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func flatJsSplitHelper(typeName string) string {
	switch typeName {
	case "GdInt":
		return "JsSplitGdInt"
	case "GdObj":
		return "JsSplitGdObj"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func flatJsScratchAccessor(typeName string) string {
	switch typeName {
	case "GdInt":
		return "this._getGdIntScratch()"
	case "GdObj":
		return "this._getGdObjScratch()"
	default:
		panic(fmt.Sprintf("unsupported flat js gdint-like type: %s", typeName))
	}
}

func getJsFuncBody(function *clang.TypedefFunction) string {
	if spec, ok := GetNativeArrayBridgeSpec(function.Name); ok {
		if HasEffectiveReturn(function) {
			panic(fmt.Sprintf("native-array webffi path does not support return values: %s", function.Name))
		}
		argName := spec.BaseArgName
		return "var _arg0 = CopyFastArrayToWasm(" + argName + ");\n" +
			"\tvar _arg1 = " + argName + ".count;\n" +
			"\t_gdFuncPtr(_arg0, _arg1);"
	}
	if function.Name == "GDExtensionSpxInputGetGlobalMousePos" {
		return "var _retValue = AllocGdVec2();\n" +
			"\t_gdFuncPtr(_retValue);\n" +
			"\tvar _scratch = this._inputMousePosScratch;\n" +
			"\tif (!_scratch) {\n" +
			"\t\t_scratch = this._inputMousePosScratch = { x: 0, y: 0 };\n" +
			"\t}\n" +
			"\tvar _floatIndex = _retValue / 4;\n" +
			"\tvar _heap = Module.HEAPF32;\n" +
			"\t_scratch.x = _heap[_floatIndex];\n" +
			"\t_scratch.y = _heap[_floatIndex + 1];\n" +
			"\tFreeGdVec2(_retValue);\n" +
			"\treturn _scratch"
	}

	sb := strings.Builder{}
	prefixTab := "\t"
	params := []string{}
	args := EffectiveArguments(function)
	rawRetType := EffectiveRawReturnType(function)

	// call the function
	if rawRetType != "" {
		sb.WriteString("var _retValue = Alloc" + rawRetType + "();")
	}
	sb.WriteString("\n")

	// convert arguments
	for i, arg := range args {
		sb.WriteString(prefixTab)
		typeName := MustPrimitiveTypeName(arg, function.Name)
		argName := "_arg" + strconv.Itoa(i)
		if usesFlatJsGdIntArg(function, arg) {
			sb.WriteString("var " + argName + " = ")
			sb.WriteString(flatJsCtor(typeName))
			sb.WriteString("(")
			sb.WriteString(arg.Name + "_high, " + arg.Name + "_low")
			sb.WriteString(");")

			sb.WriteString("\n")
			params = append(params, argName)
			continue
		}
		sb.WriteString("var " + argName + " = ")
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
	if rawRetType != "" {
		if len(params) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("_retValue")
	}
	sb.WriteString(");")

	sb.WriteString("\n")
	// convert arguments
	for i, arg := range args {
		sb.WriteString(prefixTab)
		typeName := MustPrimitiveTypeName(arg, function.Name)
		argName := "_arg" + strconv.Itoa(i)
		sb.WriteString("Free" + typeName + "(" + argName + "); \n")
	}

	if rawRetType != "" {
		sb.WriteString(prefixTab + "var _finalRetValue = ")
		if isFlatJsGdIntLikeType(rawRetType) {
			sb.WriteString("this._readGdIntLike(_retValue, ")
			sb.WriteString(flatJsScratchAccessor(rawRetType))
			sb.WriteString(");\n")
		} else {
			typeName := rawRetType
			funcName := strcase.ToCamel(typeName)
			funcName = "ToJs" + strings.ReplaceAll(funcName, "Gd", "")
			sb.WriteString(funcName + "(_retValue);\n")
		}
		sb.WriteString(prefixTab + "Free" + rawRetType + "(_retValue); \n")
		sb.WriteString(prefixTab + "return _finalRetValue")
	}
	return sb.String()
}
