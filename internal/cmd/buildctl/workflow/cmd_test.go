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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
)

type recordedCommand struct {
	dir  string
	name string
	args []string
}

type recordedEngineBuild struct {
	config   engine.BuildConfig
	repoRoot string
}

type workflowRecordingRunner struct {
	calls     []recordedCall
	commands  []recordedCommand
	demos     []string
	repoRoot  string
	stopCalls int
}

func (r *workflowRecordingRunner) RunScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return nil
}

func (r *workflowRecordingRunner) RunCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if r.repoRoot != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(r.repoRoot, dir)
	}
	r.commands = append(r.commands, recordedCommand{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	return nil
}

func (r *workflowRecordingRunner) RepoRootDir() string {
	if r.repoRoot == "" {
		return "."
	}
	return r.repoRoot
}

func (r *workflowRecordingRunner) ListDemoDirs() ([]string, error) {
	return append([]string(nil), r.demos...), nil
}

func (r *workflowRecordingRunner) StopWebServers() error {
	r.stopCalls++
	return nil
}

func TestBuildWebWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	engineBuilds := recordWorkflowEngineBuilds(t)

	if err := buildWebWorkflow(workflowBuildWebConfig{mode: "worker"}, runner); err != nil {
		t.Fatalf("buildWebWorkflow returned error: %v", err)
	}

	expectedCalls := []recordedCall{
		{script: "cmd/spx/install.sh", args: nil},
	}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
	assertWorkflowEngineBuilds(t, *engineBuilds, []recordedEngineBuild{{
		config:   engine.BuildConfig{Target: "template", Platform: "web", Mode: "worker"},
		repoRoot: runner.repoRoot,
	}})
}

