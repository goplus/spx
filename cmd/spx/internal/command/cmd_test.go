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
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCmdPropagatesSpecialCommandErrors(t *testing.T) {
	for _, name := range []string{"clear", "clearbuild", "stopweb"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			projectDir := "project"
			if name == "stopweb" {
				cmd := &CmdTool{TargetAbsDir: root}
				if err := os.Mkdir(cmd.webServerPIDPath(), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				// Removal must fail for an overlong path component on all hosts.
				if err := os.Mkdir(projectDir, 0o755); err != nil {
					t.Fatal(err)
				}
				projectDir = filepath.Join(projectDir, strings.Repeat("x", 300))
			}
			oldFlags, oldArgs := flag.CommandLine, os.Args
			flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"spx", name, "--path", root}
			t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

			cmd := &CmdTool{}
			err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", projectDir)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("RunCmd(%s) error = %v, want filesystem error", name, err)
			}
		})
	}
}

func TestRunCmdHelpSkipsProjectSetup(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no command", []string{"spx"}},
		{"help command", []string{"spx", "help"}},
		{"short help", []string{"spx", "-h"}},
		{"command help", []string{"spx", "build", "-h"}},
		{"version", []string{"spx", "version"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			oldFlags, oldArgs := flag.CommandLine, os.Args
			flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = tt.args
			t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

			cmd := &CmdTool{}
			if err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", "project"); err != nil {
				t.Fatalf("RunCmd(%v): %v", tt.args, err)
			}
			if cmd.ProjectDir != "" || cmd.TargetDir != "" {
				t.Fatalf("help prepared a project: ProjectDir=%q TargetDir=%q", cmd.ProjectDir, cmd.TargetDir)
			}
			if cwd, err := os.Getwd(); err != nil || cwd != root {
				t.Fatalf("working directory = %q, %v; want %q", cwd, err, root)
			}
		})
	}
}

func TestRunCmdRejectsUnavailableAndUnknownCommands(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"runm", "not implemented"},
		{"exportbot", "not implemented"},
		{"typo", "unknown command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			oldFlags, oldArgs := flag.CommandLine, os.Args
			flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"spx", tt.name}
			t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

			cmd := &CmdTool{}
			err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", "project")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("RunCmd(%s) error = %v; want %q", tt.name, err, tt.want)
			}
			if cmd.ProjectDir != "" || cmd.TargetDir != "" {
				t.Fatalf("rejected command prepared a project: ProjectDir=%q TargetDir=%q", cmd.ProjectDir, cmd.TargetDir)
			}
		})
	}
}

func TestCommandSpecsHaveExecution(t *testing.T) {
	seen := make(map[string]bool)
	for _, spec := range commandSpecs {
		if spec.name == "" || spec.group == "" || spec.summary == "" {
			t.Errorf("command has incomplete help metadata: %+v", spec)
		}
		if seen[spec.name] {
			t.Errorf("command %q appears more than once", spec.name)
		}
		seen[spec.name] = true
		if spec.run == nil && spec.build == noBuild && !spec.unavailable && spec.name != "help" && spec.name != "version" {
			t.Errorf("command %q can succeed without doing work", spec.name)
		}
		if spec.unavailable && (spec.setup != noSetup || spec.build != noBuild || spec.run != nil) {
			t.Errorf("unavailable command %q has execution steps", spec.name)
		}
	}
}
