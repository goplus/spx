package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/gdextensionparser"
	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/generate/ffi"
	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/generate/gdext"
	"github.com/goplus/spx/v2/pkg/gdspx/cmd/codegen/generate/webffi"
)

var (
	verbose          bool
	cleanAll         bool
	cleanGdextension bool
	cleanTypes       bool
	cleanClasses     bool
	genClangAPI      bool
	genExtensionAPI  bool
	packagePath      string
	godotPath        string
	parsedASTPath    string
	buildConfig      string
)

func init() {
	absPath, _ := filepath.Abs(".")
	var (
		defaultBuildConfig string
	)
	if strings.Contains(runtime.GOARCH, "32") {
		defaultBuildConfig = "float_32"
	} else {
		defaultBuildConfig = "float_64"
	}
	verbose = true
	genClangAPI = true
	genExtensionAPI = false
	packagePath = absPath
	godotPath = "godot"
	parsedASTPath = "_debug_parsed_ast.json"
	buildConfig = defaultBuildConfig
}

func generateCode() error {
	var (
		ast clang.CHeaderFileAST
		err error
	)
	if verbose {
		println(fmt.Sprintf(`build configuration "%s" selected`, buildConfig))
	}
	// generte c++ ext header file
	if genClangAPI {
		if verbose {
			println("Generating gdextension godot ext functions...")
		}
		gdext.GenerateHeader(packagePath)
	}

	// generate go wrap code
	if genClangAPI {
		ast, err = gdextensionparser.GenerateGDExtensionInterfaceAST(packagePath, parsedASTPath)
		if err != nil {
			panic(err)
		}
	}
	if genClangAPI {
		if verbose {
			println("Generating gdextension C wrapper functions...")
		}
		ffi.Generate(packagePath, ast)
		webffi.Generate(packagePath, ast)
		gdext.Generate(packagePath, ast)
	}

	if verbose {
		println("cli tool done")
	}
	return nil
}
func execGoFmt(filePath string) {
	cmd := exec.Command("gofmt", "-w", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Panic(fmt.Errorf("error running gofmt: \n%s\n%w", output, err))
	}
}

func execGoImports(filePath string) {
	cmd := exec.Command("goimports", "-w", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Print(fmt.Errorf("error running goimports: \n%s\n%w", output, err))
	}
}

func main() {
	generateCode()
}
