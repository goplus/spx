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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	builderai "github.com/goplus/spx/v4/cmd/spx/internal/command/builderai"
)

func TestAdaptGoModSkipsScaffoldInsideLocalSPXRepository(t *testing.T) {
	targetDir := setupAdaptGoModFixture(t)

	cmd := CmdTool{
		TargetDir:     targetDir,
		GoModTemplate: "this scaffold must not be written",
	}
	if err := cmd.adaptGoMod(); err != nil {
		t.Fatal(err)
	}

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
			{"single-line replacement", "module example.com/game\n\n// Keep the selected dependency.\nreplace github.com/goplus/spx/v4 => ../../.. // stale local path\n"},
			{"CRLF replacement block", "module example.com/game\r\n\r\nreplace (\r\n\tgithub.com/goplus/spx/v4 => ../../.. // keep this comment\r\n)\r\n\r\n"},
			{"no replacement", "// User-owned module without a replacement.\nmodule example.com/game\n"},
		} {
			t.Run(location+"/"+module.name, func(t *testing.T) {
				content := module.content
				targetDir := t.TempDir()
				if location == "local repository" {
					targetDir = setupAdaptGoModFixture(t)
				}
				goModPath := filepath.Join(targetDir, "go.mod")
				writeCommandTestFile(t, goModPath, content)
				cmd := CmdTool{TargetDir: targetDir, GoModTemplate: "must not replace existing content"}
				if err := cmd.adaptGoMod(); err != nil {
					t.Fatal(err)
				}
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
	cmd := CmdTool{TargetDir: targetDir, GoModTemplate: template}
	for range 2 {
		if err := cmd.adaptGoMod(); err != nil {
			t.Fatal(err)
		}
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

func TestAdaptGoModReportsScaffoldErrors(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "missing parent", "project")
	cmd := CmdTool{TargetDir: targetDir, GoModTemplate: "module example.com/game\n"}
	if err := cmd.adaptGoMod(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing target error = %v", err)
	}
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("failed scaffold unexpectedly created the target: %v", err)
	}

	targetDir = t.TempDir()
	goModPath := filepath.Join(targetDir, "go.mod")
	if err := os.Mkdir(goModPath, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd.TargetDir = targetDir
	if err := cmd.adaptGoMod(); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("non-file module error = %v", err)
	}
	if info, err := os.Stat(goModPath); err != nil || !info.IsDir() {
		t.Fatalf("non-file go.mod was changed: %v", err)
	}

	t.Run("inaccessible module", func(t *testing.T) {
		targetDir := t.TempDir()
		goModPath := filepath.Join(targetDir, "go.mod")
		writeCommandTestFile(t, goModPath, "module example.com/keep\n")
		restrictCommandTestPath(t, targetDir, 0o600)
		cmd := CmdTool{TargetDir: targetDir, GoModTemplate: "must not replace existing module"}
		if err := cmd.adaptGoMod(); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("inaccessible module error = %v, want permission error", err)
		}
		if err := os.Chmod(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if got := readTestFile(t, goModPath); got != "module example.com/keep\n" {
			t.Fatalf("failed module inspection changed go.mod: %q", got)
		}
	})
}

func TestShouldRunGoModTidy(t *testing.T) {
	repoTargetDir := setupAdaptGoModFixture(t)
	repoCmd := CmdTool{TargetDir: repoTargetDir}
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
	externalCmd := CmdTool{TargetDir: externalTargetDir}
	if !externalCmd.shouldRunGoModTidy() {
		t.Fatal("shouldRunGoModTidy returned false outside local repo, want true")
	}
}

func TestPrepareEnvRunsGoModTidyInTargetDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	targetDir := filepath.Join(root, "source project")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	installCommandTestTools(t, map[string]string{"go": `test -f xgo_autogen.go
printf '%s\000' "$PWD" "$@" "$SPX_TIDY_TEST_VALUE" > "$SPX_TIDY_TEST_LOG"
`})
	logPath := filepath.Join(root, "tidy.log")
	t.Setenv("SPX_TIDY_TEST_LOG", logPath)
	t.Setenv("SPX_TIDY_TEST_VALUE", "inherited value")
	cmd := CmdTool{TargetDir: targetDir, ProjectFS: webExportTestFS}
	if err := cmd.PrepareEnv("template/project", filepath.Join(targetDir, "project")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := targetDir + "\x00mod\x00tidy\x00inherited value\x00"; string(got) != want {
		t.Fatalf("Go invocation = %q, want %q", got, want)
	}
	assertWorkingDirectory(t, root)
	if _, err := os.Stat(filepath.Join(targetDir, "xgo_autogen.go")); !os.IsNotExist(err) {
		t.Fatalf("temporary generated source was not removed: %v", err)
	}
}

func TestPrepareEnvReturnsFailuresAndCleansTemporarySource(t *testing.T) {
	for _, stage := range []string{"project files", "project module", "temporary Go source", "module dependencies"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			before := workingDirectory(t)
			cmd := CmdTool{TargetDir: root, ProjectFS: webExportTestFS}
			destination := filepath.Join(root, "project")
			tempFile := filepath.Join(root, "xgo_autogen.go")
			switch stage {
			case "project files":
				writeCommandTestFile(t, destination, "file blocks project directory")
			case "project module":
				if err := os.Mkdir(filepath.Join(root, "go.mod"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "temporary Go source":
				if err := os.Mkdir(tempFile, 0o755); err != nil {
					t.Fatal(err)
				}
			case "module dependencies":
				installCommandTestTools(t, map[string]string{"go": "exit 23"})
			}
			err := cmd.PrepareEnv("template/project", destination)
			if err == nil || !strings.Contains(err.Error(), "prepare "+stage) {
				t.Fatalf("preparation error = %v, want failure in %s", err, stage)
			}
			assertWorkingDirectory(t, before)
			if stage == "module dependencies" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
					t.Fatalf("tidy error = %v, want wrapped exit status 23", err)
				}
			}
			if stage != "temporary Go source" {
				if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
					t.Fatalf("temporary source remains after failure: %v", err)
				}
			}
		})
	}
}

