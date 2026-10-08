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

func setTestCommandLine(t *testing.T, args ...string) {
	t.Helper()
	oldFlags, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = args
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
}

func TestParseCommandLineArgsRejectsRemovedGoEnv(t *testing.T) {
	setTestCommandLine(t, "spx", "run", "--goenv", "/tmp/goenv")

	cmd := &CmdTool{}
	help := cmd.initializeFlags()
	if err := cmd.parseCommandLineArgs(help); err == nil {
		t.Fatal("legacy --goenv flag was accepted")
	}
}

func TestInternalExportPackCommand(t *testing.T) {
	spec, ok := findCommand("exportpack")
	if !ok || !spec.hidden || spec.run == nil {
		t.Fatalf("exportpack spec = %+v, found %v", spec, ok)
	}
}

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
			setTestCommandLine(t, "spx", name, "--path", root)

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
			setTestCommandLine(t, tt.args...)

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
			setTestCommandLine(t, "spx", tt.name)

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

func TestRunCmdBuildsOnceBeforeImportAndAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake build tools use POSIX shell scripts")
	}
	type testCase struct {
		name, command, tags, failure string
		cache                        bool
		want, errorContext           string
	}
	// BuildDll builds both host architectures on macOS.
	dllBuild := "dll"
	if runtime.GOOS == "darwin" {
		dllBuild = "dll dll"
	}
	built := "tidy generate tidy " + dllBuild
	var cases []testCase
	for _, command := range []string{"build", "editor", "rune", "export", "exportpack"} {
		action := " run"
		if command == "build" {
			action = ""
		}
		cases = append(cases,
			testCase{name: command + "/fresh", command: command, want: built + " import" + action},
			testCase{name: command + "/cached", command: command, cache: true, want: built + action},
		)
	}
	cases = append(cases,
		testCase{name: "generation failure", command: "editor", failure: "generate", want: "tidy generate", errorContext: "code generation failed"},
		testCase{name: "tidy failure", command: "editor", failure: "tidy:2", want: "tidy generate tidy", errorContext: "go mod tidy failed"},
		testCase{name: "build failure", command: "editor", failure: "dll", want: "tidy generate tidy dll", errorContext: "go shared-library build"},
		testCase{name: "import failure", command: "editor", failure: "import", want: built + " import", errorContext: "godot import failed"},
		testCase{name: "cached generation failure", command: "editor", cache: true, failure: "generate", want: "tidy generate", errorContext: "code generation failed"},
		testCase{name: "cached tidy failure", command: "editor", cache: true, failure: "tidy:2", want: "tidy generate tidy", errorContext: "go mod tidy failed"},
		testCase{name: "cached build failure", command: "editor", cache: true, failure: "dll", want: "tidy generate tidy dll", errorContext: "go shared-library build"},
		testCase{name: "template fresh", command: "exporttemplateweb", want: built + " import run"},
		testCase{name: "template cached", command: "exporttemplateweb", cache: true, want: "tidy run"},
		testCase{name: "runtime skips import", command: "runnative", want: built + " run"},
		testCase{name: "pure engine skips import and DLL", command: "build", tags: "pure_engine", want: "tidy"},
		testCase{name: "wasm skips import", command: "buildweb", want: "tidy generate tidy wasm"},
		testCase{name: "tinygo skips import", command: "buildtinygo", want: "tidy generate tidy tinygo"},
		testCase{name: "init skips import", command: "init"},
		testCase{name: "clear skips import", command: "clear"},
		testCase{name: "clearbuild skips import", command: "clearbuild"},
		// Mobile exports still build after staging assets; stop before SDK work.
		testCase{name: "apk retains export build", command: "exportapk", failure: "generate:2", want: built + " import generate", errorContext: "code generation failed"},
		testCase{name: "ios retains export build", command: "exportios", failure: "generate:2", want: built + " import generate", errorContext: "code generation failed"},
	)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			project := filepath.Join(root, "source project")
			binDir := filepath.Join(root, "gopath", "bin")
			logPath := filepath.Join(root, "commands.log")
			for _, dir := range []string{filepath.Join(project, "assets"), binDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"main.spx", filepath.Join("assets", "index.json")} {
				if err := os.WriteFile(filepath.Join(project, name), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.cache {
				writeProjectImportCache(t, filepath.Join(project, "project"))
			}
			const script = `#!/bin/sh
set -eu
tool="${0##*/}"
case "$tool:$1" in
    gdspxtest:--version) exit 0 ;;
    xgo:go) action=generate ;;
    go:mod) action=tidy ;;
    go:build) action=wasm; for arg in "$@"; do if [ "$arg" = -buildmode=c-shared ]; then action=dll; fi; done ;;
    tinygo:build) action=tinygo ;;
    gdspx*) action=run; for arg in "$@"; do if [ "$arg" = --import ]; then action=import; fi; done ;;
    *) exit 97 ;;
esac
printf '%s\n' "$action" >> "$SPX_TEST_ORDER_LOG"
count=0
while IFS= read -r previous; do
    if [ "$previous" = "$action" ]; then count=$((count + 1)); fi
done < "$SPX_TEST_ORDER_LOG"
if [ "$SPX_TEST_ORDER_FAIL" = "$action" ] || [ "$SPX_TEST_ORDER_FAIL" = "$action:$count" ]; then exit 23; fi
if [ "$action" = generate ]; then printf 'package main\n' > xgo_autogen.go; fi
if [ "$action" = dll ]; then
    while [ "$#" -gt 0 ]; do
        if [ "$1" = -o ]; then mkdir -p "${2%/*}"; printf 'fake bridge\n' > "$2"; break; fi
        shift
    done
fi
`
			for _, tool := range []string{"go", "xgo", "tinygo", "gdspxtest", "gdspxrttest"} {
				if err := os.WriteFile(filepath.Join(binDir, tool), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GOPATH", filepath.Dir(binDir))
			t.Setenv(projectImportTimeoutEnvVar, "0")
			t.Setenv("SPX_TEST_ORDER_LOG", logPath)
			t.Setenv("SPX_TEST_ORDER_FAIL", tt.failure)
			for _, key := range []string{"GOOS", "GOARCH", "GODEBUG"} {
				t.Setenv(key, "")
			}
			setTestCommandLine(t, "spx", tt.command, "--path", project, "--tags", tt.tags)

			cmd := &CmdTool{}
			err = cmd.RunCmd("spx", ".spx", "test", webExportTestFS, "template/project", "project")
			if tt.errorContext == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.errorContext) {
				t.Fatalf("RunCmd error = %v, want %q", err, tt.errorContext)
			}
			data, err := os.ReadFile(logPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if got := strings.Join(strings.Fields(string(data)), " "); got != tt.want {
				t.Fatalf("command order = %q, want %q", got, tt.want)
			}
		})
	}
}
