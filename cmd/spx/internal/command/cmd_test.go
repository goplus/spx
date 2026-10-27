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
	"bytes"
	"embed"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/cmd/spx/internal/command/builderai"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

func TestRunCmdRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--goenv", "/tmp/goenv"},
		{"--headless=invalid"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			setTestCommandLine(t, append([]string{"spx", "run"}, args...)...)
			cmd := &CmdTool{}
			if err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", "project"); err == nil {
				t.Fatalf("RunCmd accepted invalid flags %v", args)
			}
			if cmd.ProjectDir != "" || cmd.TargetDir != "" {
				t.Fatalf("invalid flags prepared a project: %+v", cmd)
			}
		})
	}
}

func TestExtraArgsString(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{"defaults", nil, []string{"--path", "."}},
		{"empty path", []string{"--path="}, nil},
		{
			"runtime flags",
			[]string{"--path", "game with spaces", "-serveraddr", "localhost:1234", "-servermode", "-controller", "Player", "--headless", "--nomap", "--debugweb", "--fullscreen", "-v"},
			[]string{"--path", "game with spaces", "-serveraddr", "localhost:1234", "-servermode", "-controller", "Player", "--headless", "--nomap", "--debugweb", "--fullscreen", "-v"},
		},
		{
			"build flags stay in CLI",
			[]string{"--path=", "-arch", "arm64", "-tags", "custom_tag", "-target", "board", "-build", "fast", "-mode", "worker", "-o", "launcher", "-install", "-onlys", "-onlyc", "-movie"},
			nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setTestCommandLine(t, append([]string{"spx", "run"}, test.args...)...)
			cmd := &CmdTool{}
			if err := cmd.parseCommandLineArgs(cmd.initializeFlags()); err != nil {
				t.Fatal(err)
			}
			if got := cmd.Args.String(); !slices.Equal(got, test.want) {
				t.Fatalf("runtime arguments = %q, want %q", got, test.want)
			}
		})
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
				cmd := &CmdTool{TargetDir: root}
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
		failures := []string{"", "xgo:go", "go:mod", build.tool + ":build"}
		if build.name == "shared" {
			failures = append(failures, "missing library", "invalid library")
		}
		for _, failure := range failures {
			// Mobile cases exercise early failure, not SDK builds or packaging.
			if build.mobileExport && failure == "" {
				continue
			}
			name := failure
			if name == "" {
				name = "success"
			}
			t.Run(build.name+"/"+name, func(t *testing.T) {
				libraryError := failure == "missing library" || failure == "invalid library"
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

				target, tags, arch := "", "fixture_tag", runtime.GOARCH
				install := false
				cmd := &CmdTool{
					TargetDir:  projectRoot,
					ProjectDir: filepath.Join(projectRoot, ".generated", "engine"),
					GoDir:      filepath.Join(projectRoot, ".generated", "engine", "go"),
					LibPath:    filepath.Join(root, "spx-1-"+arch+".so"),
					CmdPath:    filepath.Join(binDir, "godot"),
					Args: ExtraArgs{
						Target:  &target,
						Tags:    &tags,
						Arch:    &arch,
						Install: &install,
					},
				}
				switch failure {
				case "":
					if build.name == "shared" {
						cmd.Args.Arch = nil
					}
				case "missing library":
					cmd.LibPath = ""
				case "invalid library":
					cmd.LibPath = filepath.Join(root, "bridge.so")
				}
				const staleMain = "package main\n// previous generation\n"
				if build.mobileExport {
					// A failed regeneration must not export a previous main.go.
					for name, content := range map[string]string{
						filepath.Join(projectRoot, "assets", "index.json"): "{}",
						filepath.Join(cmd.GoDir, "main.go"):                staleMain,
						filepath.Join(cmd.GoDir, "ios_fixture.go.txt"):     "package main\n",
					} {
						writeCommandTestFile(t, name, content)
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
				if libraryError {
					if err == nil || !strings.Contains(err.Error(), "unexpected library path format") {
						t.Fatalf("build error = %v, want invalid library path", err)
					}
					if cmd.LibPath != "" {
						t.Fatalf("failed build retained library path %q", cmd.LibPath)
					}
				} else if failure == "" {
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
					if (tool == "xgo" || strings.HasPrefix(args, "build ") && build.name != "wasm") && !strings.Contains(args, "-tags="+tags) {
						t.Fatalf("build omitted tags: %q", call)
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
				if failure == "xgo:go" || failure == "go:mod" || libraryError {
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
		testCase{name: "custom build tags retain import and DLL", command: "build", tags: "custom_tag", want: built + " import"},
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

func TestInitCreatesProjectScaffold(t *testing.T) {
	for _, mode := range []string{"current", "relative", "absolute"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			target, projectDir := ".", root
			if mode != "current" {
				projectDir = filepath.Join(root, "nested", "project")
				target = filepath.Join("nested", "project")
				if mode == "absolute" {
					target = projectDir
				}
			} else {
				for _, name := range []string{"go.mod", "main.spx", "assets/index.json"} {
					writeCommandTestFile(t, filepath.Join(projectDir, name), "stale")
				}
			}
			const module = "module example.com/new-game\n\ngo 1.23\n"
			cmd := &CmdTool{Args: ExtraArgs{Path: &target}, GoModTemplate: module}
			if err := cmd.Init(); err != nil {
				t.Fatal(err)
			}
			if got := readTestFile(t, filepath.Join(projectDir, "go.mod")); got != module {
				t.Fatalf("go.mod = %q, want %q", got, module)
			}
			main := readTestFile(t, filepath.Join(projectDir, "main.spx"))
			if !strings.Contains(main, "onStart => {") || !strings.Contains(main, `println("Hello, SPX!")`) {
				t.Fatalf("main.spx is missing the startup example: %q", main)
			}
			index := readTestFile(t, filepath.Join(projectDir, "assets", "index.json"))
			if !strings.Contains(index, `"width":480`) || !strings.Contains(index, `"height":360`) {
				t.Fatalf("project dimensions are missing: %q", index)
			}
			assertWorkingDirectory(t, root)
		})
	}
}

func TestInitReportsScaffoldWriteFailures(t *testing.T) {
	for _, tt := range []struct {
		name, blocked, want string
		file                bool
	}{
		{"target directory", "", "failed to create target directory", true},
		{"assets directory", "assets", "failed to create assets directory", true},
		{"asset index", "assets/index.json", "failed to create assets/index.json", false},
		{"main program", "main.spx", "failed to create main.spx", false},
		{"module file", "go.mod", "failed to create go.mod", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "project")
			blocked := filepath.Join(target, tt.blocked)
			if tt.file {
				writeCommandTestFile(t, blocked, "occupied")
			} else if err := os.MkdirAll(blocked, 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := &CmdTool{Args: ExtraArgs{Path: &target}, GoModTemplate: "module example.com/new-game\n"}
			if err := cmd.Init(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Init error = %v, want %q", err, tt.want)
			}
			if tt.file && readTestFile(t, blocked) != "occupied" {
				t.Fatal("initialization overwrote the blocking file")
			}
		})
	}
}

func TestCommandRejectsInvalidPhases(t *testing.T) {
	cmd := &CmdTool{Args: ExtraArgs{Tags: launcherStringPointer("custom_tag")}}
	if err := cmd.prepareCommand(commandSpec{name: "invalid", setup: 99}, "", ""); err == nil || !strings.Contains(err.Error(), "invalid setup") {
		t.Fatalf("setup error = %v", err)
	}
	if err := cmd.buildCommand(commandSpec{build: 99}); err == nil || !strings.Contains(err.Error(), "invalid command build phase") {
		t.Fatalf("build error = %v", err)
	}
	if got := cmd.SafeTagArgs(); got != "-tags=custom_tag" {
		t.Fatalf("tag argument = %q", got)
	}
}

func TestEngineCommandsUseGeneratedProjectDirectory(t *testing.T) {
	for _, command := range []string{"editor", "rune"} {
		t.Run(command, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(root, "generated project")
			if err := os.Mkdir(project, 0o755); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(root, "engine.log")
			t.Setenv("SPX_TEST_ENGINE_LOG", logPath)
			bin := installCommandTestTools(t, map[string]string{
				"godot": `printf '%s\000' "$PWD" "$@" > "$SPX_TEST_ENGINE_LOG"`,
			})
			setTestCommandLine(t, "spx", command, "--path", root)
			cmd := CmdTool{CmdPath: filepath.Join(bin, "godot")}
			help := cmd.initializeFlags()
			if err := cmd.parseCommandLineArgs(help); err != nil {
				t.Fatal(err)
			}
			before := workingDirectory(t)
			if err := cmd.setupPaths("generated project"); err != nil {
				t.Fatal(err)
			}
			if command == "editor" {
				err = cmd.executeEditor()
			} else {
				err = cmd.executeRune()
			}
			if err != nil {
				t.Fatal(err)
			}
			want := project + "\x00--path\x00" + project + "\x00"
			if command == "editor" {
				want += "-e\x00"
			}
			if got := readTestFile(t, logPath); got != want {
				t.Fatalf("Engine invocation = %q, want %q", got, want)
			}
			if *cmd.Args.Path != root {
				t.Fatalf("Engine invocation changed source path to %q", *cmd.Args.Path)
			}
			assertWorkingDirectory(t, before)
		})
	}
}

func TestBuildRejectsBlockedProjectFiles(t *testing.T) {
	for _, scenario := range []string{"AI module", "Go directory", "generated main", "wasm output", "TinyGo output"} {
		t.Run(scenario, func(t *testing.T) {
			installCommandTestTools(t, map[string]string{
				"xgo": `printf 'package main\n' > xgo_autogen.go`,
				"go":  `test "$*" = 'mod tidy'`,
			})
			root := t.TempDir()
			target, tags := "", ""
			cmd := &CmdTool{TargetDir: root, ProjectDir: filepath.Join(root, "project"), GoDir: filepath.Join(root, "project", "go"), Args: ExtraArgs{Target: &target, Tags: &tags}}
			want := ""
			switch scenario {
			case "AI module":
				writeCommandTestFile(t, filepath.Join(root, builderai.DescriptionFile), "summary")
				writeCommandTestFile(t, filepath.Join(root, "gox.mod", "keep"), "existing directory")
				want = "failed to read gox.mod"
			case "Go directory":
				writeCommandTestFile(t, cmd.GoDir, "existing file")
				want = "failed to create GoDir"
			case "generated main":
				writeCommandTestFile(t, filepath.Join(cmd.GoDir, "main.go", "keep"), "existing directory")
				want = "failed to rename/move generated file"
			default:
				writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, ".builds"), "existing file")
				want = "failed to create web build directory"
				if scenario == "TinyGo output" {
					want = "failed to create TinyGo build directory"
				}
			}
			var err error
			if scenario == "TinyGo output" {
				err = cmd.BuildTinyGoLib()
			} else {
				err = cmd.BuildWasm()
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("build error = %v, want %q", err, want)
			}
			if scenario == "AI module" {
				if _, err := os.Stat(filepath.Join(root, "xgo_autogen.go")); !os.IsNotExist(err) {
					t.Fatalf("ran XGo after module preparation failed: %v", err)
				}
			}
		})
	}
}

func TestTinyGoBuildOptions(t *testing.T) {
	for _, test := range []struct {
		target, tags, wantTarget string
	}{
		{"", "", "esp32-coreboard-v2"},
		{"esp32", "", "esp32-coreboard-v2"},
		{"pico", "fixture_tag", "pico"},
	} {
		t.Run(test.target, func(t *testing.T) {
			root := t.TempDir()
			argsPath := filepath.Join(root, "tinygo-args")
			t.Setenv("SPX_TEST_TINYGO_ARGS", argsPath)
			installCommandTestTools(t, map[string]string{
				"xgo":    `printf 'package main\n' > xgo_autogen.go`,
				"go":     "",
				"tinygo": `printf '%s\n' "$@" > "$SPX_TEST_TINYGO_ARGS"`,
			})
			cmd := &CmdTool{
				TargetDir: root, ProjectDir: filepath.Join(root, "project"), GoDir: filepath.Join(root, "project", "go"),
				Args: ExtraArgs{Target: &test.target, Tags: &test.tags},
			}
			if test.target == "" {
				cmd.Args.Tags = nil
			}
			if err := cmd.BuildTinyGoLib(); err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(readTestFile(t, argsPath)), "\n")
			if !slices.Contains(args, "-target="+test.wantTarget) {
				t.Fatalf("TinyGo args = %q, want target %q", args, test.wantTarget)
			}
			var tags []string
			for _, arg := range args {
				if strings.HasPrefix(arg, "-tags=") {
					tags = append(tags, strings.TrimPrefix(arg, "-tags="))
				}
			}
			var wantTags []string
			if test.tags != "" {
				wantTags = []string{test.tags}
			}
			if !slices.Equal(tags, wantTags) {
				t.Fatalf("TinyGo tags = %q, want %q", tags, wantTags)
			}
		})
	}
}

func TestSharedBuildArchitectureSelection(t *testing.T) {
	for _, platform := range []struct {
		goos, hostArch string
		supported      []string
	}{
		{"darwin", "arm64", []string{"amd64", "arm64"}},
		{"windows", "amd64", []string{"amd64", "386"}},
		{"linux", "arm64", []string{"amd64", "arm", "arm64", "386"}},
		{"freebsd", "386", []string{"386"}},
	} {
		for _, arch := range append([]string{"", "all", "invalid-architecture"}, platform.supported...) {
			t.Run(platform.goos+"/"+arch, func(t *testing.T) {
				got, err := determineTargetArchs(platform.goos, platform.hostArch, arch)
				if arch == "invalid-architecture" && platform.goos != "darwin" {
					wantError := "invalid arch " + arch + ". Valid archs for " + platform.goos + ": " + strings.Join(platform.supported, ",")
					if got != nil || err == nil || err.Error() != wantError {
						t.Fatalf("architectures = %v, %v; want error %q", got, err, wantError)
					}
					return
				}
				want := []string{arch}
				if arch == "" {
					want = []string{platform.hostArch}
				}
				if arch == "all" || platform.goos == "darwin" {
					want = platform.supported
				}
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("architectures = %v, %v; want %v", got, err, want)
				}
			})
		}
	}
	if runtime.GOOS != "darwin" {
		arch := "invalid-architecture"
		cmd := &CmdTool{ProjectDir: t.TempDir(), Args: ExtraArgs{Arch: &arch}}
		if err := cmd.BuildDll(); err == nil || !strings.Contains(err.Error(), "invalid arch") {
			t.Fatalf("invalid architecture error = %v", err)
		}
	}
}

