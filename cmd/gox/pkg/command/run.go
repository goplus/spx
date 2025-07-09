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

func (pself *CmdTool) RunPackMode(pargs ...string) error {
	// copy libs
	dllPath := path.Join(pself.RuntimeTempDir, filepath.Base(pself.LibPath))
	util.CopyFile(pself.LibPath, dllPath)
	// copy configs
	extensionPath := path.Join(pself.RuntimeTempDir, "runtime.gdextension")              // copy runtime
	util.CopyFile(path.Join(pself.ProjectDir, "runtime.gdextension.txt"), extensionPath) // copy gdextension
	args := []string{}
	for i := 0; i < len(pargs); i++ {
		if pargs[i] == "--path" {
			i++
			continue
		}
		args = append(args, pargs[i])
	}
	args = append(args, "--path")
	args = append(args, pself.RuntimeTempDir)
	args = append(args, "--gdextpath")
	args = append(args, extensionPath)
	return util.RunCommandInDir(pself.RuntimeTempDir, pself.RuntimeCmdPath, args...)
}

func (pself *CmdTool) RunWebEditor() error {
	if !util.IsFileExist(filepath.Join(pself.ProjectDir, ".builds", "web", "engineres.zip")) {
		pself.ExportWebEditor()
	}

	return pself.runWebServer()
}

func (pself *CmdTool) RunWeb() error {
	if !util.IsFileExist(filepath.Join(pself.ProjectDir, ".builds", "web", "game.zip")) {
		pself.ExportWeb()
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