func TestBuildWebWorkflowSkipInstall(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	engineBuilds := recordWorkflowEngineBuilds(t)

	if err := buildWebWorkflow(workflowBuildWebConfig{mode: "worker", skipToolInstall: true}, runner); err != nil {
		t.Fatalf("buildWebWorkflow returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
	assertWorkflowEngineBuilds(t, *engineBuilds, []recordedEngineBuild{{
		config:   engine.BuildConfig{Target: "template", Platform: "web", Mode: "worker"},
		repoRoot: runner.repoRoot,
	}})
}

func TestBuildHostRuntimeWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	engineBuilds := recordWorkflowEngineBuilds(t)

	if err := buildHostRuntimeWorkflow(runner); err != nil {
		t.Fatalf("buildHostRuntimeWorkflow returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{name: "spx", args: []string{"export"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
	assertWorkflowEngineBuilds(t, *engineBuilds, []recordedEngineBuild{{
		config:   engine.BuildConfig{Target: "template"},
		repoRoot: runner.repoRoot,
	}})
}

func TestBuildDevWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	engineBuilds := recordWorkflowEngineBuilds(t)

	if err := buildDevWorkflow(workflowBuildDevConfig{webMode: "minigame"}, runner); err != nil {
		t.Fatalf("buildDevWorkflow returned error: %v", err)
	}

	expectedCalls := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web"}},
	}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{name: "spx", args: []string{"export"}},
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
	assertWorkflowEngineBuilds(t, *engineBuilds, []recordedEngineBuild{
		{config: engine.BuildConfig{Target: "editor"}, repoRoot: runner.repoRoot},
		{config: engine.BuildConfig{Target: "template"}, repoRoot: runner.repoRoot},
		{config: engine.BuildConfig{Target: "template", Platform: "web", Mode: "minigame"}, repoRoot: runner.repoRoot},
	})
}

func TestBuildTargets(t *testing.T) {
	tests := []struct {
		name         string
		config       BuildConfig
		wantCalls    []recordedCall
		wantCommands []recordedCommand
		wantBuilds   []engine.BuildConfig
	}{
		{
			name:      "dev",
			config:    BuildConfig{Target: "dev", Mode: "minigame"},
			wantCalls: []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web"}}},
			wantCommands: []recordedCommand{
				{name: "spx", args: []string{"export"}},
				{name: "spx", args: []string{"exporttemplateweb"}},
			},
			wantBuilds: []engine.BuildConfig{
				{Target: "editor"},
				{Target: "template"},
				{Target: "template", Platform: "web", Mode: "minigame"},
			},
		},
		{
			name:       "editor",
			config:     BuildConfig{Target: "editor"},
			wantCalls:  []recordedCall{{script: "cmd/spx/install.sh"}},
			wantBuilds: []engine.BuildConfig{{Target: "editor"}},
		},
		{
			name:      "desktop",
			config:    BuildConfig{Target: "desktop"},
			wantCalls: []recordedCall{{script: "cmd/spx/install.sh"}},
			wantCommands: []recordedCommand{
				{name: "spx", args: []string{"export"}},
			},
			wantBuilds: []engine.BuildConfig{{Target: "template"}},
		},
		{
			name:      "web",
			config:    BuildConfig{Target: "web", Mode: "worker"},
			wantCalls: []recordedCall{{script: "cmd/spx/install.sh"}},
			wantCommands: []recordedCommand{
				{name: "spx", args: []string{"exporttemplateweb"}},
			},
			wantBuilds: []engine.BuildConfig{{Target: "template", Platform: "web", Mode: "worker"}},
		},
		{
			name:       "android",
			config:     BuildConfig{Target: "android"},
			wantCalls:  []recordedCall{{script: "cmd/spx/install.sh"}},
			wantBuilds: []engine.BuildConfig{{Target: "template", Platform: "android"}},
		},
		{
			name:       "ios",
			config:     BuildConfig{Target: "ios"},
			wantCalls:  []recordedCall{{script: "cmd/spx/install.sh"}},
			wantBuilds: []engine.BuildConfig{{Target: "template", Platform: "ios"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			engineBuilds := recordWorkflowEngineBuilds(t)

			if err := Build(test.config, runner); err != nil {
				t.Fatalf("Build(%#v) returned error: %v", test.config, err)
			}
			if !reflect.DeepEqual(runner.calls, test.wantCalls) {
				t.Fatalf("unexpected calls: got %#v want %#v", runner.calls, test.wantCalls)
			}
			assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, test.wantCommands)

			wantBuilds := make([]recordedEngineBuild, len(test.wantBuilds))
			for i, config := range test.wantBuilds {
				wantBuilds[i] = recordedEngineBuild{config: config, repoRoot: runner.repoRoot}
			}
			assertWorkflowEngineBuilds(t, *engineBuilds, wantBuilds)
		})
	}
}

func TestBuildRejectsUnknownTarget(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
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

	runner := &workflowRecordingRunner{repoRoot: repoRoot}
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
	runner := &workflowRecordingRunner{
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
	runner := &workflowRecordingRunner{
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

func TestRunDemoWorkflowWebWorker(t *testing.T) {
	runner := &workflowRecordingRunner{
		demos: []string{"tutorial/00-Hello"},
	}

	err := runDemoWorkflow(workflowRunDemoConfig{
		demoIndex: 1,
		mode:      "web-worker",
		movie:     "false",
		port:      8123,
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
		{dir: "tutorial/00-Hello", name: "spx", args: []string{"runwebworker", "-serveraddr=:8123"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	if runner.stopCalls != 0 {
		t.Fatalf("unexpected stopCalls: %d", runner.stopCalls)
	}
}

func TestRunDemoWorkflowWeb(t *testing.T) {
	runner := &workflowRecordingRunner{
		demos: []string{"tutorial/00-Hello"},
	}

	err := runDemoWorkflow(workflowRunDemoConfig{
		demoIndex: 1,
		mode:      "web",
		movie:     "false",
		port:      8105,
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
		{dir: "tutorial/00-Hello", name: "spx", args: []string{"runweb", "-serveraddr=:8105"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	if runner.stopCalls != 0 {
		t.Fatalf("unexpected stopCalls: %d", runner.stopCalls)
	}
}

func assertWorkflowRuntimeWorkspaceCommands(t *testing.T, got []recordedCommand, repoRoot string, want []recordedCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected commands: %#v", got)
	}
	for i := range want {
		if want[i].name == "spx" {
			projectDir, spxArgs, ok := simulatedSPXInvocation(got[i].dir, got[i].name, got[i].args...)
			if !ok || !reflect.DeepEqual(want[i].args, spxArgs) {
				t.Fatalf("unexpected command[%d]: %#v", i, got[i])
			}
			prefix := filepath.Join(repoRoot, ".tmp", "runtime-")
			if !strings.HasPrefix(projectDir, prefix) {
				t.Fatalf("unexpected runtime workspace dir[%d]: %s", i, projectDir)
			}
			continue
		}
		if want[i].name != got[i].name || !reflect.DeepEqual(want[i].args, got[i].args) {
			t.Fatalf("unexpected command[%d]: %#v", i, got[i])
		}
		if want[i].dir != "" {
			if got[i].dir != want[i].dir {
				t.Fatalf("unexpected dir[%d]: got %s want %s", i, got[i].dir, want[i].dir)
			}
			continue
		}
		prefix := filepath.Join(repoRoot, ".tmp", "runtime-")
		if !strings.HasPrefix(got[i].dir, prefix) {
			t.Fatalf("unexpected runtime workspace dir[%d]: %s", i, got[i].dir)
		}
	}
}

func recordWorkflowEngineBuilds(t *testing.T) *[]recordedEngineBuild {
	t.Helper()

	original := workflowBuildEngine
	var builds []recordedEngineBuild
	workflowBuildEngine = func(cfg engine.BuildConfig, repoRoot string) error {
		builds = append(builds, recordedEngineBuild{config: cfg, repoRoot: repoRoot})
		return nil
	}
	t.Cleanup(func() {
		workflowBuildEngine = original
	})
	return &builds
}

func assertWorkflowEngineBuilds(t *testing.T, got, want []recordedEngineBuild) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected engine builds: got %#v want %#v", got, want)
	}
}
