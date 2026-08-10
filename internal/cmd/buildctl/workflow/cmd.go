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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/runtimecmd"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

var workflowBuildEngine = engine.BuildEngine

type BuildConfig struct {
	Target string
	Mode   string
}

type workflowBuildWebConfig struct {
	mode            string
	skipToolInstall bool
}

type workflowBuildDevConfig struct {
	webMode string
}

type workflowRunDemoConfig struct {
	demoIndex int
	mode      string
	port      int
	movie     string
}

type workflowInstallAPKConfig struct {
	projectDir string
}

var defaultInstallAPKProjectDir = filepath.Join("tutorial", "00-Hello")

func Run(args []string) error {
	if len(args) == 0 {
		printWorkflowUsage()
		return errUsage
	}

	switch args[0] {
	case "install-apk":
		return runWorkflowInstallAPK(args[1:])
	case "list-demos":
		return runWorkflowListDemos(args[1:])
	case "open-template-editor":
		return runWorkflowOpenTemplateEditor(args[1:])
	case "run-demo":
		return runWorkflowRunDemo(args[1:])
	case "stop-web":
		return runWorkflowStopWeb(args[1:])
	case "help", "-h", "--help":
		printWorkflowUsage()
		return nil
	default:
		printWorkflowUsage()
		return fmt.Errorf("unknown workflow command %q", args[0])
	}
}

func printWorkflowUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl workflow <install-apk|list-demos|open-template-editor|run-demo|stop-web> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  install-apk  Export and install an Android APK for a project")
	fmt.Fprintln(osStderr, "  list-demos Print tutorial demo directories with their indexes")
	fmt.Fprintln(osStderr, "  open-template-editor  Open the template project directly in the Godot editor")
	fmt.Fprintln(osStderr, "  run-demo   Run a tutorial demo in local/native/web modes")
	fmt.Fprintln(osStderr, "  stop-web   Stop local gdspx web server processes")
}

func runWorkflowInstallAPK(args []string) error {
	cfg, err := parseWorkflowInstallAPKArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return installAPKWorkflow(cfg, runner)
}

func runWorkflowListDemos(args []string) error {
	if err := shared.ParseNoArgs("workflow list-demos", "Usage: buildctl workflow list-demos", args, osStderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return listDemosWorkflow(runner)
}

func runWorkflowOpenTemplateEditor(args []string) error {
	cfg, err := parseWorkflowOpenTemplateEditorArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return openTemplateEditorWorkflow(cfg, runner)
}

func parseWorkflowInstallAPKArgs(args []string) (workflowInstallAPKConfig, error) {
	cfg := workflowInstallAPKConfig{projectDir: defaultInstallAPKProjectDir}

	fs := flag.NewFlagSet("workflow install-apk", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.projectDir, "project-dir", cfg.projectDir, "project directory used for exportapk --install")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow install-apk [--project-dir tutorial/00-Hello]")
	}

	if err := fs.Parse(args); err != nil {
		return workflowInstallAPKConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflowInstallAPKConfig{}, errUsage
	}
	if cfg.projectDir == "" {
		return workflowInstallAPKConfig{}, errors.New("--project-dir must not be empty")
	}
	return cfg, nil
}

func parseWorkflowOpenTemplateEditorArgs(args []string) (workflowOpenTemplateEditorConfig, error) {
	cfg := workflowOpenTemplateEditorConfig{
		templateDir: defaultTemplateProjectDir,
	}

	fs := flag.NewFlagSet("workflow open-template-editor", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.templateDir, "template-dir", cfg.templateDir, "template project directory to open in the editor")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow open-template-editor [--template-dir cmd/spx/template/project]")
	}

	if err := fs.Parse(args); err != nil {
		return workflowOpenTemplateEditorConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflowOpenTemplateEditorConfig{}, errUsage
	}
	if cfg.templateDir == "" {
		return workflowOpenTemplateEditorConfig{}, errors.New("--template-dir must not be empty")
	}
	return cfg, nil
}

func runWorkflowRunDemo(args []string) error {
	cfg, err := parseWorkflowRunDemoArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return runDemoWorkflow(cfg, runner)
}