func TestPrepareEnvPreservesTidyAndCleanupFailures(t *testing.T) {
	for _, exitCode := range []string{"0", "23"} {
		t.Run("tidy exit "+exitCode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("SPX_TIDY_TEST_EXIT", exitCode)
			installCommandTestTools(t, map[string]string{"go": `/bin/rm xgo_autogen.go
/bin/mkdir xgo_autogen.go
printf keep > xgo_autogen.go/keep
exit "$SPX_TIDY_TEST_EXIT"
`})
			cmd := CmdTool{TargetDir: root, ProjectFS: webExportTestFS}
			err := cmd.PrepareEnv("template/project", filepath.Join(root, "project"))
			if err == nil || !strings.Contains(err.Error(), "remove temporary Go source") {
				t.Fatalf("cleanup error = %v", err)
			}
			var cleanupErr *os.PathError
			if !errors.As(err, &cleanupErr) || cleanupErr.Path != filepath.Join(root, "xgo_autogen.go") {
				t.Fatalf("cleanup error lost its path: %v", err)
			}
			var exitErr *exec.ExitError
			if exitCode == "23" {
				if !strings.Contains(err.Error(), "prepare module dependencies") || !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
					t.Fatalf("cleanup masked the tidy failure: %v", err)
				}
			} else if errors.As(err, &exitErr) {
				t.Fatalf("successful tidy reported an exit failure: %v", err)
			}
			if got := readTestFile(t, filepath.Join(root, "xgo_autogen.go", "keep")); got != "keep" {
				t.Fatalf("cleanup removed replacement contents: %q", got)
			}
		})
	}
}

