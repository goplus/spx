package command

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type ExtraArgs struct {
	CmdName         string
	Path            *string
	ServerAddr      *string
	ServerMode      *bool
	ControllerName  *string
	HeadlessMode    *bool
	Arch            *string
	OnlyServer      *bool
	OnlyClient      *bool
	Tags            *string
	NoMap           *bool
	Install         *bool
	DebugWebService *bool
	FullScreen      *bool
	Build           *string
}

func (e *ExtraArgs) String() []string {
	var args []string
	if *e.Path != "" {
		args = append(args, "--path", *e.Path)
	}
	if *e.ServerAddr != "" {
		args = append(args, "-serveraddr", *e.ServerAddr)
	}
	if *e.ServerMode {
		args = append(args, "-servermode")
	}
	if *e.ControllerName != "" {
		args = append(args, "-controller", *e.ControllerName)
	}
	if *e.HeadlessMode {
		args = append(args, "--headless")
	}
	if *e.NoMap {
		args = append(args, "--nomap")
	}
	if *e.DebugWebService {
		args = append(args, "--debugweb")
	}
	if *e.FullScreen {
		args = append(args, "--fullscreen")
	}
	return args
}

// CheckCmd checks if the command is valid
func (pself *CmdTool) CheckCmd(ext ...string) bool {
	cmds := []string{
		"help", "version", "editor",
		"init", "clear", "clearbuild",
		"build", "rune", "export",
		"runweb", "buildweb", "exportweb", "stopweb",
		"runm", "exportbot", "exportapk", "exportios",
		"run", "exportwebeditor", "runwebeditor", "exportwebruntime",
		"exportminigame", "runminigame",
	}
	cmds = append(cmds, ext...)

	cmdName := pself.Args.CmdName
	for _, b := range cmds {
		if b == cmdName {
			return true
		}
	}
	return false
}
func (pself *CmdTool) CheckCmdWithError(ext ...string) (err error) {
	if len(os.Args) <= 1 {
		pself.ShowHelpInfo()
		return
	}
	if !pself.CheckCmd(ext...) {
		println("invalid cmd, please refer to help")
		pself.ShowHelpInfo()
	}
	return
}

// initializeFlags initializes command line flags
func (cmd *CmdTool) initializeFlags() *bool {
	f := flag.CommandLine
	help := f.Bool("h", false, "show help information")

	// Initialize command line arguments
	cmd.Args.ServerAddr = f.String("serveraddr", "", "server address")
	cmd.Args.Path = f.String("path", ".", "project path")
	cmd.Args.ControllerName = f.String("controller", "", "controller's type name")
	cmd.Args.ServerMode = f.Bool("servermode", false, "server mode")
	cmd.Args.HeadlessMode = f.Bool("headless", false, "Headless Mode")
	cmd.Args.Arch = f.String("arch", "", "cpu arch")
	cmd.Args.OnlyServer = f.Bool("onlys", false, "mutil player mode server only")
	cmd.Args.OnlyClient = f.Bool("onlyc", false, "mutil player mode clients only")
	cmd.Args.Tags = f.String("tags", "simulation", "build tags")
	cmd.Args.NoMap = f.Bool("nomap", false, "no map mode")
	cmd.Args.Install = f.Bool("install", false, "install mode")
	cmd.Args.DebugWebService = f.Bool("debugweb", false, "open debug web service")
	cmd.Args.FullScreen = f.Bool("fullscreen", false, "full screen")
	cmd.Args.Build = f.String("build", "normal", "build mode: normal or fast")
	return help
}

// parseCommandLineArgs parses command line arguments and handles help requests
func (cmd *CmdTool) parseCommandLineArgs(help *bool, ext ...string) error {
	// Check for help command
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "h" {
		cmd.ShowHelpInfo()
		return nil
	}

	// Set command name and parse remaining arguments
	cmd.Args.CmdName = os.Args[1]
	flag.CommandLine.Parse(os.Args[2:])

	// Show help if requested
	if *help {
		cmd.ShowHelpInfo()
		return nil
	}

	// Validate command
	if !cmd.CheckCmd(ext...) {
		return fmt.Errorf("unknown command: %s", cmd.Args.CmdName)
	}

	return nil
}

// ShowHelpInfo displays comprehensive help information for the command
// It prints the command version, usage instructions, available commands with descriptions,
// and usage examples to guide users on how to use the tool effectively.
//
// Parameters:
//   - cmdName: The name of the command to display in help text (e.g., "rbx")
//   - version: The version string to display (e.g., "2.0.1")
func (pself *CmdTool) ShowHelpInfo() {
	cmdName := pself.AppName
	version := pself.Version
	msg := `
Usage:

    #CMDNAME <command> [arguments]

Available commands:

    Project Management:
    - help            # Display help information
    - version         # Display version information
    - init            # Create a #CMDNAME project in the current directory
    - editor          # Open the current project in editor mode
    - clear           # Clear the project
    - clearbuild      # Clear build artifacts

    Development:
    - build           # Build the dynamic library
    - run             # Run the current project
    - export          # Export the PC package (macOS, Windows, Linux)
    - runm            # Run the project in mutil player mode

    Web Development:
    - buildweb        # Build for WebAssembly (WASM)
    - runweb          # Launch the web server
    - exportweb       # Export the web package
    - stopweb         # Stop the web server

    Mobile & Bot Development:
    - exportbot       # Export the bot package
    - exportapk       # Export Android APK
    - exportios       # Export iOS package
    - exportminigame  # Export minigame package (supports -build=fast for faster build)

Examples:

    #CMDNAME init                      # Create a project in current path
    #CMDNAME init ./test/demo01        # Create a project at path ./test/demo01
    #CMDNAME run --path ./myproject    # Run project at specified path
    #CMDNAME build --servermode        # Build in server mode
    #CMDNAME runweb --debugweb         # Run web server with debug service
    #CMDNAME exportminigame -build=fast # Export minigame without compression (faster)
	`
	fmt.Println(cmdName + " Version = " + version + "\n" + strings.ReplaceAll(msg, "#CMDNAME", cmdName))

	fmt.Println("Available Arguments:")
	flag.PrintDefaults()
}
