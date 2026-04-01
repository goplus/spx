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

	"github.com/goplus/spx/v2/cmd/spx/pkg/impl"
	"github.com/goplus/spx/v2/cmd/spx/pkg/util"
)

var ENV_NAME = "gdspx"

// setupPaths sets up project paths based on command line arguments
func (cmd *CmdTool) setupPaths(dstRelDir string) error {
	// Set target and project directories
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

	// Step 1: Create temporary files first (before modifying go.mod)
	// This ensures import references exist when go mod tidy runs
	tempFile, _ := filepath.Abs(path.Join(pself.TargetDir, "xgo_autogen.go"))
	tmp := `
package main
import "github.com/goplus/spx/v2"
func main() {print(&spx.Game{})}
`
	os.WriteFile(tempFile, []byte(tmp), 0644)

	// Step 2: Handle go.mod file adaptively
	pself.adaptGoMod()

	// Step 4: Change to target directory and ensure dependencies are downloaded
	rawDir, _ := os.Getwd()
	os.Chdir(pself.TargetDir)

	// Step 5: Run go mod tidy to clean up dependencies
	util.RunGolang(nil, "mod", "tidy")

	// Step 6: Delete temporary files
	os.Remove(tempFile)

	os.Chdir(rawDir)
}

// adaptGoMod adapts the go.mod file for different directory structures
func (pself *CmdTool) adaptGoMod() {
	// Check if go.mod exists in project root
	rootGoModPath, _ := filepath.Abs(path.Join(pself.TargetDir, "go.mod"))
	if _, err := os.Stat(rootGoModPath); os.IsNotExist(err) {
		// No go.mod in root, create one
		pself.createDefaultGoMod(pself.TargetDir, false)
	}

	// Check if we need to add replace directive for local spx development
	absTargetDir, _ := filepath.Abs(pself.TargetDir)
	spxPath := pself.findSpxRoot(absTargetDir)

	if spxPath != "" {
		// We're in spx development environment, add replace directive
		content, err := os.ReadFile(rootGoModPath)
		if err != nil {
			return
		}

		strContent := string(content)
		// Check if replace directive already exists
		if !strings.Contains(strContent, "replace github.com/goplus/spx/v2") {
			// Calculate relative path from project to spx root
			relPath, err := filepath.Rel(absTargetDir, spxPath)
			if err != nil {
				return
			}

			// Add replace directive
			replaceDir := fmt.Sprintf("\n\nreplace github.com/goplus/spx/v2 => %s\n", relPath)
			strContent += replaceDir

			// Write back the modified content
			os.WriteFile(rootGoModPath, []byte(strContent), 0644)
		}
	}
}

// createDefaultGoMod ensures gop.mod exists if not in spx directory
func (pself *CmdTool) createDefaultGoMod(dir string, forceWrite bool) {
	// Not in spx directory, create gop.mod in target directory
	gopModPath, _ := filepath.Abs(path.Join(dir, "go.mod"))
	if _, err := os.Stat(gopModPath); os.IsNotExist(err) || forceWrite {
		gopModContent := pself.GoModTemplate
		os.WriteFile(gopModPath, []byte(gopModContent), 0644)
	}
}