func TestSharedBuildReportsMissingHostLibrary(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS always builds both supported host architectures")
	}
	arch, tags := "all", ""
	root := t.TempDir()
	cmd := &CmdTool{
		TargetDir: root, ProjectDir: filepath.Join(root, "project"), GoDir: filepath.Join(root, "project", "go"),
		LibPath: filepath.Join(root, "spx-test-"+runtime.GOARCH+".so"),
		Args:    ExtraArgs{Arch: &arch, Tags: &tags},
	}
	supported, err := determineTargetArchs(runtime.GOOS, runtime.GOARCH, arch)
	if err != nil {
		t.Fatal(err)
	}
	arch = ""
	for _, candidate := range supported {
		if candidate != runtime.GOARCH {
			arch = candidate
			break
		}
	}
	if arch == "" {
		t.Skip("platform has no supported cross-build architecture")
	}
	logPath := filepath.Join(root, "architecture.log")
	t.Setenv("SPX_TEST_ARCH_LOG", logPath)
	installCommandTestTools(t, map[string]string{
		"xgo": `printf 'package main\n' > xgo_autogen.go`,
		"go":  `if [ "$1" = build ]; then printf '%s' "$GOARCH" > "$SPX_TEST_ARCH_LOG"; fi`,
	})
	if err := cmd.BuildDll(); err == nil || !strings.Contains(err.Error(), "cannot find matched dylib for runtime arch "+runtime.GOARCH) {
		t.Fatalf("cross-build error = %v, want missing host library", err)
	}
	if cmd.LibPath != "" || readTestFile(t, logPath) != arch {
		t.Fatalf("cross-build retained host library %q or did not build %s", cmd.LibPath, arch)
	}
}

