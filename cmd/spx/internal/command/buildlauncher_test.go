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
	"context"
	"embed"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/launchpack"
)

func TestBuildLauncherDispatchesBeforeLegacySetup(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	projectDir := t.TempDir()
	otherModule := t.TempDir()
	repoRoot := findSPXRoot(t)
	writeCommandTestFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/game\n\ngo 1.25.0\n\nrequire github.com/goplus/spx/v4 v4.0.0\n\nreplace github.com/goplus/spx/v4 => "+filepath.ToSlash(repoRoot)+"\n")
	writeCommandTestFile(t, filepath.Join(otherModule, "go.mod"), "module example.com/other\n\ngo 1.25.0\n")
	goWork := filepath.Join(t.TempDir(), "go.work")
	writeCommandTestFile(t, goWork, "go 1.25.0\n\nuse (\n\t"+filepath.ToSlash(projectDir)+"\n\t"+filepath.ToSlash(otherModule)+"\n)\n")
	t.Setenv("GOWORK", goWork)
	writeCommandTestFile(t, filepath.Join(projectDir, "gox.mod"), "xgo 1.8.0\n\nproject main.spx Game github.com/goplus/spx/v4 math\n\nclass -embed *.spx SpriteImpl\n\npack assets index.json\n")
	writeCommandTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n")
	writeCommandTestFile(t, filepath.Join(projectDir, "Hero.spx"), "when green flag => {}\n")
	writeCommandTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n")
	writeCommandTestFile(t, filepath.Join(projectDir, "assets", "hero.png"), "asset")
	output := filepath.Join(projectDir, "bin", "game"+executableSuffix(runtime.GOOS))

	setTestCommandLine(t, "spx", "buildlauncher", "--path", projectDir, "-o", output, "-v")

	canonicalProjectDir, err := canonicalDirectory(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	cmd := &CmdTool{launcherBuilder: func(_ context.Context, cfg launchpack.Config) (launchpack.Result, error) {
		called = true
		if cfg.ProjectDir != canonicalProjectDir || cfg.ProjectFile != filepath.Join(canonicalProjectDir, "main.spx") || cfg.PackDir != "assets" || cfg.PackIndex != "index.json" {
			t.Fatalf("launcher config = %#v", cfg)
		}
		if len(cfg.BuildFlags) != 1 || cfg.BuildFlags[0] != "-v=true" {
			t.Fatalf("launcher build flags = %#v, want [-v=true]", cfg.BuildFlags)
		}
		if cfg.VerifyGraph == nil {
			t.Fatal("launcher graph verifier is nil")
		}
		if err := cfg.VerifyGraph(context.Background()); err != nil {
			t.Fatalf("verify launcher graph: %v", err)
		}
		if cfg.Output == output || !strings.Contains(filepath.Base(cfg.Output), "spx-launchpack") {
			t.Fatalf("launcher staging output = %q", cfg.Output)
		}
		if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
			return launchpack.Result{}, err
		}
		if err := os.WriteFile(cfg.Output, []byte("launcher"), 0o755); err != nil {
			return launchpack.Result{}, err
		}
		return launchpack.Result{Output: cfg.Output}, nil
	}}
	if err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", "project"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("injected launcher builder was not called")
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "launcher" {
		t.Fatalf("output = %q, err = %v", got, err)
	}
	if cmd.ProjectDir != "" || cmd.CmdPath != "" || cmd.LibPath != "" {
		t.Fatalf("legacy setup ran: project=%q cmd=%q lib=%q", cmd.ProjectDir, cmd.CmdPath, cmd.LibPath)
	}
}

func TestValidateBuildLauncherArgs(t *testing.T) {
	if err := validateBuildLauncherArgs(ExtraArgs{Build: launcherStringPointer("normal"), Mode: launcherStringPointer("none"), Target: launcherStringPointer("esp32")}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args ExtraArgs
	}{
		{name: "arch", args: ExtraArgs{Arch: launcherStringPointer("arm64")}},
		{name: "tags", args: ExtraArgs{Tags: launcherStringPointer("pure_engine")}},
		{name: "servermode", args: ExtraArgs{ServerMode: launcherBoolPointer(true)}},
		{name: "build", args: ExtraArgs{Build: launcherStringPointer("fast")}},
		{name: "mode", args: ExtraArgs{Mode: launcherStringPointer("worker")}},
		{name: "target", args: ExtraArgs{Target: launcherStringPointer("web")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateBuildLauncherArgs(test.args)
			if err == nil || !strings.Contains(err.Error(), "not supported") || !strings.Contains(err.Error(), "-"+test.name) {
				t.Fatalf("validateBuildLauncherArgs(%s) error = %v", test.name, err)
			}
		})
	}
}

