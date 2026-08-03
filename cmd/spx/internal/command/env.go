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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	builderai "github.com/goplus/spx/v3/cmd/spx/internal/command/builderai"
	"github.com/goplus/spx/v3/cmd/spx/internal/util"
)

const envName = "gdspx"
const projectImportTimeoutEnvVar = "SPX_GODOT_IMPORT_TIMEOUT"
const defaultProjectImportTimeout = 10 * time.Minute

var projectNameReplacer = strings.NewReplacer("_", "", " ", "", "\"", "", "\n", "", "\r", "")
var spxModuleReplaceLinePattern = regexp.MustCompile(`^(\s*)(replace\s+)?github\.com/goplus/spx/v3\s*=>\s*(\S+)(\s*//.*)?$`)

type envVar struct {
	key   string
	value string
}

type portableGoPaths struct {
	goRootBinPath string
	goBinaryPath  string
	goCacheDir    string
	goModCacheDir string
}

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
func (cmd *CmdTool) SetupEnv(version string, fs embed.FS, fsRelDir string, projectRelPath string) (err error) {
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

	cmd.PrepareEnv(fsRelDir, cmd.ProjectDir)

	if err := cmd.updateProjectName(); err != nil {
		return err
	}

	if cmd.ShouldReimport() {
		if err := cmd.Reimport(); err != nil {
			return err
		}
	}
	return
}

// PrepareEnv syncs project files.
func (cmd *CmdTool) PrepareEnv(fsRelDir, dstDir string) {
	util.CopyDir(cmd.ProjectFS, fsRelDir, dstDir, true)

	tempFile, _ := filepath.Abs(path.Join(cmd.TargetDir, "xgo_autogen.go"))
	tmp := `
package main
import "github.com/goplus/spx/v3"
func main() {print(&spx.Game{})}
`
	os.WriteFile(tempFile, []byte(tmp), 0644)

	cmd.adaptGoMod()

	rawDir, _ := os.Getwd()
	os.Chdir(cmd.TargetDir)

	if cmd.shouldRunGoModTidy() {
		util.RunGolang(nil, "mod", "tidy")
	}

	os.Remove(tempFile)

	os.Chdir(rawDir)
}

func (cmd *CmdTool) shouldRunGoModTidy() bool {
	return cmd.findSpxRoot() == "" || builderai.HasDescription(cmd.targetRootDir())
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
	if err := cmd.buildProjectImportArtifacts(); err != nil {
		return err
	}
	return cmd.runProjectImport()
}

func (cmd *CmdTool) shouldSkipProjectImport() bool {
	if cmd.RuntimeMode || cmd.usesPureEngine() {
		return true
	}
	switch cmd.Args.CmdName {
	case "buildtinygo", "buildweb":
		return true
	default:
		return false
	}
}

func shouldBuildWasmForProjectImport(cmdName string) bool {
	switch cmdName {
	case "runweb", "exportweb":
		return true
	default:
		return false
	}
}

func (cmd *CmdTool) projectImportCachePath() string {
	return path.Join(cmd.ProjectDir, ".godot", "uid_cache.bin")
}