func TestShouldReimport(t *testing.T) {
	tests := []struct {
		name        string
		cmdName     string
		cacheExists bool
		want        bool
	}{
		{name: "reimports native build", cmdName: "build", want: true},
		{name: "reimports without command metadata", want: true},
		{name: "skips buildtinygo", cmdName: "buildtinygo", want: false},
		{name: "skips buildweb", cmdName: "buildweb", want: false},
		{name: "reimports web template export", cmdName: "exporttemplateweb", want: true},
		{name: "skips exportweb", cmdName: "exportweb", want: false},
		{name: "skips exportwebworker", cmdName: "exportwebworker", want: false},
		{name: "skips exportminigame", cmdName: "exportminigame", want: false},
		{name: "skips exportminiprogram", cmdName: "exportminiprogram", want: false},
		{name: "skips native runtime", cmdName: "runnative", want: false},
		{name: "skips interpreted runtime", cmdName: "run", want: false},
		{name: "skips web runtime", cmdName: "runweb", want: false},
		{name: "skips worker runtime", cmdName: "runwebworker", want: false},
		{name: "skips when cache exists", cmdName: "exporttemplateweb", cacheExists: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			if tt.cacheExists {
				writeProjectImportCache(t, projectDir)
			}

			cmd := CmdTool{
				ProjectDir: projectDir,
				Args:       ExtraArgs{CmdName: tt.cmdName},
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
					writeCommandTestFile(t, name, "keep")
				}
				target := filepath.Join(root, "invalid-target")
				if targetKind == "file" {
					writeCommandTestFile(t, target, "file")
				}
				setTestCommandLine(t, "spx", command, "--path", target)

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

func TestEnsureBuilderAIModuleFiles(t *testing.T) {
	const template = `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v4 v4.0.0-test //xgo:class
`
	for _, test := range []struct {
		name, description string
		existing, local   bool
	}{
		{name: "new module", description: builderai.DescriptionFile},
		{name: "existing module", description: builderai.DescriptionFile, existing: true},
		{name: "raw description filename", description: builderai.DescriptionFileRaw},
		{name: "local SPX replacement", description: builderai.DescriptionFile, local: true},
		{name: "no description"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			if test.local {
				projectRoot = setupAdaptGoModFixture(t)
			}
			godotProjectDir := filepath.Join(projectRoot, "project")
			if err := os.MkdirAll(godotProjectDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if test.description != "" {
				writeCommandTestFile(t, filepath.Join(projectRoot, test.description), "summary")
			}
			if test.existing {
				writeCommandTestFile(t, filepath.Join(projectRoot, "gox.mod"), "project main.spx Game github.com/goplus/spx/v4 math\n")
				writeCommandTestFile(t, filepath.Join(projectRoot, "go.mod"), `module demo

go 1.25.0

require (
	github.com/goplus/spx/v4 v4.0.0-test //xgo:class
)
`)
			}
			cmd := CmdTool{ProjectDir: godotProjectDir, GoModTemplate: template}
			if err := cmd.ensureBuilderAIModuleFiles(projectRoot); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"gox.mod", "go.mod"} {
				if _, err := os.Stat(filepath.Join(godotProjectDir, name)); !os.IsNotExist(err) {
					t.Errorf("%s should not be generated in project/ subdir: %v", name, err)
				}
				if test.description == "" {
					if _, err := os.Stat(filepath.Join(projectRoot, name)); !os.IsNotExist(err) {
						t.Errorf("%s should not be generated without description: %v", name, err)
					}
				}
			}
			if test.description == "" {
				return
			}
			goxMod := readTestFile(t, filepath.Join(projectRoot, "gox.mod"))
			if !strings.Contains(goxMod, `import "`+builderai.ModulePath+`"`) || strings.Contains(goxMod, "\nrunner ") {
				t.Fatalf("gox.mod must import builder AI without a legacy runner:\n%s", goxMod)
			}
			goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
			if !strings.Contains(goMod, builderai.ModulePath+" "+builderai.Version) || strings.Contains(goMod, "//xgo:class") {
				t.Fatalf("go.mod must require builder AI without the SPX class marker:\n%s", goMod)
			}
			if test.local && !strings.Contains(goMod, "replace github.com/goplus/spx/v4 => ../..") {
				t.Fatalf("go.mod must retain the local SPX replacement:\n%s", goMod)
			}
		})
	}
}

func TestProjectImportTimeoutStopsTool(t *testing.T) {
	binDir := installCommandTestTools(t, map[string]string{"godot": "exec /bin/sleep 30"})
	t.Setenv(projectImportTimeoutEnvVar, "10ms")
	cmd := &CmdTool{ProjectDir: t.TempDir(), CmdPath: filepath.Join(binDir, "godot")}
	start := time.Now()
	err := cmd.runProjectImport()
	if err == nil || !strings.Contains(err.Error(), "godot import timed out after 10ms") || !strings.Contains(err.Error(), projectImportTimeoutEnvVar) {
		t.Fatalf("timeout error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timed-out import took %s", elapsed)
	}
}

func TestSetupPaths(t *testing.T) {
	for _, test := range []struct {
		name, address, wantError string
		port                     int
	}{
		{"server address", "localhost:8060", "", 8060},
		{"missing port", "localhost", "invalid server address format", 0},
		{"invalid port", "localhost:not-a-port", "invalid port number", 0},
		{"missing directory", "", "failed to resolve target directory", 0},
		{"file as directory", "", "not a directory", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := workingDirectory(t)
			target := t.TempDir()
			switch test.name {
			case "missing directory":
				target = filepath.Join(target, "missing")
			case "file as directory":
				target = filepath.Join(target, "file")
				writeCommandTestFile(t, target, "not a directory")
			}
			address := test.address
			cmd := &CmdTool{Args: ExtraArgs{Path: &target, ServerAddr: &address}}
			err := cmd.setupPaths("project")
			assertWorkingDirectory(t, before)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("setupPaths error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := filepath.EvalSymlinks(target)
			if err != nil {
				t.Fatal(err)
			}
			if cmd.ServerPort != test.port || cmd.TargetDir != resolved || *cmd.Args.Path != resolved || cmd.ProjectDir != filepath.Join(resolved, "project") {
				t.Fatalf("resolved command paths/port = %+v", cmd)
			}
		})
	}
}

func TestResolveAppPathSearchAndFallback(t *testing.T) {
	for _, scenario := range []string{"working PATH binary", "failed PATH binary", "missing PATH binary", "invalid fallback"} {
		t.Run(scenario, func(t *testing.T) {
			name := "gdspxtest" + executableSuffix(runtime.GOOS)
			tools := map[string]string{}
			if scenario == "working PATH binary" {
				tools[name] = `test "$*" = --version`
			}
			if scenario == "failed PATH binary" {
				tools[name] = "exit 3"
			}
			binDir := installCommandTestTools(t, tools)
			fallback := t.TempDir()
			fallbackBinary := filepath.Join(fallback, name)
			writeCommandTestFile(t, fallbackBinary, "fallback binary")
			if scenario == "invalid fallback" {
				fallback = "\x00"
			}
			got, err := resolveAppPath(fallback, "gdspx", "test")
			if scenario == "invalid fallback" {
				if err == nil {
					t.Fatal("invalid fallback path accepted")
				}
				return
			}
			want := fallbackBinary
			if scenario == "working PATH binary" {
				want = filepath.Join(binDir, name)
			}
			if err != nil || got != want {
				t.Fatalf("app path = %q, %v; want %q", got, err, want)
			}
			if scenario != "working PATH binary" {
				if info, err := os.Stat(got); err != nil || info.Mode()&0o111 == 0 {
					t.Fatalf("fallback binary was not made executable: %v", err)
				}
			}
		})
	}
}

func TestResolveAppPathReportsPermissionRepairFailure(t *testing.T) {
	bin := t.TempDir()
	binary := filepath.Join(bin, "gdspxtest")
	writeCommandTestFile(t, binary, "engine")
	var command, enable, disable string
	switch runtime.GOOS {
	case "linux":
		command, enable, disable = "chattr", "+i", "-i"
	case "darwin":
		command, enable, disable = "chflags", "uchg", "nouchg"
	default:
		t.Skip("requires a file with immutable permissions")
	}
	flagTool, err := exec.LookPath(command)
	if err != nil {
		t.Skipf("immutable fixture requires %s: %v", command, err)
	}
	if output, err := exec.Command(flagTool, enable, binary).CombinedOutput(); err != nil {
		// Linux requires filesystem support and CAP_LINUX_IMMUTABLE. Ordinary
		// read-only modes still allow the owner to chmod their own file.
		t.Skipf("cannot make the temporary fixture immutable: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if output, err := exec.Command(flagTool, disable, binary).CombinedOutput(); err != nil {
			t.Errorf("restore engine fixture: %v: %s", err, output)
		}
	})
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 != 0 {
		t.Fatalf("fixture must be a nonexecutable regular file: %v, %v", info, err)
	}
	t.Setenv("PATH", t.TempDir())
	got, err := resolveAppPath(bin, "gdspx", "test")
	var pathErr *os.PathError
	if got != binary || !errors.As(err, &pathErr) || pathErr.Op != "chmod" || pathErr.Path != binary {
		t.Fatalf("app path = %q, %v; want %q and its chmod error", got, err, binary)
	}
	if info, err := os.Stat(binary); err != nil || info.Mode()&0o111 != 0 {
		t.Fatalf("failed permission repair changed engine mode: %v, %v", info, err)
	}
}

func TestFindSpxRoot(t *testing.T) {
	for _, scenario := range []string{"current directory", "parent directory", "missing repository", "SPX dependency only", "unconfigured"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			start, want := root, root
			if scenario == "missing repository" || scenario == "unconfigured" {
				want = ""
			} else {
				writeLocalSpxRepoMarker(t, root)
			}
			switch scenario {
			case "unconfigured":
				start = ""
			case "parent directory":
				start = filepath.Join(root, "tutorial", "00-Hello")
				if err := os.MkdirAll(start, 0o755); err != nil {
					t.Fatal(err)
				}
			case "SPX dependency only":
				writeCommandTestFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
				writeCommandTestFile(t, filepath.Join(root, "gox.mod"), "project main.spx Game github.com/goplus/spx/v4 math\n")
				want = ""
			}
			cmd := CmdTool{TargetDir: start}
			if got := cmd.findSpxRoot(); got != want {
				t.Fatalf("findSpxRoot = %q, want %q", got, want)
			}
		})
	}
}

