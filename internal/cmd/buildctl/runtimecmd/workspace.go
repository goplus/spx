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
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

const (
	runtimeIndexJSON            = `{"map":{"width":480,"height":360}}`
	runtimeWorkspaceProjectName = "spx-runtime"
)

type runtimeWorkspace struct {
	repoRoot   string
	workDir    string
	goBinDir   string
	version    string
	outputPack string
}

func runRepoSPXCommand(runner shared.ScriptRunner, projectDir string, args ...string) error {
	commandArgs := append([]string{"run", "./cmd/spx"}, args...)
	commandArgs = append(commandArgs, "--path", projectDir)
	return runner.RunCommand(runner.RepoRootDir(), "go", commandArgs...)
}

func prepareRuntimeWorkspace(repoRoot string, includeRuntimeExtension bool) (runtimeWorkspace, func(), error) {
	version, err := shared.DefaultRuntimeVersion()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	goPath, err := shared.EnsureGoPath()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}

	tempRoot := filepath.Join(repoRoot, ".tmp")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	workspaceRoot, err := os.MkdirTemp(tempRoot, "runtime-workspace-*")
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(workspaceRoot) }
	complete := false
	defer func() {
		if !complete {
			cleanup()
		}
	}()

	workDir := filepath.Join(workspaceRoot, runtimeWorkspaceProjectName)
	for _, directory := range []string{filepath.Join(workDir, "assets"), filepath.Join(goPath, "bin")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return runtimeWorkspace{}, nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, "assets", "index.json"), []byte(runtimeIndexJSON), 0o644); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.WriteFile(filepath.Join(workDir, "main.spx"), nil, 0o644); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.RemoveAll(filepath.Join(workDir, "project", ".builds")); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if includeRuntimeExtension {
		src := filepath.Join(repoRoot, "cmd", "spx", "template", "project", "runtime.gdextension.txt")
		dst := filepath.Join(goPath, "bin", "runtime.gdextension")
		if err := shared.CopyFile(src, dst); err != nil {
			return runtimeWorkspace{}, nil, err
		}
	}

	complete = true
	return runtimeWorkspace{
		repoRoot: repoRoot, workDir: workDir, goBinDir: filepath.Join(goPath, "bin"), version: version,
		outputPack: filepath.Join(goPath, "bin", fmt.Sprintf("gdspxrt%s.pck", version)),
	}, cleanup, nil
}
