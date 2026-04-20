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

package tool

import (
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
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

type scriptRunnerAdapter struct {
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

func (a scriptRunnerAdapter) runScript(relativePath string, args ...string) error {
	return a.inner.RunScript(relativePath, args...)
}

func (a scriptRunnerAdapter) runCommand(workdir string, name string, args ...string) error {
	return a.inner.RunCommand(workdir, name, args...)
}

func (a scriptRunnerAdapter) repoRootDir() string {
	return a.inner.RepoRootDir()
}

func findRepoRoot() (string, error)  { return shared.FindRepoRoot() }
func fileExists(path string) bool    { return shared.FileExists(path) }
func ensureGoPath() (string, error)  { return shared.EnsureGoPath() }
func copyDir(src, dst string) error  { return shared.CopyDir(src, dst) }
func shellQuote(value string) string { return shared.ShellQuote(value) }
func runStreamingCommand(workdir, name string, args ...string) error {
	return shared.RunStreamingCommand(workdir, name, args...)
}
func fetchURLToFile(url, dst string) error   { return shared.FetchURLToFile(url, dst) }
func extractZip(srcZip, dstDir string) error { return shared.ExtractZip(srcZip, dstDir) }
func copyFile(src, dst string) error         { return shared.CopyFile(src, dst) }

func installTools(cfg toolInstallConfig, runner scriptRunner) error {
	args := []string{}
	if cfg.web {
		args = append(args, "--web")
	}
	if cfg.opt {
		args = append(args, "--opt")
	}
	if cfg.noEmbedRuntime {
		args = append(args, "--no-embed-runtime")
	}
	return runner.runScript("cmd/spx/install.sh", args...)
}
