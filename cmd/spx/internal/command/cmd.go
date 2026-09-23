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

	"github.com/goplus/spx/v3/cmd/spx/internal/util"
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

	launcherBuilder launcherBuilder

	// CLI args.
	Args ExtraArgs

	// Runtime.
	RuntimeMode    bool
	RuntimeTempDir string
	RuntimeCmdPath string

	GoModTemplate string
}

// RunCmd runs the CLI.
func (cmd *CmdTool) RunCmd(projectName, fileSuffix, version string, fs embed.FS, fsRelDir string, dstRelDir string) error {
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
	help := cmd.initializeFlags()
	if err := cmd.parseCommandLineArgs(help); err != nil {
		logErrorf("%v", err)
		return err
	}
	if cmd.Args.Verbose != nil && *cmd.Args.Verbose {
		enableDebugLogging()
	}
	spec, ok := findCommand(cmd.Args.CmdName)
	if !ok {
		err := fmt.Errorf("unknown command: %s", cmd.Args.CmdName)
		logErrorf("%v", err)
		return err
	}
	if err := cmd.dispatch(spec, fsRelDir, dstRelDir); err != nil {
		logErrorf("Executing %s: %v", spec.name, err)
		return err
	}
	return nil
}

// setupInterpretedPaths resolves the source and session roots without changing
// the process working directory. Other legacy commands continue to use
// setupPaths until their generated-project assumptions are migrated.
func (cmd *CmdTool) setupInterpretedPaths(dstRelDir string) error {
	targetDir, err := filepath.Abs(*cmd.Args.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}
	cmd.TargetAbsDir = filepath.Clean(targetDir)
	cmd.TargetDir = cmd.TargetAbsDir
	cmd.Args.Path = &cmd.TargetDir
	cmd.ProjectDir = filepath.Join(cmd.TargetAbsDir, dstRelDir)
	return nil
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

// handleInterpretedRunCommand runs the interpreted-mode command with minimal setup.
func (cmd *CmdTool) handleInterpretedRunCommand() error {
	cmd.RuntimeMode = true
	cmd.RuntimeTempDir, _ = filepath.Abs(filepath.Join(cmd.TargetDir, ".temp"))

	GOOS := runtime.GOOS
	if os.Getenv("GOOS") != "" {
		GOOS = os.Getenv("GOOS")
	}
	cmd.BinPostfix = executableSuffix(GOOS)
	cmd.RuntimeCmdPath = filepath.Join(cmd.GoBinPath, "gdspxrt"+cmd.Version+cmd.BinPostfix)

	args := cmd.checkMovieArgs(cmd.RuntimeTempDir)
	return cmd.RunInterpreted(args...)
}
