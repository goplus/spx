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
	"strings"
)

// ExtraArgs stores CLI values. initializeFlags supplies CLI defaults; the zero
// value represents empty options for direct command callers.
type ExtraArgs struct {
	CmdName         string
	Path            string
	ServerAddr      string
	ServerMode      bool
	ControllerName  string
	HeadlessMode    bool
	Arch            string
	OnlyServer      bool
	OnlyClient      bool
	Tags            string
	Target          string
	NoMap           bool
	Install         bool
	DebugWebService bool
	FullScreen      bool
	Build           string
	Mode            string
	Movie           bool
	Verbose         bool
	Output          string
}

func (e *ExtraArgs) String() []string {
	var args []string
	if e.Path != "" {
		args = append(args, "--path", e.Path)
	}
	if e.ServerAddr != "" {
		args = append(args, "-serveraddr", e.ServerAddr)
	}
	if e.ServerMode {
		args = append(args, "-servermode")
	}
	if e.ControllerName != "" {
		args = append(args, "-controller", e.ControllerName)
	}
	if e.HeadlessMode {
		args = append(args, "--headless")
	}
	if e.NoMap {
		args = append(args, "--nomap")
	}
	if e.DebugWebService {
		args = append(args, "--debugweb")
	}
	if e.FullScreen {
		args = append(args, "--fullscreen")
	}
	if e.Verbose {
		args = append(args, "-v")
	}
	return args
}

func (cmd *CmdTool) SafeTagArgs() string {
	tags := cmd.Args.Tags
	if tags == "" {
		return ""
	}
	return "-tags=" + tags
}

// ShowHelpInfo prints usage.
func (cmd *CmdTool) ShowHelpInfo() {
	cmdName := cmd.AppName
	version := cmd.Version
	fmt.Printf("%s Version = %s\n\nUsage:\n\n    %s <command> [arguments]\n\nAvailable commands:\n", cmdName, version, cmdName)
	group := ""
	for _, spec := range commandSpecs {
		if spec.hidden {
			continue
		}
		if spec.group != group {
			group = spec.group
			fmt.Printf("\n    %s:\n", group)
		}
		fmt.Printf("    - %-17s # %s\n", spec.name, spec.summary)
	}
	msg := `
Examples:

    #CMDNAME init                         # Create a project in current path
    #CMDNAME init ./test/demo01           # Create a project at path ./test/demo01
    #CMDNAME run --path ./myproject       # Run project in interpreted mode
    #CMDNAME runnative --path ./myproject # Run project with the native PC runtime
    #CMDNAME build --servermode           # Build in server mode
    #CMDNAME runweb --debugweb            # Run web server with debug service
    #CMDNAME buildtinygo                  # Build TinyGo static library for ESP32
    #CMDNAME buildlauncher --path ./game # Build ./game/.builds/game
    #CMDNAME exportminigame -build=fast   # Export minigame without compression (faster)
    #CMDNAME export --fullscreen          # Export with fullscreen mode
	`
	fmt.Println(strings.ReplaceAll(msg, "#CMDNAME", cmdName))

	fmt.Println("Available Arguments:")
	flag.PrintDefaults()
}

// initializeFlags binds CLI flags.
func (cmd *CmdTool) initializeFlags() *bool {
	f := flag.CommandLine
	help := f.Bool("h", false, "show help information")

	f.StringVar(&cmd.Args.ServerAddr, "serveraddr", "", "server address")
	f.StringVar(&cmd.Args.Path, "path", ".", "project path")
	f.StringVar(&cmd.Args.ControllerName, "controller", "", "controller's type name")
	f.BoolVar(&cmd.Args.ServerMode, "servermode", false, "server mode")
	f.BoolVar(&cmd.Args.HeadlessMode, "headless", false, "Headless Mode")
	f.StringVar(&cmd.Args.Arch, "arch", "", "cpu arch")
	f.BoolVar(&cmd.Args.OnlyServer, "onlys", false, "mutil player mode server only")
	f.BoolVar(&cmd.Args.OnlyClient, "onlyc", false, "mutil player mode clients only")
	f.StringVar(&cmd.Args.Tags, "tags", "", "build tags")
	f.StringVar(&cmd.Args.Target, "target", "esp32", "target board (default: esp32)")
	f.BoolVar(&cmd.Args.NoMap, "nomap", false, "no map mode")
	f.BoolVar(&cmd.Args.Install, "install", false, "install mode")
	f.BoolVar(&cmd.Args.DebugWebService, "debugweb", false, "open debug web service")
	f.BoolVar(&cmd.Args.FullScreen, "fullscreen", false, "full screen")
	f.StringVar(&cmd.Args.Build, "build", "normal", "build mode: normal or fast")
	f.StringVar(&cmd.Args.Mode, "mode", "none", "mode: none, worker, minigame")
	f.BoolVar(&cmd.Args.Movie, "movie", false, "record movie mode")
	f.BoolVar(&cmd.Args.Verbose, "v", false, "print verbose information")
	f.StringVar(&cmd.Args.Output, "o", "", "launcher output path (buildlauncher)")
	return help
}

// parseCommandLineArgs parses flags and selects the command.
func (cmd *CmdTool) parseCommandLineArgs(help *bool) error {
	if len(os.Args) <= 1 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "h" {
		cmd.Args.CmdName = "help"
		return nil
	}

	cmd.Args.CmdName = os.Args[1]
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *help {
		cmd.Args.CmdName = "help"
	}

	return nil
}
