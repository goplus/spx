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
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

func exportWebRuntime(cfg runtimeExportWebConfig, runner shared.ScriptRunner) error {
	if err := toolpkg.InstallTools(toolpkg.InstallConfig{Web: true, NoEmbedRuntime: true}, runner); err != nil {
		return err
	}

	spxCommand, err := webModeSPXCommand(cfg.mode)
	if err != nil {
		return err
	}
	outputZip, err := webModeOutputZip(cfg.mode)
	if err != nil {
		return err
	}

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.RepoRootDir(), false)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runRepoSPXCommand(runner, workspace.workDir, spxCommand); err != nil {
		return err
	}

	return shared.ZipDirectory(filepath.Join(workspace.workDir, "project", ".builds", "web"), filepath.Join(workspace.repoRoot, outputZip))
}

func ExportWebTemplateRuntime(mode string, runner shared.ScriptRunner) error {
	if err := shared.ValidateWebMode(mode); err != nil {
		return err
	}

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.RepoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runRepoSPXCommand(runner, workspace.workDir, "exporttemplateweb"); err != nil {
		return err
	}

	srcDir := filepath.Join(workspace.workDir, "project", ".builds", "webi")
	dstDir := filepath.Join(workspace.goBinDir, fmt.Sprintf("gdspxrt%s_web%s", workspace.version, mode))
	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}
	if err := shared.CopyDir(srcDir, dstDir); err != nil {
		return err
	}

	enginePack := filepath.Join(dstDir, "engine.pck")
	engineZip := filepath.Join(dstDir, "engine.zip")
	if !shared.FileExists(enginePack) {
		return fmt.Errorf("web runtime engine pack not found at %s", enginePack)
	}
	if err := os.Rename(enginePack, engineZip); err != nil {
		return err
	}

	engineJS := filepath.Join(dstDir, "engine.js")
	content, err := os.ReadFile(engineJS)
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("var EnginePackMode = '%s';\n", mode)
	return os.WriteFile(engineJS, append([]byte(prefix), content...), 0o644)
}

func webModeOutputZip(mode string) (string, error) {
	if err := shared.ValidateWebMode(mode); err != nil {
		return "", err
	}
	switch mode {
	case "normal":
		return "spx_web.zip", nil
	case "worker":
		return "spx_web_worker.zip", nil
	case "minigame":
		return "spx_web_minigame.zip", nil
	case "miniprogram":
		return "spx_web_miniprogram.zip", nil
	default:
		return "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func webModeSPXCommand(mode string) (string, error) {
	if err := shared.ValidateWebMode(mode); err != nil {
		return "", err
	}
	switch mode {
	case "normal":
		return "exportweb", nil
	case "worker":
		return "exportwebworker", nil
	case "minigame":
		return "exportminigame", nil
	case "miniprogram":
		return "exportminiprogram", nil
	default:
		return "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}
