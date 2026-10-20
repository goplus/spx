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
	"strings"
	"testing"
)

func TestResolveLauncherOutputProtectsBuildInputs(t *testing.T) {
	moduleRoot := t.TempDir()
	projectDir := filepath.Join(moduleRoot, "project")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bridgeDir := filepath.Join(moduleRoot, "cmd", "ispxnative")
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectFile := filepath.Join(projectDir, "main.spx")
	heroFile := filepath.Join(projectDir, "Hero.spx")
	for _, path := range []string{
		projectFile, heroFile,
		filepath.Join(projectDir, ".config"),
		filepath.Join(moduleRoot, "gox.mod"),
		filepath.Join(moduleRoot, "gop.mod"),
		filepath.Join(bridgeDir, "main.go"),
	} {
		if err := os.WriteFile(path, []byte("input"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inputs := launcherOutputInputs{
		ProjectDir: projectDir, ProjectExt: ".spx", PackDir: "assets",
		Protection: launcherOutputProtection{
			Files: []string{
				projectFile, heroFile,
				filepath.Join(projectDir, ".config"),
				filepath.Join(moduleRoot, "gox.mod"),
				filepath.Join(moduleRoot, "gop.mod"),
			},
			Roots: []string{bridgeDir},
		},
	}
	for _, path := range []string{
		projectFile, heroFile,
		filepath.Join(projectDir, "Future.spx"),
		filepath.Join(projectDir, ".config"),
		filepath.Join(moduleRoot, "gox.mod"),
		filepath.Join(moduleRoot, "gop.mod"),
		filepath.Join(bridgeDir, "main.go"),
		filepath.Join(projectDir, "assets", "launcher"),
	} {
		if _, err := resolveLauncherOutput(inputs, path); err == nil {
			t.Errorf("resolveLauncherOutput accepted build input %q", path)
		}
	}

	defaultOutput, err := resolveLauncherOutput(inputs, "")
	if err != nil {
		t.Fatalf("default output: %v", err)
	}
	canonicalProjectDir, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(defaultOutput, filepath.Join(canonicalProjectDir, ".builds")) {
		t.Fatalf("default output = %q", defaultOutput)
	}
}

func TestResolveLauncherOutputRejectsSymlinkParentAndTarget(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(root, "alias")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	inputs := launcherOutputInputs{ProjectDir: root, PackDir: "assets"}
	if _, err := resolveLauncherOutput(inputs, filepath.Join(linkParent, "launcher")); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink parent error = %v", err)
	}

	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputLink := filepath.Join(root, "launcher")
	if err := os.Symlink(target, outputLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := resolveLauncherOutput(inputs, outputLink); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink output error = %v", err)
	}
}

func TestResolveLauncherOutputRejectsCaseAlias(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "main.spx")
	if err := os.WriteFile(project, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "MAIN.SPX")
	if _, err := os.Stat(alias); err != nil {
		t.Skip("filesystem is case-sensitive")
	}
	_, err := resolveLauncherOutput(launcherOutputInputs{
		ProjectDir: root, PackDir: "assets",
		Protection: launcherOutputProtection{Files: []string{project}},
	}, alias)
	if err == nil || !strings.Contains(err.Error(), "build input") {
		t.Fatalf("case-alias output error = %v", err)
	}
}

func TestStageLauncherOutputUsesPrivateDirectory(t *testing.T) {
	parent := t.TempDir()
	final := filepath.Join(parent, "nested", "launcher")
	stage, cleanup, err := stageLauncherOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	stageDir := filepath.Dir(stage)
	info, err := os.Stat(stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("staging directory mode = %v", info.Mode())
	}
	finalParent := filepath.Dir(final)
	if resolved, resolveErr := filepath.EvalSymlinks(finalParent); resolveErr == nil {
		finalParent = filepath.Clean(resolved)
	}
	if filepath.Dir(stageDir) != finalParent {
		t.Fatalf("staging directory %q is not under output parent %q", stageDir, finalParent)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatalf("stage path exists before compiler creates it: err=%v", err)
	}
}

func TestCommitLauncherOutputRequiresExecutableStageAndCleansDirectory(t *testing.T) {
	parent := t.TempDir()
	final := filepath.Join(parent, "launcher")
	stage, cleanup, err := stageLauncherOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := commitLauncherOutput(stage, final, launcherOutputProtection{}); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("empty stage error = %v", err)
	}
	cleanup()

	stage, cleanup, err = stageLauncherOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageDir := filepath.Dir(stage)
	if err := commitLauncherOutput(stage, final, launcherOutputProtection{}); err != nil {
		t.Fatal(err)
	}
	cleanup()
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Fatalf("staging directory remains after commit: %v", err)
	}
	if got, err := os.ReadFile(final); err != nil || string(got) != "new" {
		t.Fatalf("committed output = %q, err = %v", got, err)
	}
}

