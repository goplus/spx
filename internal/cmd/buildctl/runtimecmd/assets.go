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
	"sort"
)

const runtimeIndexJSON = `{"map":{"width":480,"height":360}}`

type runtimeWorkspace struct {
	repoRoot   string
	workDir    string
	goBinDir   string
	version    string
	outputPack string
}

func exportPackRuntime(runner scriptRunner) error {
	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", "export"); err != nil {
		return err
	}

	exportedPack, err := findExportedPack(workspace.workDir)
	if err != nil {
		return err
	}
	if err := copyFile(exportedPack, workspace.outputPack); err != nil {
		return err
	}

	runtimeExtension := filepath.Join(workspace.goBinDir, "runtime.gdextension")
	if !fileExists(runtimeExtension) {
		return fmt.Errorf("runtime extension not found at %s", runtimeExtension)
	}

	dstZip := filepath.Join(workspace.goBinDir, fmt.Sprintf("gdspxrt.pck.%s.zip", workspace.version))
	return writeNamedZip(dstZip, map[string]string{
		"gdspxrt.pck":         workspace.outputPack,
		"runtime.gdextension": runtimeExtension,
	})
}

func exportWebRuntime(cfg runtimeExportWebConfig, runner scriptRunner) error {
	if err := installTools(toolInstallConfig{web: true, opt: true}, runner); err != nil {
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

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), false)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", spxCommand); err != nil {
		return err
	}

	return zipDirectory(filepath.Join(workspace.workDir, "project", ".builds", "web"), filepath.Join(workspace.repoRoot, outputZip))
}

func exportWebTemplateRuntime(mode string, runner scriptRunner) error {
	if err := validateWebMode(mode); err != nil {
		return err
	}

	workspace, cleanup, err := prepareRuntimeWorkspace(runner.repoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runner.runCommand(workspace.workDir, "spx", "exporttemplateweb"); err != nil {
		return err
	}

	srcDir := filepath.Join(workspace.workDir, "project", ".builds", "webi")
	dstDir := filepath.Join(workspace.goBinDir, fmt.Sprintf("gdspxrt%s_web%s", workspace.version, mode))
	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}
	if err := copyDir(srcDir, dstDir); err != nil {
		return err
	}

	enginePack := filepath.Join(dstDir, "engine.pck")
	engineZip := filepath.Join(dstDir, "engine.zip")
	if !fileExists(enginePack) {
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

func prepareRuntimeWorkspace(repoRoot string, includeRuntimeExtension bool) (runtimeWorkspace, func(), error) {
	version, err := defaultRuntimeVersion()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	goPath, err := ensureGoPath()
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}

	tempRoot := filepath.Join(repoRoot, ".tmp")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	workDir, err := os.MkdirTemp(tempRoot, "runtime-*")
	if err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.MkdirAll(filepath.Join(workDir, "assets"), 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
	}
	if err := os.MkdirAll(filepath.Join(goPath, "bin"), 0o755); err != nil {
		return runtimeWorkspace{}, nil, err
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
		if err := copyFile(src, dst); err != nil {
			return runtimeWorkspace{}, nil, err
		}
	}

	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}
	return runtimeWorkspace{
		repoRoot:   repoRoot,
		workDir:    workDir,
		goBinDir:   filepath.Join(goPath, "bin"),
		version:    version,
		outputPack: filepath.Join(goPath, "bin", fmt.Sprintf("gdspxrt%s.pck", version)),
	}, cleanup, nil
}

func webModeOutputZip(mode string) (string, error) {
	if err := validateWebMode(mode); err != nil {
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
	if err := validateWebMode(mode); err != nil {
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

func findExportedPack(workDir string) (string, error) {
	pcPack := filepath.Join(workDir, "project", ".builds", "pc", "gdexport.pck")
	if fileExists(pcPack) {
		return pcPack, nil
	}

	appResources, err := filepath.Glob(filepath.Join(workDir, "project", ".builds", "pc", "gdexport.app", "Contents", "Resources", "*.pck"))
	if err != nil {
		return "", err
	}
	sort.Strings(appResources)
	if len(appResources) > 0 {
		return appResources[0], nil
	}
	return "", fmt.Errorf("exported runtime pack not found in %s", workDir)
}
