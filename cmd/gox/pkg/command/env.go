package command

import (
	"embed"
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/goplus/spx/cmd/gox/pkg/impl"
	"github.com/goplus/spx/cmd/gox/pkg/util"
)

const ENV_NAME = "gdspx"

// setupPaths sets up project paths based on command line arguments
func (cmd *CmdTool) setupPaths(dstRelDir string) error {
	// Set target and project directories
	var err error
	cmd.TargetDir, err = filepath.Abs(*cmd.Args.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}

	cmd.ProjectDir, err = filepath.Abs(path.Join(cmd.TargetDir, dstRelDir))
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}

	// Parse server port if server address is provided
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

// PrepareEnv prepares the environment for the command
func (pself *CmdTool) PrepareEnv(fsRelDir, dstDir string) {
	util.CopyDir(pself.ProjectFS, fsRelDir, dstDir, false)
	rawDir, _ := os.Getwd()
	os.Chdir(pself.GoDir)
	util.RunGolang(nil, "mod", "tidy")
	os.Chdir(rawDir)
}

// SetupEnv sets up the environment for the command
func (pself *CmdTool) SetupEnv(version string, fs embed.FS, fsRelDir string, targetDir string, projectRelPath string) (err error) {
	// Update the CmdTool struct fields
	pself.TargetDir = targetDir
	pself.ProjectFS = fs
	pself.Version = version
	pself.ProjectRelPath = projectRelPath

	var GOOS, GOARCH = runtime.GOOS, runtime.GOARCH
	if os.Getenv("GOOS") != "" {
		GOOS = os.Getenv("GOOS")
	}
	if os.Getenv("GOARCH") != "" {
		GOARCH = os.Getenv("GOARCH")
	}
	if GOARCH != "amd64" && GOARCH != "arm64" {
		return errors.New("gdx requires an amd64, or an arm64 system")
	}
	// Update the CmdTool struct fields
	pself.BinPostfix, pself.CmdPath, err = impl.CheckAndGetAppPath(ENV_NAME, pself.Version)
	if err != nil {
		return fmt.Errorf(ENV_NAME+"requires engine to be installed as a binary at $GOPATH/bin/: %w", err)
	}
	pself.ProjectDir, _ = filepath.Abs(path.Join(pself.TargetDir, pself.ProjectRelPath))
	pself.GoDir, _ = filepath.Abs(pself.ProjectDir + "/go")

	var libraryName = fmt.Sprintf(ENV_NAME+"-%v-%v", GOOS, GOARCH)
	switch GOOS {
	case "windows":
		libraryName += ".dll"
	case "darwin":
		libraryName += ".dylib"
	default:
		libraryName += ".so"
	}
	pself.LibPath, _ = filepath.Abs(path.Join(pself.ProjectDir, "lib", libraryName))

	pself.PrepareEnv(fsRelDir, pself.ProjectDir)
	if pself.ShouldReimport() {
		pself.Reimport()
	}
	return
}

// getWasmPath returns the path to the wasm file
func (pself *CmdTool) getWasmPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	targetPath := path.Join(gopath, "bin")
	filePath := path.Join(targetPath, "igdspx.wasm")
	return filePath
}

// SetupPC sets up the PC environment by running the initialization script
func (pself *CmdTool) SetupPC() error {
	// Get current working directory
	rawdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if the initialization script exists
	toolPath, err := filepath.Abs(path.Join(rawdir, "gdspx/tools/init.sh"))
	if err != nil {
		return fmt.Errorf("failed to resolve tool path: %w", err)
	}

	// Verify the script exists
	if _, err := os.Stat(toolPath); err != nil {
		return fmt.Errorf("initialization script not found at %s: %w", toolPath, err)
	}

	// Change to the gdspx directory and run the initialization script
	if err := os.Chdir("./gdspx"); err != nil {
		return fmt.Errorf("failed to change to gdspx directory: %w", err)
	}

	// Run the initialization script
	if err := util.RunCommandInDir(".", "./tools/init.sh", "-a"); err != nil {
		return fmt.Errorf("failed to run initialization script: %w", err)
	}

	// Return to the original directory
	if err := os.Chdir(rawdir); err != nil {
		return fmt.Errorf("failed to return to original directory: %w", err)
	}

	return nil
}

// SetupWeb sets up the web environment by building the WebAssembly module
func (pself *CmdTool) SetupWeb() error {
	// Get the path for the WebAssembly file
	filePath := pself.getWasmPath()

	// Get current working directory
	rawdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Change to the igox directory
	if err := os.Chdir("../igox"); err != nil {
		return fmt.Errorf("failed to change to igox directory: %w", err)
	}

	// Build the WebAssembly module
	envVars := []string{"GOOS=js", "GOARCH=wasm"}
	if err := util.RunGolang(envVars, "build", "-o", filePath); err != nil {
		return fmt.Errorf("failed to build WebAssembly module: %w", err)
	}

	// Return to the original directory
	if err := os.Chdir(rawdir); err != nil {
		return fmt.Errorf("failed to return to original directory: %w", err)
	}

	return nil
}

// CheckEnv verifies that the target directory is a valid project directory
func (pself *CmdTool) CheckEnv() error {
	dir, err := filepath.Abs(pself.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory path: %w", err)
	}

	exist := util.CheckFileExist(dir, pself.FileSuffix, false)
	if !exist {
		return fmt.Errorf("cannot find %s file, not a valid project directory", pself.FileSuffix)
	}
	return nil
}

func (pself *CmdTool) ShouldReimport() bool {
	return !util.IsFileExist(path.Join(pself.ProjectDir, ".godot/uid_cache.bin"))
}

func (pself *CmdTool) Reimport() {
	// Call BuildDll on self instead of using the global curCmd
	pself.BuildDll()
	fmt.Println(" ================= Importing ... ================= ")
	cmd := exec.Command(pself.CmdPath, "--import", "--headless")
	cmd.Dir = pself.ProjectDir
	cmd.Start()
	cmd.Wait()
}

// Clear removes the project directory and associated files
func (pself *CmdTool) Clear() error {
	// Remove the project directory
	if err := os.RemoveAll(pself.ProjectDir); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	// Remove the gitignore file
	gitignorePath := path.Join(pself.ProjectDir, "../.gitignore")
	if err := os.Remove(gitignorePath); err != nil && !os.IsNotExist(err) {
		// Only return an error if the file exists and couldn't be removed
		return fmt.Errorf("failed to remove gitignore file: %w", err)
	}

	return nil
}
