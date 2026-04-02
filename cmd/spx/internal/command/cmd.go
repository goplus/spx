package command

import (
	"embed"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

const PcExportName = "gdexport"

// CmdTool stores command state.
type CmdTool struct {
	// Project.
	FileSuffix     string
	AppName        string
	Version        string
	ProjectRelPath string
	ProjectDir     string
	GoDir          string
	TargetDir      string
	TargetAbsDir   string
	WebDir         string
	GoBinPath      string

	// Embedded assets.
	ProjectFS  embed.FS
	PlatformFS embed.FS

	// Build.
	ServerPort int
	CmdPath    string
	LibPath    string
	BinPostfix string

	// CLI args.
	Args ExtraArgs

	// Runtime.
	RuntimeMode    bool
	RuntimeTempDir string
	RuntimeCmdPath string

	GoModTemplate string

	// Codegen.
	UseXgobuildForCodegen bool

	// Portable Go.
	GoEnvDir    string
	GoRoot      string
	GoPath      string
	CustomGoEnv bool
}

// RunCmd runs the CLI.
func (cmd *CmdTool) RunCmd(projectName, fileSuffix, version string, fs embed.FS, fsRelDir string, dstRelDir string, ext ...string) (err error) {
	cmd.AppName = projectName
	cmd.FileSuffix = fileSuffix
	cmd.Version = version
	cmd.ProjectFS = fs
	cmd.ProjectRelPath = dstRelDir
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	paths := filepath.SplitList(gopath)
	cmd.GoBinPath, _ = filepath.Abs(filepath.Join(paths[0], "bin"))

	cmd.Args = ExtraArgs{}
	if len(os.Args) < 2 {
		cmd.ShowHelpInfo()
		return
	}

	help := cmd.initializeFlags()

	err = cmd.parseCommandLineArgs(help, ext...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	if cmd.Args.CmdName == "init" {
		return cmd.Init()
	}

	err = cmd.setupPaths(dstRelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up paths: %v\n", err)
		return err
	}

	if cmd.handleSpecialCommands() {
		return nil
	}

	if cmd.Args.CmdName == "runi" {
		return cmd.handleRuniCommand()
	}

	if isRuntimeModeCommand(cmd.Args.CmdName) {
		cmd.RuntimeMode = true
	}

	if cmd.Args.IxgoGen != nil && *cmd.Args.IxgoGen {
		cmd.UseXgobuildForCodegen = true
	}

	if cmd.Args.GoEnv != nil && *cmd.Args.GoEnv != "" {
		if err := cmd.setupPortableGoEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup portable Go environment: %v\n", err)
			return fmt.Errorf("failed to setup portable Go environment: %w", err)
		}
	}

	err = cmd.CheckEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment check failed: %v\n", err)
		return err
	}

	// Work around goplus/spx#619.
	os.Setenv("GODEBUG", "asyncpreemptoff=1")

	cmd.WebDir, _ = filepath.Abs(filepath.Join(cmd.ProjectDir, ".builds", "web"))

	err = cmd.SetupEnv(version, fs, fsRelDir, dstRelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup environment: %v\n", err)
		return err
	}

	return cmd.executeCommand()
}

// handleSpecialCommands handles commands without setup.
func (cmd *CmdTool) handleSpecialCommands() bool {
	switch cmd.Args.CmdName {
	case "help", "version":
		cmd.ShowHelpInfo()
		return true
	case "clear":
		if err := cmd.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clear project: %v\n", err)
		}
		return true
	case "stopweb":
		if err := cmd.StopWeb(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop web server: %v\n", err)
		}
		return true
	}
	return false
}

// executeCommand runs the main command flow.
func (cmd *CmdTool) executeCommand() error {
	if err := cmd.handleBuildPhase(); err != nil {
		return err
	}

	err := cmd.handleExecutionPhase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
	}
	return err

}

