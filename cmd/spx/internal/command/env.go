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
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	builderai "github.com/goplus/spx/v4/cmd/spx/internal/command/builderai"
	"github.com/goplus/spx/v4/cmd/spx/internal/util"
)

const envName = "gdspx"
const projectImportTimeoutEnvVar = "SPX_GODOT_IMPORT_TIMEOUT"
const defaultProjectImportTimeout = 10 * time.Minute

var projectNameReplacer = strings.NewReplacer("_", "", " ", "", "\"", "", "\n", "", "\r", "")

// CheckEnv validates the target directory.
func (cmd *CmdTool) CheckEnv() error {
	dir, err := filepath.Abs(cmd.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory path: %w", err)
	}

	exist := util.CheckFileExist(dir, cmd.FileSuffix, false)
	if !exist {
		return fmt.Errorf("cannot find %s file, not a valid project directory", cmd.FileSuffix)
	}
	return nil
}

// SetupEnv prepares command state.
func (cmd *CmdTool) SetupEnv(version string, fs embed.FS, fsRelDir string, projectRelPath string) error {
	cmd.ProjectFS = fs
	cmd.Version = version
	cmd.ProjectRelPath = projectRelPath

	goos, goarch, err := resolveTargetPlatform()
	if err != nil {
		return err
	}

	if err := cmd.setupEnginePaths(); err != nil {
		return err
	}

	if err := cmd.setupProjectPaths(goos, goarch); err != nil {
		return err
	}

	if err := cmd.PrepareEnv(fsRelDir, cmd.ProjectDir); err != nil {
		return err
	}

	return cmd.updateProjectName()
}

// PrepareEnv syncs project files and resolves module dependencies without
// changing the process working directory.
func (cmd *CmdTool) PrepareEnv(fsRelDir, dstDir string) (err error) {
	if err := util.CopyDir(cmd.ProjectFS, fsRelDir, dstDir, true); err != nil {
		return fmt.Errorf("prepare project files: %w", err)
	}
	if err := cmd.adaptGoMod(); err != nil {
		return fmt.Errorf("prepare project module: %w", err)
	}
	if !cmd.shouldRunGoModTidy() {
		return nil
	}

	tempFile := filepath.Join(cmd.TargetDir, "xgo_autogen.go")
	tmp := `
package main
import "github.com/goplus/spx/v4"
func main() {print(&spx.Game{})}
`
	if err := os.WriteFile(tempFile, []byte(tmp), 0o644); err != nil {
		return fmt.Errorf("prepare temporary Go source: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(tempFile); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, fmt.Errorf("remove temporary Go source: %w", removeErr))
		}
	}()
	if err := util.ExecCommand(util.CommandOptions{Dir: cmd.TargetDir}, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("prepare module dependencies: %w", err)
	}
	return nil
}

// ShouldReimport reports whether Godot reimport is needed.
func (cmd *CmdTool) ShouldReimport() bool {
	if cmd.shouldSkipProjectImport() {
		return false
	}
	return !util.IsFileExist(cmd.projectImportCachePath())
}

// Reimport refreshes Godot import data.
func (cmd *CmdTool) Reimport() error {
	if !cmd.shouldSkipProjectImport() {
		if err := cmd.BuildDll(); err != nil {
			return err
		}
	}
	return cmd.runProjectImport()
}

// Clear removes generated project files.
func (cmd *CmdTool) Clear() error {
	for _, artifact := range []string{
		cmd.ProjectDir,
		path.Join(cmd.TargetDir, ".temp"),
		path.Join(cmd.TargetDir, "go.sum"),
		path.Join(cmd.TargetDir, "xgo_autogen.go"),
	} {
		if err := os.RemoveAll(artifact); err != nil {
			return fmt.Errorf("failed to remove project directory: %w", err)
		}
	}
	return nil
}

// ClearBuild removes build artifacts while preserving the generated project.
func (cmd *CmdTool) ClearBuild() error {
	buildDir := filepath.Join(cmd.ProjectDir, ".builds")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to remove build directory: %w", err)
	}
	return nil
}