func TestHideIOSFilesPreservesDirectoryBoundaries(t *testing.T) {
	for _, scenario := range []string{"literal project path", "missing directory", "unreadable directory", "blocked destination"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			cmd := &CmdTool{ProjectDir: filepath.Join(root, "project[one]")}
			goDir := filepath.Join(cmd.ProjectDir, "go")
			var warnings bytes.Buffer
			spxlog.SetOutput(&warnings)
			t.Cleanup(func() {
				spxlog.SetStdoutOutput(os.Stdout)
				spxlog.SetStderrOutput(os.Stderr)
			})

			if scenario == "missing directory" {
				cmd.hideIOSFiles()
				if warnings.Len() != 0 {
					t.Fatalf("first build warned about missing Go directory: %s", warnings.String())
				}
				return
			}
			if scenario == "unreadable directory" {
				writeCommandTestFile(t, goDir, "occupied")
				cmd.hideIOSFiles()
				if !strings.Contains(warnings.String(), goDir) || readTestFile(t, goDir) != "occupied" {
					t.Fatalf("directory failure did not preserve its source or report its path: %s", warnings.String())
				}
				return
			}

			writeCommandTestFile(t, filepath.Join(goDir, "ios_main.go"), "iOS source")
			writeCommandTestFile(t, filepath.Join(goDir, "ios_saved.go.txt"), "already hidden")
			writeCommandTestFile(t, filepath.Join(goDir, "main.go"), "main source")
			otherSource := filepath.Join(root, "projecto", "go", "ios_other.go")
			writeCommandTestFile(t, otherSource, "other project")
			if scenario == "blocked destination" {
				writeCommandTestFile(t, filepath.Join(goDir, "ios_blocked.go"), "blocked source")
				writeCommandTestFile(t, filepath.Join(goDir, "ios_blocked.go.txt", "keep"), "existing directory")
			}
			cmd.hideIOSFiles()
			if readTestFile(t, filepath.Join(goDir, "ios_main.go.txt")) != "iOS source" {
				t.Fatal("iOS source contents changed")
			}
			if _, err := os.Stat(filepath.Join(goDir, "ios_main.go")); !os.IsNotExist(err) {
				t.Fatalf("iOS source was not hidden: %v", err)
			}
			if readTestFile(t, filepath.Join(goDir, "ios_saved.go.txt")) != "already hidden" ||
				readTestFile(t, filepath.Join(goDir, "main.go")) != "main source" || readTestFile(t, otherSource) != "other project" {
				t.Fatal("hiding iOS sources changed an excluded file or another project")
			}
			if scenario == "blocked destination" {
				if readTestFile(t, filepath.Join(goDir, "ios_blocked.go")) != "blocked source" ||
					readTestFile(t, filepath.Join(goDir, "ios_blocked.go.txt", "keep")) != "existing directory" ||
					!strings.Contains(warnings.String(), "Failed to rename") {
					t.Fatalf("rename failure did not preserve both paths and warn: %s", warnings.String())
				}
			} else if warnings.Len() != 0 {
				t.Fatalf("unexpected warning: %s", warnings.String())
			}
		})
	}
}

func writeCommandTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func setTestCommandLine(t *testing.T, args ...string) {
	t.Helper()
	oldFlags, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = args
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
}

// installCommandTestTools isolates command lookup so no host SDK or browser can
// accidentally run when a test exercises external-tool orchestration.
func installCommandTestTools(t *testing.T, scripts map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake command tools use POSIX shell scripts")
	}
	binDir := t.TempDir()
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nset -eu\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	return binDir
}

// Restrict only a temporary test fixture; restore its mode before TempDir cleanup.
func restrictCommandTestPath(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions enforced for an unprivileged user")
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(name, info.Mode().Perm()); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore fixture permissions: %v", err)
		}
	})
}
