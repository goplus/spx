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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goplus/spx/v2/internal/release"
)

func TestParseWorkflowOpenTemplateEditorArgsDefault(t *testing.T) {
	cfg, err := parseWorkflowOpenTemplateEditorArgs(nil)
	if err != nil {
		t.Fatalf("parseWorkflowOpenTemplateEditorArgs returned error: %v", err)
	}
	if cfg.templateDir != defaultTemplateProjectDir {
		t.Fatalf("unexpected templateDir: %q", cfg.templateDir)
	}
	if cfg.workspaceDir != defaultTemplateEditorWorkspaceDir {
		t.Fatalf("unexpected workspaceDir: %q", cfg.workspaceDir)
	}
}

func TestOpenTemplateEditorWorkflow(t *testing.T) {
	repoRoot := t.TempDir()
	templateProjectDir := filepath.Join(repoRoot, "cmd", "spx", "template", "project")

	mustMkdirAll(t, templateProjectDir)
	mustWriteFile(t, filepath.Join(templateProjectDir, "project.godot"), []byte(`[application]`+"\n"+`config/name="spx"`+"\n"))

	runner := &recordingRunner{repoRoot: repoRoot}
	cfg := workflowOpenTemplateEditorConfig{}
	if err := openTemplateEditorWorkflow(cfg, runner); err != nil {
		t.Fatalf("openTemplateEditorWorkflow returned error: %v", err)
	}

	expectedCommands := []recordedCommand{
		{dir: repoRoot, name: "gdspx" + release.DefaultReleaseMeta().Runtime.Version, args: []string{"--path", templateProjectDir, "--editor", "--recovery-mode"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
}

func TestResolveTemplateEditorPathAcceptsCustomProjectDirName(t *testing.T) {
	repoRoot := t.TempDir()
	templateProjectDir := filepath.Join(repoRoot, "custom-template-dir")

	mustMkdirAll(t, templateProjectDir)
	mustWriteFile(t, filepath.Join(templateProjectDir, "project.godot"), []byte(`[application]`+"\n"+`config/name="spx"`+"\n"))

	got, err := resolveTemplateEditorPath(templateProjectDir, repoRoot)
	if err != nil {
		t.Fatalf("resolveTemplateEditorPath returned error: %v", err)
	}
	if got != templateProjectDir {
		t.Fatalf("resolveTemplateEditorPath = %q, want %q", got, templateProjectDir)
	}
}