func TestCommitLauncherOutputRejectsInvalidStage(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			final := filepath.Join(parent, "launcher")
			stage := stageLauncherTestOutput(t, final)
			want := "regular file"
			switch kind {
			case "missing":
				want = "inspect staged output"
			case "directory":
				if err := os.Mkdir(stage, 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(parent, "other")
				if err := os.WriteFile(target, []byte("target"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, stage); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			if err := commitLauncherOutput(stage, final, launcherOutputProtection{}); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("invalid stage error = %v, want %q", err, want)
			}
			if _, err := os.Lstat(final); !os.IsNotExist(err) {
				t.Fatalf("invalid stage created final output: %v", err)
			}
		})
	}
}

func TestCommitLauncherOutputRechecksProtectedInputs(t *testing.T) {
	for _, kind := range []string{"file", "root"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			final := filepath.Join(parent, "main.spx")
			protection := launcherOutputProtection{Files: []string{final}}
			if kind == "root" {
				root := filepath.Join(parent, "source")
				final = filepath.Join(root, "main.go")
				protection = launcherOutputProtection{Roots: []string{root}}
			}
			writeCommandTestFile(t, final, "source")
			stage := stageLauncherTestOutput(t, filepath.Join(parent, "launcher"))
			if err := os.WriteFile(stage, []byte("launcher"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := commitLauncherOutput(stage, final, protection); err == nil || !strings.Contains(err.Error(), "build input") {
				t.Fatalf("protected commit error = %v", err)
			}
			if got := readTestFile(t, final); got != "source" {
				t.Fatalf("protected input = %q, want source", got)
			}
		})
	}
}

func stageLauncherTestOutput(t *testing.T, final string) string {
	t.Helper()
	stage, cleanup, err := stageLauncherOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return stage
}

