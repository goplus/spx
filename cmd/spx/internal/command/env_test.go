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
	"time"

	builderai "github.com/goplus/spx/v2/cmd/spx/internal/command/builderai"
)

func TestAdaptGoModAddsLocalReplaceForGeneratedGoMod(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	cmd := CmdTool{
		TargetDir:     targetDir,
		TargetAbsDir:  targetDir,
		GoModTemplate: "module github.com/goplus/spxdemo\n\ngo 1.25.0\n",
	}
	cmd.adaptGoMod()

	_, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err == nil {
		t.Fatalf("ReadFile(go.mod) returned error: %v", err)
	}
}

func TestAdaptGoModRepairsStaleLocalReplacePath(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	goModPath := filepath.Join(targetDir, "go.mod")
	content := "module github.com/goplus/spxdemo\n\ngo 1.25.0\n\nreplace github.com/goplus/spx/v2 => ../../..\n"
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) returned error: %v", err)
	}

	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir}
	cmd.adaptGoMod()

	_, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("ReadFile(go.mod) returned error: %v", err)
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

func TestShouldRunGoModTidy(t *testing.T) {
	repoTargetDir := setupAdaptGoModFixture(t)
	repoCmd := CmdTool{TargetDir: repoTargetDir, TargetAbsDir: repoTargetDir}
	if repoCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned true in local repo, want false")
	}

	if err := os.WriteFile(filepath.Join(repoTargetDir, builderai.DescriptionFile), []byte("summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", builderai.DescriptionFile, err)
	}
	if !repoCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned false for local ai project, want true")
	}

	externalTargetDir := t.TempDir()
	externalCmd := CmdTool{TargetDir: externalTargetDir, TargetAbsDir: externalTargetDir}
	if !externalCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned false outside local repo, want true")
	}
}

func TestShouldReimport(t *testing.T) {
	tests := []struct {
		name        string
		cmdName     string
		runtimeMode bool
		cacheExists bool
		tags        string
		want        bool
	}{
		{name: "skips buildweb", cmdName: "buildweb", want: false},
		{name: "reimports web template export", cmdName: "exporttemplateweb", want: true},
		{name: "reimports exportweb when cache missing", cmdName: "exportweb", want: true},
		{name: "skips runtime mode", cmdName: "runweb", runtimeMode: true, want: false},
		{name: "skips pure engine mode", cmdName: "build", tags: "pure_engine", want: false},
		{name: "skips when cache exists", cmdName: "exportweb", cacheExists: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			if tt.cacheExists {
				writeProjectImportCache(t, projectDir)
			}

			cmd := CmdTool{
				ProjectDir:  projectDir,
				RuntimeMode: tt.runtimeMode,
				Args:        ExtraArgs{CmdName: tt.cmdName},
			}
			if tt.tags != "" {
				tags := tt.tags
				cmd.Args.Tags = &tags
			}

			if got := cmd.ShouldReimport(); got != tt.want {
				t.Fatalf("ShouldReimport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectImportUsesRecoveryMode(t *testing.T) {
	t.Setenv(projectImportTimeoutEnvVar, "0")
	cmd := CmdTool{CmdPath: "godot", ProjectDir: "/tmp/spx-project"}
	execCmd, _, _, cancel := cmd.newProjectImportCommand()
	defer cancel()

	got := strings.Join(execCmd.Args[1:], " ")
	want := "--headless --path /tmp/spx-project --import --recovery-mode"
	if got != want {
		t.Fatalf("project import args = %q, want %q", got, want)
	}
}

func TestProjectImportTimeout(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{name: "uses default", envValue: "", want: defaultProjectImportTimeout},
		{name: "supports override", envValue: "90s", want: 90 * time.Second},
		{name: "trims whitespace", envValue: " 90s ", want: 90 * time.Second},
		{name: "supports disable", envValue: "0", want: 0},
		{name: "falls back for invalid value", envValue: "abc", want: defaultProjectImportTimeout},
		{name: "falls back for negative value", envValue: "-1s", want: defaultProjectImportTimeout},
	}

	cmd := CmdTool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(projectImportTimeoutEnvVar, tt.envValue)
			if got := cmd.projectImportTimeout(); got != tt.want {
				t.Fatalf("projectImportTimeout() = %s, want %s", got, tt.want)
			}
		})
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

func writeProjectImportCache(t *testing.T, projectDir string) {
	t.Helper()

	cachePath := filepath.Join(projectDir, ".godot", "uid_cache.bin")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cache"), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
}
