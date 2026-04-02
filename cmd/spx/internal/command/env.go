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

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

var ENV_NAME = "gdspx"

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

	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		cmd.BinPostfix = ""
		if runtime.GOOS == "windows" {
			cmd.BinPostfix = ".exe"
		}
		cmd.CmdPath = ""
	} else {
		cmd.BinPostfix, cmd.CmdPath, err = resolveAppPath(cmd.GoBinPath, ENV_NAME, cmd.Version, cmd.CustomGoEnv)
		if err != nil {
			return fmt.Errorf(ENV_NAME+"requires engine to be installed as a binary at %s: %w", cmd.GoBinPath, err)
		}
	}

	cmd.ProjectDir, _ = filepath.Abs(path.Join(cmd.TargetDir, cmd.ProjectRelPath))
	cmd.GoDir, _ = filepath.Abs(cmd.ProjectDir + "/go")

	if cmd.Args.Tags == nil || !strings.Contains(*cmd.Args.Tags, "pure_engine") {
		cmd.RuntimeCmdPath = path.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)
	}
	cmd.RuntimeTempDir, _ = filepath.Abs(path.Join(cmd.TargetDir, ".temp"))
	os.Mkdir(cmd.RuntimeTempDir, 0755)

	var libraryName = fmt.Sprintf(ENV_NAME+"-%v-%v", GOOS, GOARCH)
	switch GOOS {
	case "windows":
		libraryName += ".dll"
	case "darwin":
		libraryName += ".dylib"
	default:
		libraryName += ".so"
	}
	cmd.LibPath, _ = filepath.Abs(path.Join(cmd.ProjectDir, "lib", libraryName))

	cmd.PrepareEnv(fsRelDir, cmd.ProjectDir)

	targetDir, _ := filepath.Abs(cmd.TargetDir)
	projectName := filepath.Base(targetDir)
	projectName = strings.ReplaceAll(projectName, "_", "")
	projectName = strings.ReplaceAll(projectName, " ", "")
	engineFilePath := path.Join(cmd.ProjectDir, "project.godot")
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

	if cmd.ShouldReimport() {
		cmd.Reimport()
	}
	return
}

// PrepareEnv syncs project files.
func (cmd *CmdTool) PrepareEnv(fsRelDir, dstDir string) {
	util.CopyDir(cmd.ProjectFS, fsRelDir, dstDir, false)

	tempFile, _ := filepath.Abs(path.Join(cmd.TargetDir, "xgo_autogen.go"))
	tmp := `
package main
import "github.com/goplus/spx/v2"
func main() {print(&spx.Game{})}
`
	os.WriteFile(tempFile, []byte(tmp), 0644)

	cmd.adaptGoMod()

	rawDir, _ := os.Getwd()
	os.Chdir(cmd.TargetDir)

	util.RunGolang(nil, "mod", "tidy")

	os.Remove(tempFile)

	os.Chdir(rawDir)
}

// ShouldReimport reports whether Godot reimport is needed.
func (cmd *CmdTool) ShouldReimport() bool {
	// TinyGo skips Godot import.
	if cmd.Args.CmdName == "buildtinygo" {
		return false
	}
	return !util.IsFileExist(path.Join(cmd.ProjectDir, ".godot/uid_cache.bin")) && !cmd.RuntimeMode
}

