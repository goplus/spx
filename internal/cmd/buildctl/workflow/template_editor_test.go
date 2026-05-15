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
	"os"
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

	mustMkdirAll(t, filepath.Join(templateProjectDir, "engine"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "engine", "scene.tscn"), []byte("[gd_scene]\n"))
	mustMkdirAll(t, filepath.Join(templateProjectDir, "go"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "go", "ios_adapter.go.txt"), []byte("package main\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "export_presets.cfg"), []byte("[preset.0]\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "gdspx.gdextension"), []byte("extension\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "gdspx.gdextension.uid"), []byte("uid\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "main.tscn"), []byte("[gd_scene]\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "project.godot"), []byte(`[application]`+"\n"+`config/name="spx"`+"\n"))
	mustWriteFile(t, filepath.Join(templateProjectDir, "runtime.gdextension.txt"), []byte("runtime extension\n"))

	runner := &recordingRunner{repoRoot: repoRoot}
	cfg := workflowOpenTemplateEditorConfig{}
	if err := openTemplateEditorWorkflow(cfg, runner); err != nil {
		t.Fatalf("openTemplateEditorWorkflow returned error: %v", err)
	}

	workspaceRoot := filepath.Join(repoRoot, defaultTemplateEditorWorkspaceDir, "spx")
	expectedCommands := []recordedCommand{
		{dir: workspaceRoot, name: "spx", args: []string{"build"}},
		{dir: workspaceRoot, name: "gdspx" + release.DefaultReleaseMeta().Runtime.Version, args: []string{"--path", "./project", "--editor"}},
	}
	if !reflect.DeepEqual(runner.commands, expectedCommands) {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}

	assertFileExists(t, filepath.Join(workspaceRoot, "main.spx"))
	assertDirExists(t, filepath.Join(workspaceRoot, "project", ".godot"))
	assertDirExists(t, filepath.Join(workspaceRoot, "project", ".builds"))
	assertDirExists(t, filepath.Join(workspaceRoot, "project", "go"))
	assertSymlinkTarget(t, filepath.Join(workspaceRoot, "project", "engine"), filepath.Join(templateProjectDir, "engine"))
	assertSymlinkTarget(t, filepath.Join(workspaceRoot, "project", "project.godot"), filepath.Join(templateProjectDir, "project.godot"))
	assertSymlinkTarget(t, filepath.Join(workspaceRoot, "project", "go", "ios_adapter.go.txt"), filepath.Join(templateProjectDir, "go", "ios_adapter.go.txt"))
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory: %s", path)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file: %s", path)
	}
}

func assertSymlinkTarget(t *testing.T, linkPath, targetPath string) {
	t.Helper()
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink: %s", linkPath)
	}
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink %s: %v", linkPath, err)
	}
	if filepath.Clean(got) != filepath.Clean(targetPath) {
		t.Fatalf("symlink target = %s, want %s", got, targetPath)
	}
}
