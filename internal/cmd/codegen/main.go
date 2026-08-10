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
	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
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
	godotSourcePath string
	parsedASTPath   string
	buildConfig     string
)

var requiredSPXModuleFiles = []string{
	"SCsub",
	"config.py",
	"gdextension_interface.h",
	"spx_ext_mgr.h",
}

var requiredGodotSourceFiles = []string{
	"SConstruct",
	"version.py",
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
	godotSourcePath = resolveOptionalSource(repoRoot, os.Getenv("GODOT_SRC"))
	parsedASTPath = "_debug_parsed_ast.json"
	buildConfig = defaultBuildConfig
}

func resolveSPXModuleSource(repoRoot, override string) string {
	if absRepoRoot, err := filepath.Abs(repoRoot); err == nil {
		repoRoot = absRepoRoot
	}

	moduleSource := strings.TrimSpace(override)
	if moduleSource == "" {
		moduleSource = filepath.Join(repoRoot, release.DefaultRuntimeLock().Module.Path)
	} else if !filepath.IsAbs(moduleSource) {
		moduleSource = filepath.Join(repoRoot, moduleSource)
	}
	if absModuleSource, err := filepath.Abs(moduleSource); err == nil {
		moduleSource = absModuleSource
	}
	return filepath.Clean(moduleSource)
}

func resolveOptionalSource(repoRoot, override string) string {
	source := strings.TrimSpace(override)
	if source == "" {
		return ""
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(repoRoot, source)
	}
	if absSource, err := filepath.Abs(source); err == nil {
		source = absSource
	}
	return filepath.Clean(source)
}

func validateDirectory(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", name, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", name, path)
	}
	return nil
}

func validateCodegenInputs(spxModuleSource, godotSource string) error {
	if err := validateDirectory("SPX_MODULE_SRC", spxModuleSource); err != nil {
		return err
	}
	for _, name := range requiredSPXModuleFiles {
		path := filepath.Join(spxModuleSource, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("SPX module file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SPX module file %q is not a regular file", path)
		}
	}
	managerHeaders, err := filepath.Glob(filepath.Join(spxModuleSource, "spx*mgr.h"))
	if err != nil {
		return fmt.Errorf("find SPX manager headers: %w", err)
	}
	if len(managerHeaders) == 0 {
		return fmt.Errorf("SPX module %q has no spx*mgr.h manager header", spxModuleSource)
	}
	if godotSource != "" {
		if err := validateDirectory("GODOT_SRC", godotSource); err != nil {
			return err
		}
		for _, name := range requiredGodotSourceFiles {
			path := filepath.Join(godotSource, name)
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("Godot source file %q: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("Godot source file %q is not a regular file", path)
			}
		}
	}
	return nil
}

func generateCode() error {
	// Validate every external input before generators can create or replace files.
	if err := validateCodegenInputs(spxModulePath, godotSourcePath); err != nil {
		return err
	}

	var (
		ast clang.CHeaderFileAST
		err error
	)
	if verbose {
		spxlog.Info(`build configuration "%s" selected`, buildConfig)
		spxlog.Info(`SPX module source "%s" selected`, spxModulePath)
		if godotSourcePath != "" {
			spxlog.Info(`Godot source "%s" selected`, godotSourcePath)
		}
	}
	// generte c++ ext header file
	if genClangAPI {
		if verbose {
			spxlog.Info("Generating gdextension godot ext functions...")
		}
		if err := gdext.GenerateHeader(packagePath, spxModulePath); err != nil {
			return fmt.Errorf("generate GDExtension header: %w", err)
		}
	}

	// generate go wrap code
	if genClangAPI {
		ast, err = gdextensionparser.GenerateGDExtensionInterfaceAST(packagePath, parsedASTPath)
		if err != nil {
			return fmt.Errorf("parse GDExtension interface: %w", err)
		}
	}
	if genClangAPI {
		if verbose {
			spxlog.Info("Generating gdextension C wrapper functions...")
		}
		if err := ffi.Generate(packagePath, ast); err != nil {
			return fmt.Errorf("generate native bindings: %w", err)
		}
		if err := webffi.Generate(packagePath, spxModulePath, ast); err != nil {
			return fmt.Errorf("generate Web bindings: %w", err)
		}
		if err := gdext.Generate(packagePath, spxModulePath, ast); err != nil {
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