// setupPaths resolves command paths and server options without changing cwd.
func (cmd *CmdTool) setupPaths(dstRelDir string) error {
	if err := cmd.resolveProjectPaths(*cmd.Args.Path, dstRelDir); err != nil {
		return err
	}
	info, err := os.Stat(cmd.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to inspect target directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("target path %q is not a directory", cmd.TargetDir)
	}

	if *cmd.Args.ServerAddr != "" {
		addr := *cmd.Args.ServerAddr
		parts := strings.Split(addr, ":")
		if len(parts) < 2 {
			return fmt.Errorf("invalid server address format: %s", addr)
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid port number: %w", err)
		}
		cmd.ServerPort = port
	}

	return nil
}

func (cmd *CmdTool) setupEnginePaths() error {
	binPostfix, cmdPath, err := resolveAppPath(cmd.GoBinPath, envName, cmd.Version)
	if err != nil {
		return fmt.Errorf("%s requires engine to be installed as a binary at %s: %w", envName, cmd.GoBinPath, err)
	}
	cmd.BinPostfix = binPostfix
	cmd.CmdPath = cmdPath
	return nil
}

func (cmd *CmdTool) setupProjectPaths(goos, goarch string) error {
	// SetupEnv can be called independently of CLI path resolution.
	if err := cmd.resolveProjectPaths(cmd.TargetDir, cmd.ProjectRelPath); err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}
	targetDir, projectDir := cmd.TargetDir, cmd.ProjectDir
	cmd.GoDir = filepath.Join(projectDir, "go")

	cmd.RuntimeCmdPath = path.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)

	runtimeTempDir := filepath.Join(targetDir, ".temp")
	cmd.RuntimeTempDir = runtimeTempDir
	if err := os.MkdirAll(runtimeTempDir, 0o755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	cmd.LibPath = filepath.Join(projectDir, "lib", libraryFileName(envName, goos, goarch))
	return nil
}

func (cmd *CmdTool) updateProjectName() error {
	targetDir, err := filepath.Abs(cmd.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}
	projectName := projectNameReplacer.Replace(filepath.Base(targetDir))

	engineFilePath := path.Join(cmd.ProjectDir, "project.godot")
	content, err := os.ReadFile(engineFilePath)
	if err != nil {
		return fmt.Errorf("failed to read project file: %w", err)
	}

	replacedContent := strings.ReplaceAll(
		string(content),
		`config/name="spx"`,
		fmt.Sprintf(`config/name="%s"`, projectName),
	)
	if err := os.WriteFile(engineFilePath, []byte(replacedContent), 0o644); err != nil {
		return fmt.Errorf("failed to write project file: %w", err)
	}
	return nil
}

// adaptGoMod creates a missing module scaffold outside a local SPX repository.
func (cmd *CmdTool) adaptGoMod() error {
	if cmd.findSpxRoot() != "" {
		return nil
	}

	return cmd.createDefaultGoMod(cmd.TargetDir, false)
}

// createDefaultGoMod writes the default go.mod.
func (cmd *CmdTool) createDefaultGoMod(dir string, forceWrite bool) error {
	goModPath := filepath.Join(dir, "go.mod")
	if !forceWrite {
		info, err := os.Stat(goModPath)
		if err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("project module %q is not a regular file", goModPath)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	return os.WriteFile(goModPath, []byte(cmd.GoModTemplate), 0o644)
}

func (cmd *CmdTool) shouldRunGoModTidy() bool {
	return cmd.findSpxRoot() == "" || builderai.HasDescription(cmd.TargetDir)
}

func (cmd *CmdTool) shouldSkipProjectImport() bool {
	spec, _ := findCommand(cmd.Args.CmdName)
	return cmd.RuntimeMode || spec.setup == webExportSetup ||
		spec.build == tinyGoBuild || spec.build == wasmBuild
}

func (cmd *CmdTool) runProjectImport() error {
	logInfof("Importing project resources")
	execCmd, ctx, timeout, cancel := cmd.newProjectImportCommand()
	defer cancel()
	execCmd.Dir = cmd.ProjectDir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	if err := execCmd.Run(); err != nil {
		if timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("godot import timed out after %s; override with %s: %w", timeout, projectImportTimeoutEnvVar, err)
		}
		args := execCmd.Args
		if len(args) > 0 {
			args = args[1:]
		}
		return fmt.Errorf(
			"godot import failed (cmd=%q dir=%q args=%q): %w",
			cmd.CmdPath,
			cmd.ProjectDir,
			args,
			err,
		)
	}
	return nil
}

