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
)

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
	return runWebCommand(cmd.ExportWeb, cmd.runWebServer)
}

func (cmd *CmdTool) RunWebWorker() error {
	return runWebCommand(cmd.ExportWebWorker, cmd.runWebServer)
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
	extensionPath := path.Join(cmd.GoBinPath, "runtime.gdextension")

	if _, err := os.Stat(extensionPath); os.IsNotExist(err) {
		return fmt.Errorf("runtime.gdextension not found at %s. Please run 'spx install' first", extensionPath)
	}

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
	libPath := path.Join(cmd.GoBinPath, libName)
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		return fmt.Errorf("shared library %s not found at %s. Please run 'make install' first", libName, cmd.GoBinPath)
	}

	args := cmd.buildRuntimeArgs(pargs, cmd.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(cmd.RuntimeTempDir, cmd.RuntimeCmdPath, args...)
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

// runWebCommand exports before serving.
func runWebCommand(exportFn func() error, serverFn func() error) error {
	if err := exportFn(); err != nil {
		return err
	}
	return serverFn()
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
