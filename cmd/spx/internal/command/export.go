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

package command

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

// ExportBuild runs a platform export with the current Godot project.
func (cmd *CmdTool) ExportBuild(platform string) error {
	logInfof("Starting export: platform=%s, project=%s", platform, cmd.ProjectDir)
	os.MkdirAll(filepath.Join(cmd.ProjectDir, ".builds", strings.ToLower(platform)), 0o755)
	execCmd := exec.Command(cmd.CmdPath, "--headless", "--quit", "--path", cmd.ProjectDir, "--export-debug", platform)
	err := execCmd.Run()
	if err != nil {
		logWarnf("Export failed for platform=%s: %v", platform, err)
	}
	return err
}

// Export exports the current project for the host desktop platform.
func (cmd *CmdTool) Export() error {
	targetDir := filepath.Join(cmd.ProjectDir, ".builds", "pc")
	targetPath, platformName, err := resolveDesktopExportTarget(runtime.GOOS, filepath.Join(targetDir, PcExportName))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	return util.RunCommandInDir(cmd.ProjectDir, cmd.CmdPath, "--headless", "--quit", "--path", cmd.ProjectDir, "--export-debug", platformName, targetPath)
}

func (cmd *CmdTool) prepareExport() error {
	projectDir, _ := filepath.Abs(cmd.ProjectDir)
	util.CopyDir2(filepath.Join(projectDir, "..", "assets"), filepath.Join(cmd.ProjectDir, "assets"))
	return nil
}
