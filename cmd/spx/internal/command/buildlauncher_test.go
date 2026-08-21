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
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/launchpack"
)

func TestBuildLauncherDispatchesBeforeLegacySetup(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	projectDir := t.TempDir()
	otherModule := t.TempDir()
	repoRoot := findSPXRoot(t)
	writeLauncherTestFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/game\n\ngo 1.25.0\n\nrequire github.com/goplus/spx/v3 v3.0.0\n\nreplace github.com/goplus/spx/v3 => "+filepath.ToSlash(repoRoot)+"\n")
	writeLauncherTestFile(t, filepath.Join(otherModule, "go.mod"), "module example.com/other\n\ngo 1.25.0\n")
	goWork := filepath.Join(t.TempDir(), "go.work")
	writeLauncherTestFile(t, goWork, "go 1.25.0\n\nuse (\n\t"+filepath.ToSlash(projectDir)+"\n\t"+filepath.ToSlash(otherModule)+"\n)\n")
	t.Setenv("GOWORK", goWork)
	writeLauncherTestFile(t, filepath.Join(projectDir, "gox.mod"), "xgo 1.8.0\n\nproject main.spx Game github.com/goplus/spx/v3 math\n\nclass -embed *.spx SpriteImpl\n\npack assets index.json\n")
	writeLauncherTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n")
	writeLauncherTestFile(t, filepath.Join(projectDir, "Hero.spx"), "when green flag => {}\n")
	writeLauncherTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n")
	writeLauncherTestFile(t, filepath.Join(projectDir, "assets", "hero.png"), "asset")
	output := filepath.Join(projectDir, "bin", "game"+executableSuffix(runtime.GOOS))

	oldFlags, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"spx", "buildlauncher", "--path", projectDir, "-o", output, "-v"}
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

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
	oldFlags, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"spx", "buildlauncher", "extra"}
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

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

func TestBuildLauncherFailurePreservesExistingOutput(t *testing.T) {
	projectDir := t.TempDir()
	output := filepath.Join(projectDir, "game")
	writeLauncherTestFile(t, output, "old")
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

func writeLauncherTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func launcherStringPointer(value string) *string { return &value }

func launcherBoolPointer(value bool) *bool { return &value }
