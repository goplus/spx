package command

import (
	"embed"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/cmd/gox/pkg/util"
)

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

	// Resource files
	ProjectFS    embed.FS // Embedded project filesystem
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
}

// RunCmd executes the specified command with the given parameters
func (cmd *CmdTool) RunCmd(projectName, fileSuffix, version string, fs embed.FS, fsRelDir string, dstRelDir string, ext ...string) (err error) {
	// Store the parameters in the CmdTool struct
	cmd.AppName = projectName
	cmd.FileSuffix = fileSuffix
	cmd.Version = version
	cmd.ProjectFS = fs
	cmd.ProjectRelPath = dstRelDir

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
	switch cmd.Args.CmdName {
	case "help", "version":
		cmd.ShowHelpInfo()
		return nil
	case "clear":
		if err := cmd.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clear project: %v\n", err)
			return err
		}
		return nil
	case "stopweb":
		if err := cmd.StopWeb(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop web server: %v\n", err)
			return err
		}
		return nil
	case "init":
		return nil
	}

	// Check environment
	err = cmd.CheckEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment check failed: %v\n", err)
		return err
	}

	// Set up the web directory path
	cmd.WebDir, _ = filepath.Abs(filepath.Join(cmd.ProjectDir, ".builds", "web"))

	// Setup environment
	err = cmd.SetupEnv(version, fs, fsRelDir, cmd.TargetDir, dstRelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup environment: %v\n", err)
		return err
	}

	// Execute the command

	// Handle build commands
	switch cmd.Args.CmdName {
	case "editor", "run", "export", "build":
		cmd.BuildDll()
	case "buildweb", "runweb", "exportweb":
		cmd.BuildWasm()
	}

	// Execute the command
	switch cmd.Args.CmdName {
	case "editor":
		args := cmd.Args.String()
		args = append(args, "-e")
		err = util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
	case "run":
		err = util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, cmd.Args.String()...)
	case "export":
		err = cmd.Export()
	case "runweb":
		err = cmd.RunWeb()
	case "exportweb":
		err = cmd.ExportWeb()
	case "exportapk":
		err = cmd.ExportApk()
	case "exportios":
		err = cmd.ExportIos()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Command execution failed: %v\n", err)
		return err
	}

	return nil
}