func TestPrepareCommandRejectsInvalidPlatformEngineAndSession(t *testing.T) {
	for _, scenario := range []string{"unsupported architecture", "invalid engine path", "missing target directory", "blocked session directory", "missing project template"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GOARCH", "amd64")
			t.Setenv("PATH", t.TempDir())
			root := t.TempDir()
			serverAddr := ""
			cmd := &CmdTool{
				Version: "test", FileSuffix: "spx", GoBinPath: filepath.Join(root, "bin"),
				Args: ExtraArgs{Path: &root, ServerAddr: &serverAddr},
			}
			writeCommandTestFile(t, filepath.Join(root, "main.spx"), "")
			t.Setenv("GODEBUG", os.Getenv("GODEBUG"))
			want := ""
			switch scenario {
			case "unsupported architecture":
				t.Setenv("GOARCH", "386")
				want = "requires an amd64, or an arm64 system"
			case "invalid engine path":
				cmd.GoBinPath = "invalid\x00path"
				want = "requires engine to be installed"
			case "missing target directory":
				target := filepath.Join(root, "missing")
				cmd.Args.Path = &target
				want = "failed to resolve target directory"
			case "blocked session directory":
				writeCommandTestFile(t, filepath.Join(root, ".temp"), "keep")
				want = "failed to create temp directory"
			case "missing project template":
				want = "prepare project files"
			}
			err := cmd.prepareCommand(commandSpec{setup: projectSetup}, "template/project", "project")
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("setup error = %v, want %q", err, want)
			}
			if scenario == "missing target directory" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing source error = %v, want ErrNotExist", err)
				}
				if _, err := os.Stat(*cmd.Args.Path); !os.IsNotExist(err) {
					t.Fatalf("failed setup created the missing source directory: %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "xgo_autogen.go")); !os.IsNotExist(err) {
				t.Fatalf("failed setup generated project files: %v", err)
			}
		})
	}
}

