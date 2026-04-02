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

func (pself *CmdTool) Run(arg string) (err error) {
	return util.RunCommandInDir(pself.ProjectDir, pself.CmdPath, arg)
}

// buildRuntimeArgs builds gdspxrt args.
func (pself *CmdTool) buildRuntimeArgs(inputArgs []string, tempDir, extPath string, extraArgs ...string) []string {
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

func (pself *CmdTool) RunPackMode(pargs ...string) error {
	dllPath := path.Join(pself.RuntimeTempDir, filepath.Base(pself.LibPath))
	util.CopyFile(pself.LibPath, dllPath)
	extensionPath := path.Join(pself.RuntimeTempDir, "runtime.gdextension")
	util.CopyFile(path.Join(pself.ProjectDir, "runtime.gdextension.txt"), extensionPath)

	args := pself.buildRuntimeArgs(pargs, pself.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(pself.RuntimeTempDir, pself.RuntimeCmdPath, args...)
}

func (pself *CmdTool) RunWeb() error {
	return runWebCommand(pself.ExportWeb, pself.runWebServer)
}

func (pself *CmdTool) RunWebWorker() error {
	return runWebCommand(pself.ExportWebWorker, pself.runWebServer)
}

// runWebCommand exports before serving.
func runWebCommand(exportFn func() error, serverFn func() error) error {
	if err := exportFn(); err != nil {
		return err
	}
	return serverFn()
}

func (pself *CmdTool) webServerPIDPath() string {
	baseDir := pself.TargetAbsDir
	if baseDir == "" {
		baseDir = pself.TargetDir
	}
	pidPath, _ := filepath.Abs(path.Join(baseDir, ".gdspx_web_server.pid"))
	return pidPath
}

func (pself *CmdTool) runWebServer() error {
	port := pself.ServerPort
	if err := pself.StopWeb(); err != nil {
		return err
	}
	scriptPath := filepath.Join(pself.ProjectDir, ".godot", "gdspx_web_server.py")
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	executeDir := filepath.Join(pself.ProjectDir, ".builds/web")
	executeDir = strings.ReplaceAll(executeDir, "\\", "/")

	pythonCmd := "python"
	if _, err := exec.LookPath("python"); err != nil {
		if _, err := exec.LookPath("python3"); err != nil {
			return fmt.Errorf("neither python nor python3 command found in PATH")
		}
		pythonCmd = "python3"
	}

	cmd := exec.Command(pythonCmd, scriptPath, "-r", executeDir, "-p", fmt.Sprint(port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting server: %v", err)
	}
	if err := os.WriteFile(pself.webServerPIDPath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to record web server pid: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		_ = os.Remove(pself.webServerPIDPath())
		if err != nil {
			return fmt.Errorf("web server exited early: %w", err)
		}
		return fmt.Errorf("web server exited unexpectedly without an error")
	case <-time.After(500 * time.Millisecond):
	}
	fmt.Printf("Web server running at http://127.0.0.1:%d\n", port)
	return nil
}

func (pself *CmdTool) StopWeb() (err error) {
	pidBytes, readErr := os.ReadFile(pself.webServerPIDPath())
	if os.IsNotExist(readErr) {
		return pself.stopOrphanedWebServerByPort()
	}
	if readErr != nil {
		return fmt.Errorf("failed to read web server pid: %w", readErr)
	}

	pid, convErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if convErr != nil {
		_ = os.Remove(pself.webServerPIDPath())
		return pself.stopOrphanedWebServerByPort()
	}

	if pself.killWebServerProcess(pid) {
		_ = os.Remove(pself.webServerPIDPath())
		return nil
	}

	_ = os.Remove(pself.webServerPIDPath())
	return pself.stopOrphanedWebServerByPort()
}

func (pself *CmdTool) stopOrphanedWebServerByPort() error {
	if pself.ServerPort <= 0 {
		return nil
	}

	pids, err := pself.listListeningPIDs(pself.ServerPort)
	if err != nil {
		return nil
	}

	for _, pid := range pids {
		if pself.killWebServerProcess(pid) {
			break
		}
	}
	return nil
}

func (pself *CmdTool) listListeningPIDs(port int) ([]int, error) {
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

func (pself *CmdTool) killWebServerProcess(pid int) bool {
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

func (pself *CmdTool) RunPureEngine(pargs ...string) error {
	rawdir, _ := os.Getwd()
	os.Chdir(pself.GoDir)

	binaryName := "main"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	envVars := []string{"CGO_ENABLED=0"}
	if pself.Args.Tags != nil && *pself.Args.Tags != "" {
		err := util.RunGolang(envVars, "build", "-tags="+*pself.Args.Tags, "-o", binaryName)
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

	binaryPath := filepath.Join(pself.GoDir, binaryName)
	os.Chdir(rawdir)
	return util.RunCommandInDir(pself.TargetDir, binaryPath, pargs...)
}

func (pself *CmdTool) RunWithAiMode(pargs ...string) error {
	return pself.RunPackMode(pargs...)
}

// RunInterpreted runs the project with a prebuilt runtime.
func (pself *CmdTool) RunInterpreted(pargs ...string) error {
	extensionPath := path.Join(pself.GoBinPath, "runtime.gdextension")

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
	libPath := path.Join(pself.GoBinPath, libName)
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		return fmt.Errorf("shared library %s not found at %s. Please run 'make install' first", libName, pself.GoBinPath)
	}

	args := pself.buildRuntimeArgs(pargs, pself.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(pself.RuntimeTempDir, pself.RuntimeCmdPath, args...)
}
