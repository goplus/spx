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

func TestAdaptGoModAddsLocalReplaceForGeneratedGoMod(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	cmd := CmdTool{
		TargetDir:     targetDir,
		GoModTemplate: "module github.com/goplus/spxdemo\n\ngo 1.25.0\n",
	}
	cmd.adaptGoMod()

	content, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile(go.mod) returned error: %v", err)
	}
	if !strings.Contains(string(content), "replace github.com/goplus/spx/v2 => ../..") {
		t.Fatalf("go.mod content = %q, want replace path ../..", string(content))
	}
}

func TestAdaptGoModRepairsStaleLocalReplacePath(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	goModPath := filepath.Join(targetDir, "go.mod")
	content := "module github.com/goplus/spxdemo\n\ngo 1.25.0\n\nreplace github.com/goplus/spx/v2 => ../../..\n"
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) returned error: %v", err)
	}

	cmd := CmdTool{TargetDir: targetDir}
	cmd.adaptGoMod()

	updated, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("ReadFile(go.mod) returned error: %v", err)
	}
	if strings.Contains(string(updated), "../../..") {
		t.Fatalf("go.mod content = %q, stale replace path still present", string(updated))
	}
	if !strings.Contains(string(updated), "replace github.com/goplus/spx/v2 => ../..") {
		t.Fatalf("go.mod content = %q, want repaired replace path ../..", string(updated))
	}
}

func setupAdaptGoModFixture(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	targetDir := filepath.Join(repoRoot, "tutorial", "05-Animation")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", targetDir, err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "gox.mod"), []byte("project main.spx Game github.com/goplus/spx/v2 math\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(gox.mod) returned error: %v", err)
	}
	return targetDir
}
