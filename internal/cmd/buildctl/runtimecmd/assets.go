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
	"runtime"
	"sort"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
	"github.com/goplus/spx/v3/internal/release"
)

const runtimeIndexJSON = `{"map":{"width":480,"height":360}}`

// runtimeWorkspaceProjectName is the stable project name seen by cmd/spx and
// embedded in the exported Godot pack. The parent directory remains unique so
// concurrent exports do not share state.
const runtimeWorkspaceProjectName = "spx-runtime"

type runtimeWorkspace struct {
	repoRoot   string
	workDir    string
	goBinDir   string
	version    string
	outputPack string
}

func ExportPackRuntime(runner shared.ScriptRunner) error {
	workspace, cleanup, err := prepareRuntimeWorkspace(runner.RepoRootDir(), true)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := runRepoSPXCommand(runner, workspace.workDir, "export"); err != nil {
		return err
	}

	exportedPack, err := findExportedPack(workspace.workDir)
	if err != nil {
		return err
	}
	if err := shared.CopyFile(exportedPack, workspace.outputPack); err != nil {
		return err
	}

	runtimeExtension := filepath.Join(workspace.goBinDir, "runtime.gdextension")
	if !shared.FileExists(runtimeExtension) {
		return fmt.Errorf("runtime extension not found at %s", runtimeExtension)
	}

	dstZip := filepath.Join(workspace.goBinDir, release.RuntimeAssetZipName)
	if err := shared.WriteNamedZip(dstZip, map[string]string{
		"gdspxrt.pck":         workspace.outputPack,
		"runtime.gdextension": runtimeExtension,
	}); err != nil {
		return err
	}
	return writeLocalRuntimeManifestIfComplete(workspace.repoRoot, workspace.goBinDir)
}

// writeLocalRuntimeManifestIfComplete publishes the explicit source-mode
// runtime declaration after the Engine and PCK have both been produced. A
// standalone export-pack may legitimately run before the Engine build, so it
// leaves no incomplete manifest in that case.
func writeLocalRuntimeManifestIfComplete(repoRoot, goBinDir string) error {
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	enginePath := filepath.Join(goBinDir, spec.RuntimeName)
	if _, err := os.Lstat(enginePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect local runtime Engine: %w", err)
	}
	packPath := filepath.Join(goBinDir, spec.PackName)
	manifest, err := release.NewLocalRuntimeManifest(lock, spec.GOOS, spec.GOARCH, enginePath, packPath)
	if err != nil {
		return err
	}
	manifestPath, err := release.LocalRuntimeManifestPath(repoRoot, lock, spec.GOOS, spec.GOARCH)
	if err != nil {
		return err
	}
	if err := release.PublishLocalRuntimeManifest(manifestPath, manifest, enginePath, packPath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Local runtime manifest: %s\n", manifestPath)
	return nil
}

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

func runRepoSPXCommand(runner shared.ScriptRunner, projectDir string, args ...string) error {
	commandArgs := []string{"run", "./cmd/spx"}
	commandArgs = append(commandArgs, args...)
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
	cleanup := func() {
		_ = os.RemoveAll(workspaceRoot)
	}
	complete := false
	defer func() {
		if !complete {
			cleanup()
		}
	}()

	workDir := filepath.Join(workspaceRoot, runtimeWorkspaceProjectName)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
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
		if err := shared.CopyFile(src, dst); err != nil {
			return runtimeWorkspace{}, nil, err
		}
	}

	complete = true
	return runtimeWorkspace{
		repoRoot:   repoRoot,
		workDir:    workDir,
		goBinDir:   filepath.Join(goPath, "bin"),
		version:    version,
		outputPack: filepath.Join(goPath, "bin", fmt.Sprintf("gdspxrt%s.pck", version)),
	}, cleanup, nil
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

func findExportedPack(workDir string) (string, error) {
	pcPack := filepath.Join(workDir, "project", ".builds", "pc", "gdexport.pck")
	if shared.FileExists(pcPack) {
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
