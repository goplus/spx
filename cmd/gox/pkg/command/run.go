package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

type projConf struct {
	Robots []string `json:"robots"`
}

const defaultWebServerPort = 8005

type webServerPIDRecord struct {
	PID          int    `json:"pid"`
	LocalPIDPath string `json:"localPidPath,omitempty"`
}

var (
	isPortAvailableFn    = isPortAvailable
	findListeningPIDsFn  = findListeningPIDs
	processCommandLineFn = processCommandLine
	killPIDFn            = killPID
	waitForPortFreeFn    = waitForPortFree
)

func (pself *CmdTool) Run(arg string) (err error) {
	return util.RunCommandInDir(pself.ProjectDir, pself.CmdPath, arg)
}

// buildRuntimeArgs builds the arguments for running gdspxrt.
// It filters out --path and adds the runtime-specific arguments.
func (pself *CmdTool) buildRuntimeArgs(inputArgs []string, tempDir, extPath string, extraArgs ...string) []string {
	args := []string{}
	for i := 0; i < len(inputArgs); i++ {
		if inputArgs[i] == "--path" {
			i++ // Skip the path value
			continue
		}
		args = append(args, inputArgs[i])
	}
	args = append(args, "--path", tempDir)
	args = append(args, "--gdextpath", extPath)
	args = append(args, extraArgs...)
	args = append(args, "--no-header") // disable engine's header output
	return args
}

func (pself *CmdTool) RunPackMode(pargs ...string) error {
	// copy libs
	dllPath := path.Join(pself.RuntimeTempDir, filepath.Base(pself.LibPath))
	util.CopyFile(pself.LibPath, dllPath)
	// copy configs
	extensionPath := path.Join(pself.RuntimeTempDir, "runtime.gdextension")              // copy runtime
	util.CopyFile(path.Join(pself.ProjectDir, "runtime.gdextension.txt"), extensionPath) // copy gdextension

	args := pself.buildRuntimeArgs(pargs, pself.RuntimeTempDir, extensionPath)
	return util.RunCommandInDir(pself.RuntimeTempDir, pself.RuntimeCmdPath, args...)
}

func (pself *CmdTool) RunWeb() error {
	return runWebCommand(pself.ExportWeb, pself.runWebServer)
}

func (pself *CmdTool) RunWebWorker() error {
	return runWebCommand(pself.ExportWebWorker, pself.runWebServer)
}

// Always re-export before serving so direct `spx runweb` matches `make run-web`
// and never reuses stale web artifacts from a previous invocation.
func runWebCommand(exportFn func() error, serverFn func() error) error {
	if err := exportFn(); err != nil {
		return err
	}
	return serverFn()
}

func (pself *CmdTool) webServerPIDPath() string {
	pidPath, _ := filepath.Abs(path.Join(pself.TargetDir, ".gdspx_web_server.pid"))
	return pidPath
}

func (pself *CmdTool) webServerPort() int {
	if pself.ServerPort != 0 {
		return pself.ServerPort
	}
	return defaultWebServerPort
}

func (pself *CmdTool) globalWebServerPIDPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("gdspx_web_server_%d.pid", pself.webServerPort()))
}

func (pself *CmdTool) webServerPIDPaths() []string {
	paths := []string{pself.webServerPIDPath()}
	globalPIDPath := pself.globalWebServerPIDPath()
	if globalPIDPath != paths[0] {
		paths = append(paths, globalPIDPath)
	}
	return paths
}

func (pself *CmdTool) writeWebServerPIDFiles(pid int) error {
	localPIDPath := pself.webServerPIDPath()
	pidBytes := []byte(strconv.Itoa(pid))
	if err := os.WriteFile(localPIDPath, pidBytes, 0644); err != nil {
		return fmt.Errorf("failed to record web server pid: %w", err)
	}

	recordBytes, err := json.Marshal(webServerPIDRecord{
		PID:          pid,
		LocalPIDPath: localPIDPath,
	})
	if err != nil {
		_ = os.Remove(localPIDPath)
		return fmt.Errorf("failed to marshal web server pid record: %w", err)
	}
	if err := os.WriteFile(pself.globalWebServerPIDPath(), recordBytes, 0644); err != nil {
		_ = os.Remove(localPIDPath)
		return fmt.Errorf("failed to record global web server pid: %w", err)
	}
	return nil
}

func (pself *CmdTool) cleanupWebServerPIDFiles(paths map[string]struct{}) {
	for _, pidPath := range pself.webServerPIDPaths() {
		paths[pidPath] = struct{}{}
	}
	for pidPath := range paths {
		if pidPath == "" {
			continue
		}
		_ = os.Remove(pidPath)
	}
}

func (pself *CmdTool) readWebServerPIDRecord(pidPath string) (webServerPIDRecord, error) {
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		return webServerPIDRecord{}, err
	}

	if pidPath == pself.globalWebServerPIDPath() {
		var record webServerPIDRecord
		if err := json.Unmarshal(pidBytes, &record); err == nil && record.PID > 0 {
			return record, nil
		}
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return webServerPIDRecord{}, err
	}
	return webServerPIDRecord{PID: pid}, nil
}

