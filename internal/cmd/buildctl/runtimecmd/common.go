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

package runtimecmd

import (
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v2/internal/cmd/buildctl/tool"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

type scriptRunner interface {
	runScript(relativePath string, args ...string) error
	runCommand(workdir string, name string, args ...string) error
	repoRootDir() string
}

type commandRunner struct {
	repoRoot string
}

type sharedScriptRunnerAdapter struct {
	inner shared.ScriptRunner
}

func (r commandRunner) runScript(relativePath string, args ...string) error {
	return shared.CommandRunner{RepoRoot: r.repoRoot}.RunScript(relativePath, args...)
}

func (r commandRunner) runCommand(workdir string, name string, args ...string) error {
	return shared.CommandRunner{RepoRoot: r.repoRoot}.RunCommand(workdir, name, args...)
}

func (r commandRunner) repoRootDir() string {
	return r.repoRoot
}

func (a sharedScriptRunnerAdapter) runScript(relativePath string, args ...string) error {
	return a.inner.RunScript(relativePath, args...)
}

func (a sharedScriptRunnerAdapter) runCommand(workdir string, name string, args ...string) error {
	return a.inner.RunCommand(workdir, name, args...)
}

func (a sharedScriptRunnerAdapter) repoRootDir() string {
	return a.inner.RepoRootDir()
}

func findRepoRoot() (string, error)     { return shared.FindRepoRoot() }
func fileExists(path string) bool       { return shared.FileExists(path) }
func validateWebMode(mode string) error { return shared.ValidateWebMode(mode) }

type toolInstallConfig struct {
	web bool
	opt bool
}

func installTools(cfg toolInstallConfig, runner scriptRunner) error {
	return toolpkg.InstallTools(toolpkg.InstallConfig{Web: cfg.web, Opt: cfg.opt}, runnerAdapter{inner: runner})
}

type runnerAdapter struct {
	inner scriptRunner
}

func (a runnerAdapter) RunScript(relativePath string, args ...string) error {
	return a.inner.runScript(relativePath, args...)
}

func (a runnerAdapter) RunCommand(workdir string, name string, args ...string) error {
	return a.inner.runCommand(workdir, name, args...)
}

func (a runnerAdapter) RepoRootDir() string {
	return a.inner.repoRootDir()
}

func buildWasmRuntime(cfg runtimeBuildWasmConfig, runner scriptRunner) error {
	if err := installTools(toolInstallConfig{web: true, opt: cfg.opt}, runner); err != nil {
		return err
	}
	if !cfg.opt {
		return nil
	}
	return compressWasmArtifacts()
}
