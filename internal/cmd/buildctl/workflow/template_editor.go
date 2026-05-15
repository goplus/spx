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
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
)

const (
	defaultTemplateProjectDir         = "cmd/spx/template/project"
	defaultTemplateEditorWorkspaceDir = ".tmp/template-editor"
)

type workflowOpenTemplateEditorConfig struct {
	templateDir  string
	workspaceDir string
}

func openTemplateEditorWorkflow(cfg workflowOpenTemplateEditorConfig, runner scriptRunner) error {
	if cfg.templateDir == "" {
		cfg.templateDir = defaultTemplateProjectDir
	}
	if cfg.workspaceDir == "" {
		cfg.workspaceDir = defaultTemplateEditorWorkspaceDir
	}

	workspaceRoot, err := prepareTemplateEditorWorkspace(cfg, runner.repoRootDir())
	if err != nil {
		return err
	}

	version, err := shared.DefaultRuntimeVersion()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Opening template project via workspace: %s\n", workspaceRoot)
	if err := runner.runCommand(workspaceRoot, "spx", "build"); err != nil {
		return err
	}
	return runner.runCommand(workspaceRoot, "gdspx"+version, "--path", "./project", "--editor")
}

func prepareTemplateEditorWorkspace(cfg workflowOpenTemplateEditorConfig, repoRoot string) (string, error) {
	templateProjectDir := cfg.templateDir
	if !filepath.IsAbs(templateProjectDir) {
		templateProjectDir = filepath.Join(repoRoot, templateProjectDir)
	}
	templateProjectDir = filepath.Clean(templateProjectDir)

	info, err := os.Stat(templateProjectDir)
	if err != nil {
		return "", fmt.Errorf("template project directory %s: %w", templateProjectDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("template project path is not a directory: %s", templateProjectDir)
	}

	workspaceBase := cfg.workspaceDir
	if !filepath.IsAbs(workspaceBase) {
		workspaceBase = filepath.Join(repoRoot, workspaceBase)
	}
	workspaceBase = filepath.Clean(workspaceBase)
	workspaceRoot := filepath.Join(workspaceBase, "spx")
	workspaceProjectDir := filepath.Join(workspaceRoot, "project")

	for _, dir := range []string{
		workspaceRoot,
		filepath.Join(workspaceProjectDir, ".builds"),
		filepath.Join(workspaceProjectDir, ".godot"),
		filepath.Join(workspaceProjectDir, "go"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create workspace directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.spx"), nil, 0o644); err != nil {
		return "", fmt.Errorf("create workspace main.spx: %w", err)
	}

	for _, name := range []string{
		"engine",
		"export_presets.cfg",
		"gdspx.gdextension",
		"gdspx.gdextension.uid",
		"main.tscn",
		"project.godot",
		"runtime.gdextension.txt",
	} {
		if err := ensureSymlink(filepath.Join(templateProjectDir, name), filepath.Join(workspaceProjectDir, name)); err != nil {
			return "", err
		}
	}

	goTemplateDir := filepath.Join(templateProjectDir, "go")
	goEntries, err := os.ReadDir(goTemplateDir)
	if err != nil {
		return "", fmt.Errorf("read template Go directory %s: %w", goTemplateDir, err)
	}
	for _, entry := range goEntries {
		if err := ensureSymlink(filepath.Join(goTemplateDir, entry.Name()), filepath.Join(workspaceProjectDir, "go", entry.Name())); err != nil {
			return "", err
		}
	}

	return workspaceRoot, nil
}

func ensureSymlink(targetPath, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", linkPath, err)
	}

	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			existingTarget, err := os.Readlink(linkPath)
			if err == nil {
				resolvedTarget := existingTarget
				if !filepath.IsAbs(resolvedTarget) {
					resolvedTarget = filepath.Join(filepath.Dir(linkPath), resolvedTarget)
				}
				if filepath.Clean(resolvedTarget) == filepath.Clean(targetPath) {
					return nil
				}
			}
		}
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("remove existing path %s: %w", linkPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing path %s: %w", linkPath, err)
	}

	if err := os.Symlink(targetPath, linkPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", linkPath, targetPath, err)
	}
	return nil
}
