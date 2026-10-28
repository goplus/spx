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

	"github.com/goplus/spx/v4/cmd/spx/internal/util"
)

const PcExportName = "gdexport"

// CmdTool stores command state.
type CmdTool struct {
	// Project.
	FileSuffix string
	AppName    string
	Version    string
	ProjectDir string
	GoDir      string
	TargetDir  string // Canonical absolute source root; distinct from generated ProjectDir.
	WebDir     string
	GoBinPath  string

	// Embedded assets.
	ProjectFS  embed.FS
	PlatformFS embed.FS

	// Build.
	ServerPort int
	CmdPath    string
	LibPath    string

	launcherBuilder launcherBuilder

	// CLI args.
	Args ExtraArgs

	// Runtime.
	RuntimeCmdPath string

	GoModTemplate string
}

// RunCmd runs the CLI.
func (cmd *CmdTool) RunCmd(projectName, fileSuffix, version string, fs embed.FS, fsRelDir string, dstRelDir string) error {
	cmd.AppName = projectName
	cmd.FileSuffix = fileSuffix
	cmd.Version = version
	cmd.ProjectFS = fs
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

// handleInterpretedRunCommand runs the interpreted-mode command with minimal setup.
func (cmd *CmdTool) handleInterpretedRunCommand() error {
	args := cmd.checkMovieArgs(cmd.runtimeTempDir())
	return cmd.RunInterpreted(args...)
}

// executeEditor runs the editor command.
func (cmd *CmdTool) executeEditor() error {
	args := cmd.engineArgs(cmd.ProjectDir)
	args = append(args, "-e")
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
}

// executeRune runs the rune command.
func (cmd *CmdTool) executeRune() error {
	args := cmd.checkMovieArgs(cmd.ProjectDir)
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, args...)
}

// executeRunNative runs the native desktop runtime command.
func (cmd *CmdTool) executeRunNative() error {
	args := cmd.checkMovieArgs(cmd.runtimeTempDir())
	return cmd.RunPackMode(args...)
}

// resolveProjectPaths resolves the real source root and generated project path
// without changing the process working directory.
func (cmd *CmdTool) resolveProjectPaths(sourceDir, dstRelDir string) error {
	targetDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}
	targetDir, err = filepath.EvalSymlinks(targetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}
	cmd.TargetDir = targetDir
	cmd.Args.Path = &cmd.TargetDir
	cmd.ProjectDir = filepath.Join(cmd.TargetDir, dstRelDir)
	return nil
}

func (cmd *CmdTool) checkMovieArgs(rootDir string) []string {
	args := cmd.engineArgs(rootDir)
	if cmd.Args.Movie != nil && *cmd.Args.Movie {
		dir, _ := filepath.Abs(filepath.Join(rootDir, "output"))
		fpath := filepath.Join(dir, "movie.avi")
		os.MkdirAll(dir, os.ModePerm)
		args = append(args, "--write-movie", fpath)
	}
	return args
}

func (cmd *CmdTool) engineArgs(rootDir string) []string {
	args := cmd.Args
	args.Path = &rootDir
	return args.String()
}