func parseWorkflowRunDemoArgs(args []string) (workflowRunDemoConfig, error) {
	cfg := workflowRunDemoConfig{
		mode:  "run",
		port:  8106,
		movie: "false",
	}

	fs := flag.NewFlagSet("workflow run-demo", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.IntVar(&cfg.demoIndex, "demo-index", 0, "1-based index under tutorial/")
	fs.StringVar(&cfg.mode, "mode", cfg.mode, "demo mode: editor, run, runnative, rune, web, or web-worker")
	fs.IntVar(&cfg.port, "port", cfg.port, "web server port used by web demo modes")
	fs.StringVar(&cfg.movie, "movie", cfg.movie, "movie flag passed through to spx native/editor modes")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow run-demo --demo-index N [--mode editor|run|runnative|rune|web|web-worker] [--port 8106] [--movie false]")
	}

	if err := fs.Parse(args); err != nil {
		return workflowRunDemoConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflowRunDemoConfig{}, errUsage
	}
	if cfg.demoIndex <= 0 {
		return workflowRunDemoConfig{}, errors.New("--demo-index must be greater than 0")
	}
	switch cfg.mode {
	case "editor", "run", "runnative", "rune", "web", "web-worker":
	default:
		return workflowRunDemoConfig{}, fmt.Errorf("unsupported demo mode: %s", cfg.mode)
	}
	if cfg.port <= 0 {
		return workflowRunDemoConfig{}, errors.New("--port must be greater than 0")
	}
	return cfg, nil
}

