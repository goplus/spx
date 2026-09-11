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
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/common"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/ffi"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/gdext"
	"github.com/goplus/spx/v3/internal/cmd/codegen/generate/webffi"
	spxlog "github.com/goplus/spx/v3/internal/cmd/codegen/internal/log"
	"github.com/goplus/spx/v3/internal/release"
)

var (
	verbose         bool
	genClangAPI     bool
	genExtensionAPI bool
	packagePath     string
	spxModulePath   string
	parsedASTPath   string
	buildConfig     string
)

var requiredCodegenModuleFiles = []string{
	"gdextension_interface.h",
	"spx_ext_mgr.h",
}

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
	repoRoot := filepath.Clean(filepath.Join(absPath, "../../.."))
	spxModulePath = resolveSPXModuleSource(repoRoot, os.Getenv("SPX_MODULE_SRC"))
	parsedASTPath = "_debug_parsed_ast.json"
	buildConfig = defaultBuildConfig
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

func generateCode() error {
	// Validate every external input before generators can create or replace files.
	if err := validateCodegenInputs(spxModulePath); err != nil {
		return err
	}

	if verbose {
		spxlog.Info(`build configuration "%s" selected`, buildConfig)
		spxlog.Info(`SPX module source "%s" selected`, spxModulePath)
	}
	// Prepare both header spellings and metadata before rendering bindings.
	if genClangAPI {
		if verbose {
			spxlog.Info("Generating gdextension godot ext functions...")
		}
		headers, err := gdext.PrepareHeaders(spxModulePath)
		if err != nil {
			return fmt.Errorf("prepare GDExtension headers: %w", err)
		}
		if err := os.WriteFile(filepath.Join(packagePath, common.NativeRelDir, "gdextension_spx_ext.h"), []byte(headers.Raw), 0o644); err != nil {
			return fmt.Errorf("generate GDExtension header: %w", err)
		}
		ast, err := gdextensionparser.GenerateGDExtensionInterfaceAST(packagePath, parsedASTPath)
		if err != nil {
			return fmt.Errorf("parse GDExtension interface: %w", err)
		}
		if verbose {
			spxlog.Info("Generating gdextension C wrapper functions...")
		}
		generation := common.NewGenerationContext(ast, headers.Metadata)
		nativeGenerator := ffi.Generator{GenerationContext: generation}
		webGenerator := webffi.Generator{GenerationContext: generation}
		extensionGenerator := gdext.Generator{GenerationContext: generation}
		if err := nativeGenerator.Generate(packagePath); err != nil {
			return fmt.Errorf("generate native bindings: %w", err)
		}
		if err := webGenerator.Generate(packagePath, spxModulePath); err != nil {
			return fmt.Errorf("generate Web bindings: %w", err)
		}
		if err := extensionGenerator.Generate(packagePath, spxModulePath, headers); err != nil {
			return fmt.Errorf("generate GDExtension sources: %w", err)
		}
	}

	if verbose {
		spxlog.Info("CLI tool completed")
	}
	return nil
}

func main() {
	if err := generateCode(); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: %v\n", err)
		os.Exit(1)
	}
}