// findSpxRoot finds the spx root directory by looking for gop.mod
func (pself *CmdTool) findSpxRoot(startDir string) string {
	currentDir := filepath.Dir(startDir)
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

	// Check if pure_engine tag is present
	if pself.Args.Tags != nil && strings.Contains(*pself.Args.Tags, "pure_engine") {
		// Skip engine detection for pure_engine mode
		pself.BinPostfix = ""
		if runtime.GOOS == "windows" {
			pself.BinPostfix = ".exe"
		}
		pself.CmdPath = "" // Set empty path since we don't need engine
	} else {
		// Update the CmdTool struct fields
		pself.BinPostfix, pself.CmdPath, err = impl.CheckAndGetAppPath(pself.GoBinPath, ENV_NAME, pself.Version, pself.CustomGoEnv)
		if err != nil {
			return fmt.Errorf(ENV_NAME+"requires engine to be installed as a binary at %s: %w", pself.GoBinPath, err)
		}
	}

	pself.ProjectDir, _ = filepath.Abs(path.Join(pself.TargetDir, pself.ProjectRelPath))
	pself.GoDir, _ = filepath.Abs(pself.ProjectDir + "/go")

	// setup runtime path - skip for pure_engine mode
	if pself.Args.Tags == nil || !strings.Contains(*pself.Args.Tags, "pure_engine") {
		pself.RuntimeCmdPath = path.Join(pself.GoBinPath, "gdspxrt"+pself.Version+pself.BinPostfix)
		pckName := pself.RuntimeCmdPath
		pckName = pckName[:len(pckName)-len(pself.BinPostfix)]
		pself.RuntimePckPath = pckName + ".pck"
	}
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

func (pself *CmdTool) getProjectWasmPath() string {
	return path.Join(pself.ProjectDir, ".builds", "web", "ispx.wasm")
}

// getWasmPaths returns the wasm file path and its optional brotli-compressed pair.
// When a command has just built a project-local wasm, prefer that artifact so
// `spx runweb` and `spx exportweb` use the freshly built binary instead of the
// preinstalled runtime copy under GOPATH/bin.
func (pself *CmdTool) getWasmPaths() (string, string) {
	projectWasmPath := pself.getProjectWasmPath()
	if util.IsFileExist(projectWasmPath) {
		projectWasmBrPath := projectWasmPath + ".br"
		if util.IsFileExist(projectWasmBrPath) {
			return projectWasmPath, projectWasmBrPath
		}
		return projectWasmPath, ""
	}

	wasmPath := path.Join(pself.GoBinPath, "ispx.wasm")
	wasmBrPath := wasmPath + ".br"
	if util.IsFileExist(wasmBrPath) {
		return wasmPath, wasmBrPath
	}
	return wasmPath, ""
}

// getIspxWebDir returns the path to the ispx web runtime directory.
func (pself *CmdTool) getIspxWebDir() (string, error) {
	ispxWebDir := path.Join(pself.GoBinPath, "ispx")
	if _, err := os.Stat(ispxWebDir); os.IsNotExist(err) {
		return "", fmt.Errorf("ispx web runtime not found at %s; "+
			"run 'cd cmd/spx && ./install.sh --web' to install", ispxWebDir)
	}
	return ispxWebDir, nil
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
	// TinyGo build doesn't need Godot import process
	if pself.Args.CmdName == "buildtinygo" {
		return false
	}
	return !util.IsFileExist(path.Join(pself.ProjectDir, ".godot/uid_cache.bin")) && !pself.RuntimeMode
}

func (pself *CmdTool) Reimport() {
	// Choose appropriate build method based on command type
	switch pself.Args.CmdName {
	case "buildtinygo":
		// TinyGo build doesn't need Godot import process
		return
	case "buildweb", "runweb", "exportweb":
		// Web-related commands use BuildWasm
		pself.BuildWasm()
	default:
		// Other commands use BuildDll
		pself.BuildDll()
	}
	fmt.Println(" ================= Importing ... ================= ")
	cmd := exec.Command(pself.CmdPath, "--import", "--headless")
	cmd.Dir = pself.ProjectDir
	cmd.Start()
	cmd.Wait()
}

// setupPortableGoEnv configures the portable Go environment
func (cmd *CmdTool) setupPortableGoEnv() error {
	goEnvDir, err := filepath.Abs(*cmd.Args.GoEnv)
	if err != nil {
		return fmt.Errorf("invalid goenv path: %w", err)
	}

	cmd.GoEnvDir = goEnvDir
	cmd.CustomGoEnv = true

	// Check if goenv directory exists
	if _, err := os.Stat(goEnvDir); os.IsNotExist(err) {
		return fmt.Errorf("goenv directory does not exist: %s", goEnvDir)
	}

	// Find matching Go toolchain
	cmd.GoRoot = path.Join(goEnvDir, "gotoolchain", "go")
	cmd.GoPath = filepath.Join(goEnvDir, "go")
	if _, err := os.Stat(cmd.GoRoot); os.IsNotExist(err) {
		return fmt.Errorf("portable Go toolchain not found at the expected path: %s\n\nThis is expected to be provided by the SPX release package. If you are setting this up manually, please ensure the Go toolchain is extracted to '%s/gotoolchain/go'.", cmd.GoRoot, goEnvDir)
	}
	// Set GoBinPath to point to portable Go environment's bin directory
	cmd.GoBinPath, err = filepath.Abs(filepath.Join(cmd.GoPath, "bin"))
	if err != nil {
		return fmt.Errorf("failed to resolve Go bin path: %w", err)
	}

	// Verify GOPATH/bin directory exists
	if _, err := os.Stat(cmd.GoBinPath); os.IsNotExist(err) {
		return fmt.Errorf("Go bin directory not found: %s", cmd.GoBinPath)
	}

	// Verify GOROOT/bin exists
	goRootBinPath := filepath.Join(cmd.GoRoot, "bin")
	if _, err := os.Stat(goRootBinPath); os.IsNotExist(err) {
		return fmt.Errorf("Go bin directory not found: %s", goRootBinPath)
	}

	// Setup cache directories under goenv/.cache
	goCacheDir := filepath.Join(goEnvDir, ".cache", "build")
	goModCacheDir := filepath.Join(goEnvDir, ".cache", "mod")

	// Create cache directories if they don't exist
	os.MkdirAll(goCacheDir, 0755)
	os.MkdirAll(goModCacheDir, 0755)

	// Set environment variables
	os.Setenv("GOROOT", cmd.GoRoot)
	os.Setenv("GOPATH", cmd.GoPath)
	os.Setenv("GOTOOLCHAIN", "")
	os.Setenv("GOCACHE", goCacheDir)
	os.Setenv("GOMODCACHE", goModCacheDir)

	// Update PATH: Add both GOPATH/bin (for gdspx, gdspxrt, etc.) and GOROOT/bin (for go compiler)
	currentPath := os.Getenv("PATH")
	newPath := cmd.GoBinPath + string(os.PathListSeparator) + goRootBinPath + string(os.PathListSeparator) + currentPath
	os.Setenv("PATH", newPath)

	// Log the configuration
	fmt.Printf("Using portable Go environment:\n")
	fmt.Printf("  GOROOT: %s\n", cmd.GoRoot)
	fmt.Printf("  GOPATH: %s\n", cmd.GoPath)
	fmt.Printf("  GoBinPath: %s (for gdspx, gdspxrt, etc.)\n", cmd.GoBinPath)
	fmt.Printf("  Go binary: %s\n", filepath.Join(goRootBinPath, "go"))
	fmt.Printf("  GOCACHE: %s\n", goCacheDir)
	fmt.Printf("  GOMODCACHE: %s\n", goModCacheDir)

	// Verify Go works
	goExe := "go"
	if runtime.GOOS == "windows" {
		goExe = "go.exe"
	}
	goPath := filepath.Join(goRootBinPath, goExe)
	if _, err := os.Stat(goPath); os.IsNotExist(err) {
		return fmt.Errorf("Go executable not found: %s", goPath)
	}

	return nil
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
	// Remove go.mod
	if err := os.RemoveAll(path.Join(pself.TargetDir, "go.sum")); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	// Remove go.mod
	if err := os.RemoveAll(path.Join(pself.TargetDir, "xgo_autogen.go")); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	return nil
}