func (pself *CmdTool) runWebServer() error {
	port := pself.webServerPort()
	if err := pself.StopWeb(); err != nil {
		return err
	}
	scriptPath := filepath.Join(pself.ProjectDir, ".godot", "gdspx_web_server.py")
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	executeDir := filepath.Join(pself.ProjectDir, ".builds/web")
	executeDir = strings.ReplaceAll(executeDir, "\\", "/")

	// Check if python command is available, try python3 if not
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
	if err := pself.writeWebServerPIDFiles(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		pself.cleanupWebServerPIDFiles(map[string]struct{}{})
		if err != nil {
			return fmt.Errorf("web server exited early: %w", err)
		}
		return fmt.Errorf("web server exited unexpectedly without an error")
	// Wait briefly to detect immediate startup failures; if the server
	// is still running after this window, assume it started successfully.
	case <-time.After(500 * time.Millisecond):
	}
	fmt.Printf("Web server running at http://127.0.0.1:%d\n", port)
	return nil
}

func (pself *CmdTool) StopWeb() (err error) {
	stopped := make(map[int]struct{})
	cleanupPaths := make(map[string]struct{})
	for _, pidPath := range pself.webServerPIDPaths() {
		cleanupPaths[pidPath] = struct{}{}

		record, readErr := pself.readWebServerPIDRecord(pidPath)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			continue
		}
		if record.LocalPIDPath != "" {
			cleanupPaths[record.LocalPIDPath] = struct{}{}
		}
		if record.PID <= 0 {
			continue
		}
		if _, ok := stopped[record.PID]; ok {
			continue
		}
		stoppedServer, stopErr := stopRecordedWebServerPID(record.PID)
		if stopErr != nil {
			pself.cleanupWebServerPIDFiles(cleanupPaths)
			return stopErr
		}
		if stoppedServer {
			stopped[record.PID] = struct{}{}
		}
	}

	defer pself.cleanupWebServerPIDFiles(cleanupPaths)
	return pself.stopWebServerOnPort(stopped)
}

func stopRecordedWebServerPID(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	cmdline, err := processCommandLineFn(pid)
	if err != nil && runtime.GOOS != "windows" {
		return false, nil
	}
	if cmdline != "" && !isGdspxWebServerCommand(cmdline) {
		return false, nil
	}

	if err := killPIDFn(pid); err != nil {
		return false, err
	}
	return true, nil
}

func (pself *CmdTool) stopWebServerOnPort(stopped map[int]struct{}) error {
	port := pself.webServerPort()
	if port <= 0 {
		return nil
	}

	if len(stopped) > 0 {
		if err := waitForPortFreeFn(port, 1500*time.Millisecond); err == nil {
			return nil
		}
	}
	if isPortAvailableFn(port) {
		return nil
	}

	listeningPIDs, err := findListeningPIDsFn(port)
	if err == nil {
		for _, pid := range listeningPIDs {
			if _, ok := stopped[pid]; ok {
				continue
			}
			cmdline, cmdErr := processCommandLineFn(pid)
			if cmdErr != nil || !isGdspxWebServerCommand(cmdline) {
				continue
			}
			if killErr := killPIDFn(pid); killErr != nil {
				return killErr
			}
			stopped[pid] = struct{}{}
		}
		if len(stopped) > 0 {
			if err := waitForPortFreeFn(port, 2*time.Second); err == nil {
				return nil
			}
		}
	}

	if isPortAvailableFn(port) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("port %d is already in use and the existing gdspx web server could not be identified: %w", port, err)
	}
	return fmt.Errorf("port %d is already in use by another process", port)
}

func isGdspxWebServerCommand(cmdline string) bool {
	return strings.Contains(cmdline, "gdspx_web_server.py")
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func waitForPortFree(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isPortAvailableFn(port) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if isPortAvailableFn(port) {
		return nil
	}
	return fmt.Errorf("timed out waiting for port %d to become available", port)
}

func findListeningPIDs(port int) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		return nil, errors.New("listing listening pids is not supported on windows")
	default:
		return findListeningPIDsUnix(port)
	}
}

func findListeningPIDsUnix(port int) ([]int, error) {
	cmd := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-t")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	for _, field := range strings.Fields(string(output)) {
		pid, convErr := strconv.Atoi(strings.TrimSpace(field))
		if convErr != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func processCommandLine(pid int) (string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId = %d\").CommandLine", pid))
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	default:
		cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	}
}

func killPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func (pself *CmdTool) RunPureEngine(pargs ...string) error {
	// Build the Go binary first
	rawdir, _ := os.Getwd()
	os.Chdir(pself.GoDir)

	// Build the executable
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

	// Run the binary
	binaryPath := filepath.Join(pself.GoDir, binaryName)
	os.Chdir(rawdir)
	return util.RunCommandInDir(pself.TargetDir, binaryPath, pargs...)
}

func (pself *CmdTool) RunWithAiMode(pargs ...string) error {
	return pself.RunPackMode(pargs...)
}

// RunInterpreted runs the project in interpreted mode.
func (pself *CmdTool) RunInterpreted(pargs ...string) error {
	// Get gdextension path from GOPATH/bin
	extensionPath := path.Join(pself.GoBinPath, "runtime.gdextension")

	// Verify runtime.gdextension exists
	if _, err := os.Stat(extensionPath); os.IsNotExist(err) {
		return fmt.Errorf("runtime.gdextension not found at %s. Please run 'spx install' first", extensionPath)
	}

	// Verify the shared library exists
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

	// Build command arguments using common function
	args := pself.buildRuntimeArgs(pargs, pself.RuntimeTempDir, extensionPath)
	// Run the gdspxrt runtime
	return util.RunCommandInDir(pself.RuntimeTempDir, pself.RuntimeCmdPath, args...)
}
