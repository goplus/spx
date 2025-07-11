package command

import (
	"embed"
	_ "embed"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

const PcExportName = "gdexport"

// CmdTool represents the main command tool for managing project operations
type CmdTool struct {
	// Project information
	FileSuffix     string // File suffix of the project file
	AppName        string // Name of the application
	Version        string // Version of the application
	ProjectRelPath string // Relative path to the project
	ProjectDir     string // Absolute path to the project directory
	GoDir          string // Absolute path to the Go directory
	TargetDir      string // Target directory for operations
	WebDir         string // Web directory for web operations
	GoBinPath      string

	// Resource files
	ProjectFS    embed.FS // Embedded project filesystem
	PlatformFS   embed.FS // Embedded platform filesystem
	GitignoreTxt string   // Gitignore template content
	RunSh        string   // Run script content
	MainSh       string   // Main script content

	// Build and runtime information
	ServerPort int    // Server port for web operations
	CmdPath    string // Path to the command executable
	LibPath    string // Path to the library
	BinPostfix string // Binary postfix

	// Command line arguments
	Args ExtraArgs // Command line arguments

	// runtime mode
	RuntimeMode    bool
	RuntimeTempDir string
	RuntimePckPath string
	RuntimeCmdPath string
}

// RunCmd executes the specified command with the given parameters
func (cmd *CmdTool) RunCmd(projectName, fileSuffix, version string, fs embed.FS, fsRelDir string, dstRelDir string, ext ...string) (err error) {
	// Store the parameters in the CmdTool struct
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
	// Check if we have enough arguments
	if len(os.Args) < 2 {
		cmd.ShowHelpInfo()
		return
	}

	// Initialize flags
	help := cmd.initializeFlags()

	// Parse command line arguments
	err = cmd.parseCommandLineArgs(help, ext...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// Setup paths
	err = cmd.setupPaths(dstRelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up paths: %v\n", err)
		return err
	}

	// Handle special commands that don't need full setup
	if cmd.handleSpecialCommands() {
		return nil
	}

	// Set runtime mode
	if cmd.Args.CmdName == "run" || cmd.Args.CmdName == "runweb" {
		cmd.RuntimeMode = true
	}

	// Check environment
	err = cmd.CheckEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment check failed: %v\n", err)
		return err
	}

	// fix https://github.com/goplus/spx/issues/619
	// fatal error: non-Go code set up signal handler without SA_ONSTACK flag
	os.Setenv("GODEBUG", "asyncpreemptoff=1")

	// Set up the web directory path
	cmd.WebDir, _ = filepath.Abs(filepath.Join(cmd.ProjectDir, ".builds", "web"))

	// Setup environment
	err = cmd.SetupEnv(version, fs, fsRelDir, dstRelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup environment: %v\n", err)
		return err
	}

	// Handle init command
	if cmd.Args.CmdName == "init" {
		return nil
	}

	// Execute the command based on its type
	return cmd.executeCommand()
}

// handleSpecialCommands handles commands that don't need full setup
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

// executeCommand executes the main command logic
func (cmd *CmdTool) executeCommand() error {
	// First, handle build phase if needed
	if err := cmd.handleBuildPhase(); err != nil {
		return err
	}

	// Then, handle execution phase
	err := cmd.handleExecutionPhase()
	if err != nil {
		println("executeCommand error: ", err.Error())
	}
	return err

}

// handleBuildPhase handles the build phase for commands that need it
func (cmd *CmdTool) handleBuildPhase() error {
	// 添加调试日志
	fmt.Printf("[DEBUG] handleBuildPhase: command=%s, tags=%v\n", cmd.Args.CmdName, cmd.Args.Tags)

	switch cmd.Args.CmdName {
	case "buildtinygo":
		fmt.Println("[DEBUG] Executing BuildTinyGoLib")
		return cmd.BuildTinyGoLib()
	case "editor", "rune", "export", "build", "run":
		fmt.Println("[DEBUG] Checking BuildDll conditions")
		// Skip BuildDll for pure_engine mode
		if cmd.Args.Tags == nil || !strings.Contains(*cmd.Args.Tags, "pure_engine") {
			fmt.Println("[DEBUG] Executing BuildDll")
			return cmd.BuildDll()
		} else {
			fmt.Println("[DEBUG] Skipping BuildDll for pure_engine mode")
		}
	case "buildweb", "runweb", "exportweb":
		fmt.Println("[DEBUG] Executing BuildWasm")
		return cmd.BuildWasm()
	default:
		fmt.Printf("[DEBUG] No build phase needed for command: %s\n", cmd.Args.CmdName)
	}
	return nil
}

// handleExecutionPhase handles the execution phase for commands that need it
func (cmd *CmdTool) handleExecutionPhase() error {
	switch cmd.Args.CmdName {
	case "buildtinygo":
		// Build-only command, no execution phase needed
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
		// For build-only commands, no execution needed
		return nil
	}
}

// executeEditor handles the editor command execution
func (cmd *CmdTool) executeEditor() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		return fmt.Errorf("editor command is not supported in pure_engine mode")
	}
	args := cmd.Args.String()
	args = append(args, "-e")
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
}

// executeRune handles the rune command execution
func (cmd *CmdTool) executeRune() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		return fmt.Errorf("rune command is not supported in pure_engine mode")
	}
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, cmd.Args.String()...)
}

// executeRun handles the run command execution
func (cmd *CmdTool) executeRun() error {
	if cmd.Args.Tags != nil && strings.Contains(*cmd.Args.Tags, "pure_engine") {
		// For pure_engine mode, run the Go binary directly
		return cmd.RunPureEngine(cmd.Args.String()...)
	} else {
		return cmd.RunPackMode(cmd.Args.String()...)
	}
}
