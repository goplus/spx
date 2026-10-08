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
	"embed"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	builderai "github.com/goplus/spx/v3/cmd/spx/internal/command/builderai"
)

func TestAdaptGoModSkipsScaffoldInsideLocalSPXRepository(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	cmd := CmdTool{
		TargetDir:     targetDir,
		TargetAbsDir:  targetDir,
		GoModTemplate: "this scaffold must not be written",
	}
	cmd.adaptGoMod()

	if _, err := os.Stat(filepath.Join(targetDir, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("local SPX workspace unexpectedly created a scaffold go.mod: %v", err)
	}
}

func TestClearBuildRemovesArtifactsOnly(t *testing.T) {
	projectDir := t.TempDir()
	buildDir := filepath.Join(projectDir, ".builds")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", buildDir, err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "artifact"), []byte("build"), 0o644); err != nil {
		t.Fatalf("WriteFile(build artifact) returned error: %v", err)
	}
	projectFile := filepath.Join(projectDir, "project.gd")
	if err := os.WriteFile(projectFile, []byte("project"), 0o644); err != nil {
		t.Fatalf("WriteFile(project file) returned error: %v", err)
	}

	cmd := CmdTool{ProjectDir: projectDir}
	if err := cmd.ClearBuild(); err != nil {
		t.Fatalf("ClearBuild() returned error: %v", err)
	}
	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Fatalf("build directory still exists or returned unexpected error: %v", err)
	}
	if _, err := os.Stat(projectFile); err != nil {
		t.Fatalf("project file was removed: %v", err)
	}
}

func TestAdaptGoModPreservesExistingModule(t *testing.T) {
	for _, location := range []string{"local repository", "external project"} {
		for _, module := range []struct{ name, content string }{
			{"single-line replacement", "module example.com/game\n\n// Keep the selected dependency.\nreplace github.com/goplus/spx/v3 => ../../.. // stale local path\n"},
			{"CRLF replacement block", "module example.com/game\r\n\r\nreplace (\r\n\tgithub.com/goplus/spx/v3 => ../../.. // keep this comment\r\n)\r\n\r\n"},
			{"no replacement", "// User-owned module without a replacement.\nmodule example.com/game\n"},
		} {
			t.Run(location+"/"+module.name, func(t *testing.T) {
				content := module.content
				targetDir := t.TempDir()
				if location == "local repository" {
					targetDir = setupAdaptGoModFixture(t)
				}
				goModPath := filepath.Join(targetDir, "go.mod")
				if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir, GoModTemplate: "must not replace existing content"}
				cmd.adaptGoMod()
				got, err := os.ReadFile(goModPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != content {
					t.Fatalf("go.mod changed: got %q, want %q", got, content)
				}
			})
		}
	}
}

func TestAdaptGoModCreatesExternalScaffoldOnce(t *testing.T) {
	targetDir := t.TempDir()
	const template = "module example.com/game\r\n\r\n// Exact scaffold bytes.\r\n"
	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir, GoModTemplate: template}
	for range 2 {
		cmd.adaptGoMod()
		got, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != template {
			t.Fatalf("go.mod = %q, want %q", got, template)
		}
		cmd.GoModTemplate = "must not replace the existing scaffold"
	}
}

func TestAdaptGoModIgnoresScaffoldErrors(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "missing parent", "project")
	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir, GoModTemplate: "module example.com/game\n"}
	cmd.adaptGoMod()
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("failed scaffold unexpectedly created the target: %v", err)
	}

	targetDir = t.TempDir()
	goModPath := filepath.Join(targetDir, "go.mod")
	if err := os.Mkdir(goModPath, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd.TargetDir, cmd.TargetAbsDir = targetDir, targetDir
	cmd.adaptGoMod()
	if info, err := os.Stat(goModPath); err != nil || !info.IsDir() {
		t.Fatalf("non-file go.mod was changed: %v", err)
	}
}

