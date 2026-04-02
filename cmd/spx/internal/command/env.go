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
var projectNameReplacer = strings.NewReplacer("_", "", " ", "", "\"", "", "\n", "", "\r", "")

type envVar struct {
	key   string
	value string
}

type portableGoPaths struct {
	goRootBinPath string
	goBinaryPath  string
	goCacheDir    string
	goModCacheDir string
}

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

	goos, goarch, err := resolveTargetPlatform()
	if err != nil {
		return err
	}

	if err := cmd.setupEnginePaths(); err != nil {
		return err
	}

	if err := cmd.setupProjectPaths(goos, goarch); err != nil {
		return err
	}

	cmd.PrepareEnv(fsRelDir, cmd.ProjectDir)

	if err := cmd.updateProjectName(); err != nil {
		return err
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

func resolveTargetPlatform() (string, string, error) {
	goos := runtime.GOOS
	if value := os.Getenv("GOOS"); value != "" {
		goos = value
	}

	goarch := runtime.GOARCH
	if value := os.Getenv("GOARCH"); value != "" {
		goarch = value
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", errors.New("gdx requires an amd64, or an arm64 system")
	}

	return goos, goarch, nil
}

func (cmd *CmdTool) setupEnginePaths() error {
	if cmd.usesPureEngine() {
		cmd.BinPostfix = ""
		if runtime.GOOS == "windows" {
			cmd.BinPostfix = ".exe"
		}
		cmd.CmdPath = ""
		return nil
	}

	binPostfix, cmdPath, err := resolveAppPath(cmd.GoBinPath, ENV_NAME, cmd.Version, cmd.CustomGoEnv)
	if err != nil {
		return fmt.Errorf("%s requires engine to be installed as a binary at %s: %w", ENV_NAME, cmd.GoBinPath, err)
	}
	cmd.BinPostfix = binPostfix
	cmd.CmdPath = cmdPath
	return nil
}

func (cmd *CmdTool) setupProjectPaths(goos, goarch string) error {
	projectDir, err := filepath.Abs(path.Join(cmd.TargetDir, cmd.ProjectRelPath))
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}
	cmd.ProjectDir = projectDir

	goDir, err := filepath.Abs(filepath.Join(projectDir, "go"))
	if err != nil {
		return fmt.Errorf("failed to resolve Go directory: %w", err)
	}
	cmd.GoDir = goDir

	if cmd.usesPureEngine() {
		cmd.RuntimeCmdPath = ""
	} else {
		cmd.RuntimeCmdPath = path.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)
	}

	runtimeTempDir, err := filepath.Abs(path.Join(cmd.TargetDir, ".temp"))
	if err != nil {
		return fmt.Errorf("failed to resolve temp directory: %w", err)
	}
	cmd.RuntimeTempDir = runtimeTempDir
	if err := os.MkdirAll(runtimeTempDir, 0o755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	libPath, err := filepath.Abs(path.Join(projectDir, "lib", libraryFileName(goos, goarch)))
	if err != nil {
		return fmt.Errorf("failed to resolve library path: %w", err)
	}
	cmd.LibPath = libPath
	return nil
}

func libraryFileName(goos, goarch string) string {
	libraryName := fmt.Sprintf("%s-%s-%s", ENV_NAME, goos, goarch)
	switch goos {
	case "windows":
		return libraryName + ".dll"
	case "darwin":
		return libraryName + ".dylib"
	default:
		return libraryName + ".so"
	}
}

func (cmd *CmdTool) updateProjectName() error {
	targetDir, err := filepath.Abs(cmd.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}
	projectName := projectNameReplacer.Replace(filepath.Base(targetDir))

	engineFilePath := path.Join(cmd.ProjectDir, "project.godot")
	content, err := os.ReadFile(engineFilePath)
	if err != nil {
		return fmt.Errorf("failed to read project file: %w", err)
	}

	replacedContent := strings.ReplaceAll(
		string(content),
		`config/name="spx"`,
		fmt.Sprintf(`config/name="%s"`, projectName),
	)
	if err := os.WriteFile(engineFilePath, []byte(replacedContent), 0o644); err != nil {
		return fmt.Errorf("failed to write project file: %w", err)
	}
	return nil
}

func (cmd *CmdTool) usesPureEngine() bool {
	return cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine")
}

// setupPortableGoEnv configures a portable Go toolchain.
func (cmd *CmdTool) setupPortableGoEnv() error {
	goPaths, err := cmd.resolvePortableGoEnvPaths()
	if err != nil {
		return err
	}

	if err := cmd.applyPortableGoEnv(goPaths); err != nil {
		return err
	}

	printPortableGoEnv(cmd, goPaths)
	return nil
}