// handleBuildPhase runs the build step.
func (cmd *CmdTool) handleBuildPhase() error {
	fmt.Printf("[DEBUG] handleBuildPhase: command=%s %s\n", cmd.Args.CmdName, cmd.SafeTagArgs())

	switch cmd.Args.CmdName {
	case "buildtinygo":
		fmt.Println("[DEBUG] Executing BuildTinyGoLib")
		return cmd.BuildTinyGoLib()
	case "editor", "rune", "export", "build", "run":
		fmt.Println("[DEBUG] Checking BuildDll conditions")
		if cmd.Args.Tags == nil || !strings.Contains(*cmd.Args.Tags, "pure_engine") {
			fmt.Println("[DEBUG] Executing BuildDll")
			return cmd.BuildDll()
		} else {
			fmt.Println("[DEBUG] Skipping BuildDll for pure_engine mode")
		}
	default:
		if shouldBuildWasmForCommand(cmd.Args.CmdName) {
			fmt.Println("[DEBUG] Executing BuildWasm")
			return cmd.BuildWasm()
		}
		fmt.Printf("[DEBUG] No build phase needed for command: %s\n", cmd.Args.CmdName)
	}
	return nil
}

// handleExecutionPhase runs the command step.
func (cmd *CmdTool) handleExecutionPhase() error {
	switch cmd.Args.CmdName {
	case "buildtinygo":
		return nil
	case "editor":
		return cmd.executeEditor()
	case "rune":
		return cmd.executeRune()
	case "run":
		return cmd.executeRun()
	case "runweb":
		return cmd.RunWeb()
	case "runwebworker":
		return cmd.RunWebWorker()
	case "export":
		return cmd.Export()
	case "exporttemplateweb":
		return cmd.ExportTemplateWeb()
	case "exportweb":
		return cmd.ExportWeb()
	case "exportwebworker":
		return cmd.ExportWebWorker()
	case "exportapk":
		return cmd.ExportApk()
	case "exportios":
		return cmd.ExportIos()
	case "exportminigame":
		return cmd.ExportMinigame()
	case "exportminiprogram":
		return cmd.ExportMiniprogram()
	default:
		return nil
	}
}

// executeEditor runs the editor command.
func (cmd *CmdTool) executeEditor() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		return fmt.Errorf("editor command is not supported in pure_engine mode")
	}
	args := cmd.Args.String()
	args = append(args, "-e")
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
}

// executeRune runs the rune command.
func (cmd *CmdTool) executeRune() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		return fmt.Errorf("rune command is not supported in pure_engine mode")
	}
	args := cmd.checkMovieArgs(cmd.ProjectDir)
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
}

// executeRun runs the run command.
func (cmd *CmdTool) executeRun() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		args := cmd.Args.String()
		return cmd.RunPureEngine(args...)
	} else {
		args := cmd.checkMovieArgs(cmd.RuntimeTempDir)
		return cmd.RunPackMode(args...)
	}
}

func (cmd *CmdTool) checkMovieArgs(rootDir string) []string {
	args := cmd.Args.String()
	if cmd.Args.Movie != nil && *cmd.Args.Movie {
		dir, _ := filepath.Abs(filepath.Join(rootDir, "output"))
		fpath := filepath.Join(dir, "movie.avi")
		os.MkdirAll(dir, os.ModePerm)
		args = append(args, "--write-movie", fpath)
	}
	return args
}

// isRuntimeModeCommand reports whether the command uses runtime assets.
func isRuntimeModeCommand(cmdName string) bool {
	switch cmdName {
	case "run", "runweb", "runwebworker":
		return true
	default:
		return false
	}
}

// shouldBuildWasmForCommand reports whether the command needs wasm output.
func shouldBuildWasmForCommand(cmdName string) bool {
	switch cmdName {
	case "buildweb", "exportweb", "runweb", "runwebworker":
		return true
	default:
		return false
	}
}

// handleRuniCommand runs runi with minimal setup.
func (cmd *CmdTool) handleRuniCommand() error {
	cmd.RuntimeMode = true
	cmd.RuntimeTempDir, _ = filepath.Abs(filepath.Join(cmd.TargetDir, ".temp"))
	os.MkdirAll(cmd.RuntimeTempDir, 0755)

	cmd.BinPostfix = ""
	GOOS := runtime.GOOS
	if os.Getenv("GOOS") != "" {
		GOOS = os.Getenv("GOOS")
	}
	if GOOS == "windows" {
		cmd.BinPostfix = ".exe"
	}
	cmd.RuntimeCmdPath = filepath.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)

	args := cmd.checkMovieArgs(cmd.RuntimeTempDir)
	return cmd.RunInterpreted(args...)
}