// Reimport refreshes Godot import data.
func (cmd *CmdTool) Reimport() {
	switch cmd.Args.CmdName {
	case "buildtinygo":
		return
	case "buildweb", "runweb", "exportweb":
		cmd.BuildWasm()
	default:
		cmd.BuildDll()
	}
	fmt.Println(" ================= Importing ... ================= ")
	execCmd := exec.Command(cmd.CmdPath, "--import", "--headless")
	execCmd.Dir = cmd.ProjectDir
	execCmd.Start()
	execCmd.Wait()
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

// setupPortableGoEnv configures a portable Go toolchain.
func (cmd *CmdTool) setupPortableGoEnv() error {
	goEnvDir, err := filepath.Abs(*cmd.Args.GoEnv)
	if err != nil {
		return fmt.Errorf("invalid goenv path: %w", err)
	}

	cmd.GoEnvDir = goEnvDir
	cmd.CustomGoEnv = true

	if _, err := os.Stat(goEnvDir); os.IsNotExist(err) {
		return fmt.Errorf("goenv directory does not exist: %s", goEnvDir)
	}

	cmd.GoRoot = path.Join(goEnvDir, "gotoolchain", "go")
	cmd.GoPath = filepath.Join(goEnvDir, "go")
	if _, err := os.Stat(cmd.GoRoot); os.IsNotExist(err) {
		return fmt.Errorf("portable Go toolchain not found at the expected path: %s\n\nThis is expected to be provided by the SPX release package. If you are setting this up manually, please ensure the Go toolchain is extracted to '%s/gotoolchain/go'.", cmd.GoRoot, goEnvDir)
	}
	cmd.GoBinPath, err = filepath.Abs(filepath.Join(cmd.GoPath, "bin"))
	if err != nil {
		return fmt.Errorf("failed to resolve Go bin path: %w", err)
	}

	if _, err := os.Stat(cmd.GoBinPath); os.IsNotExist(err) {
		return fmt.Errorf("Go bin directory not found: %s", cmd.GoBinPath)
	}

	goRootBinPath := filepath.Join(cmd.GoRoot, "bin")
	if _, err := os.Stat(goRootBinPath); os.IsNotExist(err) {
		return fmt.Errorf("Go bin directory not found: %s", goRootBinPath)
	}

	goCacheDir := filepath.Join(goEnvDir, ".cache", "build")
	goModCacheDir := filepath.Join(goEnvDir, ".cache", "mod")

	os.MkdirAll(goCacheDir, 0755)
	os.MkdirAll(goModCacheDir, 0755)

	os.Setenv("GOROOT", cmd.GoRoot)
	os.Setenv("GOPATH", cmd.GoPath)
	os.Setenv("GOTOOLCHAIN", "")
	os.Setenv("GOCACHE", goCacheDir)
	os.Setenv("GOMODCACHE", goModCacheDir)

	currentPath := os.Getenv("PATH")
	newPath := cmd.GoBinPath + string(os.PathListSeparator) + goRootBinPath + string(os.PathListSeparator) + currentPath
	os.Setenv("PATH", newPath)

	fmt.Printf("Using portable Go environment:\n")
	fmt.Printf("  GOROOT: %s\n", cmd.GoRoot)
	fmt.Printf("  GOPATH: %s\n", cmd.GoPath)
	fmt.Printf("  GoBinPath: %s (for gdspx, gdspxrt, etc.)\n", cmd.GoBinPath)
	fmt.Printf("  Go binary: %s\n", filepath.Join(goRootBinPath, "go"))
	fmt.Printf("  GOCACHE: %s\n", goCacheDir)
	fmt.Printf("  GOMODCACHE: %s\n", goModCacheDir)

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
	rootGoModPath, _ := filepath.Abs(path.Join(cmd.TargetDir, "go.mod"))
	if _, err := os.Stat(rootGoModPath); os.IsNotExist(err) {
		cmd.createDefaultGoMod(cmd.TargetDir, false)
	}

	absTargetDir, _ := filepath.Abs(cmd.TargetDir)
	spxPath := cmd.findSpxRoot(absTargetDir)

	if spxPath != "" {
		content, err := os.ReadFile(rootGoModPath)
		if err != nil {
			return
		}

		strContent := string(content)
		if !strings.Contains(strContent, "replace github.com/goplus/spx/v2") {
			relPath, err := filepath.Rel(absTargetDir, spxPath)
			if err != nil {
				return
			}

			replaceDir := fmt.Sprintf("\n\nreplace github.com/goplus/spx/v2 => %s\n", relPath)
			strContent += replaceDir

			os.WriteFile(rootGoModPath, []byte(strContent), 0644)
		}
	}
}

// createDefaultGoMod writes the default go.mod.
func (cmd *CmdTool) createDefaultGoMod(dir string, forceWrite bool) {
	gopModPath, _ := filepath.Abs(path.Join(dir, "go.mod"))
	if _, err := os.Stat(gopModPath); os.IsNotExist(err) || forceWrite {
		gopModContent := cmd.GoModTemplate
		os.WriteFile(gopModPath, []byte(gopModContent), 0644)
	}
}

// findSpxRoot finds a local spx repo.
func (cmd *CmdTool) findSpxRoot(startDir string) string {
	currentDir := filepath.Dir(startDir)
	for {
		gopModPath := path.Join(currentDir, "gop.mod")
		if _, err := os.Stat(gopModPath); err == nil {
			content, err := os.ReadFile(gopModPath)
			if err == nil && strings.Contains(string(content), "github.com/goplus/spx/v2") {
				return currentDir
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return ""
}

func resolveAppPath(gobinDir, tag, version string, customGoEnv bool) (string, string, error) {
	binPostfix := ""
	if runtime.GOOS == "windows" {
		binPostfix = ".exe"
	}

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
