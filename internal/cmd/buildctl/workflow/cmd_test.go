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
	"testing"
)

type recordedCommand struct {
	dir  string
	name string
	args []string
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