func TestBuildLauncherRejectsPositionalArgs(t *testing.T) {
	setTestCommandLine(t, "spx", "buildlauncher", "extra")

	err := (&CmdTool{}).RunCmd("spx", ".spx", "test", embed.FS{}, "", "project")
	if err == nil || !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("positional argument error = %v", err)
	}
}

func TestBuildLauncherBuildFlags(t *testing.T) {
	if got := buildLauncherBuildFlags(ExtraArgs{Verbose: launcherBoolPointer(true)}); len(got) != 1 || got[0] != "-v=true" {
		t.Fatalf("verbose build flags = %#v", got)
	}
	if got := buildLauncherBuildFlags(ExtraArgs{}); got != nil {
		t.Fatalf("default build flags = %#v, want nil", got)
	}
}

func TestLoadLauncherProject(t *testing.T) {
	const project = "project main.spx Game github.com/goplus/spx/v4 math\n"
	const wildcard = "project *.spx Game github.com/goplus/spx/v4 math\n"
	const pack = "pack assets index.json\n"
	for _, test := range []struct {
		name, metadata, filename string
		sources                  []string
		wantFile, wantError      string
	}{
		{"explicit entry", project + pack, "gox.mod", []string{"main.spx", "Hero.spx", "notes.txt"}, "main.spx", ""},
		{"legacy metadata", project + pack, "gop.mod", []string{"main.spx"}, "main.spx", ""},
		{"wildcard prefers main", wildcard + pack, "gox.mod", []string{"main.spx", "Hero.spx"}, "main.spx", ""},
		{"single candidate", wildcard + pack, "gox.mod", []string{"Game.spx"}, "Game.spx", ""},
		{"ignores other project kinds", "project *.gmx Game example.com/other\n" + project + pack, "gox.mod", []string{"main.spx"}, "main.spx", ""},
		{"invalid metadata", "unknown directive\n", "gox.mod", nil, "", "parse project metadata"},
		{"no SPX project", "xgo 1.8.0\n", "gox.mod", nil, "", "no .spx project"},
		{"multiple projects", project + pack + wildcard + pack, "gox.mod", nil, "", "multiple .spx projects"},
		{"missing pack", project, "gox.mod", nil, "", "must declare 'pack assets index.json'"},
		{"missing entry", project + pack, "gox.mod", nil, "", "no project file matching"},
		{"class files are not entrypoints", project + "class -embed *.spx SpriteImpl\n" + pack, "gox.mod", []string{"Hero.spx"}, "", "no project file matching"},
		{"ambiguous entry", wildcard + pack, "gox.mod", []string{"First.spx", "Second.spx"}, "", "multiple project files"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeCommandTestFile(t, filepath.Join(root, test.filename), test.metadata)
			for _, name := range test.sources {
				writeCommandTestFile(t, filepath.Join(root, name), "onStart => {}\n")
			}
			got, err := loadLauncherProject(root, root)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("loadLauncherProject error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.dir != root || got.file != filepath.Join(root, test.wantFile) || got.metadataFile != filepath.Join(root, test.filename) || got.extension != ".spx" || got.packDir != "assets" || got.packIndex != "index.json" {
				t.Fatalf("launcher project = %+v", got)
			}
			var wantSources []string
			for _, name := range test.sources {
				if filepath.Ext(name) == ".spx" {
					wantSources = append(wantSources, filepath.Join(root, name))
				}
			}
			slices.Sort(wantSources)
			if !slices.Equal(got.sourceFiles, wantSources) {
				t.Fatalf("source files = %q, want %q", got.sourceFiles, wantSources)
			}
		})
	}
}

