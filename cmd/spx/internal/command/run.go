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
	"bytes"
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

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
	"github.com/goplus/spx/v2/cmd/spx/internal/runtimeasset"
)

var prepareEmbeddedRuntimeAssets = runtimeasset.Prepare

// Keep in sync with cmd/spxrunner/runner.GDExtensionTemplate.
const defaultRuntimeGDExtensionTemplate = `[configuration]

entry_symbol = "gdspx_init"
compatibility_minimum = 4.1

[libraries]

macos.debug.x86_64 = "gdspx-darwin-amd64.dylib"
macos.release.x86_64 = "gdspx-darwin-amd64.dylib"
macos.debug.arm64 = "gdspx-darwin-arm64.dylib"
macos.release.arm64 = "gdspx-darwin-arm64.dylib"
windows.debug.x86_64 = "gdspx-windows-amd64.dll"
windows.release.x86_64 = "gdspx-windows-amd64.dll"
linux.debug.x86_64 = "gdspx-linux-amd64.so"
linux.release.x86_64 = "gdspx-linux-amd64.so"
`

type projConf struct {
	Robots []string `json:"robots"`
}

func (cmd *CmdTool) Run(arg string) (err error) {
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, arg)
}

func (cmd *CmdTool) RunPackMode(pargs ...string) error {
	dllPath := path.Join(cmd.RuntimeTempDir, filepath.Base(cmd.LibPath))
	util.CopyFile(cmd.LibPath, dllPath)
	extensionPath := path.Join(cmd.RuntimeTempDir, "runtime.gdextension")
	util.CopyFile(path.Join(cmd.ProjectDir, "runtime.gdextension.txt"), extensionPath)

	args := cmd.buildRuntimeArgs(pargs, cmd.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(cmd.RuntimeTempDir, cmd.RuntimeCmdPath, args...)
}

func (cmd *CmdTool) RunWeb() error {
	return runWebCommandWithSetup(func() error {
		return cmd.installRepoWebRuntime(webNormalMode)
	}, cmd.ExportWeb, cmd.runWebServer)
}

func (cmd *CmdTool) RunWebWorker() error {
	return runWebCommandWithSetup(func() error {
		return cmd.installRepoWebRuntime(webWorkerMode)
	}, cmd.ExportWebWorker, cmd.runWebServer)
}

func (cmd *CmdTool) StopWeb() (err error) {
	pidBytes, readErr := os.ReadFile(cmd.webServerPIDPath())
	if os.IsNotExist(readErr) {
		return cmd.stopOrphanedWebServerByPort()
	}
	if readErr != nil {
		return fmt.Errorf("failed to read web server pid: %w", readErr)
	}

	pid, convErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if convErr != nil {
		_ = os.Remove(cmd.webServerPIDPath())
		return cmd.stopOrphanedWebServerByPort()
	}

	if cmd.killWebServerProcess(pid) {
		_ = os.Remove(cmd.webServerPIDPath())
		return nil
	}

	_ = os.Remove(cmd.webServerPIDPath())
	return cmd.stopOrphanedWebServerByPort()
}

func (cmd *CmdTool) RunPureEngine(pargs ...string) error {
	rawdir, _ := os.Getwd()
	os.Chdir(cmd.GoDir)

	binaryName := "main"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	envVars := []string{"CGO_ENABLED=0"}
	if cmd.Args.Tags != nil && *cmd.Args.Tags != "" {
		err := util.RunGolang(envVars, "build", "-tags="+*cmd.Args.Tags, "-o", binaryName)
		if err != nil {
			os.Chdir(rawdir)
			return fmt.Errorf("failed to build Go binary: %w", err)
		}
	} else {
		err := util.RunGolang(envVars, "build", "-o", binaryName)
		if err != nil {
			os.Chdir(rawdir)
			return fmt.Errorf("failed to build Go binary: %w", err)
		}
	}

	binaryPath := filepath.Join(cmd.GoDir, binaryName)
	os.Chdir(rawdir)
	return util.RunCommandInDir(cmd.TargetDir, binaryPath, pargs...)
}

func (cmd *CmdTool) RunWithAiMode(pargs ...string) error {
	return cmd.RunPackMode(pargs...)
}

// RunInterpreted runs the project with a prebuilt runtime.
func (cmd *CmdTool) RunInterpreted(pargs ...string) error {
	runtimeName := "gdspxrt" + cmd.Version + cmd.BinPostfix
	GOOS := runtime.GOOS
	GOARCH := runtime.GOARCH
	var libExt string
	switch GOOS {
	case "windows":
		libExt = ".dll"
	case "darwin":
		libExt = ".dylib"
	default:
		libExt = ".so"
	}
	libName := fmt.Sprintf("gdspx-%s-%s%s", GOOS, GOARCH, libExt)
	packName := runtimePackFileName(runtimeName)

	runtimePath, libPath, err := cmd.resolveInterpretedRuntimeAssets(runtimeName, packName, libName)
	if err != nil {
		return err
	}
	cmd.RuntimeCmdPath = runtimePath

	extensionPath, err := cmd.prepareInterpretedRuntimeDir(libPath)
	if err != nil {
		return err
	}

	args := cmd.buildRuntimeArgs(pargs, cmd.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(cmd.RuntimeTempDir, runtimePath, args...)
}

// buildRuntimeArgs builds gdspxrt args.
func (cmd *CmdTool) buildRuntimeArgs(inputArgs []string, tempDir, extPath string, extraArgs ...string) []string {
	args := []string{}
	for i := 0; i < len(inputArgs); i++ {
		if inputArgs[i] == "--path" {
			i++
			continue
		}
		args = append(args, inputArgs[i])
	}
	args = append(args, "--path", tempDir)
	args = append(args, "--gdextpath", extPath)
	args = append(args, extraArgs...)
	args = append(args, "--no-header")
	return args
}

func (cmd *CmdTool) runtimeSearchDirs() []string {
	var dirs []string
	appendDir := func(dir string) {
		if dir == "" {
			return
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}
		for _, existing := range dirs {
			if existing == absDir {
				return
			}
		}
		dirs = append(dirs, absDir)
	}

	appendDir(cmd.GoBinPath)

	exePath, err := os.Executable()
	if err == nil {
		if realExePath, realErr := filepath.EvalSymlinks(exePath); realErr == nil {
			exePath = realExePath
		}
		appendDir(filepath.Dir(exePath))
	}

	return dirs
}

func (cmd *CmdTool) findRuntimeAsset(name string) (string, error) {
	searchDirs := cmd.runtimeSearchDirs()
	if len(searchDirs) == 0 {
		return "", fmt.Errorf("no runtime search directories configured")
	}

	candidates := make([]string, 0, len(searchDirs))
	for _, dir := range searchDirs {
		candidate := filepath.Join(dir, name)
		candidates = append(candidates, candidate)
		if util.IsFileExist(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("searched %s", strings.Join(candidates, ", "))
}

func (cmd *CmdTool) resolveInterpretedRuntimeAssets(runtimeName, packName, libName string) (runtimePath, libPath string, err error) {
	embeddedDir, ok, err := prepareEmbeddedRuntimeAssets(cmd.Version, runtimeName, packName, libName)
	if err != nil {
		return "", "", fmt.Errorf("prepare embedded runtime assets: %w", err)
	}
	if ok {
		return filepath.Join(embeddedDir, runtimeName), filepath.Join(embeddedDir, libName), nil
	}

	runtimePath, runtimeErr := cmd.findRuntimeAsset(runtimeName)
	libPath, libErr := cmd.findRuntimeAsset(libName)
	if runtimeErr == nil {
		packPath := runtimePackPath(runtimePath)
		if _, err := os.Stat(packPath); err == nil && libErr == nil {
			return runtimePath, libPath, nil
		} else if os.IsNotExist(err) {
			runtimeErr = fmt.Errorf("runtime pack %s not found next to %s", filepath.Base(packPath), runtimePath)
		} else if err != nil {
			runtimeErr = fmt.Errorf("runtime pack check failed for %s: %w", packPath, err)
		}
	}

	var reasons []string
	if runtimeErr != nil {
		reasons = append(reasons, runtimeErr.Error())
	}
	if libErr != nil {
		reasons = append(reasons, libErr.Error())
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "embedded runtime assets are not available in this spx binary")
	}
	return "", "", fmt.Errorf("interpreted runtime assets not available: %s", strings.Join(reasons, "; "))
}

func runtimePackFileName(runtimeName string) string {
	if strings.HasSuffix(runtimeName, ".exe") {
		return strings.TrimSuffix(runtimeName, ".exe") + ".pck"
	}
	return runtimeName + ".pck"
}

func runtimePackPath(runtimePath string) string {
	return filepath.Join(filepath.Dir(runtimePath), runtimePackFileName(filepath.Base(runtimePath)))
}

func (cmd *CmdTool) prepareInterpretedRuntimeDir(libPath string) (string, error) {
	if err := os.MkdirAll(cmd.RuntimeTempDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create runtime temp dir %s: %w", cmd.RuntimeTempDir, err)
	}

	// Place the shared library next to runtime.gdextension so Godot can resolve it
	// without depending on a pre-installed runtime.gdextension file.
	dstLibPath := filepath.Join(cmd.RuntimeTempDir, filepath.Base(libPath))
	if err := util.CopyFile(libPath, dstLibPath); err != nil {
		return "", fmt.Errorf("failed to copy shared library %s to %s: %w", libPath, dstLibPath, err)
	}

	extensionPath := filepath.Join(cmd.RuntimeTempDir, "runtime.gdextension")
	if err := os.WriteFile(extensionPath, []byte(defaultRuntimeGDExtensionTemplate), 0o644); err != nil {
		return "", fmt.Errorf("failed to write runtime.gdextension: %w", err)
	}
	return extensionPath, nil
}

// runWebCommand exports before serving.
func runWebCommand(exportFn func() error, serverFn func() error) error {
	return runWebCommandWithSetup(nil, exportFn, serverFn)
}

func runWebCommandWithSetup(setupFn func() error, exportFn func() error, serverFn func() error) error {
	if setupFn != nil {
		if err := setupFn(); err != nil {
			return err
		}
	}
	if err := exportFn(); err != nil {
		return err
	}
	return serverFn()
}

func (cmd *CmdTool) installRepoWebRuntime(mode string) error {
	if cmd.hasInstalledWebRuntimeAssets(mode) {
		return nil
	}
	spxRoot := cmd.findSpxRoot()
	if spxRoot == "" {
		return nil
	}
	scriptPath := filepath.Join(spxRoot, "cmd", "spx", "install.sh")
	if !util.IsFileExist(scriptPath) {
		return nil
	}
	return util.ExecCommand(util.CommandOptions{}, "bash", scriptPath, "--web")
}

func (cmd *CmdTool) hasInstalledWebRuntimeAssets(mode string) bool {
	requiredPaths := []string{
		filepath.Join(cmd.GoBinPath, "ispx"),
		filepath.Join(cmd.GoBinPath, "ispx.wasm"),
		filepath.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+"_web"+mode),
	}
	for _, requiredPath := range requiredPaths {
		if !util.IsFileExist(requiredPath) {
			return false
		}
	}
	return true
}

func (cmd *CmdTool) webServerPIDPath() string {
	baseDir := cmd.TargetAbsDir
	if baseDir == "" {
		baseDir = cmd.TargetDir
	}
	pidPath, _ := filepath.Abs(path.Join(baseDir, ".gdspx_web_server.pid"))
	return pidPath
}

func (cmd *CmdTool) runWebServer() error {
	port := cmd.ServerPort
	if err := cmd.StopWeb(); err != nil {
		return err
	}
	scriptPath := filepath.Join(cmd.ProjectDir, ".godot", "gdspx_web_server.py")
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	executeDir := filepath.Join(cmd.ProjectDir, ".builds/web")
	executeDir = strings.ReplaceAll(executeDir, "\\", "/")

	pythonCmd := "python"
	if _, err := exec.LookPath("python"); err != nil {
		if _, err := exec.LookPath("python3"); err != nil {
			return fmt.Errorf("neither python nor python3 command found in PATH")
		}
		pythonCmd = "python3"
	}

	execCmd := exec.Command(pythonCmd, scriptPath, "-r", executeDir, "-p", fmt.Sprint(port))
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("error starting server: %v", err)
	}
	if err := os.WriteFile(cmd.webServerPIDPath(), []byte(strconv.Itoa(execCmd.Process.Pid)), 0644); err != nil {
		_ = execCmd.Process.Kill()
		return fmt.Errorf("failed to record web server pid: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- execCmd.Wait()
	}()

	select {
	case err := <-done:
		_ = os.Remove(cmd.webServerPIDPath())
		if err != nil {
			return fmt.Errorf("web server exited early: %w", err)
		}
		return fmt.Errorf("web server exited unexpectedly without an error")
	case <-time.After(500 * time.Millisecond):
	}
	fmt.Printf("web server running at http://127.0.0.1:%d\n", port)
	return nil
}

func (cmd *CmdTool) stopOrphanedWebServerByPort() error {
	if cmd.ServerPort <= 0 {
		return nil
	}

	pids, err := cmd.listListeningPIDs(cmd.ServerPort)
	if err != nil {
		return nil
	}

	for _, pid := range pids {
		if cmd.killWebServerProcess(pid) {
			break
		}
	}
	return nil
}

func (cmd *CmdTool) listListeningPIDs(port int) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		return listListeningPIDsWindows(port)
	default:
		return listListeningPIDsUnix(port)
	}
}

