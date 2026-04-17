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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdaptGoModDoesNotGenerateGoModInLocalRepo(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	cmd := CmdTool{
		TargetDir:     targetDir,
		TargetAbsDir:  targetDir,
		GoModTemplate: "module github.com/goplus/spxdemo\n\ngo 1.25.0\n",
	}
	cmd.adaptGoMod()

	_, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err == nil {
		t.Fatal("ReadFile(go.mod) returned nil error, want file to remain absent")
	}
}

func TestAdaptGoModCreatesGoModOutsideLocalRepo(t *testing.T) {
	targetDir := t.TempDir()
	cmd := CmdTool{
		TargetDir:     targetDir,
		TargetAbsDir:  targetDir,
		GoModTemplate: "module github.com/goplus/spxdemo\n\ngo 1.25.0\n",
	}
	cmd.adaptGoMod()

	content, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile(go.mod) returned error: %v", err)
	}
	if string(content) != cmd.GoModTemplate {
		t.Fatalf("go.mod content = %q, want %q", string(content), cmd.GoModTemplate)
	}
}

func TestShouldRunGoModTidy(t *testing.T) {
	repoTargetDir := setupAdaptGoModFixture(t)
	repoCmd := CmdTool{TargetDir: repoTargetDir, TargetAbsDir: repoTargetDir}
	if repoCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned true in local repo, want false")
	}

	externalTargetDir := t.TempDir()
	externalCmd := CmdTool{TargetDir: externalTargetDir, TargetAbsDir: externalTargetDir}
	if !externalCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned false outside local repo, want true")
	}
}

func TestEnsureSpxModuleReplaceRepairsIndentedReplaceBlock(t *testing.T) {
	content := "module github.com/goplus/spxdemo\n\ngo 1.25.0\n\nreplace (\n\tgithub.com/goplus/spx/v2 => ../../..\n)\n"

	updated := ensureSpxModuleReplace(content, "../..")

	if strings.Contains(updated, "../../..") {
		t.Fatalf("updated content = %q, stale replace path still present", updated)
	}
	if !strings.Contains(updated, "\tgithub.com/goplus/spx/v2 => ../..") {
		t.Fatalf("updated content = %q, want repaired indented replace path ../..", updated)
	}
	if count := strings.Count(updated, "github.com/goplus/spx/v2 =>"); count != 1 {
		t.Fatalf("updated content has %d replace directives, want 1", count)
	}
}

func TestEnsureSpxModuleReplacePreservesTrailingBlankLinesAndCRLF(t *testing.T) {
	content := "module github.com/goplus/spxdemo\r\n\r\ngo 1.25.0\r\n\r\n"

	updated := ensureSpxModuleReplace(content, "../..")
	want := content + "replace github.com/goplus/spx/v2 => ../..\r\n"
	if updated != want {
		t.Fatalf("updated content = %q, want %q", updated, want)
	}
}

func setupAdaptGoModFixture(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	// Keep the fixture two levels below the temp repo root so filepath.Rel resolves to ../...
	targetDir := filepath.Join(repoRoot, "tutorial", "05-Animation")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", targetDir, err)
	}
	writeLocalSpxRepoMarker(t, repoRoot)
	return targetDir
}