func TestPrepareCommandNormalizesRelativeProjectPath(t *testing.T) {
	targetDir, err := filepath.EvalSymlinks(setupAdaptGoModFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Dir(targetDir))
	relativePath := filepath.Base(targetDir)
	wantAbsDir, err := filepath.Abs(relativePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "main.spx"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("GODEBUG", os.Getenv("GODEBUG"))
	serverAddr, tags := "", "pure_engine"
	cmd := CmdTool{
		FileSuffix: "spx", Version: "test", ProjectFS: webExportTestFS,
		GoModTemplate: "must not create a nested local module",
		Args:          ExtraArgs{CmdName: "build", Path: &relativePath, ServerAddr: &serverAddr, Tags: &tags},
	}
	spec, ok := findCommand("build")
	if !ok {
		t.Fatal("build command is not registered")
	}
	if err := cmd.prepareCommand(spec, "template/project", "project"); err != nil {
		t.Fatal(err)
	}
	if cmd.TargetAbsDir != wantAbsDir || !filepath.IsAbs(cmd.TargetAbsDir) {
		t.Fatalf("TargetAbsDir = %q, want %q", cmd.TargetAbsDir, wantAbsDir)
	}
	if cmd.TargetDir != "." || *cmd.Args.Path != "." {
		t.Fatalf("relative command paths = %q, %q, want .", cmd.TargetDir, *cmd.Args.Path)
	}
	if cmd.ProjectDir != filepath.Join(wantAbsDir, "project") {
		t.Fatalf("ProjectDir = %q, want generated project below %q", cmd.ProjectDir, wantAbsDir)
	}
	if _, err := os.Stat(filepath.Join(wantAbsDir, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("relative local project unexpectedly received a nested module: %v", err)
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

func TestPrepareEnvRunsGoModTidyInTargetDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake Go tool uses a POSIX shell script")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	targetDir := filepath.Join(root, "source project")
	binDir := filepath.Join(root, "tools")
	for _, dir := range []string{targetDir, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const script = `#!/bin/sh
set -eu
test -f xgo_autogen.go
printf '%s\000' "$PWD" "$@" "$SPX_TIDY_TEST_VALUE" > "$SPX_TIDY_TEST_LOG"
`
	if err := os.WriteFile(filepath.Join(binDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "tidy.log")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SPX_TIDY_TEST_LOG", logPath)
	t.Setenv("SPX_TIDY_TEST_VALUE", "inherited value")
	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir, ProjectFS: webExportTestFS}
	cmd.PrepareEnv("template/project", filepath.Join(targetDir, "project"))
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := targetDir + "\x00mod\x00tidy\x00inherited value\x00"; string(got) != want {
		t.Fatalf("Go invocation = %q, want %q", got, want)
	}
	if cwd, err := os.Getwd(); err != nil || cwd != root {
		t.Fatalf("working directory = %q (%v), want %q", cwd, err, root)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "xgo_autogen.go")); !os.IsNotExist(err) {
		t.Fatalf("temporary generated source was not removed: %v", err)
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
		{name: "skips exportweb", cmdName: "exportweb", want: false},
		{name: "skips exportwebworker", cmdName: "exportwebworker", want: false},
		{name: "skips exportminigame", cmdName: "exportminigame", want: false},
		{name: "skips exportminiprogram", cmdName: "exportminiprogram", want: false},
		{name: "skips runtime mode", cmdName: "runweb", runtimeMode: true, want: false},
		{name: "skips pure engine mode", cmdName: "build", tags: "pure_engine", want: false},
		{name: "skips when cache exists", cmdName: "exporttemplateweb", cacheExists: true, want: false},
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
	// Keep the fixture nested so root detection must search parent directories.
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

func TestRunCmdRejectsInvalidTargetBeforeClearing(t *testing.T) {
	for _, command := range []string{"clear", "clearbuild"} {
		for _, targetKind := range []string{"missing", "file"} {
			t.Run(command+"/"+targetKind, func(t *testing.T) {
				root := t.TempDir()
				t.Chdir(root)
				protected := []string{
					filepath.Join("project", ".builds", "artifact"),
					filepath.Join(".temp", "artifact"),
					"go.sum", "xgo_autogen.go",
				}
				for _, name := range protected {
					if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(name, []byte("keep"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				target := filepath.Join(root, "invalid-target")
				if targetKind == "file" {
					if err := os.WriteFile(target, []byte("file"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				oldFlags, oldArgs := flag.CommandLine, os.Args
				flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
				flag.CommandLine.SetOutput(io.Discard)
				os.Args = []string{"spx", command, "--path", target}
				t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

				err := (&CmdTool{}).RunCmd("spx", ".spx", "test", embed.FS{}, "", "project")
				if err == nil {
					t.Error("RunCmd accepted an invalid target directory")
				}
				for _, name := range protected {
					if got, err := os.ReadFile(name); err != nil || string(got) != "keep" {
						t.Errorf("original directory file %s = %q, %v; want untouched", name, got, err)
					}
				}
			})
		}
	}
}