func (cmd *CmdTool) resolvePortableGoEnvPaths() (portableGoPaths, error) {
	goEnvDir, err := filepath.Abs(*cmd.Args.GoEnv)
	if err != nil {
		return portableGoPaths{}, fmt.Errorf("invalid goenv path: %w", err)
	}

	cmd.GoEnvDir = goEnvDir
	cmd.CustomGoEnv = true

	if _, err := os.Stat(goEnvDir); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("goenv directory does not exist: %s", goEnvDir)
	}

	cmd.GoRoot = path.Join(goEnvDir, "gotoolchain", "go")
	cmd.GoPath = filepath.Join(goEnvDir, "go")
	if _, err := os.Stat(cmd.GoRoot); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("portable Go toolchain not found at the expected path: %s\n\nThis is expected to be provided by the SPX release package. If you are setting this up manually, please ensure the Go toolchain is extracted to '%s/gotoolchain/go'.", cmd.GoRoot, goEnvDir)
	}

	cmd.GoBinPath, err = filepath.Abs(filepath.Join(cmd.GoPath, "bin"))
	if err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to resolve Go bin path: %w", err)
	}
	if _, err := os.Stat(cmd.GoBinPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("Go bin directory not found: %s", cmd.GoBinPath)
	}

	goPaths := portableGoPaths{
		goRootBinPath: filepath.Join(cmd.GoRoot, "bin"),
		goCacheDir:    filepath.Join(goEnvDir, ".cache", "build"),
		goModCacheDir: filepath.Join(goEnvDir, ".cache", "mod"),
	}
	if _, err := os.Stat(goPaths.goRootBinPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("Go bin directory not found: %s", goPaths.goRootBinPath)
	}

	goPaths.goBinaryPath = goBinaryPath(goPaths.goRootBinPath)
	if _, err := os.Stat(goPaths.goBinaryPath); os.IsNotExist(err) {
		return portableGoPaths{}, fmt.Errorf("Go executable not found: %s", goPaths.goBinaryPath)
	}

	if err := os.MkdirAll(goPaths.goCacheDir, 0o755); err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to create Go build cache directory: %w", err)
	}
	if err := os.MkdirAll(goPaths.goModCacheDir, 0o755); err != nil {
		return portableGoPaths{}, fmt.Errorf("failed to create Go module cache directory: %w", err)
	}

	return goPaths, nil
}

func (cmd *CmdTool) applyPortableGoEnv(goPaths portableGoPaths) error {
	pathEntries := []string{cmd.GoBinPath, goPaths.goRootBinPath}
	if pathValue := os.Getenv("PATH"); pathValue != "" {
		pathEntries = append(pathEntries, pathValue)
	}
	if err := setEnvVars(
		envVar{key: "GOROOT", value: cmd.GoRoot},
		envVar{key: "GOPATH", value: cmd.GoPath},
		envVar{key: "GOTOOLCHAIN", value: ""},
		envVar{key: "GOCACHE", value: goPaths.goCacheDir},
		envVar{key: "GOMODCACHE", value: goPaths.goModCacheDir},
		envVar{key: "PATH", value: strings.Join(pathEntries, string(os.PathListSeparator))},
	); err != nil {
		return err
	}
	return nil
}

func printPortableGoEnv(cmd *CmdTool, goPaths portableGoPaths) {
	const portableGoEnvFormat = "" +
		"Using portable Go environment:\n" +
		"  GOROOT: %s\n" +
		"  GOPATH: %s\n" +
		"  GoBinPath: %s (for gdspx, gdspxrt, etc.)\n" +
		"  Go binary: %s\n" +
		"  GOCACHE: %s\n" +
		"  GOMODCACHE: %s\n"

	fmt.Printf(portableGoEnvFormat,
		cmd.GoRoot,
		cmd.GoPath,
		cmd.GoBinPath,
		goPaths.goBinaryPath,
		goPaths.goCacheDir,
		goPaths.goModCacheDir,
	)
}

func goBinaryPath(goRootBinPath string) string {
	goExe := "go"
	if runtime.GOOS == "windows" {
		goExe = "go.exe"
	}
	return filepath.Join(goRootBinPath, goExe)
}

func setEnvVars(vars ...envVar) error {
	for _, env := range vars {
		if err := os.Setenv(env.key, env.value); err != nil {
			return fmt.Errorf("set %s: %w", env.key, err)
		}
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