func (cmd *CmdTool) newProjectImportCommand() (*exec.Cmd, context.Context, time.Duration, context.CancelFunc) {
	timeout := cmd.projectImportTimeout()
	// Avoid loading project GDExtensions during resource import.
	args := []string{"--headless", "--path", cmd.ProjectDir, "--import", "--recovery-mode"}
	if timeout <= 0 {
		return exec.Command(cmd.CmdPath, args...), context.Background(), 0, func() {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return exec.CommandContext(ctx, cmd.CmdPath, args...), ctx, timeout, cancel
}

func (cmd *CmdTool) projectImportCachePath() string {
	return path.Join(cmd.ProjectDir, ".godot", "uid_cache.bin")
}

func (cmd *CmdTool) projectImportTimeout() time.Duration {
	timeoutValue := strings.TrimSpace(os.Getenv(projectImportTimeoutEnvVar))
	if timeoutValue == "" {
		return defaultProjectImportTimeout
	}
	if timeoutValue == "0" {
		return 0
	}

	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil {
		logWarnf("Invalid %s=%q, using default %s", projectImportTimeoutEnvVar, timeoutValue, defaultProjectImportTimeout)
		return defaultProjectImportTimeout
	}
	if timeout < 0 {
		logWarnf("Ignoring negative %s=%q, using default %s", projectImportTimeoutEnvVar, timeoutValue, defaultProjectImportTimeout)
		return defaultProjectImportTimeout
	}
	return timeout
}

// findSpxRoot finds a local spx repo.
func (cmd *CmdTool) findSpxRoot() string {
	return findSpxRootFrom(cmd.TargetDir)
}

// getIspxWebDir returns the installed web runtime.
func (cmd *CmdTool) getIspxWebDir() (string, error) {
	ispxWebDir := path.Join(cmd.GoBinPath, "ispx")
	if _, err := os.Stat(ispxWebDir); os.IsNotExist(err) {
		return "", fmt.Errorf("ispx web runtime not found at %s; "+
			"run 'make setup-web MODE=normal' from the SPX repository to install it", ispxWebDir)
	}
	return ispxWebDir, nil
}

func resolveTargetPlatform() (string, string, error) {
	goos := runtime.GOOS
	if value := os.Getenv("GOOS"); value != "" {
		goos = value
	}

	goarch := runtime.GOARCH
	if value := os.Getenv("GOARCH"); value != "" {
		goarch = value
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", errors.New("gdx requires an amd64, or an arm64 system")
	}

	return goos, goarch, nil
}

func findSpxRootFrom(startDir string) string {
	if startDir == "" {
		return ""
	}
	currentDir := filepath.Clean(startDir)
	for {
		if isLocalSpxRepoRoot(currentDir) {
			return currentDir
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return ""
}

func isLocalSpxRepoRoot(dir string) bool {
	goModPath := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	if !hasGoModuleDeclaration(string(content), "github.com/goplus/spx/v4") {
		return false
	}
	return util.IsFileExist(filepath.Join(dir, "cmd", "spx", "install.sh"))
}

func hasGoModuleDeclaration(content, modulePath string) bool {
	moduleDecl := "module " + modulePath
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		return line == moduleDecl
	}
	return false
}

func resolveAppPath(gobinDir, tag, version string) (string, string, error) {
	binPostfix := executableSuffix(runtime.GOOS)

	tagName := tag + version
	dstFileName := tagName + binPostfix
	gdx, err := exec.LookPath(dstFileName)
	if err == nil {
		if _, err := exec.Command(gdx, "--version").CombinedOutput(); err == nil {
			return binPostfix, gdx, nil
		}
	}

	cmdPath := path.Join(gobinDir, dstFileName)
	info, err := os.Stat(cmdPath)
	if err != nil && !os.IsNotExist(err) {
		return binPostfix, "", err
	} else if err == nil && info.Mode()&0o111 == 0 {
		if err := os.Chmod(cmdPath, 0o755); err != nil {
			return binPostfix, cmdPath, err
		}
	}
	return binPostfix, cmdPath, nil
}