func TestBuildLauncherFailurePreservesExistingOutput(t *testing.T) {
	projectDir := t.TempDir()
	output := filepath.Join(projectDir, "game")
	writeCommandTestFile(t, output, "old")
	stage, cleanup, err := stageLauncherOutput(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	cleanup()
	if got, err := os.ReadFile(output); err != nil || string(got) != "old" {
		t.Fatalf("existing output = %q, err = %v", got, err)
	}
	stage, cleanup, err = stageLauncherOutput(output)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(stage, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := commitLauncherOutput(stage, output, launcherOutputProtection{}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "new" {
		t.Fatalf("committed output = %q, err = %v", got, err)
	}
}

func TestLauncherOutputRejectsInvalidParentsAndTargets(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	writeCommandTestFile(t, file, "existing")
	for _, tc := range []struct{ name, path, want string }{
		{"empty", "", "empty"},
		{"file", file, "not a directory"},
		{"missing", filepath.Join(root, "missing"), "does not exist"},
		{"invalid", "invalid\x00path", "inspect output parent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateLauncherOutputParent(tc.path, false); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parent error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := validateLauncherOutputTarget("invalid\x00path"); err == nil || !strings.Contains(err.Error(), "inspect output") {
		t.Fatalf("target error = %v", err)
	}
	if _, err := resolveLauncherOutput(launcherOutputInputs{ProjectDir: filepath.Join(root, "missing")}, "launcher"); err == nil || !strings.Contains(err.Error(), "resolve project directory") {
		t.Fatalf("project error = %v", err)
	}
	if _, err := canonicalizeLauncherOutputAlias(filepath.Join("invalid\x00path", "launcher")); err == nil {
		t.Fatal("accepted invalid parent")
	}
	for _, final := range []string{filepath.Join(file, "launcher"), root, filepath.Join("invalid\x00path", "launcher")} {
		if _, cleanup, err := stageLauncherOutput(final); err == nil {
			cleanup()
			t.Fatalf("staged invalid output %q", final)
		}
	}
	relative, err := filepath.Rel(workingDirectory(t), filepath.Join(root, "relative-launcher"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveLauncherOutput(launcherOutputInputs{ProjectDir: root, PackDir: "assets"}, relative)
	if err != nil || got != filepath.Join(root, "relative-launcher") {
		t.Fatalf("relative output = %q, %v", got, err)
	}
	if err := validateLauncherOutputProtection(file, launcherOutputProtection{Files: []string{""}, Roots: []string{"", "invalid\x00path"}}); err == nil || !strings.Contains(err.Error(), "compare output") {
		t.Fatalf("protection error = %v", err)
	}
	if _, err := launcherPathWithinFilesystem(root, "invalid\x00path"); err == nil {
		t.Fatal("ignored invalid target during containment check")
	}
}

func TestCommitLauncherOutputRevalidatesDestinationAndCleanup(t *testing.T) {
	for _, kind := range []string{"invalid parent", "missing parent", "directory target", "protected destination", "nonprivate stage"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			final := filepath.Join(root, "launcher")
			stage := stageLauncherTestOutput(t, final)
			if err := os.WriteFile(stage, []byte("new launcher"), 0o755); err != nil {
				t.Fatal(err)
			}
			want := ""
			switch kind {
			case "invalid parent":
				final = filepath.Join("invalid\x00path", "launcher")
				want = "resolve output parent"
			case "missing parent":
				final = filepath.Join(root, "missing", "launcher")
				want = "does not exist"
			case "directory target":
				if err := os.Mkdir(final, 0o755); err != nil {
					t.Fatal(err)
				}
				want = "not a regular file"
			case "protected destination":
				writeCommandTestFile(t, final, "previous launcher")
				restrictCommandTestPath(t, root, 0o500)
				want = "install output"
			case "nonprivate stage":
				if err := os.Chmod(filepath.Dir(stage), 0o755); err != nil {
					t.Fatal(err)
				}
				want = "clean staging directory"
			}
			if err := commitLauncherOutput(stage, final, launcherOutputProtection{}); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("commit error = %v, want %q", err, want)
			}
			if kind == "protected destination" {
				if got := readTestFile(t, final); got != "previous launcher" {
					t.Fatalf("failed commit replaced destination: %q", got)
				}
				if got := readTestFile(t, stage); got != "new launcher" {
					t.Fatalf("failed commit changed staged output: %q", got)
				}
			}
			if kind == "nonprivate stage" {
				if got := readTestFile(t, final); got != "new launcher" {
					t.Fatalf("installed output = %q", got)
				}
			}
		})
	}
	root := t.TempDir()
	final := filepath.Join(root, "launcher")
	for _, stage := range []string{filepath.Join(root, "unrelated", "stage"), filepath.Join(root, ".launcher.spx-launchpack-missing", "stage")} {
		if err := removeLauncherStageDir(stage, final); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateLauncherStage(filepath.Join(root, "missing", "stage")); err == nil || !strings.Contains(err.Error(), "parent does not exist") {
		t.Fatalf("missing staging parent = %v", err)
	}
}

func TestStageLauncherOutputRejectsProtectedParent(t *testing.T) {
	for _, parent := range []string{"existing", "missing"} {
		t.Run(parent, func(t *testing.T) {
			root := t.TempDir()
			restrictCommandTestPath(t, root, 0o500)
			final := filepath.Join(root, "launcher")
			want := "create staging directory"
			if parent == "missing" {
				final = filepath.Join(root, "missing", "launcher")
				want = "create output directory"
			}
			if _, cleanup, err := stageLauncherOutput(final); err == nil || !strings.Contains(err.Error(), want) {
				if cleanup != nil {
					cleanup()
				}
				t.Fatalf("staging error = %v, want %q", err, want)
			}
		})
	}
}