func (cmd *CmdTool) buildProjectImportArtifacts() error {
	switch {
	case cmd.shouldSkipProjectImport():
		return nil
	case shouldBuildWasmForProjectImport(cmd.Args.CmdName):
		return cmd.BuildWasm()
	default:
		return cmd.BuildDll()
	}
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

// Clear removes generated project files.
func (cmd *CmdTool) Clear() error {
	if err := os.RemoveAll(cmd.ProjectDir); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	if err := os.RemoveAll(path.Join(cmd.TargetDir, ".temp")); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}
	if err := os.RemoveAll(path.Join(cmd.TargetDir, "go.sum")); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	if err := os.RemoveAll(path.Join(cmd.TargetDir, "xgo_autogen.go")); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
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

func (cmd *CmdTool) setupEnginePaths() error {
	if cmd.usesPureEngine() {
		cmd.BinPostfix = executableSuffix(runtime.GOOS)
		cmd.CmdPath = ""
		return nil
	}

	binPostfix, cmdPath, err := resolveAppPath(cmd.GoBinPath, envName, cmd.Version, cmd.CustomGoEnv)
	if err != nil {
		return fmt.Errorf("%s requires engine to be installed as a binary at %s: %w", envName, cmd.GoBinPath, err)
	}
	cmd.BinPostfix = binPostfix
	cmd.CmdPath = cmdPath
	return nil
}

func (cmd *CmdTool) setupProjectPaths(goos, goarch string) error {
	projectDir, err := filepath.Abs(path.Join(cmd.TargetDir, cmd.ProjectRelPath))
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}
	cmd.ProjectDir = projectDir

	goDir, err := filepath.Abs(filepath.Join(projectDir, "go"))
	if err != nil {
		return fmt.Errorf("failed to resolve Go directory: %w", err)
	}
	cmd.GoDir = goDir

	if cmd.usesPureEngine() {
		cmd.RuntimeCmdPath = ""
	} else {
		cmd.RuntimeCmdPath = path.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)
	}

	runtimeTempDir, err := filepath.Abs(path.Join(cmd.TargetDir, ".temp"))
	if err != nil {
		return fmt.Errorf("failed to resolve temp directory: %w", err)
	}
	cmd.RuntimeTempDir = runtimeTempDir
	if err := os.MkdirAll(runtimeTempDir, 0o755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	libPath, err := filepath.Abs(path.Join(projectDir, "lib", libraryFileName(envName, goos, goarch)))
	if err != nil {
		return fmt.Errorf("failed to resolve library path: %w", err)
	}
	cmd.LibPath = libPath
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

func (cmd *CmdTool) usesPureEngine() bool {
	return cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine")
}

// setupPortableGoEnv configures a portable Go toolchain.
func (cmd *CmdTool) setupPortableGoEnv() error {
	goPaths, err := cmd.resolvePortableGoEnvPaths()
	if err != nil {
		return err
	}

	if err := cmd.applyPortableGoEnv(goPaths); err != nil {
		return err
	}

	printPortableGoEnv(cmd, goPaths)
	return nil
}

func (cmd *CmdTool) resolvePortableGoEnvPaths() (portableGoPaths, error) {
	goEnvDir, err := filepath.Abs(*cmd.Args.GoEnv)
	if err != nil {
		return portableGoPaths{}, fmt.Errorf("invalid goenv path: %w", err)
	}

	cmd.GoEnvDir = goEnvDir
	cmd.CustomGoEnv = true

	if _, err := os.Stat(goEnvDir); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("goenv directory does not exist: %s", goEnvDir)
	}

	cmd.GoRoot = path.Join(goEnvDir, "gotoolchain", "go")
	cmd.GoPath = filepath.Join(goEnvDir, "go")
	if _, err := os.Stat(cmd.GoRoot); os.IsNotExist(err) {
		//lint:ignore ST1005 This is a user-facing setup hint with complete sentences.
		return portableGoPaths{}, fmt.Errorf("portable Go toolchain not found at the expected path: %s\n\nThis is expected to be provided by the SPX release package. If you are setting this up manually, please ensure the Go toolchain is extracted to '%s/gotoolchain/go'.", cmd.GoRoot, goEnvDir)
	}

	cmd.GoBinPath, err = filepath.Abs(filepath.Join(cmd.GoPath, "bin"))
	if err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to resolve Go bin path: %w", err)
	}
	if _, err := os.Stat(cmd.GoBinPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("go bin directory not found: %s", cmd.GoBinPath)
	}

	goPaths := portableGoPaths{
		goRootBinPath: filepath.Join(cmd.GoRoot, "bin"),
		goCacheDir:    filepath.Join(goEnvDir, ".cache", "build"),
		goModCacheDir: filepath.Join(goEnvDir, ".cache", "mod"),
	}
	if _, err := os.Stat(goPaths.goRootBinPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("go bin directory not found: %s", goPaths.goRootBinPath)
	}

	goPaths.goBinaryPath = goBinaryPath(goPaths.goRootBinPath)
	if _, err := os.Stat(goPaths.goBinaryPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("go executable not found: %s", goPaths.goBinaryPath)
	}

	if err := os.MkdirAll(goPaths.goCacheDir, 0o755); err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to create Go build cache directory: %w", err)
	}
	if err := os.MkdirAll(goPaths.goModCacheDir, 0o755); err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to create Go module cache directory: %w", err)
	}

	return goPaths, nil
}