func listListeningPIDsUnix(port int) ([]int, error) {
	if _, err := exec.LookPath("lsof"); err != nil {
		return nil, err
	}

	cmd := exec.Command("lsof", "-nP", "-t", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return parsePIDList(output), nil
}

func listListeningPIDsWindows(port int) ([]int, error) {
	cmd := exec.Command("netstat", "-ano", "-p", "tcp")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	portSuffix := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.HasSuffix(fields[1], portSuffix) {
			continue
		}
		if !strings.EqualFold(fields[3], "LISTENING") && !strings.EqualFold(fields[3], "LISTEN") {
			continue
		}
		pid, err := strconv.Atoi(fields[4])
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func parsePIDList(output []byte) []int {
	var pids []int
	for _, field := range bytes.Fields(output) {
		pid, err := strconv.Atoi(string(field))
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func (cmd *CmdTool) killWebServerProcess(pid int) bool {
	if pid <= 0 || !looksLikeGDSPXWebServerProcess(pid) {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = process.Kill()
	return true
}

func looksLikeGDSPXWebServerProcess(pid int) bool {
	switch runtime.GOOS {
	case "windows":
		return looksLikeGDSPXWebServerProcessWindows(pid)
	default:
		return looksLikeGDSPXWebServerProcessUnix(pid)
	}
}

func looksLikeGDSPXWebServerProcessUnix(pid int) bool {
	cmd := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "gdspx_web_server.py")
}

func looksLikeGDSPXWebServerProcessWindows(pid int) bool {
	commandLine, err := windowsProcessCommandLine(pid)
	if err != nil {
		return false
	}
	return looksLikeGDSPXWebServerCommandLine(commandLine)
}

func windowsProcessCommandLine(pid int) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsProcessCommandLineQuery(pid))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func windowsProcessCommandLineQuery(pid int) string {
	return fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId = %d\").CommandLine", pid)
}

func looksLikeGDSPXWebServerCommandLine(commandLine string) bool {
	return strings.Contains(strings.ToLower(commandLine), "gdspx_web_server.py")
}
