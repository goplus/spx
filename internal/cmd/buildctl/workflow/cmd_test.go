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

package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

type recordedCommand struct {
	dir  string
	name string
	args []string
}

type recordedCall struct {
	script string
	args   []string
}

type recordingRunner struct {
	calls       []recordedCall
	commands    []recordedCommand
	repoRoot    string
	demos       []string
	stopCalls   int
	scriptErr   error
	commandErr  error
	listErr     error
	stopErr     error
	commandHook func(string, string, ...string) error
}

func TestWorkflowDispatchAndArgumentErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: errUsage},
		{name: "help", args: []string{"help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown workflow command "unknown"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Run(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Run(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains == "" {
				if err != nil {
					t.Fatalf("Run(%q) returned error: %v", test.args, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Run(%q) error = %v, want substring %q", test.args, err, test.contains)
			}
		})
	}

	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing index", args: nil, contains: "--demo-index must be greater than 0"},
		{name: "zero index", args: []string{"--demo-index", "0"}, contains: "--demo-index must be greater than 0"},
		{name: "invalid mode", args: []string{"--demo-index", "1", "--mode", "unknown"}, contains: "unsupported demo mode"},
		{name: "invalid port", args: []string{"--demo-index", "1", "--port", "0"}, contains: "--port must be greater than 0"},
		{name: "positional argument", args: []string{"--demo-index", "1", "extra"}, wantErr: errUsage},
	} {
		t.Run("run-demo/reject/"+test.name, func(t *testing.T) {
			_, err := parseWorkflowRunDemoArgs(test.args)
			if err == nil {
				t.Fatalf("parseWorkflowRunDemoArgs(%q) succeeded", test.args)
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("parseWorkflowRunDemoArgs(%q) error = %v, want %v", test.args, err, test.wantErr)
			}
			if test.contains != "" && !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("parseWorkflowRunDemoArgs(%q) error = %v, want substring %q", test.args, err, test.contains)
			}
		})
	}
}

func TestWorkflowLifecycleAndRunnerErrors(t *testing.T) {
	runner := &recordingRunner{demos: []string{"tutorial/00-Hello", "tutorial/01-Weather"}}
	if err := listDemosWorkflow(runner); err != nil {
		t.Fatalf("listDemosWorkflow returned error: %v", err)
	}
	if err := stopWebWorkflow(runner); err != nil {
		t.Fatalf("stopWebWorkflow returned error: %v", err)
	}
	if runner.stopCalls != 1 {
		t.Fatalf("stopWebServers calls = %d, want 1", runner.stopCalls)
	}

	if err := runDemoWorkflow(workflowRunDemoConfig{demoIndex: 3, mode: "run", movie: "false"}, runner); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("out-of-range demo error = %v, want range error", err)
	}

	sentinel := errors.New("runner failed")
	for _, test := range []struct {
		name string
		run  func(*recordingRunner) error
		set  func(*recordingRunner)
	}{
		{name: "list", set: func(r *recordingRunner) { r.listErr = sentinel }, run: func(r *recordingRunner) error { return listDemosWorkflow(r) }},
		{name: "stop", set: func(r *recordingRunner) { r.stopErr = sentinel }, run: func(r *recordingRunner) error { return stopWebWorkflow(r) }},
		{name: "command", set: func(r *recordingRunner) { r.commandErr = sentinel }, run: func(r *recordingRunner) error {
			return runDemoWorkflow(workflowRunDemoConfig{demoIndex: 1, mode: "run", movie: "false"}, r)
		}},
		{name: "script", set: func(r *recordingRunner) { r.scriptErr = sentinel }, run: func(r *recordingRunner) error {
			return runDemoWorkflow(workflowRunDemoConfig{demoIndex: 1, mode: "web", movie: "false", port: 8106}, r)
		}},
	} {
		t.Run("propagate/"+test.name, func(t *testing.T) {
			failed := &recordingRunner{demos: []string{"tutorial/00-Hello"}}
			test.set(failed)
			if err := test.run(failed); !errors.Is(err, sentinel) {
				t.Fatalf("error = %v, want runner error", err)
			}
		})
	}
}