func (cmd *CmdTool) applyPortableGoEnv(goPaths portableGoPaths) error {
	pathEntries := []string{cmd.GoBinPath, goPaths.goRootBinPath}
	if pathValue := os.Getenv("PATH"); pathValue != "" {
		pathEntries = append(pathEntries, pathValue)
	}
	if err := setEnvVars(
		envVar{key: "GOROOT", value: cmd.GoRoot},
		envVar{key: "GOPATH", value: cmd.GoPath},
		envVar{key: "GOTOOLCHAIN", value: ""},
		envVar{key: "GOCACHE", value: goPaths.goCacheDir},
		envVar{key: "GOMODCACHE", value: goPaths.goModCacheDir},
		envVar{key: "PATH", value: strings.Join(pathEntries, string(os.PathListSeparator))},
	); err != nil {
		return err
	}
	return nil
}

func printPortableGoEnv(cmd *CmdTool, goPaths portableGoPaths) {
	const portableGoEnvFormat = "" +
		"using portable Go environment:\n" +
		"  GOROOT: %s\n" +
		"  GOPATH: %s\n" +
		"  GoBinPath: %s (for gdspx, gdspxrt, etc.)\n" +
		"  Go binary: %s\n" +
		"  GOCACHE: %s\n" +
		"  GOMODCACHE: %s\n"

	fmt.Printf(portableGoEnvFormat,
		cmd.GoRoot,
		cmd.GoPath,
		cmd.GoBinPath,
		goPaths.goBinaryPath,
		goPaths.goCacheDir,
		goPaths.goModCacheDir,
	)
}

func goBinaryPath(goRootBinPath string) string {
	return filepath.Join(goRootBinPath, goBinaryName(runtime.GOOS))
}

func setEnvVars(vars ...envVar) error {
	for _, env := range vars {
		if err := os.Setenv(env.key, env.value); err != nil {
			return fmt.Errorf("set %s: %w", env.key, err)
		}
	}
	return nil
}

// setupPaths resolves paths.
func (cmd *CmdTool) setupPaths(dstRelDir string) error {
	var err error
	cmd.TargetAbsDir, err = filepath.Abs(*cmd.Args.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}

	os.Chdir(cmd.TargetAbsDir)
	cmd.TargetDir = "."
	cmd.Args.Path = &cmd.TargetDir

	cmd.ProjectDir, err = filepath.Abs(path.Join(cmd.TargetDir, dstRelDir))
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
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

// adaptGoMod patches go.mod for local development.
func (cmd *CmdTool) adaptGoMod() {
	spxPath := cmd.findSpxRoot()
	if spxPath != "" {
		return
	}

	rootGoModPath, _ := filepath.Abs(filepath.Join(cmd.TargetDir, "go.mod"))
	if _, err := os.Stat(rootGoModPath); os.IsNotExist(err) {
		if err := cmd.createDefaultGoMod(cmd.TargetDir, false); err != nil {
			return
		}
	}

	absTargetDir := cmd.TargetAbsDir
	content, err := os.ReadFile(rootGoModPath)
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(absTargetDir, spxPath)
	if err != nil {
		return
	}

	strContent := ensureSpxModuleReplace(string(content), filepath.ToSlash(relPath))
	if strContent == string(content) {
		return
	}
	if err := os.WriteFile(rootGoModPath, []byte(strContent), 0644); err != nil {
		return
	}
}

// ensureSpxModuleReplace idempotently upserts the local spx replace directive,
// repairing stale single-line or block entries while preserving newline style.
func ensureSpxModuleReplace(content, relPath string) string {
	const replaceLine = "github.com/goplus/spx/v3 => "

	wantLine := replaceLine + relPath
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		hasCR := strings.HasSuffix(line, "\r")
		trimmed := strings.TrimSuffix(line, "\r")
		match := spxModuleReplaceLinePattern.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}

		if match[3] == relPath {
			return content
		}

		replacePrefix := match[1]
		if match[2] != "" {
			replacePrefix += "replace "
		}

		lines[i] = replacePrefix + wantLine + match[4]
		if hasCR {
			lines[i] += "\r"
		}
		return strings.Join(lines, "\n")
	}

	return appendSpxModuleReplace(content, "replace "+wantLine)
}