func TestLoadLauncherProjectRejectsInvalidFiles(t *testing.T) {
	const metadata = "project main.spx Game github.com/goplus/spx/v4 math\npack assets index.json\n"
	for _, kind := range []string{"missing metadata", "both metadata files", "metadata directory", "metadata symlink", "source directory", "source symlink", "missing source directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			projectDir := root
			metaPath := filepath.Join(root, "gox.mod")
			writeCommandTestFile(t, metaPath, metadata)
			writeCommandTestFile(t, filepath.Join(root, "main.spx"), "onStart => {}\n")
			want := "not a regular non-symlink file"
			switch kind {
			case "missing metadata":
				if err := os.Remove(metaPath); err != nil {
					t.Fatal(err)
				}
				want = "no gox.mod or gop.mod"
			case "both metadata files":
				writeCommandTestFile(t, filepath.Join(root, "gop.mod"), metadata)
				want = "has both"
			case "metadata directory", "metadata symlink":
				if err := os.Remove(metaPath); err != nil {
					t.Fatal(err)
				}
				if kind == "metadata directory" {
					if err := os.Mkdir(metaPath, 0o755); err != nil {
						t.Fatal(err)
					}
				} else {
					writeCommandTestFile(t, filepath.Join(root, "metadata"), metadata)
					if err := os.Symlink("metadata", metaPath); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				}
			case "source directory":
				if err := os.Mkdir(filepath.Join(root, "Hero.spx"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "source symlink":
				if err := os.Symlink("main.spx", filepath.Join(root, "Hero.spx")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "missing source directory":
				projectDir = filepath.Join(root, "missing")
				want = "read project directory"
			}
			if _, err := loadLauncherProject(root, projectDir); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("loadLauncherProject error = %v, want %q", err, want)
			}
		})
	}
}

func TestBuildLauncherFailureDoesNotPublishOutput(t *testing.T) {
	for _, scenario := range []string{"builder failure", "missing stage", "portable config", "protected output", "invalid project", "missing Go"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			writeCommandTestFile(t, filepath.Join(root, "go.mod"), "module example.com/game\n\ngo 1.25.0\n\nrequire github.com/goplus/spx/v4 v4.0.0\nreplace github.com/goplus/spx/v4 => "+filepath.ToSlash(findSPXRoot(t))+"\n")
			writeCommandTestFile(t, filepath.Join(root, "gox.mod"), "project main.spx Game github.com/goplus/spx/v4 math\npack assets index.json\n")
			writeCommandTestFile(t, filepath.Join(root, "main.spx"), "onStart => {}\n")
			output := filepath.Join(root, "launcher")
			writeCommandTestFile(t, output, "old launcher")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			setTestCommandLine(t, "spx", "buildlauncher")
			if err := flag.CommandLine.Parse(nil); err != nil {
				t.Fatal(err)
			}
			cmd := &CmdTool{Args: ExtraArgs{Path: &root, Output: &output}}
			called := false
			cmd.launcherBuilder = func(context.Context, launchpack.Config) (launchpack.Result, error) {
				called = true
				if scenario == "builder failure" {
					return launchpack.Result{}, errors.New("compiler failed")
				}
				return launchpack.Result{}, nil
			}
			want := ""
			switch scenario {
			case "builder failure":
				want = "compiler failed"
			case "missing stage":
				want = "inspect staged output"
			case "portable config":
				writeCommandTestFile(t, filepath.Join(root, ".config"), "{")
				want = "portable config"
			case "protected output":
				output = filepath.Join(root, "main.spx")
				want = "source extension"
			case "invalid project":
				root = filepath.Join(root, "missing")
				want = "project path"
			case "missing Go":
				t.Setenv("PATH", t.TempDir())
				want = "Go command"
			}
			before := readTestFile(t, output)
			if err := cmd.runBuildLauncher(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("buildlauncher error = %v, want %q", err, want)
			}
			if called != (scenario == "builder failure" || scenario == "missing stage") {
				t.Fatalf("builder called = %v for %s", called, scenario)
			}
			if got := readTestFile(t, output); got != before {
				t.Fatalf("failure replaced output: %q, want %q", got, before)
			}
			entries, err := filepath.Glob(filepath.Join(filepath.Dir(output), ".*.spx-launchpack-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("failed build retained staging paths: %q", entries)
			}
		})
	}
}

func findSPXRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "cmd", "ispxnative", "main.go")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("SPX source root not found")
		}
		dir = parent
	}
}

func launcherStringPointer(value string) *string { return &value }

func launcherBoolPointer(value bool) *bool { return &value }
