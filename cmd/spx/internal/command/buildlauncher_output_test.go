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

func TestCommitLauncherOutputRejectsStagedSymlink(t *testing.T) {
	parent := t.TempDir()
	final := filepath.Join(parent, "launcher")
	stage, cleanup, err := stageLauncherOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "other")
	if err := os.WriteFile(target, []byte("target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, stage); err != nil {
		cleanup()
		t.Skipf("symlink unavailable: %v", err)
	}
	defer cleanup()
	if err := commitLauncherOutput(stage, final, launcherOutputProtection{}); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("staged symlink error = %v", err)
	}
}

func TestCommitLauncherOutputRechecksProtectedInputs(t *testing.T) {
	parent := t.TempDir()
	final := filepath.Join(parent, "main.spx")
	if err := os.WriteFile(final, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	stage, cleanup, err := stageLauncherOutput(filepath.Join(parent, "launcher"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(stage, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	protection := launcherOutputProtection{Files: []string{final}}
	if err := commitLauncherOutput(stage, final, protection); err == nil || !strings.Contains(err.Error(), "build input") {
		t.Fatalf("protected commit error = %v", err)
	}
	if data, err := os.ReadFile(final); err != nil || string(data) != "source" {
		t.Fatalf("protected input = %q, err = %v", data, err)
	}
}

func TestCommitLauncherOutputRechecksProtectedRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "source")
	final := filepath.Join(root, "main.go")
	writeLauncherTestFile(t, final, "source")
	stage, cleanup, err := stageLauncherOutput(filepath.Join(parent, "launcher"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(stage, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	protection := launcherOutputProtection{Roots: []string{root}}
	if err := commitLauncherOutput(stage, final, protection); err == nil || !strings.Contains(err.Error(), "build input") {
		t.Fatalf("protected root commit error = %v", err)
	}
	if data, err := os.ReadFile(final); err != nil || string(data) != "source" {
		t.Fatalf("protected root input = %q, err = %v", data, err)
	}
}
