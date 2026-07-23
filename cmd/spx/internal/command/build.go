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

package command

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/goplus/spx/v3/cmd/spx/internal/util"
)

func (cmd *CmdTool) BuildWasm() error {
	cmd.genGo()

	webBuildDir := path.Join(cmd.ProjectDir, ".builds/web/")
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create web build directory: %w", err)
	}
	filePath := path.Join(webBuildDir, "ispx.wasm")

	return cmd.withGoDir(func() error {
		logDebugf("Building WebAssembly binary: %s", filePath)
		envVars := []string{"GOOS=js", "GOARCH=wasm"}

		util.RunGolang(envVars, "build", "-o", filePath)
		return nil
	})
}

// BuildTinyGoLib builds a TinyGo static library.
func (cmd *CmdTool) BuildTinyGoLib() error {
	cmd.genGo()

	target := *cmd.Args.Target
	if target == "" || target == "esp32" {
		target = "esp32-coreboard-v2"
	}

	tinyGoBuildDir := path.Join(cmd.ProjectDir, ".builds/tinygo/")
	if err := os.MkdirAll(tinyGoBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create TinyGo build directory: %w", err)
	}
	outputPath := path.Join(tinyGoBuildDir, "golib.o")

	args := []string{
		"build",
		"-o", outputPath,
		"-target=" + target,
		"-no-debug",
		"-opt=2",
		"-gc=leaking",
		"-scheduler=none",
	}
	if tags := *cmd.Args.Tags; tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, ".")

	envVars := []string{"GODEBUG=gotypesalias=0"}

	if err := cmd.withGoDir(func() error {
		logInfof("Building TinyGo static library for target: %s", target)
		if err := util.RunTinyGo(envVars, args...); err != nil {
			return fmt.Errorf("tinygo build failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	logInfof("Built TinyGo static library: %s", outputPath)
	return nil
}

func (cmd *CmdTool) BuildDll() error {
	if err := cmd.hideIOSFiles(); err != nil {
		return err
	}

	targetArchs, err := cmd.determineTargetArchs()
	if err != nil {
		return err
	}

	tagStr := cmd.genGo()

	return cmd.withGoDir(func() error {
		if err := cmd.executeDllBuild(targetArchs, tagStr); err != nil {
			return err
		}
		if cmd.LibPath == "" {
			return fmt.Errorf("build error: cannot find matched dylib for runtime arch %s", runtime.GOARCH)
		}
		return nil
	})
}

// withGoDir runs f in cmd.GoDir.
func (cmd *CmdTool) withGoDir(f func() error) error {
	rawdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if err := os.Chdir(cmd.GoDir); err != nil {
		return fmt.Errorf("failed to change directory to GoDir %s: %w", cmd.GoDir, err)
	}

	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			logWarnf("Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	return f()
}

// hideIOSFiles renames ios* files to .txt files.
func (cmd *CmdTool) hideIOSFiles() error {
	searchPattern := filepath.Join(cmd.ProjectDir, "go", "ios*")
	files, err := filepath.Glob(searchPattern)
	if err != nil {
		logWarnf("Glob failed for pattern %s: %v", searchPattern, err)
		return nil
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".txt") {
			newName := file + ".txt"
			if err := os.Rename(file, newName); err != nil {
				logWarnf("Failed to rename %s to %s: %v", file, newName, err)
			}
		}
	}
	return nil
}

// determineTargetArchs resolves target architectures.
func (cmd *CmdTool) determineTargetArchs() ([]string, error) {
	if runtime.GOOS == "darwin" {
		return []string{"amd64", "arm64"}, nil
	}

	tarArch := *cmd.Args.Arch
	if tarArch == "" {
		return []string{runtime.GOARCH}, nil
	}

	var validArchs []string
	switch runtime.GOOS {
	case "windows":
		validArchs = []string{"amd64", "386"}
	case "darwin":
		validArchs = []string{"amd64", "arm64"}
	case "linux":
		validArchs = []string{"amd64", "arm", "arm64", "386"}
	default:
		validArchs = []string{runtime.GOARCH}
	}

	if tarArch == "all" {
		return validArchs, nil
	}

	if slices.Contains(validArchs, tarArch) {
		return []string{tarArch}, nil
	}

	return nil, fmt.Errorf("invalid arch %s. Valid archs for %s: %s",
		tarArch, runtime.GOOS, strings.Join(validArchs, ","))
}

func (cmd *CmdTool) genGo() string {
	rawdir, err := os.Getwd()
	if err != nil {
		logFatalf("Failed to get current working directory: %v", err)
	}

	spxProjPath := filepath.Join(cmd.ProjectDir, "..")

	if err := cmd.genGoUsingXgoCLI(rawdir, spxProjPath); err != nil {
		logFatalf("Code generation failed using xgo CLI: %v", err)
	}

	return cmd.SafeTagArgs()
}

// genGoUsingXgoCLI generates code with xgo.
func (cmd *CmdTool) genGoUsingXgoCLI(rawdir, spxProjPath string) error {
	if err := cmd.ensureBuilderAIModuleFiles(spxProjPath); err != nil {
		return err
	}

	if err := os.Chdir(spxProjPath); err != nil {
		return fmt.Errorf("failed to change directory to project root for XGo: %w", err)
	}
	defer func() {
		if err := os.Chdir(rawdir); err != nil {
			logWarnf("Failed to restore working directory to %s: %v", rawdir, err)
		}
	}()

	tagStr := cmd.SafeTagArgs()
	logDebugf("GenGo tags: %s", tagStr)
	envVars := []string{""}

	args := []string{"go"}
	if tagStr != "" {
		args = append(args, tagStr)
	}
	util.RunXGo(envVars, args...)

	if err := os.MkdirAll(cmd.GoDir, 0755); err != nil {
		return fmt.Errorf("failed to create GoDir: %w", err)
	}

	sourceFile := path.Join(spxProjPath, "xgo_autogen.go")
	destFile := path.Join(cmd.GoDir, "main.go")

	if err := os.Rename(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to rename/move generated file %s to %s: %w", sourceFile, destFile, err)
	}

	if cmd.shouldRunGoModTidy() {
		util.RunGolang(nil, "mod", "tidy")
	}

	return nil
}

// executeDllBuild runs a multi-arch C-shared build.
func (cmd *CmdTool) executeDllBuild(archs []string, tagStr string) error {
	rawPath := filepath.Base(cmd.LibPath)
	rawDir := filepath.Dir(cmd.LibPath)

	cmd.LibPath = ""
	baseEnvs := []string{"CGO_ENABLED=1"}

	buildArgs := []string{"build"}
	if tagStr != "" {
		buildArgs = append(buildArgs, tagStr)
	}
	buildArgs = append(buildArgs, "-buildmode=c-shared")

	strs := strings.Split(rawPath, "-")
	if len(strs) < 3 {
		return fmt.Errorf("unexpected library path format: %s. Expected format like base-ver-arch.ext", rawPath)
	}
	baseName := strings.Join(strs[:2], "-")

	extParts := strings.Split(strs[2], ".")
	fileExt := extParts[len(extParts)-1]

	for _, arch := range archs {
		newPath := filepath.Join(rawDir, fmt.Sprintf("%s-%s.%s", baseName, arch, fileExt))

		if arch == runtime.GOARCH {
			cmd.LibPath = newPath
		}

		envs := append(baseEnvs, "GOARCH="+arch)
		currentArgs := append(buildArgs, "-o", newPath)

		logDebugf("Building shared library: envs=%s, args=%s", envs, currentArgs)
		util.RunGolang(envs, currentArgs...)
	}
	return nil
}
