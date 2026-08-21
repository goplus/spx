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
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

type runtimeExportPackConfig struct {
	engineAssetDir string
}

var prepareRuntimePackEngine = engine.PrepareLinuxRuntimePackAssets

func runRuntimeExportPack(args []string) error {
	cfg, err := parseRuntimeExportPackArgs(args)
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
	return exportPackRuntime(cfg, shared.CommandRunner{RepoRoot: repoRoot})
}

func ExportPackRuntime(runner shared.ScriptRunner) error {
	return exportPackRuntime(runtimeExportPackConfig{}, runner)
}

func parseRuntimeExportPackArgs(args []string) (runtimeExportPackConfig, error) {
	cfg := runtimeExportPackConfig{}
	fs := flag.NewFlagSet("runtime export-pack", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.engineAssetDir, "engine-asset-dir", "", "read Linux/amd64 engine assets from a local directory")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl runtime export-pack [--engine-asset-dir path]")
	}
	if err := fs.Parse(args); err != nil {
		return runtimeExportPackConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return runtimeExportPackConfig{}, errUsage
	}
	return cfg, nil
}

func exportPackRuntime(cfg runtimeExportPackConfig, runner shared.ScriptRunner) error {
	if cfg.engineAssetDir != "" {
		if err := prepareRuntimePackEngine(runner.RepoRootDir(), cfg.engineAssetDir); err != nil {
			return err
		}
	}
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
	if err := shared.WriteNamedZip(filepath.Join(workspace.goBinDir, release.RuntimeAssetZipName), map[string]string{
		"gdspxrt.pck":         workspace.outputPack,
		"runtime.gdextension": runtimeExtension,
	}); err != nil {
		return err
	}
	return writeLocalRuntimeManifestIfComplete(workspace.repoRoot, workspace.goBinDir)
}

func findExportedPack(workDir string) (string, error) {
	pcDir := filepath.Join(workDir, "project", ".builds", "pc")
	pcPack := filepath.Join(pcDir, "gdexport.pck")
	if shared.FileExists(pcPack) {
		return pcPack, nil
	}
	appResources, err := filepath.Glob(filepath.Join(pcDir, "gdexport.app", "Contents", "Resources", "*.pck"))
	if err != nil {
		return "", err
	}
	sort.Strings(appResources)
	if len(appResources) != 0 {
		return appResources[0], nil
	}
	return "", fmt.Errorf("exported runtime pack not found in %s", workDir)
}
