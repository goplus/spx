/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExportPackDarwinUsesPackOnlyExport(t *testing.T) {
	projectDir := t.TempDir()
	cmd := &CmdTool{ProjectDir: projectDir, CmdPath: "godot"}

	var gotDir, gotName string
	var gotArgs []string
	err := cmd.exportPack(goosDarwin, func(dir, name string, args ...string) error {
		gotDir = dir
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})
	if err != nil {
		t.Fatalf("exportPack() error = %v", err)
	}

	targetPath := filepath.Join(projectDir, ".builds", "pc", "gdexport.pck")
	wantArgs := []string{
		"--headless", "--quit", "--path", projectDir,
		"--export-pack", "Mac", targetPath,
	}
	if gotDir != projectDir || gotName != "godot" || !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("export command = (%q, %q, %#v), want (%q, %q, %#v)", gotDir, gotName, gotArgs, projectDir, "godot", wantArgs)
	}
	if info, err := os.Stat(filepath.Dir(targetPath)); err != nil || !info.IsDir() {
		t.Fatalf("pack output directory was not created: %v", err)
	}
}

func TestExportPackRejectsUnsupportedPlatformBeforeRunning(t *testing.T) {
	called := false
	cmd := &CmdTool{ProjectDir: t.TempDir(), CmdPath: "godot"}
	err := cmd.exportPack("plan9", func(string, string, ...string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("exportPack() accepted an unsupported platform")
	}
	if called {
		t.Fatal("export command ran for an unsupported platform")
	}
}

func TestPrepareExportStagesAssetsFromLogicalProject(t *testing.T) {
	sourceProjectDir := filepath.Join(t.TempDir(), "game")
	generatedProjectDir := filepath.Join(sourceProjectDir, "project")
	if err := os.MkdirAll(filepath.Join(sourceProjectDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceProjectDir, "assets", "index.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &CmdTool{TargetAbsDir: sourceProjectDir, ProjectDir: generatedProjectDir}
	if err := cmd.prepareExport(); err != nil {
		t.Fatalf("prepareExport() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(generatedProjectDir, "assets", "index.json")); err != nil {
		t.Fatalf("staged project asset: %v", err)
	}
}

func TestPrepareExportRejectsMissingLogicalProject(t *testing.T) {
	cmd := &CmdTool{ProjectDir: filepath.Join(t.TempDir(), "generated")}
	if err := cmd.prepareExport(); err == nil {
		t.Fatal("prepareExport() accepted an empty logical project directory")
	}
}

func TestPrepareExportRejectsAssetSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "game")
	destination := filepath.Join(source, "project")
	if err := os.MkdirAll(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "assets", "linked.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cmd := &CmdTool{TargetAbsDir: source, ProjectDir: destination}
	if err := cmd.prepareExport(); err == nil {
		t.Fatal("prepareExport() accepted a symlinked asset")
	}
}

func TestPrepareExportRejectsDestinationInsideSourceAssets(t *testing.T) {
	source := filepath.Join(t.TempDir(), "game")
	if err := os.MkdirAll(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := &CmdTool{
		TargetAbsDir: source,
		ProjectDir:   filepath.Join(source, "assets", "generated"),
	}
	if err := cmd.prepareExport(); err == nil {
		t.Fatal("prepareExport() accepted a destination inside the source assets")
	}
}
