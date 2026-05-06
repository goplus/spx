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
		logErrorf("%v", err)
		return err
	}
	if cmd.Args.Verbose != nil && *cmd.Args.Verbose {
		enableDebugLogging()
	}

	if cmd.Args.CmdName == "init" {
		err = cmd.Init()
		if err != nil {
			logErrorf("Initializing project: %v", err)
		}
		return err
	}

	err = cmd.setupPaths(dstRelDir)
	if err != nil {
		logErrorf("Setting up paths: %v", err)
		return err
	}

	if cmd.handleSpecialCommands() {
		return nil
	}

	if cmd.Args.GoEnv != nil && *cmd.Args.GoEnv != "" {
		if err := cmd.setupPortableGoEnv(); err != nil {
			logErrorf("Setting up portable Go environment: %v", err)
			return fmt.Errorf("failed to setup portable Go environment: %w", err)
		}
	}

	if isInterpretedRunCommand(cmd.Args.CmdName) {
		err = cmd.handleInterpretedRunCommand()
		if err != nil {
			logErrorf("Executing interpreted run command: %v", err)
		}
		return err
	}

	if isRuntimeModeCommand(cmd.Args.CmdName) {
		cmd.RuntimeMode = true
	}

	err = cmd.CheckEnv()
	if err != nil {
		logErrorf("Environment check failed: %v", err)
		return err
	}

	// Work around goplus/spx#619.
	os.Setenv("GODEBUG", "asyncpreemptoff=1")

	cmd.WebDir, _ = filepath.Abs(filepath.Join(cmd.ProjectDir, ".builds", "web"))

	err = cmd.SetupEnv(version, fs, fsRelDir, dstRelDir)
	if err != nil {
		logErrorf("Setting up environment: %v", err)
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
			logErrorf("Clearing project: %v", err)
		}
		return true
	case "stopweb":
		if err := cmd.StopWeb(); err != nil {
			logErrorf("Stopping web server: %v", err)
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
		logErrorf("Executing command: %v", err)
	}
	return err

}

// handleBuildPhase runs the build step.
func (cmd *CmdTool) handleBuildPhase() error {
	logDebugf("Handling build phase: command=%s %s", cmd.Args.CmdName, cmd.SafeTagArgs())

	switch cmd.Args.CmdName {
	case "buildtinygo":
		logDebugf("Running TinyGo library build")
		return cmd.BuildTinyGoLib()
	case "editor", "rune", "export", "build", "runnative":
		logDebugf("Checking DLL build conditions")
		if cmd.Args.Tags == nil || !strings.Contains(*cmd.Args.Tags, "pure_engine") {
			logDebugf("Running DLL build")
			return cmd.BuildDll()
		} else {
			logDebugf("Skipping DLL build for pure_engine mode")
		}
	default:
		if shouldBuildWasmForCommand(cmd.Args.CmdName) {
			logDebugf("Running WebAssembly build")
			return cmd.BuildWasm()
		}
		logDebugf("No build phase needed for command: %s", cmd.Args.CmdName)
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
		return nil
	case "runnative":
		return cmd.executeRunNative()
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

// executeRunNative runs the native desktop runtime command.
func (cmd *CmdTool) executeRunNative() error {
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

func isInterpretedRunCommand(cmdName string) bool {
	switch cmdName {
	case "run":
		return true
	default:
		return false
	}
}

// isRuntimeModeCommand reports whether the command uses runtime assets.
func isRuntimeModeCommand(cmdName string) bool {
	switch cmdName {
	case "runnative", "runweb", "runwebworker":
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

// handleInterpretedRunCommand runs the interpreted-mode command with minimal setup.
func (cmd *CmdTool) handleInterpretedRunCommand() error {
	cmd.RuntimeMode = true
	cmd.RuntimeTempDir, _ = filepath.Abs(filepath.Join(cmd.TargetDir, ".temp"))

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