func TestPrepareCommandAcceptsRelativeTargetDirectory(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "directory"
		if linked {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			target, err := filepath.EvalSymlinks(setupAdaptGoModFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(filepath.Dir(target))
			before := workingDirectory(t)
			t.Setenv("GOOS", "linux")
			t.Setenv("GOARCH", "amd64")
			binDir := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			writeCommandTestFile(t, filepath.Join(binDir, "gdspxtest"+executableSuffix(runtime.GOOS)), "engine fixture")
			source := filepath.Base(target)
			if linked {
				source = "linked-game"
				if err := os.Symlink(target, source); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			serverAddr := ""
			cmd := CmdTool{
				Version: "test", FileSuffix: "spx", ProjectFS: webExportTestFS, GoBinPath: binDir,
				Args: ExtraArgs{Path: &source, ServerAddr: &serverAddr},
			}
			writeCommandTestFile(t, filepath.Join(target, "main.spx"), "")
			t.Setenv("GODEBUG", os.Getenv("GODEBUG"))
			if err := cmd.prepareCommand(commandSpec{setup: projectSetup}, "template/project", "project"); err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(target, "project")
			for name, paths := range map[string][2]string{
				"engine":   {cmd.CmdPath, filepath.Join(binDir, "gdspxtest"+executableSuffix(runtime.GOOS))},
				"runtime":  {cmd.RuntimeCmdPath, filepath.Join(binDir, "gdspxrttest"+executableSuffix(runtime.GOOS))},
				"source":   {cmd.TargetDir, target},
				"argument": {*cmd.Args.Path, target},
				"project":  {cmd.ProjectDir, project},
				"go":       {cmd.GoDir, filepath.Join(project, "go")},
				"library":  {cmd.LibPath, filepath.Join(project, "lib", libraryFileName(envName, "linux", "amd64"))},
			} {
				if paths[0] != paths[1] {
					t.Errorf("%s path = %q, want %q", name, paths[0], paths[1])
				}
			}
			if _, err := os.Stat(filepath.Join(project, "project.godot")); err != nil {
				t.Fatal(err)
			}
			assertWorkingDirectory(t, before)
			// Root consumers must not depend on the caller's cwd or generated directory.
			t.Chdir(t.TempDir())
			if got, want := cmd.findSpxRoot(), filepath.Dir(filepath.Dir(target)); got != want {
				t.Fatalf("source repository = %q, want %q", got, want)
			}
			if cmd.shouldRunGoModTidy() {
				t.Fatal("local source project unexpectedly requires module tidy")
			}
			if _, err := os.Stat(filepath.Join(target, "go.mod")); !os.IsNotExist(err) {
				t.Fatalf("local source project was scaffolded: %v", err)
			}
			roots := cmd.interpretedRoots()
			if roots.ProjectDir != target || roots.SessionDir != filepath.Join(target, ".temp") {
				t.Fatalf("interpreted roots after cwd change = %+v", roots)
			}
			if got, want := cmd.webServerPIDPath(), filepath.Join(target, ".gdspx_web_server.pid"); got != want {
				t.Fatalf("PID path = %q, want %q", got, want)
			}
		})
	}
}

func TestCommandsReportDeletedWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot remove the current working directory")
	}
	root := t.TempDir()
	cwd := filepath.Join(root, "removed")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	if err := os.Remove(cwd); err != nil {
		t.Fatal(err)
	}
	// macOS can still resolve an unlinked cwd; these cases require Getwd to fail.
	if cwd, err := os.Getwd(); err == nil {
		t.Skipf("Getwd still resolves the removed directory as %q", cwd)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Getwd in removed working directory = %v", err)
	}
	target := "."
	cmd := CmdTool{Args: ExtraArgs{Path: &target}}
	if err := cmd.setupPaths("project"); err == nil || !strings.Contains(err.Error(), "failed to resolve target directory") {
		t.Fatalf("deleted working directory error = %v", err)
	}
	cmd = CmdTool{GoBinPath: "relative-runtime"}
	if dirs := cmd.runtimeSearchDirs(); len(dirs) == 0 || dirs[0] != cmd.GoBinPath {
		t.Fatalf("runtime search lost configured directory when cwd disappeared: %v", dirs)
	}
}

func TestClearStopsAtProtectedArtifact(t *testing.T) {
	artifacts := []string{"project", ".temp", "go.sum", "xgo_autogen.go"}
	for blockedIndex, artifact := range artifacts {
		t.Run(artifact, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range artifacts {
				writeCommandTestFile(t, filepath.Join(root, name, "keep"), "protected contents")
			}
			protected := filepath.Join(root, artifact)
			restrictCommandTestPath(t, protected, 0o500)
			cmd := &CmdTool{TargetDir: root, ProjectDir: filepath.Join(root, "project")}
			if err := cmd.Clear(); err == nil || !strings.Contains(err.Error(), "failed to remove project directory") {
				t.Fatalf("clear error = %v", err)
			}
			for index, name := range artifacts {
				artifactPath := filepath.Join(root, name)
				if index < blockedIndex {
					if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
						t.Fatalf("earlier artifact %q remains: %v", name, err)
					}
				} else if got := readTestFile(t, filepath.Join(artifactPath, "keep")); got != "protected contents" {
					t.Fatalf("clear changed protected or later artifact %q: %q", name, got)
				}
			}
		})
	}
}

