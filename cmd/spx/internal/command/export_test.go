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
	"testing"
)

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
