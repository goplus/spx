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
)

type workflowRunner interface {
	scriptRunner
	runCommand(workdir string, name string, args ...string) error
	listDemoDirs() ([]string, error)
	stopWebServers() error
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

func runWorkflow(args []string) error {
	if len(args) == 0 {
		printWorkflowUsage()
		return errUsage
	}

	switch args[0] {
	case "build-web":
		return runWorkflowBuildWeb(args[1:])
	case "build-dev":
		return runWorkflowBuildDev(args[1:])
	case "install-apk":
		return runWorkflowInstallAPK(args[1:])
	case "list-demos":
		return runWorkflowListDemos(args[1:])
	case "run-demo":
		return runWorkflowRunDemo(args[1:])
	case "setup-dev":
		return runWorkflowBuildDev(args[1:])
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
	fmt.Fprintln(osStderr, "Usage: buildctl workflow <build-dev|build-web|install-apk|list-demos|run-demo|stop-web> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  build-dev  Run the full local development build workflow")
	fmt.Fprintln(osStderr, "  build-web  Build web templates and export runtime assets")
	fmt.Fprintln(osStderr, "  install-apk  Export and install an Android APK for a project")
	fmt.Fprintln(osStderr, "  list-demos Print tutorial demo directories with their indexes")
	fmt.Fprintln(osStderr, "  run-demo   Run a tutorial demo in local/native/web modes")
	fmt.Fprintln(osStderr, "  stop-web   Stop local gdspx web server processes")
}

func runWorkflowBuildWeb(args []string) error {
	cfg, err := parseWorkflowBuildWebArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return buildWebWorkflow(cfg, runner)
}

func parseWorkflowBuildWebArgs(args []string) (workflowBuildWebConfig, error) {
	cfg := workflowBuildWebConfig{mode: "normal"}

	fs := flag.NewFlagSet("workflow build-web", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.mode, "mode", cfg.mode, "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow build-web [--mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return workflowBuildWebConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflowBuildWebConfig{}, errUsage
	}
	if err := validateWebMode(cfg.mode); err != nil {
		return workflowBuildWebConfig{}, err
	}
	return cfg, nil
}

func runWorkflowBuildDev(args []string) error {
	cfg, err := parseWorkflowBuildDevArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return buildDevWorkflow(cfg, runner)
}

func runWorkflowInstallAPK(args []string) error {
	cfg, err := parseWorkflowInstallAPKArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return installAPKWorkflow(cfg, runner)
}

func runWorkflowListDemos(args []string) error {
	if err := parseWorkflowListDemosArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return listDemosWorkflow(runner)
}

func parseWorkflowListDemosArgs(args []string) error {
	fs := flag.NewFlagSet("workflow list-demos", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow list-demos")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errUsage
	}
	return nil
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

func runWorkflowRunDemo(args []string) error {
	cfg, err := parseWorkflowRunDemoArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
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
	fs.StringVar(&cfg.mode, "mode", cfg.mode, "demo mode: editor, run, rune, web, or web-worker")
	fs.IntVar(&cfg.port, "port", cfg.port, "web server port used by web demo modes")
	fs.StringVar(&cfg.movie, "movie", cfg.movie, "movie flag passed through to spx native/editor modes")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow run-demo --demo-index N [--mode editor|run|rune|web|web-worker] [--port 8106] [--movie false]")
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
	case "editor", "run", "rune", "web", "web-worker":
	default:
		return workflowRunDemoConfig{}, fmt.Errorf("unsupported demo mode: %s", cfg.mode)
	}
	if cfg.port <= 0 {
		return workflowRunDemoConfig{}, errors.New("--port must be greater than 0")
	}
	return cfg, nil
}

func runWorkflowStopWeb(args []string) error {
	if err := parseWorkflowStopWebArgs(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	runner := commandRunner{repoRoot: repoRoot}
	return stopWebWorkflow(runner)
}

func parseWorkflowStopWebArgs(args []string) error {
	fs := flag.NewFlagSet("workflow stop-web", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow stop-web")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errUsage
	}
	return nil
}

func parseWorkflowBuildDevArgs(args []string) (workflowBuildDevConfig, error) {
	cfg := workflowBuildDevConfig{webMode: "normal"}

	fs := flag.NewFlagSet("workflow build-dev", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.webMode, "web-mode", cfg.webMode, "web mode for the final web engine build step")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl workflow build-dev [--web-mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return workflowBuildDevConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return workflowBuildDevConfig{}, errUsage
	}
	if err := validateWebMode(cfg.webMode); err != nil {
		return workflowBuildDevConfig{}, err
	}
	return cfg, nil
}

func buildWebWorkflow(cfg workflowBuildWebConfig, runner scriptRunner) error {
	if !cfg.skipToolInstall {
		if err := installTools(toolInstallConfig{}, runner); err != nil {
			return err
		}
	}
	if err := runEngineBuildWorkflow(runner, engineBuildConfig{
		target:   "template",
		platform: "web",
		mode:     cfg.mode,
	}); err != nil {
		return err
	}
	return exportWebTemplateRuntime(cfg.mode, runner)
}

func buildHostRuntimeWorkflow(runner scriptRunner) error {
	if err := runEngineBuildWorkflow(runner, engineBuildConfig{target: "template"}); err != nil {
		return err
	}
	return exportPackRuntime(runner)
}

func buildDevWorkflow(cfg workflowBuildDevConfig, runner scriptRunner) error {
	printWorkflowStep(1, 4, "Install spx toolchain and web runtime")
	if err := installTools(toolInstallConfig{web: true}, runner); err != nil {
		return err
	}

	printWorkflowStep(2, 4, "Build host editor")
	if err := runEngineBuildWorkflow(runner, engineBuildConfig{target: "editor"}); err != nil {
		return err
	}

	printWorkflowStep(3, 4, "Build host runtime template and export pack")
	if err := buildHostRuntimeWorkflow(runner); err != nil {
		return err
	}

	printWorkflowStep(4, 4, "Build web template and export assets")
	if err := buildWebWorkflow(workflowBuildWebConfig{mode: cfg.webMode, skipToolInstall: true}, runner); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "===> build-dev done, use 'make run DEMO_INDEX=N' to run demo")
	return nil
}

func listDemosWorkflow(runner workflowRunner) error {
	demos, err := runner.listDemoDirs()
	if err != nil {
		return err
	}
	for i, demo := range demos {
		fmt.Fprintf(os.Stdout, "%d: %s\n", i+1, demo)
	}
	return nil
}

func runDemoWorkflow(cfg workflowRunDemoConfig, runner workflowRunner) error {
	demos, err := runner.listDemoDirs()
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
		return runner.runCommand(demo, "spx", "editor", movieArg)
	case "run":
		fmt.Fprintf(os.Stdout, "Running demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.runCommand(demo, "spx", "run", movieArg)
	case "rune":
		fmt.Fprintf(os.Stdout, "Running editor demo #%d: %s\n", cfg.demoIndex, demo)
		return runner.runCommand(demo, "spx", "rune", movieArg)
	case "web":
		fmt.Fprintf(os.Stdout, "Running web demo #%d: %s\n", cfg.demoIndex, demo)
		if err := buildWasmRuntime(runtimeBuildWasmConfig{}, runner); err != nil {
			return err
		}
		return runner.runCommand(demo, "spx", "runweb", fmt.Sprintf("-serveraddr=:%d", cfg.port))
	case "web-worker":
		fmt.Fprintf(os.Stdout, "Running web worker mode: demo #%d: %s\n", cfg.demoIndex, demo)
		if err := buildWasmRuntime(runtimeBuildWasmConfig{}, runner); err != nil {
			return err
		}
		return runner.runCommand(demo, "spx", "runwebworker", fmt.Sprintf("-serveraddr=:%d", cfg.port))
	default:
		return fmt.Errorf("unsupported demo mode: %s", cfg.mode)
	}
}

func stopWebWorkflow(runner workflowRunner) error {
	fmt.Fprintln(os.Stdout, "Stopping running processes...")
	if err := runner.stopWebServers(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Processes stopped.")
	return nil
}

func installAPKWorkflow(cfg workflowInstallAPKConfig, runner workflowRunner) error {
	projectDir := cfg.projectDir
	if projectDir == "" {
		projectDir = defaultInstallAPKProjectDir
	}
	if !filepath.IsAbs(projectDir) {
		projectDir = filepath.Join(runner.repoRootDir(), projectDir)
	}
	projectDir = filepath.Clean(projectDir)

	info, err := os.Stat(projectDir)
	if err != nil {
		return fmt.Errorf("project directory %s: %w", projectDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project directory is not a directory: %s", projectDir)
	}

	return runner.runCommand(filepath.Join("cmd", "spx"), "go", "run", ".", "exportapk", "--install", "--path", projectDir)
}

func runEngineBuildWorkflow(runner scriptRunner, cfg engineBuildConfig) error {
	args := []string{"./internal/cmd/buildctl/buildctl.sh", "engine", "build", "--target", cfg.target}
	if cfg.platform != "" {
		args = append(args, "--platform", cfg.platform)
	}
	if cfg.mode != "" {
		args = append(args, "--mode", cfg.mode)
	}
	return runner.runCommand(".", "bash", args...)
}

func printWorkflowStep(step, total int, label string) {
	fmt.Fprintf(os.Stdout, "===> Step %d/%d: %s\n", step, total, label)
}