func TestUpdateProjectNameReportsReadAndWriteFailures(t *testing.T) {
	for _, scenario := range []string{"missing project file", "read-only project file"} {
		t.Run(scenario, func(t *testing.T) {
			cmd := &CmdTool{TargetDir: t.TempDir(), ProjectDir: t.TempDir()}
			file := filepath.Join(cmd.ProjectDir, "project.godot")
			want := "failed to read project file"
			if scenario == "read-only project file" {
				writeCommandTestFile(t, file, `config/name="spx"`)
				restrictCommandTestPath(t, file, 0o400)
				want = "failed to write project file"
			}
			if err := cmd.updateProjectName(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("project name error = %v, want %q", err, want)
			}
			if scenario == "read-only project file" && readTestFile(t, file) != `config/name="spx"` {
				t.Fatal("failed update changed project file")
			}
		})
	}
}

func TestReimportBuildFailurePreventsGodotLaunch(t *testing.T) {
	root := t.TempDir()
	log := filepath.Join(root, "import.log")
	t.Setenv("SPX_IMPORT_LOG", log)
	bin := installCommandTestTools(t, map[string]string{
		"xgo":   "exit 23",
		"godot": `printf imported > "$SPX_IMPORT_LOG"`,
	})
	arch := ""
	cmd := &CmdTool{TargetDir: root, ProjectDir: filepath.Join(root, "project"), CmdPath: filepath.Join(bin, "godot"), Args: ExtraArgs{CmdName: "exporttemplateweb", Arch: &arch}}
	if err := cmd.Reimport(); err == nil || !strings.Contains(err.Error(), "code generation failed") {
		t.Fatalf("reimport build error = %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("failed build still imported project: %v", err)
	}
}

func TestPrepareCommandRejectsProjectWithoutSource(t *testing.T) {
	for _, name := range []string{"build", "exportweb"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeCommandTestFile(t, filepath.Join(root, "README.md"), "not a source project")
			address := ""
			cmd := &CmdTool{FileSuffix: ".spx", Args: ExtraArgs{Path: &root, ServerAddr: &address}}
			spec, _ := findCommand(name)
			if err := cmd.prepareCommand(spec, "template/project", "project"); err == nil || !strings.Contains(err.Error(), "not a valid project directory") {
				t.Fatalf("project validation error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "project")); !os.IsNotExist(err) {
				t.Fatalf("invalid project was prepared: %v", err)
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

	writeCommandTestFile(t, filepath.Join(projectDir, ".godot", "uid_cache.bin"), "cache")
}

func writeLocalSpxRepoMarker(t *testing.T, root string) {
	t.Helper()
	writeCommandTestFile(t, filepath.Join(root, "go.mod"), "module github.com/goplus/spx/v4\n")
	script := filepath.Join(root, "cmd", "spx", "install.sh")
	writeCommandTestFile(t, script, "#!/bin/bash\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
}
