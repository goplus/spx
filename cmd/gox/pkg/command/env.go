package command

import (
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

	"github.com/goplus/spx/v2/cmd/gox/pkg/impl"
	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

var ENV_NAME = "gdspx"

// setupPaths sets up project paths based on command line arguments
func (cmd *CmdTool) setupPaths(dstRelDir string) error {
	// Set target and project directories
	var err error
	cmd.TargetDir, err = filepath.Abs(*cmd.Args.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}

	os.Chdir(cmd.TargetDir)
	cmd.TargetDir = "."
	cmd.Args.Path = &cmd.TargetDir

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

	// Handle go.mod file adaptively
	pself.adaptGoMod()

	rawDir, _ := os.Getwd()
	os.Chdir(pself.GoDir)
	util.RunGolang(nil, "mod", "tidy")

	os.Chdir(rawDir)
}

// adaptGoMod adapts the go.mod file for different directory structures
func (pself *CmdTool) adaptGoMod() {
	projDir := path.Join(pself.GoDir, "../")
	goModPath := path.Join(projDir, "go.mod")
	// Check if go.mod exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return
	}

	// Read go.mod content
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return
	}

	strContent := string(content)

	// Check if we're in a spx directory by looking for spx/gop.mod
	currentDir, _ := os.Getwd()
	spxPath := pself.findSpxRoot(currentDir)

	if spxPath != "" {
		// We found spx root, calculate relative path
		relPath, err := filepath.Rel(projDir, spxPath)
		if err != nil {
			relPath = "../../../" // fallback to original
		}

		// Update the replace directive
		oldPattern := `replace github.com/goplus/spx/v2 => ../../../`
		newReplace := fmt.Sprintf("replace github.com/goplus/spx/v2 => %s", relPath)
		replacedContent := strings.ReplaceAll(strContent, oldPattern, newReplace)

		// Write back the modified content
		os.WriteFile(goModPath, []byte(replacedContent), 0644)
	} else {
		// create default go mod file if not exist
		pself.createDefaultGoMod(pself.TargetDir, false)
		// copy root mod to project dir
		util.CopyFile(path.Join(pself.TargetDir, "go.mod"), path.Join(projDir, "go.mod"))
	}
}

// createDefaultGoMod ensures gop.mod exists if not in spx directory
func (pself *CmdTool) createDefaultGoMod(dir string, forceWrite bool) {
	// Not in spx directory, create gop.mod in target directory
	gopModPath := path.Join(dir, "go.mod")
	if _, err := os.Stat(gopModPath); os.IsNotExist(err) || forceWrite {
		gopModContent := pself.GoModTemplate
		os.WriteFile(gopModPath, []byte(gopModContent), 0644)
	}
}

// findSpxRoot finds the spx root directory by looking for gop.mod
func (pself *CmdTool) findSpxRoot(startDir string) string {
	currentDir := startDir
	for {
		gopModPath := path.Join(currentDir, "gop.mod")
		if _, err := os.Stat(gopModPath); err == nil {
			// Check if this gop.mod contains spx project definition
			content, err := os.ReadFile(gopModPath)
			if err == nil && strings.Contains(string(content), "github.com/goplus/spx/v2") {
				return currentDir
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root directory
			break
		}
		currentDir = parent
	}
	return ""
}

// SetupEnv sets up the environment for the command
func (pself *CmdTool) SetupEnv(version string, fs embed.FS, fsRelDir string, projectRelPath string) (err error) {
	// Update the CmdTool struct fields
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
	pself.BinPostfix, pself.CmdPath, err = impl.CheckAndGetAppPath(pself.GoBinPath, ENV_NAME, pself.Version)
	if err != nil {
		return fmt.Errorf(ENV_NAME+"requires engine to be installed as a binary at %s: %w", pself.GoBinPath, err)
	}
	pself.ProjectDir, _ = filepath.Abs(path.Join(pself.TargetDir, pself.ProjectRelPath))
	pself.GoDir, _ = filepath.Abs(pself.ProjectDir + "/go")

	// setup runtime path
	pself.RuntimeCmdPath = path.Join(pself.GoBinPath, "gdspxrt"+pself.Version+pself.BinPostfix)
	pckName := pself.RuntimeCmdPath
	pckName = pckName[:len(pckName)-len(pself.BinPostfix)]
	pself.RuntimePckPath = pckName + ".pck"
	pself.RuntimeTempDir, _ = filepath.Abs(path.Join(pself.TargetDir, ".temp"))
	os.Mkdir(pself.RuntimeTempDir, 0755)

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

	// Update project name
	targetDir, _ := filepath.Abs(pself.TargetDir)
	projectName := filepath.Base(targetDir)
	projectName = strings.ReplaceAll(projectName, "_", "")
	projectName = strings.ReplaceAll(projectName, " ", "")
	engineFilePath := path.Join(pself.ProjectDir, "project.godot")
	content, err := os.ReadFile(engineFilePath)
	if err != nil {
		return fmt.Errorf("Failed to read project file: %v", err)
	}
	strContent := string(content)

	oldStr := `config/name="spx"`
	newStr := fmt.Sprintf(`config/name="%s"`, projectName)
	replacedContent := strings.ReplaceAll(strContent, oldStr, newStr)
	err = os.WriteFile(engineFilePath, []byte(replacedContent), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write project file: %v", err)
	}

	if pself.ShouldReimport() {
		pself.Reimport()
	}
	return
}

// getWasmPath returns the path to the wasm file
func (pself *CmdTool) getWasmPath() string {
	filePath := path.Join(pself.GoBinPath, "gdspx.wasm")
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
	return !util.IsFileExist(path.Join(pself.ProjectDir, ".godot/uid_cache.bin")) && !pself.RuntimeMode
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

	if err := os.RemoveAll(path.Join(pself.TargetDir, ".temp")); err != nil {
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
