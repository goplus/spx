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

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

const defaultTemplateProjectDir = "cmd/spx/template/project"

type workflowOpenTemplateEditorConfig struct {
	templateDir string
}

func openTemplateEditorWorkflow(cfg workflowOpenTemplateEditorConfig, runner shared.ScriptRunner) error {
	if cfg.templateDir == "" {
		cfg.templateDir = defaultTemplateProjectDir
	}

	templateProjectDir, err := resolveTemplateEditorPath(cfg.templateDir, runner.RepoRootDir())
	if err != nil {
		return err
	}

	version, err := shared.DefaultRuntimeVersion()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Opening template project directly: %s\n", templateProjectDir)
	return runner.RunCommand(
		runner.RepoRootDir(),
		"gdspx"+version,
		"--path", templateProjectDir,
		"--editor",
		"--recovery-mode",
	)
}

func resolveTemplateEditorPath(templateDir, repoRoot string) (string, error) {
	templateProjectDir := templateDir
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

	projectFilePath := filepath.Join(templateProjectDir, "project.godot")
	projectInfo, err := os.Stat(projectFilePath)
	if err != nil {
		return "", fmt.Errorf("template project directory must contain project.godot: %w", err)
	}
	if projectInfo.IsDir() {
		return "", fmt.Errorf("template project path contains a directory named project.godot: %s", projectFilePath)
	}

	return templateProjectDir, nil
}