func appendSpxModuleReplace(content, replaceLine string) string {
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	switch {
	case content == "":
		return replaceLine + lineEnding
	case strings.HasSuffix(content, lineEnding+lineEnding):
		return content + replaceLine + lineEnding
	case strings.HasSuffix(content, lineEnding):
		return content + lineEnding + replaceLine + lineEnding
	default:
		return content + lineEnding + lineEnding + replaceLine + lineEnding
	}
}

// createDefaultGoMod writes the default go.mod.
func (cmd *CmdTool) createDefaultGoMod(dir string, forceWrite bool) error {
	goModPath, _ := filepath.Abs(filepath.Join(dir, "go.mod"))
	if _, err := os.Stat(goModPath); os.IsNotExist(err) || forceWrite {
		goModContent := cmd.GoModTemplate
		if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
			return err
		}
	}
	return nil
}

// findSpxRoot finds a local spx repo.
func (cmd *CmdTool) findSpxRoot() string {
	return findSpxRootFrom(cmd.targetRootDir())
}

func (cmd *CmdTool) targetRootDir() string {
	if cmd.TargetAbsDir != "" {
		return cmd.TargetAbsDir
	}
	if cmd.TargetDir != "" {
		absTargetDir, err := filepath.Abs(cmd.TargetDir)
		if err == nil {
			return absTargetDir
		}
	}
	if cmd.ProjectDir != "" {
		projectDir, err := filepath.Abs(cmd.ProjectDir)
		if err == nil {
			return filepath.Dir(projectDir)
		}
	}
	return ""
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
	if !hasGoModuleDeclaration(string(content), "github.com/goplus/spx/v3") {
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

func resolveAppPath(gobinDir, tag, version string, customGoEnv bool) (string, string, error) {
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
	if os.IsNotExist(err) {
		if customGoEnv {
			return binPostfix, cmdPath, nil
		}
	} else if err != nil {
		return binPostfix, "", err
	} else if info.Mode()&0o111 == 0 {
		if err := os.Chmod(cmdPath, 0o755); err != nil {
			return binPostfix, cmdPath, err
		}
	}
	return binPostfix, cmdPath, nil
}

func (cmd *CmdTool) getProjectWasmPath() string {
	return path.Join(cmd.ProjectDir, ".builds", "web", "ispx.wasm")
}

// getWasmPaths prefers project-local wasm.
func (cmd *CmdTool) getWasmPaths() (string, string) {
	projectWasmPath := cmd.getProjectWasmPath()
	if util.IsFileExist(projectWasmPath) {
		projectWasmBrPath := projectWasmPath + ".br"
		if util.IsFileExist(projectWasmBrPath) {
			return projectWasmPath, projectWasmBrPath
		}
		return projectWasmPath, ""
	}

	wasmPath := path.Join(cmd.GoBinPath, "ispx.wasm")
	wasmBrPath := wasmPath + ".br"
	if util.IsFileExist(wasmBrPath) {
		return wasmPath, wasmBrPath
	}
	return wasmPath, ""
}

// getIspxWebDir returns the installed web runtime.
func (cmd *CmdTool) getIspxWebDir() (string, error) {
	ispxWebDir := path.Join(cmd.GoBinPath, "ispx")
	if _, err := os.Stat(ispxWebDir); os.IsNotExist(err) {
		return "", fmt.Errorf("ispx web runtime not found at %s; "+
			"run 'go run ./internal/cmd/buildctl tool install --web' to install", ispxWebDir)
	}
	return ispxWebDir, nil
}
