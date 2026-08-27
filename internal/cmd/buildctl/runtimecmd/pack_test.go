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
	"os"
	"path/filepath"
	"testing"
)

func TestExportPackRuntimePreparesExplicitEngineAssets(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	assetDir := filepath.Join(runner.repoRoot, "runtime-assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPrepare := prepareRuntimePackEngine
	defer func() { prepareRuntimePackEngine = oldPrepare }()
	var gotRepoRoot, gotAssetDir string
	prepareRuntimePackEngine = func(repoRoot, dir string) error {
		gotRepoRoot, gotAssetDir = repoRoot, dir
		return nil
	}

	if err := exportPackRuntime(runtimeExportPackConfig{engineAssetDir: assetDir}, runner); err != nil {
		t.Fatalf("exportPackRuntime returned error: %v", err)
	}
	if gotRepoRoot != runner.repoRoot || gotAssetDir != assetDir {
		t.Fatalf("engine preparation = (%q, %q), want (%q, %q)", gotRepoRoot, gotAssetDir, runner.repoRoot, assetDir)
	}
}

func TestFindExportedPack(t *testing.T) {
	root := t.TempDir()
	pcDir := filepath.Join(root, "project", ".builds", "pc")
	direct := filepath.Join(pcDir, "gdexport.pck")
	mustWriteFile(t, direct, nil)
	if got, err := findExportedPack(root); err != nil || got != direct {
		t.Fatalf("findExportedPack() = %q, %v; want %q, nil", got, err, direct)
	}
}

func TestFindExportedPackRejectsLegacyAppOutput(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "project", ".builds", "pc", "gdexport.app", "Contents", "Resources", "gdexport.pck")
	mustWriteFile(t, legacy, nil)
	if _, err := findExportedPack(root); err == nil {
		t.Fatal("findExportedPack accepted a full macOS app export")
	}
}

func TestFindExportedPackRejectsMissingOutput(t *testing.T) {
	if _, err := findExportedPack(t.TempDir()); err == nil {
		t.Fatal("findExportedPack accepted missing output")
	}
}

func TestExportPackRuntimeCleansFailedWorkspace(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	runner.commandHook = func(string, string, ...string) error { return errors.New("export failed") }

	if err := ExportPackRuntime(runner); err == nil {
		t.Fatal("ExportPackRuntime accepted failed export")
	}
	entries, err := os.ReadDir(filepath.Join(runner.repoRoot, ".tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed runtime workspaces remain: %v", entries)
	}
}
