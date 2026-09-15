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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/ffi"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/gdext"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/webffi"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"
	"github.com/goplus/spx/v3/internal/release"
)

// codegenConfig contains the paths for one generation run.
type codegenConfig struct {
	codegenDir    string
	spxModulePath string
	parsedASTPath string
}

var requiredCodegenModuleFiles = []string{
	"gdextension_interface.h",
	"spx_ext_mgr.h",
	"web/js/engine/gdspx.util.js",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	codegenDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve codegen directory: %w", err)
	}
	repoRoot := filepath.Clean(filepath.Join(codegenDir, "../../.."))
	return generateCode(codegenConfig{
		codegenDir:    codegenDir,
		spxModulePath: resolveSPXModuleSource(repoRoot, os.Getenv("SPX_MODULE_SRC")),
		parsedASTPath: filepath.Join(codegenDir, "_debug_parsed_ast.json"),
	})
}

func resolveSPXModuleSource(repoRoot, override string) string {
	if absRepoRoot, err := filepath.Abs(repoRoot); err == nil {
		repoRoot = absRepoRoot
	}

	moduleSource := strings.TrimSpace(override)
	if moduleSource == "" {
		moduleSource = filepath.Join(repoRoot, filepath.FromSlash(release.DefaultRuntimeLock().Module.Path))
	} else if !filepath.IsAbs(moduleSource) {
		moduleSource = filepath.Join(repoRoot, moduleSource)
	}
	if absModuleSource, err := filepath.Abs(moduleSource); err == nil {
		moduleSource = absModuleSource
	}
	return filepath.Clean(moduleSource)
}

func validateCodegenInputs(spxModuleSource string) error {
	info, err := os.Stat(spxModuleSource)
	if err != nil {
		return fmt.Errorf("SPX_MODULE_SRC %q: %w", spxModuleSource, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("SPX_MODULE_SRC %q is not a directory", spxModuleSource)
	}
	for _, name := range requiredCodegenModuleFiles {
		path := filepath.Join(spxModuleSource, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("SPX module file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SPX module file %q is not a regular file", path)
		}
	}
	return nil
}

func generateCode(config codegenConfig) error {
	if err := validateCodegenInputs(config.spxModulePath); err != nil {
		return err
	}

	spxlog.Info("SPX module source %q selected", config.spxModulePath)
	headers, err := gdext.PrepareHeaders(config.spxModulePath)
	if err != nil {
		return fmt.Errorf("prepare GDExtension headers: %w", err)
	}
	// Parse source return types before publishing the lowered output-parameter ABI.
	if err := common.WriteGeneratedFile(filepath.Join(config.codegenDir, common.NativeRelDir, "gdextension_spx_ext.h"), []byte(headers.Raw), 0o644); err != nil {
		return fmt.Errorf("generate GDExtension header: %w", err)
	}
	ast, err := gdextensionparser.GenerateGDExtensionInterfaceAST(config.codegenDir, config.parsedASTPath)
	if err != nil {
		return fmt.Errorf("parse GDExtension interface: %w", err)
	}
	generation := common.NewGenerationContext(ast, headers.Metadata)
	nativeGenerator := ffi.Generator{GenerationContext: generation}
	webGenerator := webffi.Generator{GenerationContext: generation}
	extensionGenerator := gdext.Generator{GenerationContext: generation}
	if err := nativeGenerator.Generate(config.codegenDir); err != nil {
		return fmt.Errorf("generate native bindings: %w", err)
	}
	if err := webGenerator.Generate(config.codegenDir, config.spxModulePath); err != nil {
		return fmt.Errorf("generate Web bindings: %w", err)
	}
	if err := extensionGenerator.Generate(config.codegenDir, config.spxModulePath, headers); err != nil {
		return fmt.Errorf("generate GDExtension sources: %w", err)
	}

	spxlog.Info("Code generation completed")
	return nil
}
