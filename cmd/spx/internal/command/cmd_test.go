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
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestGoBuildCommandBoundaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake build tools use POSIX shell scripts")
	}
	for _, build := range []struct {
		name         string
		tool         string
		run          func(*CmdTool) error
		mobileExport bool
	}{
		{"wasm", "go", (*CmdTool).BuildWasm, false},
		{"tinygo", "tinygo", (*CmdTool).BuildTinyGoLib, false},
		{"shared", "go", (*CmdTool).BuildDll, false},
		{"apk", "go", (*CmdTool).ExportApk, true},
		{"ios", "go", (*CmdTool).ExportIos, true},
	} {
		for _, failure := range []string{"", "xgo:go", "go:mod", build.tool + ":build"} {
			// Mobile cases exercise early failure, not SDK builds or packaging.
			if build.mobileExport && failure == "" {
				continue
			}
			name := failure
			if name == "" {
				name = "success"
			}
			t.Run(build.name+"/"+name, func(t *testing.T) {
				root, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				projectRoot := filepath.Join(root, "source project")
				binDir := filepath.Join(root, "tools")
				logPath := filepath.Join(root, "commands.log")
				for _, dir := range []string{projectRoot, binDir} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				const script = `#!/bin/sh
set -eu
tool="${0##*/}"
printf '%s|%s|%s|%s|%s|%s|%s\n' "$tool" "$PWD" "$*" "${GOOS-}" "${GOARCH-}" "${CGO_ENABLED-}" "${GODEBUG-}" >> "$SPX_TEST_BUILD_LOG"
if [ "$SPX_TEST_BUILD_FAIL" = "$tool:$1" ]; then
    exit 23
fi
case "$tool" in
    go|xgo|tinygo) ;;
    *) exit 97 ;;
esac
if [ "$tool" = "xgo" ]; then
    printf 'package main\n' > xgo_autogen.go
fi
`
				for _, tool := range []string{"go", "xgo", "tinygo", "godot", "xcrun", "lipo", "adb", "ios-deploy"} {
					if err := os.WriteFile(filepath.Join(binDir, tool), []byte(script), 0755); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				t.Setenv("SPX_TEST_BUILD_LOG", logPath)
				t.Setenv("SPX_TEST_BUILD_FAIL", failure)
				for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "GODEBUG"} {
					t.Setenv(key, "")
				}

				target, tags, arch := "", "", runtime.GOARCH
				install := false
				cmd := &CmdTool{
					TargetDir:    projectRoot,
					TargetAbsDir: projectRoot,
					ProjectDir:   filepath.Join(projectRoot, ".generated"),
					GoDir:        filepath.Join(projectRoot, ".generated", "go"),
					LibPath:      filepath.Join(root, "spx-1-"+arch+".so"),
					CmdPath:      filepath.Join(binDir, "godot"),
					Args: ExtraArgs{
						Target:  &target,
						Tags:    &tags,
						Arch:    &arch,
						Install: &install,
					},
				}
				const staleMain = "package main\n// previous generation\n"
				if build.mobileExport {
					// A failed regeneration must not export a previous main.go.
					for _, dir := range []string{filepath.Join(projectRoot, "assets"), cmd.GoDir} {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
					}
					for name, content := range map[string]string{
						filepath.Join(projectRoot, "assets", "index.json"): "{}",
						filepath.Join(cmd.GoDir, "main.go"):                staleMain,
						filepath.Join(cmd.GoDir, "ios_fixture.go.txt"):     "package main\n",
					} {
						if err := os.WriteFile(name, []byte(content), 0644); err != nil {
							t.Fatal(err)
						}
					}
					t.Setenv("ANDROID_NDK_ROOT", filepath.Join(root, "ndk"))
				}
				if failure == "go:mod" && !cmd.shouldRunGoModTidy() {
					t.Skip("fixture is inside a detected local SPX repository")
				}
				before, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if current, err := os.Getwd(); err != nil || current != before {
						if err := os.Chdir(before); err != nil {
							t.Errorf("restore working directory: %v", err)
						}
					}
				})

				err = build.run(cmd)
				if after, cwdErr := os.Getwd(); cwdErr != nil || after != before {
					t.Fatalf("working directory = %q (%v), want %q", after, cwdErr, before)
				}
				if failure == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
						t.Fatalf("build error = %v, want wrapped exit status 23", err)
					}
				}

				data, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatal(err)
				}
				calls := strings.Split(strings.TrimSpace(string(data)), "\n")
				buildCalls := 0
				for i, call := range calls {
					fields := strings.Split(call, "|")
					if len(fields) != 7 {
						t.Fatalf("invalid command record %q", call)
					}
					tool, dir, args := fields[0], fields[1], fields[2]
					if failure != "" && tool+":"+strings.SplitN(args, " ", 2)[0] == failure && i != len(calls)-1 {
						t.Fatalf("commands continued after %s failed: %v", failure, calls[i+1:])
					}
					if i == 0 && tool != "xgo" {
						t.Fatalf("first command = %q, want xgo", call)
					}
					wantDir := projectRoot
					if strings.HasPrefix(args, "build ") {
						buildCalls++
						wantDir = cmd.GoDir
						if tool != build.tool {
							t.Fatalf("build tool = %q, want %q", tool, build.tool)
						}
						switch build.name {
						case "wasm":
							if fields[3] != "js" || fields[4] != "wasm" {
								t.Fatalf("web build environment = %v", fields[3:])
							}
						case "tinygo":
							if fields[6] != "gotypesalias=0" {
								t.Fatalf("TinyGo GODEBUG = %q", fields[6])
							}
						case "shared", "apk", "ios":
							if fields[5] != "1" || fields[4] == "" {
								t.Fatalf("shared build environment = %v", fields[3:])
							}
						}
					}
					if dir != wantDir {
						t.Fatalf("%s command directory = %q, want %q", tool, dir, wantDir)
					}
				}
				if failure == "xgo:go" || failure == "go:mod" {
					if buildCalls != 0 {
						t.Fatalf("ran %d builds after generation failed", buildCalls)
					}
				} else if buildCalls == 0 {
					t.Fatal("no build command was executed")
				}
				if failure == "xgo:go" {
					if len(calls) != 1 {
						t.Fatalf("executed %d commands after xgo failure, want 1", len(calls))
					}
					if build.mobileExport {
						if got, err := os.ReadFile(filepath.Join(cmd.GoDir, "main.go")); err != nil || string(got) != staleMain {
							t.Fatalf("previous main.go = %q (%v), want unchanged stale fixture", got, err)
						}
					}
				} else {
					mainPath := filepath.Join(cmd.GoDir, "main.go")
					if got, err := os.ReadFile(mainPath); err != nil || string(got) != "package main\n" {
						t.Fatalf("generated main = %q (%v)", got, err)
					}
					if _, err := os.Stat(filepath.Join(projectRoot, "xgo_autogen.go")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("generated source was not moved: %v", err)
					}
				}
				if build.mobileExport {
					for _, name := range []string{
						filepath.Join(cmd.ProjectDir, "lib"),
						filepath.Join(cmd.ProjectDir, ".builds"),
						filepath.Join(cmd.ProjectDir, ".godot", "tmp", "gobuild"),
						filepath.Join(cmd.GoDir, "ios_fixture.go"),
					} {
						if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
							t.Errorf("export continued after %s failed: %s exists or is inaccessible (%v)", failure, name, err)
						}
					}
					if got, err := os.ReadFile(filepath.Join(cmd.GoDir, "ios_fixture.go.txt")); err != nil || string(got) != "package main\n" {
						t.Fatalf("iOS artifact was restored after build failure: %q (%v)", got, err)
					}
				}
			})
		}
	}
}
