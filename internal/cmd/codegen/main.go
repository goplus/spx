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

package main

import (
	"os"
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
	verbose         bool
	genClangAPI     bool
	genExtensionAPI bool
	packagePath     string
	godotPath       string
	parsedASTPath   string
	buildConfig     string
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
		spxlog.Info("CLI tool completed")
	}
	return nil
}

func main() {
	generateCode()
}
