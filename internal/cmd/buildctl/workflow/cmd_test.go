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
	"strings"
	"testing"
)

type recordedCommand struct {
	dir  string
	name string
	args []string
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

func TestRunDemoWorkflowWebWorker(t *testing.T) {
	runner := &recordingRunner{
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
	runner := &recordingRunner{
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
