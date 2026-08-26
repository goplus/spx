/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/launchpack"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

const spxModulePath = "github.com/goplus/spx/v3"

type launcherBuilder func(context.Context, launchpack.Config) (launchpack.Result, error)

func (cmd *CmdTool) runBuildLauncher() error {
	// A launcher is a host executable; target, runtime, and multiplayer flags
	// have no meaning for this packaging-only command.
	if err := validateBuildLauncherArgs(cmd.Args); err != nil {
		return err
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("buildlauncher: unexpected positional arguments: %q", flag.Args())
	}
	projectPath := valueOrDefault(cmd.Args.Path, ".")
	projectDir, err := canonicalDirectory(projectPath)
	if err != nil {
		return fmt.Errorf("buildlauncher: project path: %w", err)
	}
	goCommand, err := resolveGoCommand()
	if err != nil {
		return fmt.Errorf("buildlauncher: Go command: %w", err)
	}
	project, source, err := resolveLauncherInputs(context.Background(), projectDir, goCommand)
	if err != nil {
		return err
	}
	protected := launcherOutputProtection{
		Files: append([]string{project.file, project.metadataFile, filepath.Join(project.dir, ".config")}, project.sourceFiles...),
	}
	protected.Files = append(protected.Files, source.protectedFiles...)
	if source.root != "" {
		protected.Roots = append(protected.Roots, filepath.Join(source.root, "cmd", "ispxnative"))
	}
	final, err := resolveLauncherOutput(launcherOutputInputs{
		ProjectDir: project.dir, ProjectExt: project.extension,
		PackDir: project.packDir, Protection: protected,
	}, valueOf(cmd.Args.Output))
	if err != nil {
		return err
	}
	stage, cleanup, err := stageLauncherOutput(final)
	if err != nil {
		return err
	}
	defer cleanup()

	snapshot, err := projectpolicy.SnapshotPortableConfig(project.dir)
	if err != nil {
		return fmt.Errorf("buildlauncher: portable config: %w", err)
	}
	config := launchpack.Config{
		ProjectDir: project.dir, ProjectFile: project.file, ProjectExt: project.extension,
		PackDir: project.packDir, PackIndex: project.packIndex, PortableConfig: snapshot,
		RuntimeIdentity: launchpack.RuntimeIdentity{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Source: launchpack.SourceIdentity{
			SelectedPath: spxModulePath, SelectedVersion: source.selectedVersion,
			EffectivePath: source.effectivePath, EffectiveVersion: source.effectiveVersion,
			Main: source.main, SourceMode: source.sourceMode,
		},
		GoCommand: source.goCommand, WorkDir: source.workDir, GoWork: source.goWork,
		GraphFlags: source.graphFlags, BuildFlags: buildLauncherBuildFlags(cmd.Args), Output: stage,
		VerifyGraph: source.verifyGraph,
		IO:          launchpack.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Env: source.env},
	}
	if source.sourceMode {
		config.RuntimeSourceRoot = source.root
		config.BridgePackage = spxModulePath + "/cmd/ispxnative"
	}
	builder := cmd.launcherBuilder
	if builder == nil {
		builder = launchpack.BuildLauncher
	}
	if _, err := builder(context.Background(), config); err != nil {
		return fmt.Errorf("buildlauncher: %w", err)
	}
	if err := commitLauncherOutput(stage, final, protected); err != nil {
		return err
	}
	logInfof("Built launcher: %s", final)
	return nil
}

func validateBuildLauncherArgs(args ExtraArgs) error {
	for _, unsupported := range []struct {
		name  string
		value string
	}{
		{name: "serveraddr", value: valueOf(args.ServerAddr)},
		{name: "controller", value: valueOf(args.ControllerName)},
		{name: "arch", value: valueOf(args.Arch)},
		{name: "tags", value: valueOf(args.Tags)},
	} {
		if unsupported.value != "" {
			return fmt.Errorf("buildlauncher: -%s is not supported for the host-only launcher (got %q)", unsupported.name, unsupported.value)
		}
	}

	for _, unsupported := range []struct {
		name  string
		value bool
	}{
		{name: "servermode", value: boolValue(args.ServerMode)},
		{name: "headless", value: boolValue(args.HeadlessMode)},
		{name: "onlys", value: boolValue(args.OnlyServer)},
		{name: "onlyc", value: boolValue(args.OnlyClient)},
		{name: "nomap", value: boolValue(args.NoMap)},
		{name: "install", value: boolValue(args.Install)},
		{name: "debugweb", value: boolValue(args.DebugWebService)},
		{name: "fullscreen", value: boolValue(args.FullScreen)},
		{name: "movie", value: boolValue(args.Movie)},
	} {
		if unsupported.value {
			return fmt.Errorf("buildlauncher: -%s is not supported for the host-only launcher (got true)", unsupported.name)
		}
	}

	if target := valueOrDefault(args.Target, "esp32"); target != "esp32" {
		return fmt.Errorf("buildlauncher: -target is not supported for the host-only launcher (got %q)", target)
	}
	if build := valueOrDefault(args.Build, "normal"); build != "normal" {
		return fmt.Errorf("buildlauncher: -build is not supported for the host-only launcher (got %q)", build)
	}
	if mode := valueOrDefault(args.Mode, "none"); mode != "none" {
		return fmt.Errorf("buildlauncher: -mode is not supported for the host-only launcher (got %q)", mode)
	}
	return nil
}

func buildLauncherBuildFlags(args ExtraArgs) []string {
	if boolValue(args.Verbose) {
		return []string{"-v=true"}
	}
	return nil
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
