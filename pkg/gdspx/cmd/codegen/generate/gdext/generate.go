// Package gdextensionwrapper generates C code to wrap all of the gdextension
// methods to call functions on the gdextension_api_structs to work
// around the Cgo C function pointer limitation.
package gdext

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/gdextensionparser/clang"
	. "github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/generate/common"

	"github.com/iancoleman/strcase"
)

var (
	//go:embed gdextension_spx_ext.cpp.tmpl
	gdSpxExtCpp string

	//go:embed godot_js_spx.cpp.tmpl
	gdJsSpxCpp string

	//go:embed gdextension_spx_ext.h.tmpl
	gdSpxExtH string
)

func fileCopy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return dstFile.Sync()
}
func GenerateHeader(projectPath string) {
	dir := filepath.Join(projectPath, "../../godot/core/extension")
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		println("dir not exist", dir)
		return
	}
	outputFile := filepath.Join(projectPath, RelDir, "gdextension_spx_ext.h")
	generateSpxExtHeader(dir, outputFile, true)
}
func Generate(projectPath string, ast clang.CHeaderFileAST) {
	dir := filepath.Join(projectPath, "../../godot/core/extension")
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		println("dir not exist", dir)
		return
	}
	err = generateGdCppFile(projectPath, gdSpxExtCpp, ast, "gdextension_spx_ext.cpp")
	if err != nil {
		panic(err)
	}
	outputFile := filepath.Join(projectPath, RelDir, "gdextension_spx_ext.cpp")
	fileCopy(outputFile, filepath.Join(dir, "gdextension_spx_ext.cpp"))
	os.Remove(outputFile)

	// use the new format header
	outputFile = filepath.Join(projectPath, RelDir, "gdextension_spx_ext.h")
	generateSpxExtHeader(dir, outputFile, false)
	fileCopy(outputFile, filepath.Join(dir, "gdextension_spx_ext.h"))

	err = generateGdCppFile(projectPath, gdJsSpxCpp, ast, "godot_js_spx.cpp")
	if err != nil {
		panic(err)
	}
	outputFile = filepath.Join(projectPath, RelDir, "godot_js_spx.cpp")
	fileCopy(outputFile, filepath.Join(filepath.Join(projectPath, "../../godot/platform/web"), "godot_js_spx.cpp"))
	os.Remove(outputFile)
}

func generateGdCppFile(projectPath string, templateStr string, ast clang.CHeaderFileAST, outputFileName string) error {
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
		"trimPrefix":          TrimPrefix,
		"loadProcAddressName": LoadProcAddressName,
		"isManagerMethod":     IsManagerMethod,
		"getManagerName":      GetManagerName,
	}

	tmpl, err := template.New(outputFileName).
		Funcs(funcs).
		Parse(templateStr)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, ManagerData{Ast: ast, Mangers: GetManagers(ast)})
	if err != nil {
		return err
	}

	headerFileName := filepath.Join(projectPath, RelDir, outputFileName)
	f, err := os.Create(headerFileName)
	f.Write(b.Bytes())
	f.Close()
	return err
}
