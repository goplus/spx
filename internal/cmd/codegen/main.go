package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser"
	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/ffi"
	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/gdext"
	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/webffi"
	spxlog "github.com/goplus/spx/v2/internal/cmd/codegen/internal/log"
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
	godotPath = filepath.Clean(filepath.Join(absPath, "../../../godot"))
	if envPath := os.Getenv("GODOT_SRC"); envPath != "" {
		if absEnvPath, err := filepath.Abs(envPath); err == nil {
			godotPath = absEnvPath
		} else {
			godotPath = envPath
		}
	}
	parsedASTPath = "_debug_parsed_ast.json"
	buildConfig = defaultBuildConfig
}

func generateCode() error {
	var (
		ast clang.CHeaderFileAST
		err error
	)
	if verbose {
		spxlog.Info(`build configuration "%s" selected`, buildConfig)
		spxlog.Info(`godot source "%s" selected`, godotPath)
	}
	// generte c++ ext header file
	if genClangAPI {
		if verbose {
			spxlog.Info("Generating gdextension godot ext functions...")
		}
		gdext.GenerateHeader(packagePath, godotPath)
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
			spxlog.Info("Generating gdextension C wrapper functions...")
		}
		ffi.Generate(packagePath, ast)
		webffi.Generate(packagePath, godotPath, ast)
		gdext.Generate(packagePath, godotPath, ast)
	}

	if verbose {
		spxlog.Info("cli tool done")
	}
	return nil
}
func execGoFmt(filePath string) {
	cmd := exec.Command("gofmt", "-w", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		spxlog.Error("error running gofmt:\n%s\n%v", output, err)
		panic(fmt.Errorf("error running gofmt: \n%s\n%w", output, err))
	}
}

func execGoImports(filePath string) {
	cmd := exec.Command("goimports", "-w", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		spxlog.Error("error running goimports:\n%s\n%v", output, err)
	}
}

func main() {
	generateCode()
}
