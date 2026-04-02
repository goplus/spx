package workflow

import "github.com/goplus/spx/v2/internal/cmd/buildctl/shared"

type BuildWebConfig struct {
	Mode            string
	SkipToolInstall bool
}

type BuildDevConfig struct {
	WebMode string
}

type RunDemoConfig struct {
	DemoIndex int
	Mode      string
	Port      int
	Movie     string
}

type InstallAPKConfig struct {
	ProjectDir string
}

func Run(args []string) error { return runWorkflow(args) }
func ParseWorkflowBuildWebArgs(args []string) (BuildWebConfig, error) {
	cfg, err := parseWorkflowBuildWebArgs(args)
	return BuildWebConfig{Mode: cfg.mode, SkipToolInstall: cfg.skipToolInstall}, err
}
func ParseWorkflowBuildDevArgs(args []string) (BuildDevConfig, error) {
	cfg, err := parseWorkflowBuildDevArgs(args)
	return BuildDevConfig{WebMode: cfg.webMode}, err
}
func ParseWorkflowRunDemoArgs(args []string) (RunDemoConfig, error) {
	cfg, err := parseWorkflowRunDemoArgs(args)
	return RunDemoConfig{DemoIndex: cfg.demoIndex, Mode: cfg.mode, Port: cfg.port, Movie: cfg.movie}, err
}
func ParseWorkflowInstallAPKArgs(args []string) (InstallAPKConfig, error) {
	cfg, err := parseWorkflowInstallAPKArgs(args)
	return InstallAPKConfig{ProjectDir: cfg.projectDir}, err
}
func ParseWorkflowListDemosArgs(args []string) error { return parseWorkflowListDemosArgs(args) }
func ParseWorkflowStopWebArgs(args []string) error   { return parseWorkflowStopWebArgs(args) }
func BuildWebWorkflow(cfg BuildWebConfig, runner shared.ScriptRunner) error {
	return buildWebWorkflow(workflowBuildWebConfig{mode: cfg.Mode, skipToolInstall: cfg.SkipToolInstall}, sharedScriptRunnerAdapter{inner: runner})
}
func BuildHostRuntimeWorkflow(runner shared.ScriptRunner) error {
	return buildHostRuntimeWorkflow(sharedScriptRunnerAdapter{inner: runner})
}
func BuildDevWorkflow(cfg BuildDevConfig, runner shared.ScriptRunner) error {
	return buildDevWorkflow(workflowBuildDevConfig{webMode: cfg.WebMode}, sharedScriptRunnerAdapter{inner: runner})
}
func ListDemosWorkflow(runner shared.WorkflowRunner) error {
	return listDemosWorkflow(sharedWorkflowRunnerAdapter{inner: runner})
}
func RunDemoWorkflow(cfg RunDemoConfig, runner shared.WorkflowRunner) error {
	return runDemoWorkflow(workflowRunDemoConfig{demoIndex: cfg.DemoIndex, mode: cfg.Mode, port: cfg.Port, movie: cfg.Movie}, sharedWorkflowRunnerAdapter{inner: runner})
}
func StopWebWorkflow(runner shared.WorkflowRunner) error {
	return stopWebWorkflow(sharedWorkflowRunnerAdapter{inner: runner})
}
func InstallAPKWorkflow(cfg InstallAPKConfig, runner shared.WorkflowRunner) error {
	return installAPKWorkflow(workflowInstallAPKConfig{projectDir: cfg.ProjectDir}, sharedWorkflowRunnerAdapter{inner: runner})
}