func runWorkflowStopWeb(args []string) error {
	if err := shared.ParseNoArgs("workflow stop-web", "Usage: buildctl workflow stop-web", args, osStderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	runner := shared.CommandRunner{RepoRoot: repoRoot}
	return stopWebWorkflow(runner)
}

func Build(cfg BuildConfig, runner shared.ScriptRunner) error {
	switch cfg.Target {
	case "dev":
		return buildDevWorkflow(workflowBuildDevConfig{webMode: cfg.Mode}, runner)
	case "editor":
		if err := toolpkg.InstallTools(toolpkg.InstallConfig{}, runner); err != nil {
			return err
		}
		return workflowBuildEngine(engine.BuildConfig{Target: "editor"}, runner.RepoRootDir())
	case "desktop":
		if err := toolpkg.InstallTools(toolpkg.InstallConfig{}, runner); err != nil {
			return err
		}
		if err := workflowBuildEngine(engine.BuildConfig{Target: "template"}, runner.RepoRootDir()); err != nil {
			return err
		}
		return runtimecmd.ExportPackRuntime(runner)
	case "web":
		return buildWebWorkflow(workflowBuildWebConfig{mode: cfg.Mode}, runner)
	case "android", "ios":
		if err := toolpkg.InstallTools(toolpkg.InstallConfig{}, runner); err != nil {
			return err
		}
		return workflowBuildEngine(engine.BuildConfig{Target: "template", Platform: cfg.Target}, runner.RepoRootDir())
	default:
		return fmt.Errorf("unsupported build target: %s", cfg.Target)
	}
}

func buildWebWorkflow(cfg workflowBuildWebConfig, runner shared.ScriptRunner) error {
	if !cfg.skipToolInstall {
		if err := toolpkg.InstallTools(toolpkg.InstallConfig{}, runner); err != nil {
			return err
		}
	}
	if err := workflowBuildEngine(engine.BuildConfig{
		Target:   "template",
		Platform: "web",
		Mode:     cfg.mode,
	}, runner.RepoRootDir()); err != nil {
		return err
	}
	return runtimecmd.ExportWebTemplateRuntime(cfg.mode, runner)
}

func buildHostRuntimeWorkflow(runner shared.ScriptRunner) error {
	if err := workflowBuildEngine(engine.BuildConfig{Target: "template"}, runner.RepoRootDir()); err != nil {
		return err
	}
	return runtimecmd.ExportPackRuntime(runner)
}

func buildDevWorkflow(cfg workflowBuildDevConfig, runner shared.ScriptRunner) error {
	printWorkflowStep(1, 4, "Build host editor")
	if err := workflowBuildEngine(engine.BuildConfig{Target: "editor"}, runner.RepoRootDir()); err != nil {
		return err
	}

	printWorkflowStep(2, 4, "Build host runtime template and export pack")
	if err := buildHostRuntimeWorkflow(runner); err != nil {
		return err
	}

	printWorkflowStep(3, 4, "Build web template and export assets")
	if err := buildWebWorkflow(workflowBuildWebConfig{mode: cfg.webMode, skipToolInstall: true}, runner); err != nil {
		return err
	}

	printWorkflowStep(4, 4, "Install spx toolchain and web runtime")
	if err := toolpkg.InstallTools(toolpkg.InstallConfig{Web: true}, runner); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Build complete. Run a demo with: make run DEMO_INDEX=N")
	return nil
}

func listDemosWorkflow(runner shared.WorkflowRunner) error {
	demos, err := runner.ListDemoDirs()
	if err != nil {
		return err
	}
	for i, demo := range demos {
		fmt.Fprintf(os.Stdout, "%d: %s\n", i+1, demo)
	}
	return nil
}

func runDemoWorkflow(cfg workflowRunDemoConfig, runner shared.WorkflowRunner) error {
	demos, err := runner.ListDemoDirs()
	if err != nil {
		return err
	}
	if cfg.demoIndex > len(demos) {
		return fmt.Errorf("demo-index %d out of range (have %d demos)", cfg.demoIndex, len(demos))
	}

	demo := demos[cfg.demoIndex-1]
	movieArg := fmt.Sprintf("-movie=%s", cfg.movie)

	switch cfg.mode {
	case "editor":
		fmt.Fprintf(os.Stdout, "Opening editor for demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.RunCommand(demo, "spx", "editor", movieArg)
	case "run":
		fmt.Fprintf(os.Stdout, "Running demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.RunCommand(demo, "spx", "run", movieArg)
	case "runnative":
		fmt.Fprintf(os.Stdout, "Running native demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.RunCommand(demo, "spx", "runnative", movieArg)
	case "rune":
		fmt.Fprintf(os.Stdout, "Running editor demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.RunCommand(demo, "spx", "rune", movieArg)
	case "web":
		fmt.Fprintf(os.Stdout, "Running web demo #%d: %s\n", cfg.demoIndex, demo)
		if err := runtimecmd.BuildWasmRuntime(runtimecmd.BuildWasmConfig{}, runner); err != nil {
			return err
		}
		return runner.RunCommand(demo, "spx", "runweb", fmt.Sprintf("-serveraddr=:%d", cfg.port))
	case "web-worker":
		fmt.Fprintf(os.Stdout, "Running web worker mode: demo #%d: %s\n", cfg.demoIndex, demo)
		if err := runtimecmd.BuildWasmRuntime(runtimecmd.BuildWasmConfig{}, runner); err != nil {
			return err
		}
		return runner.RunCommand(demo, "spx", "runwebworker", fmt.Sprintf("-serveraddr=:%d", cfg.port))
	default:
		return fmt.Errorf("unsupported demo mode: %s", cfg.mode)
	}
}

func stopWebWorkflow(runner shared.WorkflowRunner) error {
	fmt.Fprintln(os.Stdout, "Stopping running processes...")
	if err := runner.StopWebServers(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Processes stopped.")
	return nil
}

func installAPKWorkflow(cfg workflowInstallAPKConfig, runner shared.WorkflowRunner) error {
	projectDir := cfg.projectDir
	if projectDir == "" {
		projectDir = defaultInstallAPKProjectDir
	}
	if !filepath.IsAbs(projectDir) {
		projectDir = filepath.Join(runner.RepoRootDir(), projectDir)
	}
	projectDir = filepath.Clean(projectDir)

	info, err := os.Stat(projectDir)
	if err != nil {
		return fmt.Errorf("project directory %s: %w", projectDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project directory is not a directory: %s", projectDir)
	}

	return runner.RunCommand(filepath.Join("cmd", "spx"), "go", "run", ".", "exportapk", "--install", "--path", projectDir)
}

func printWorkflowStep(step, total int, label string) {
	fmt.Fprintf(os.Stdout, "Step %d/%d: %s\n", step, total, label)
}
