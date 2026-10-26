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

	"github.com/goplus/spx/v4/cmd/spx/internal/util"
)

func (cmd *CmdTool) BuildWasm() error {
	if _, err := cmd.genGo(); err != nil {
		return err
	}

	webBuildDir := path.Join(cmd.ProjectDir, ".builds/web/")
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create web build directory: %w", err)
	}
	filePath := path.Join(webBuildDir, "ispx.wasm")

	logDebugf("Building WebAssembly binary: %s", filePath)
	envVars := []string{"GOOS=js", "GOARCH=wasm"}
	if err := util.ExecCommand(util.CommandOptions{Dir: cmd.GoDir, Env: envVars}, "go", "build", "-o", filePath); err != nil {
		return fmt.Errorf("webassembly build failed: %w", err)
	}
	return nil
}

// BuildTinyGoLib builds a TinyGo static library.
func (cmd *CmdTool) BuildTinyGoLib() error {
	tagStr, err := cmd.genGo()
	if err != nil {
		return err
	}

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
	if tagStr != "" {
		args = append(args, tagStr)
	}
	args = append(args, ".")

	envVars := []string{"GODEBUG=gotypesalias=0"}
	logInfof("Building TinyGo static library for target: %s", target)
	if err := util.ExecCommand(util.CommandOptions{Dir: cmd.GoDir, Env: envVars}, "tinygo", args...); err != nil {
		return fmt.Errorf("tinygo build failed: %w", err)
	}

	logInfof("Built TinyGo static library: %s", outputPath)
	return nil
}

func (cmd *CmdTool) BuildDll() error {
	cmd.hideIOSFiles()

	requestedArch := ""
	if cmd.Args.Arch != nil {
		requestedArch = *cmd.Args.Arch
	}
	targetArchs, err := determineTargetArchs(runtime.GOOS, runtime.GOARCH, requestedArch)
	if err != nil {
		return err
	}

	tagStr, err := cmd.genGo()
	if err != nil {
		return err
	}
	if err := cmd.executeDllBuild(targetArchs, tagStr); err != nil {
		return err
	}
	if cmd.LibPath == "" {
		return fmt.Errorf("build error: cannot find matched dylib for runtime arch %s", runtime.GOARCH)
	}
	return nil
}

// genGo generates code from the source root without changing the process directory.
func (cmd *CmdTool) genGo() (string, error) {
	if err := cmd.ensureBuilderAIModuleFiles(cmd.TargetDir); err != nil {
		return "", err
	}

	tagStr := cmd.SafeTagArgs()
	logDebugf("GenGo tags: %s", tagStr)
	args := []string{"go"}
	if tagStr != "" {
		args = append(args, tagStr)
	}
	if err := util.ExecCommand(util.CommandOptions{Dir: cmd.TargetDir}, "xgo", args...); err != nil {
		return "", fmt.Errorf("code generation failed using xgo CLI: %w", err)
	}

	if err := os.MkdirAll(cmd.GoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create GoDir: %w", err)
	}

	sourceFile := path.Join(cmd.TargetDir, "xgo_autogen.go")
	destFile := path.Join(cmd.GoDir, "main.go")

	if err := os.Rename(sourceFile, destFile); err != nil {
		return "", fmt.Errorf("failed to rename/move generated file %s to %s: %w", sourceFile, destFile, err)
	}

	if cmd.shouldRunGoModTidy() {
		if err := util.ExecCommand(util.CommandOptions{Dir: cmd.TargetDir}, "go", "mod", "tidy"); err != nil {
			return "", fmt.Errorf("go mod tidy failed: %w", err)
		}
	}

	return tagStr, nil
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
		if err := util.ExecCommand(util.CommandOptions{Dir: cmd.GoDir, Env: envs}, "go", currentArgs...); err != nil {
			return fmt.Errorf("go shared-library build for %s failed: %w", arch, err)
		}
	}
	return nil
}

// hideIOSFiles renames ios* files to .txt files.
func (cmd *CmdTool) hideIOSFiles() {
	dir := filepath.Join(cmd.ProjectDir, "go")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		logWarnf("Failed to read iOS source directory %s: %v", dir, err)
		return
	}

	for _, file := range files {
		name := file.Name()
		if !strings.HasPrefix(name, "ios") || strings.HasSuffix(name, ".txt") {
			continue
		}
		source := filepath.Join(dir, name)
		if err := os.Rename(source, source+".txt"); err != nil {
			logWarnf("Failed to rename %s to %s: %v", source, source+".txt", err)
		}
	}
}

// determineTargetArchs resolves target architectures.
func determineTargetArchs(goos, hostArch, requestedArch string) ([]string, error) {
	if goos == "darwin" {
		return []string{"amd64", "arm64"}, nil
	}

	if requestedArch == "" {
		return []string{hostArch}, nil
	}

	var validArchs []string
	switch goos {
	case "windows":
		validArchs = []string{"amd64", "386"}
	case "linux":
		validArchs = []string{"amd64", "arm", "arm64", "386"}
	default:
		validArchs = []string{hostArch}
	}

	if requestedArch == "all" {
		return validArchs, nil
	}

	if slices.Contains(validArchs, requestedArch) {
		return []string{requestedArch}, nil
	}

	return nil, fmt.Errorf("invalid arch %s. Valid archs for %s: %s",
		requestedArch, goos, strings.Join(validArchs, ","))
}
