package workflow

import (
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

type workflowRecordingRunner struct {
	calls     []recordedCall
	commands  []recordedCommand
	demos     []string
	repoRoot  string
	stopCalls int
}

func (r *workflowRecordingRunner) runScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return nil
}

func (r *workflowRecordingRunner) runCommand(workdir string, name string, args ...string) error {
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

func (r *workflowRecordingRunner) repoRootDir() string {
	if r.repoRoot == "" {
		return "."
	}
	return r.repoRoot
}

func (r *workflowRecordingRunner) listDemoDirs() ([]string, error) {
	return append([]string(nil), r.demos...), nil
}

func (r *workflowRecordingRunner) stopWebServers() error {
	r.stopCalls++
	return nil
}

func TestInstallToolsWebOpt(t *testing.T) {
	runner := &recordingRunner{}

	if err := installTools(toolInstallConfig{web: true, opt: true}, runner); err != nil {
		t.Fatalf("installTools returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web", "--opt"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestParseWorkflowBuildWebArgsDefault(t *testing.T) {
	cfg, err := parseWorkflowBuildWebArgs(nil)
	if err != nil {
		t.Fatalf("parseWorkflowBuildWebArgs returned error: %v", err)
	}
	if cfg.mode != "normal" {
		t.Fatalf("expected normal mode, got %s", cfg.mode)
	}
}

func TestBuildWebWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)

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
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "template", "--platform", "web", "--mode", "worker"}},
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
}

func TestBuildWebWorkflowSkipInstall(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)

	if err := buildWebWorkflow(workflowBuildWebConfig{mode: "worker", skipToolInstall: true}, runner); err != nil {
		t.Fatalf("buildWebWorkflow returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "template", "--platform", "web", "--mode", "worker"}},
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
}

func TestBuildHostRuntimeWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)

	if err := buildHostRuntimeWorkflow(runner); err != nil {
		t.Fatalf("buildHostRuntimeWorkflow returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	expectedCommands := []recordedCommand{
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "template"}},
		{name: "spx", args: []string{"export"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
}

func TestBuildDevWorkflow(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)

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
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "editor"}},
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "template"}},
		{name: "spx", args: []string{"export"}},
		{dir: runner.repoRoot, name: "bash", args: []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", "template", "--platform", "web", "--mode", "minigame"}},
		{name: "spx", args: []string{"exporttemplateweb"}},
	}
	assertWorkflowRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, expectedCommands)
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

func TestRunDemoWorkflowNative(t *testing.T) {
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
		{script: "cmd/spx/install.sh", args: []string{"--web"}},
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
		{script: "cmd/spx/install.sh", args: []string{"--web"}},
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
