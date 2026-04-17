/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package command

import (
	"flag"
	"fmt"
	"os"
	"slices"
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
	Target          *string
	NoMap           *bool
	Install         *bool
	DebugWebService *bool
	FullScreen      *bool
	Build           *string
	Mode            *string
	Movie           *bool
	GoEnv           *string
	Verbose         *bool
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
	if *e.Verbose {
		args = append(args, "-v")
	}
	return args
}

// CheckCmd reports whether the command is supported.
func (cmd *CmdTool) CheckCmd(ext ...string) bool {
	cmds := []string{
		"help", "version", "editor",
		"init", "clear", "clearbuild",
		"build", "buildtinygo", "rune", "export",
		"runweb", "buildweb", "exportweb", "stopweb", "runwebworker",
		"runm", "exportbot", "exportapk", "exportios",
		"run", "runi", "exporttemplateweb", "exportminigame", "exportminiprogram", "exportwebworker",
	}
	cmds = append(cmds, ext...)

	cmdName := cmd.Args.CmdName
	return slices.Contains(cmds, cmdName)
}
func (cmd *CmdTool) CheckCmdWithError(ext ...string) (err error) {
	if len(os.Args) <= 1 {
		cmd.ShowHelpInfo()
		return
	}
	if !cmd.CheckCmd(ext...) {
		fmt.Fprintf(os.Stderr, "Error: invalid cmd, please refer to help\n")
		cmd.ShowHelpInfo()
	}
	return
}

func (cmd *CmdTool) SafeTagArgs() string {
	tags := cmd.Args.Tags
	if tags == nil || *tags == "" {
		return ""
	}
	return "-tags=" + *tags
}

// ShowHelpInfo prints usage.
func (cmd *CmdTool) ShowHelpInfo() {
	cmdName := cmd.AppName
	version := cmd.Version
	msg := `
Usage:

    #CMDNAME <command> [arguments]

Available commands:

    Basic Commands:
    - help            # Display help information
    - version         # Display version information

    Project Management:
    - init            # Create a #CMDNAME project in the current directory
    - clear           # Clear the project
    - clearbuild      # Clear build artifacts

    Development & Building:
    - build           # Build the dynamic library
    - buildtinygo     # Build static library using TinyGo for ESP32
    - run             # Run the current project (dynamic load res mode)
    - editor          # Open the current project in editor mode
    - rune            # Run with engine after import all assets

    Web Development:
    - buildweb        # Build for WebAssembly (WASM)
    - runweb          # Launch the web server
    - runwebworker    # Run web worker
    - exportweb       # Export the web package
    - exportwebworker # Export web worker package
    - exporttemplateweb # Export template web package
    - stopweb         # Stop the web server

    Export & Distribution:
    - export          # Export the PC package (macOS, Windows, Linux)
    - exportapk       # Export Android APK
    - exportios       # Export iOS package
    - exportminigame  # Export minigame package (supports -build=fast for faster build)
    - exportminiprogram # Export mini program package
    - exportbot       # Export the bot package

    Multiplayer:
    - runm            # Run the project in multiplayer mode

Examples:

    #CMDNAME init                         # Create a project in current path
    #CMDNAME init ./test/demo01           # Create a project at path ./test/demo01
    #CMDNAME run --path ./myproject       # Run project at specified path
    #CMDNAME runi --path ./myproject      # Run in interpreted mode (requires pre-installed dll)
    #CMDNAME run --goenv=/path/to/goenv     # Run with a portable goenv
    #CMDNAME build --servermode           # Build in server mode
    #CMDNAME runweb --debugweb            # Run web server with debug service
    #CMDNAME buildtinygo                  # Build TinyGo static library for ESP32
    #CMDNAME exportminigame -build=fast   # Export minigame without compression (faster)
    #CMDNAME run -tags=pure_engine        # Run in pure engine mode
    #CMDNAME export --fullscreen          # Export with fullscreen mode
	`
	fmt.Println(cmdName + " Version = " + version + "\n" + strings.ReplaceAll(msg, "#CMDNAME", cmdName))

	fmt.Println("Available Arguments:")
	flag.PrintDefaults()
}

// initializeFlags binds CLI flags.
func (cmd *CmdTool) initializeFlags() *bool {
	f := flag.CommandLine
	help := f.Bool("h", false, "show help information")

	cmd.Args.ServerAddr = f.String("serveraddr", "", "server address")
	cmd.Args.Path = f.String("path", ".", "project path")
	cmd.Args.ControllerName = f.String("controller", "", "controller's type name")
	cmd.Args.ServerMode = f.Bool("servermode", false, "server mode")
	cmd.Args.HeadlessMode = f.Bool("headless", false, "Headless Mode")
	cmd.Args.Arch = f.String("arch", "", "cpu arch")
	cmd.Args.OnlyServer = f.Bool("onlys", false, "mutil player mode server only")
	cmd.Args.OnlyClient = f.Bool("onlyc", false, "mutil player mode clients only")
	cmd.Args.Tags = f.String("tags", "", "build tags")
	cmd.Args.Target = f.String("target", "esp32", "target board (default: esp32)")
	cmd.Args.NoMap = f.Bool("nomap", false, "no map mode")
	cmd.Args.Install = f.Bool("install", false, "install mode")
	cmd.Args.DebugWebService = f.Bool("debugweb", false, "open debug web service")
	cmd.Args.FullScreen = f.Bool("fullscreen", false, "full screen")
	cmd.Args.Build = f.String("build", "normal", "build mode: normal or fast")
	cmd.Args.Mode = f.String("mode", "none", "mode: none, worker, minigame")
	cmd.Args.Movie = f.Bool("movie", false, "record movie mode")
	cmd.Args.GoEnv = f.String("goenv", "", "portable goenv directory with gotoolchain/go and go/bin")
	cmd.Args.Verbose = f.Bool("v", false, "print verbose information")
	return help
}

// parseCommandLineArgs parses the CLI input.
func (cmd *CmdTool) parseCommandLineArgs(help *bool, ext ...string) error {
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "h" {
		cmd.ShowHelpInfo()
		return nil
	}

	cmd.Args.CmdName = os.Args[1]
	flag.CommandLine.Parse(os.Args[2:])

	if *help {
		cmd.ShowHelpInfo()
		return nil
	}

	if !cmd.CheckCmd(ext...) {
		return fmt.Errorf("unknown command: %s", cmd.Args.CmdName)
	}

	return nil
}
