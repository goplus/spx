package command

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

type projConf struct {
	Robots []string `json:"robots"`
}

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
	if !util.IsFileExist(filepath.Join(pself.ProjectDir, ".builds", "web", "game.zip")) {
		pself.ExportWeb()
	}
	return pself.runWebServer()
}

func (pself *CmdTool) RunWebWorker() error {
	if !util.IsFileExist(filepath.Join(pself.ProjectDir, ".builds", "web", "game.zip")) {
		pself.ExportWebWorker()
	}
	return pself.runWebServer()
}

func (pself *CmdTool) runWebServer() error {
	port := pself.ServerPort
	pself.StopWeb()
	scriptPath := filepath.Join(pself.ProjectDir, ".godot", "gdspx_web_server.py")
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	executeDir := filepath.Join(pself.ProjectDir, ".builds/web")
	executeDir = strings.ReplaceAll(executeDir, "\\", "/")
	println("web server running at http://127.0.0.1:" + fmt.Sprint(port))

	// 检查 python 命令是否可用，不可用则尝试 python3
	pythonCmd := "python"
	if _, err := exec.LookPath("python"); err != nil {
		// python 不可用，尝试 python3
		if _, err := exec.LookPath("python3"); err != nil {
			return fmt.Errorf("neither python nor python3 command found in PATH")
		}
		pythonCmd = "python3"
	}

	cmd := exec.Command(pythonCmd, scriptPath, "-r", executeDir, "-p", fmt.Sprint(port))
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting server: %v", err)
	}
	return nil
}

func (pself *CmdTool) StopWeb() (err error) {
	if runtime.GOOS == "windows" {
		content := "taskkill /F /IM python.exe\r\ntaskkill /F /IM pythonw.exe\r\n"
		tempFileName := "temp_kill.bat"
		os.WriteFile(tempFileName, []byte(content), 0644)
		cmd := exec.Command("cmd.exe", "/C", tempFileName)
		cmd.Run()
		os.Remove(tempFileName)
	} else {
		cmd := exec.Command("pkill", "-f", "gdx_web_server.py")
		cmd.Run()
	}
	return
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