func TestBuildRejectsUnknownTarget(t *testing.T) {
	runner := &recordingRunner{repoRoot: t.TempDir()}
	if err := Build(BuildConfig{Target: "unknown"}, runner); err == nil {
		t.Fatal("Build accepted an unknown target")
	}
}

func TestParseWorkflowRunDemoArgs(t *testing.T) {
	cfg, err := parseWorkflowRunDemoArgs([]string{"--demo-index", "2", "--mode", "web-worker", "--port", "8123", "--movie", "true"})
	if err != nil {
		t.Fatalf("parseWorkflowRunDemoArgs returned error: %v", err)
	}
	if cfg.demoIndex != 2 || cfg.mode != "web-worker" || cfg.port != 8123 || cfg.movie != "true" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseWorkflowInstallAPKArgsDefault(t *testing.T) {
	cfg, err := parseWorkflowInstallAPKArgs(nil)
	if err != nil {
		t.Fatalf("parseWorkflowInstallAPKArgs returned error: %v", err)
	}
	if cfg.projectDir != filepath.Join("tutorial", "00-Hello") {
		t.Fatalf("unexpected projectDir: %q", cfg.projectDir)
	}
}

func TestInstallAPKWorkflow(t *testing.T) {
	repoRoot := t.TempDir()
	projectDir := filepath.Join(repoRoot, "tutorial", "00-Hello")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	runner := &recordingRunner{repoRoot: repoRoot}
	if err := installAPKWorkflow(workflowInstallAPKConfig{}, runner); err != nil {
		t.Fatalf("installAPKWorkflow returned error: %v", err)
	}

	expectedCommands := []recordedCommand{
		{dir: filepath.Join(repoRoot, "cmd", "spx"), name: "go", args: []string{"run", ".", "exportapk", "--install", "--path", projectDir}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
}

func TestRunDemoWorkflowInterpreted(t *testing.T) {
	runner := &recordingRunner{
		demos: []string{"tutorial/00-Hello", "tutorial/01-Weather"},
	}

	err := runDemoWorkflow(workflowRunDemoConfig{
		demoIndex: 2,
		mode:      "run",
		movie:     "true",
		port:      8106,
	}, runner)
	if err != nil {
		t.Fatalf("runDemoWorkflow returned error: %v", err)
	}

	expectedCommands := []recordedCommand{
		{dir: "tutorial/01-Weather", name: "spx", args: []string{"run", "-movie=true"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	if runner.stopCalls != 0 {
		t.Fatalf("unexpected stopCalls: %d", runner.stopCalls)
	}
}

func TestRunDemoWorkflowNative(t *testing.T) {
	runner := &recordingRunner{
		demos: []string{"tutorial/00-Hello", "tutorial/01-Weather"},
	}

	err := runDemoWorkflow(workflowRunDemoConfig{
		demoIndex: 2,
		mode:      "runnative",
		movie:     "true",
		port:      8106,
	}, runner)
	if err != nil {
		t.Fatalf("runDemoWorkflow returned error: %v", err)
	}

	expectedCommands := []recordedCommand{
		{dir: "tutorial/01-Weather", name: "spx", args: []string{"runnative", "-movie=true"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	if runner.stopCalls != 0 {
		t.Fatalf("unexpected stopCalls: %d", runner.stopCalls)
	}
}

func TestRunDemoWorkflowWeb(t *testing.T) {
	for _, tt := range []struct {
		mode, command string
		port          int
		serverArg     string
	}{
		{"web-worker", "runwebworker", 8123, "-serveraddr=:8123"},
		{"web", "runweb", 8105, "-serveraddr=:8105"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			runner := &recordingRunner{demos: []string{"tutorial/00-Hello"}}
			err := runDemoWorkflow(workflowRunDemoConfig{
				demoIndex: 1,
				mode:      tt.mode,
				movie:     "false",
				port:      tt.port,
			}, runner)
			if err != nil {
				t.Fatalf("runDemoWorkflow returned error: %v", err)
			}
			expectedScripts := []recordedCall{
				{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}},
			}
			if !reflect.DeepEqual(runner.calls, expectedScripts) {
				t.Fatalf("unexpected script calls: %#v", runner.calls)
			}
			expectedCommands := []recordedCommand{
				{dir: "tutorial/00-Hello", name: "spx", args: []string{tt.command, tt.serverArg}},
			}
			if !reflect.DeepEqual(runner.commands, expectedCommands) {
				t.Fatalf("unexpected commands: %#v", runner.commands)
			}
			if runner.stopCalls != 0 {
				t.Fatalf("unexpected stopCalls: %d", runner.stopCalls)
			}
		})
	}
}

func TestWorkflowEntryPointValidation(t *testing.T) {
	for _, tt := range []struct {
		command string
		valid   []string
	}{
		{"install-apk", nil}, {"list-demos", nil}, {"open-template-editor", nil},
		{"run-demo", []string{"--demo-index", "1"}}, {"stop-web", nil},
	} {
		t.Run(tt.command, func(t *testing.T) {
			if err := Run([]string{tt.command, "--help"}); err != nil {
				t.Fatal(err)
			}
			if err := Run([]string{tt.command, "--invalid"}); err == nil {
				t.Fatal("invalid flag accepted")
			}
			t.Chdir(t.TempDir())
			if err := Run(append([]string{tt.command}, tt.valid...)); err == nil || !strings.Contains(err.Error(), "repository root") {
				t.Fatalf("outside repository = %v", err)
			}
		})
	}
	for _, args := range [][]string{
		{"install-apk", "--project-dir", ""}, {"install-apk", "extra"},
		{"open-template-editor", "--template-dir", ""}, {"open-template-editor", "extra"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%q) accepted invalid arguments", args)
		}
	}
}

func TestWorkflowEntryPointCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("isolated POSIX command fixtures")
	}
	for _, command := range []string{"install-apk", "list-demos", "open-template-editor", "run-demo", "stop-web"} {
		t.Run(command, func(t *testing.T) {
			root := newWorkflowFixture(t)
			project := filepath.Join(root, "tutorial", "00-Hello")
			mustMkdirAll(t, project)
			mustWriteFile(t, filepath.Join(root, defaultTemplateProjectDir, "project.godot"), nil)
			bin := filepath.Join(os.Getenv("GOPATH"), "bin")
			record := filepath.Join(root, "command")
			t.Setenv("WORKFLOW_RECORD", record)
			for _, name := range []string{"go", "spx", "gdspx" + release.DefaultRuntimeLock().RuntimeVersion} {
				writeWorkflowTool(t, filepath.Join(bin, name), "printf '%s\\n' \"$PWD\" \"$@\" > \"$WORKFLOW_RECORD\"\n")
			}
			writeWorkflowTool(t, filepath.Join(bin, "pgrep"), "exit 1\n")
			t.Setenv("PATH", bin)
			args := []string{command}
			if command == "run-demo" {
				args = append(args, "--demo-index", "1", "--mode", "editor", "--movie", "true")
			}
			if err := Run(args); err != nil {
				t.Fatal(err)
			}
			want := []string{}
			switch command {
			case "install-apk":
				want = []string{filepath.Join(root, "cmd", "spx"), "run", ".", "exportapk", "--install", "--path", project}
			case "open-template-editor":
				want = []string{root, "--path", filepath.Join(root, defaultTemplateProjectDir), "--editor", "--recovery-mode"}
			case "run-demo":
				want = []string{project, "editor", "-movie=true"}
			}
			data, err := os.ReadFile(record)
			if len(want) == 0 {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("read-only workflow ran command: %q, %v", data, err)
				}
			} else if err != nil || string(data) != strings.Join(want, "\n")+"\n" {
				t.Fatalf("command = %q, %v; want %q", data, err, want)
			}
		})
	}
}

func TestWorkflowAdditionalFailurePaths(t *testing.T) {
	failure := errors.New("fixture failure")
	for _, mode := range []string{"editor", "rune", "unknown", "web-worker"} {
		runner := &recordingRunner{demos: []string{"tutorial/first"}}
		if mode == "web-worker" {
			runner.scriptErr = failure
		}
		err := runDemoWorkflow(workflowRunDemoConfig{demoIndex: 1, mode: mode, movie: "true"}, runner)
		switch mode {
		case "unknown":
			if err == nil || !strings.Contains(err.Error(), "unsupported demo mode") {
				t.Fatalf("unknown mode = %v", err)
			}
		case "web-worker":
			if !errors.Is(err, failure) || len(runner.commands) != 0 {
				t.Fatalf("web failure = %v, commands = %v", err, runner.commands)
			}
		default:
			if err != nil || len(runner.commands) != 1 || !reflect.DeepEqual(runner.commands[0].args, []string{mode, "-movie=true"}) {
				t.Fatalf("%s command = %v, %v", mode, runner.commands, err)
			}
		}
	}
	if err := runDemoWorkflow(workflowRunDemoConfig{demoIndex: 1}, &recordingRunner{listErr: failure}); !errors.Is(err, failure) {
		t.Fatalf("list failure = %v", err)
	}
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "file"), nil)
	for _, project := range []string{filepath.Join(root, "missing"), filepath.Join(root, "file")} {
		runner := &recordingRunner{repoRoot: root}
		if err := installAPKWorkflow(workflowInstallAPKConfig{projectDir: project}, runner); err == nil || len(runner.commands) != 0 {
			t.Fatalf("invalid project %s accepted: %v", project, err)
		}
	}
}

func TestBuildWorkflowsWithOfflineTools(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("fixture executables require Linux or macOS")
	}
	for _, target := range []string{"dev", "editor", "desktop", "web", "android", "ios"} {
		t.Run(target, func(t *testing.T) {
			runner := newWorkflowBuildFixture(t)
			if err := Build(BuildConfig{Target: target, Mode: "normal"}, runner); err != nil {
				t.Fatal(err)
			}
			wantScripts := []recordedCall{{script: "cmd/spx/install.sh"}}
			if target == "dev" {
				wantScripts[0].args = []string{"--web"}
			}
			if !reflect.DeepEqual(runner.calls, wantScripts) {
				t.Fatalf("tool installation=%v,want %v", runner.calls, wantScripts)
			}
			wantExports := map[string][]string{"dev": {"exportpack", "exporttemplateweb"}, "desktop": {"exportpack"}, "web": {"exporttemplateweb"}}[target]
			var gotExports []string
			for _, cmd := range runner.commands {
				gotExports = append(gotExports, cmd.args[2])
			}
			if !reflect.DeepEqual(gotExports, wantExports) {
				t.Fatalf("exports=%v,want %v", gotExports, wantExports)
			}
			testRecord, err := os.ReadFile(filepath.Join(runner.repoRoot, "module-tests"))
			if target == "dev" {
				if err != nil || string(testRecord) != "list\ntest\n" {
					t.Fatalf("module tests = %q (%v), want list then test", testRecord, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s unexpectedly ran module tests: %q (%v)", target, testRecord, err)
			}
			if entries, err := os.ReadDir(filepath.Join(runner.repoRoot, ".tmp")); err == nil && len(entries) != 0 {
				t.Fatalf("temporary export workspaces leaked: %v", entries)
			}
		})
	}
}

func TestBuildWorkflowFailureStopsSubsequentSteps(t *testing.T) {
	failure := errors.New("tool install failed")
	for _, target := range []string{"editor", "desktop", "web", "android", "ios"} {
		runner := &recordingRunner{scriptErr: failure}
		if err := Build(BuildConfig{Target: target}, runner); !errors.Is(err, failure) || len(runner.calls) != 1 || len(runner.commands) != 0 {
			t.Fatalf("%s install failure=%v,calls=%v,commands=%v", target, err, runner.calls, runner.commands)
		}
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("fixture executables require Linux or macOS")
	}
	for _, stage := range []string{"editor", "test-list", "test-empty", "test-run", "host", "web", "install"} {
		t.Run("dev/"+stage, func(t *testing.T) {
			runner := newWorkflowBuildFixture(t)
			t.Setenv("WORKFLOW_FAIL", stage)
			if stage == "editor" {
				t.Setenv("SPX_MODULE_SRC", filepath.Join(runner.repoRoot, "missing-module"))
			}
			if stage == "install" {
				runner.scriptErr = failure
			}
			err := Build(BuildConfig{Target: "dev", Mode: "normal"}, runner)
			if err == nil {
				t.Fatal("failed build succeeded")
			}
			wantExports := map[string]int{"editor": 0, "test-list": 0, "test-empty": 0, "test-run": 0, "host": 0, "web": 1, "install": 2}[stage]
			if len(runner.commands) != wantExports {
				t.Fatalf("exports after %s failure=%v,want %d", stage, runner.commands, wantExports)
			}
			if stage == "install" {
				if !errors.Is(err, failure) {
					t.Fatalf("installer error=%v", err)
				}
			} else if len(runner.calls) != 0 {
				t.Fatalf("failed build installed tools: %v", runner.calls)
			}
			if strings.HasPrefix(stage, "test-") {
				if _, err := os.Stat(filepath.Join(runner.repoRoot, "template-built")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("module test failure continued into template compilation")
				}
			}
		})
	}
	for _, target := range []string{"desktop", "web"} {
		t.Run(target+"/engine failure", func(t *testing.T) {
			runner := newWorkflowBuildFixture(t)
			t.Setenv("SPX_MODULE_SRC", filepath.Join(runner.repoRoot, "missing-module"))
			if err := Build(BuildConfig{Target: target, Mode: "normal"}, runner); err == nil || len(runner.commands) != 0 {
				t.Fatalf("engine failure=%v,commands=%v", err, runner.commands)
			}
		})
	}
}

func TestParseWorkflowOpenTemplateEditorArgsDefault(t *testing.T) {
	cfg, err := parseWorkflowOpenTemplateEditorArgs(nil)
	if err != nil {
		t.Fatalf("parseWorkflowOpenTemplateEditorArgs returned error: %v", err)
	}
	if cfg.templateDir != defaultTemplateProjectDir {
		t.Fatalf("unexpected templateDir: %q", cfg.templateDir)
	}
}

func TestOpenTemplateEditorWorkflow(t *testing.T) {
	repoRoot := t.TempDir()
	templateProjectDir := filepath.Join(repoRoot, "cmd", "spx", "template", "project")

	mustMkdirAll(t, templateProjectDir)
	mustWriteFile(t, filepath.Join(templateProjectDir, "project.godot"), []byte(`[application]`+"\n"+`config/name="spx"`+"\n"))

	runner := &recordingRunner{repoRoot: repoRoot}
	cfg := workflowOpenTemplateEditorConfig{}
	if err := openTemplateEditorWorkflow(cfg, runner); err != nil {
		t.Fatalf("openTemplateEditorWorkflow returned error: %v", err)
	}

	expectedCommands := []recordedCommand{
		{dir: repoRoot, name: "gdspx" + release.DefaultRuntimeLock().RuntimeVersion, args: []string{"--path", templateProjectDir, "--editor", "--recovery-mode"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
}

func TestResolveTemplateEditorPathAcceptsCustomProjectDirName(t *testing.T) {
	repoRoot := t.TempDir()
	templateProjectDir := filepath.Join(repoRoot, "custom-template-dir")

	mustMkdirAll(t, templateProjectDir)
	mustWriteFile(t, filepath.Join(templateProjectDir, "project.godot"), []byte(`[application]`+"\n"+`config/name="spx"`+"\n"))

	got, err := resolveTemplateEditorPath(templateProjectDir, repoRoot)
	if err != nil {
		t.Fatalf("resolveTemplateEditorPath returned error: %v", err)
	}
	if got != templateProjectDir {
		t.Fatalf("resolveTemplateEditorPath = %q, want %q", got, templateProjectDir)
	}
}

func TestTemplateEditorRejectsInvalidProjects(t *testing.T) {
	for _, scenario := range []string{"missing directory", "file directory", "missing project", "project directory"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(root, "project")
			want := "template project directory"
			switch scenario {
			case "file directory":
				mustWriteFile(t, project, nil)
				want = "not a directory"
			case "missing project":
				mustMkdirAll(t, project)
				want = "must contain project.godot"
			case "project directory":
				mustMkdirAll(t, filepath.Join(project, "project.godot"))
				want = "directory named project.godot"
			}
			runner := &recordingRunner{repoRoot: root}
			if err := openTemplateEditorWorkflow(workflowOpenTemplateEditorConfig{templateDir: project}, runner); err == nil || !strings.Contains(err.Error(), want) || len(runner.commands) != 0 {
				t.Fatalf("invalid project = %v, commands = %v", err, runner.commands)
			}
		})
	}
}

func (r *recordingRunner) RunScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return r.scriptErr
}

func (r *recordingRunner) RunCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if r.repoRoot != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(r.repoRoot, dir)
	}
	r.commands = append(r.commands, recordedCommand{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	if r.commandHook != nil {
		return r.commandHook(dir, name, args...)
	}
	return r.commandErr
}

func (r *recordingRunner) RepoRootDir() string {
	if r.repoRoot == "" {
		return "."
	}
	return r.repoRoot
}

func (r *recordingRunner) ListDemoDirs() ([]string, error) {
	return append([]string(nil), r.demos...), r.listErr
}

func (r *recordingRunner) StopWebServers() error {
	r.stopCalls++
	return r.stopErr
}

func newWorkflowBuildFixture(t *testing.T) *recordingRunner {
	t.Helper()
	root := newWorkflowFixture(t)
	for _, name := range []string{"SCsub", "config.py"} {
		mustWriteFile(t, filepath.Join(root, "module", name), nil)
	}
	mustWriteFile(t, filepath.Join(root, "module", "spx_scons_profile.json"), []byte(`{"schema":1,"common":[],"editor_release":[],"template_release":[]}`))
	lock := release.DefaultRuntimeLock()
	bin := filepath.Join(root, "tools")
	for _, name := range []string{"xgo", "go", "python3"} {
		writeWorkflowTool(t, filepath.Join(bin, name), "exit 0\n")
	}
	// Avoid consulting the host SDK or compiler when resolving macOS build flags.
	t.Setenv("SDKROOT", root)
	t.Setenv("CC", filepath.Join(bin, "go"))
	t.Setenv("CXX", filepath.Join(bin, "go"))
	for _, name := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"} {
		t.Setenv(name, "")
	}
	t.Setenv("WORKFLOW_ROOT", root)
	t.Setenv("WORKFLOW_FAIL", "")
	writeWorkflowTool(t, filepath.Join(bin, "git"), "printf '%s\\n' '"+lock.Godot.Commit+"'\n")
	venv := filepath.Join(root, ".bin", "scons-"+lock.Toolchain.SCons, "bin")
	writeWorkflowTool(t, filepath.Join(venv, "python"), "printf '%s\\n' '"+lock.Toolchain.SCons+"'\n")
	writeWorkflowTool(t, filepath.Join(venv, "scons"), `case "$*" in
  *tests=yes*) printf enabled > "$WORKFLOW_ROOT/module-tests-enabled" ;;
esac
case "$*" in
  *target=template_release*) printf built > "$WORKFLOW_ROOT/template-built" ;;
esac
case "$*" in
  *platform=linuxbsd*|*platform=macos*) [ "$WORKFLOW_FAIL" != host ] || exit 23 ;;
  *platform=web*) [ "$WORKFLOW_FAIL" != web ] || exit 23 ;;
esac
exit 0
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	env, err := shared.ResolveBuildEnvironment(root, "")
	if err != nil {
		t.Fatal(err)
	}
	hostPlatform := map[string]string{"linux": "linuxbsd", "darwin": "macos"}[runtime.GOOS]
	for _, name := range []string{
		"godot." + hostPlatform + ".template_release." + env.Arch,
		"godot_ios.zip", "android_debug.apk", "android_release.apk", "android_source.zip",
		"godot.web.template_release.wasm32.nothreads.zip",
	} {
		mustWriteFile(t, filepath.Join(env.EngineDir, "bin", name), []byte("built "+name))
	}
	writeWorkflowTool(t, filepath.Join(env.EngineDir, "bin", "godot."+hostPlatform+".editor.dev."+env.Arch), `set -eu
test -s .spx_build_lock/pid
test -s "$WORKFLOW_ROOT/module-tests-enabled"
if [ "$#" = 4 ]; then
  printf 'list\n' >> "$WORKFLOW_ROOT/module-tests"
  [ "$WORKFLOW_FAIL" != test-list ] || exit 23
  if [ "$WORKFLOW_FAIL" != test-empty ]; then printf '[SPX] fixture\n'; fi
else
  printf 'test\n' >> "$WORKFLOW_ROOT/module-tests"
  [ "$WORKFLOW_FAIL" != test-run ] || exit 23
fi
`)
	writeWorkflowTool(t, filepath.Join(env.EngineDir, "platform", "android", "java", "gradlew"), "exit 0\n")
	javaHome := filepath.Join(root, "java")
	writeWorkflowTool(t, filepath.Join(javaHome, "bin", "java"), "printf '%s\\n' 'openjdk version \""+lock.Toolchain.JDK+".0.0\"'\n")
	t.Setenv("JAVA_HOME", javaHome)
	t.Setenv("PATH", filepath.Join(javaHome, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	sdk := filepath.Join(root, ".local", "share", "emsdk", "emsdk")
	if runtime.GOOS == "darwin" {
		sdk = filepath.Join(root, "Library", "Application Support", "emsdk", "emsdk")
	}
	sdkBin := filepath.Join(sdk, "upstream", "emscripten")
	for _, name := range []string{"emcc", "em++"} {
		writeWorkflowTool(t, filepath.Join(sdkBin, name), "printf '%s\\n' 'emcc fixture "+lock.Toolchain.EMSDK+"'\n")
	}
	writeWorkflowTool(t, filepath.Join(sdk, "emsdk"), "[ \"$1\" = activate ]\n")
	mustWriteFile(t, filepath.Join(sdk, "emsdk_env.sh"), []byte("export PATH="+shared.ShellQuote(sdkBin)+":\"$PATH\"\n"))
	t.Setenv("EM_CACHE", filepath.Join(root, "em-cache"))
	t.Setenv("EM_CONFIG", filepath.Join(root, "em-config"))
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("extension"))
	runner := &recordingRunner{repoRoot: root}
	runner.commandHook = func(dir, name string, args ...string) error {
		if dir != root || name != "go" || len(args) != 5 || args[0] != "run" || args[1] != "./cmd/spx" || args[3] != "--path" {
			t.Fatalf("unexpected export command %s %s %q", dir, name, args)
		}
		if shared.FileExists(filepath.Join(root, "module-tests-enabled")) {
			if data, err := os.ReadFile(filepath.Join(root, "module-tests")); err != nil || string(data) != "list\ntest\n" {
				t.Fatalf("export ran before successful module tests: %q (%v)", data, err)
			}
		}
		output := filepath.Join(args[4], "project", ".builds")
		switch args[2] {
		case "exportpack":
			mustWriteFile(t, filepath.Join(output, "pc", "gdexport.pck"), []byte("exported pack"))
		case "exporttemplateweb":
			mustWriteFile(t, filepath.Join(output, "webi", "engine.pck"), []byte("web pack"))
			mustWriteFile(t, filepath.Join(output, "webi", "engine.js"), []byte("engine"))
		default:
			t.Fatalf("unexpected export %q", args[2])
		}
		return nil
	}
	return runner
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newWorkflowFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"HOME": root, "USERPROFILE": root, "APPDATA": filepath.Join(root, "AppData"),
		"GOPATH": filepath.Join(root, "go"), "GODOT_SRC": filepath.Join(root, "godot"),
		"SPX_MODULE_SRC": filepath.Join(root, "module"), "PLATFORM": "", "OS": "",
	} {
		t.Setenv(key, value)
	}
	mustWriteFile(t, filepath.Join(root, "go.mod"), nil)
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "install.sh"), nil)
	mustMkdirAll(t, filepath.Join(root, "internal", "tools"))
	t.Chdir(root)
	return root
}

func writeWorkflowTool(t *testing.T, path, script string) {
	t.Helper()
	mustWriteFile(t, path, []byte("#!/bin/sh\n"+script))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
