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

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/workflow"
	"github.com/goplus/spx/v3/internal/release"
)

func TestRunDispatchesUsageAndUnknownCommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: shared.ErrUsage},
		{name: "help", args: []string{"help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown command "unknown"`},
		{name: "runtime help", args: []string{"runtime", "help"}},
		{name: "runtime unknown", args: []string{"runtime", "unknown"}, contains: `unknown runtime command "unknown"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("run(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains == "" {
				if err != nil {
					t.Fatalf("run(%q) returned error: %v", test.args, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("run(%q) error = %v, want substring %q", test.args, err, test.contains)
			}
		})
	}
}

func TestParseSetupCommandArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want setupConfig
	}{
		{name: "host", args: []string{"host"}, want: setupConfig{target: "host"}},
		{name: "web default", args: []string{"web"}, want: setupConfig{target: "web", mode: "normal"}},
		{name: "full mode", args: []string{"full", "--mode", "worker"}, want: setupConfig{target: "full", mode: "worker"}},
		{name: "local assets", args: []string{"web", "--asset-dir", "artifacts/runtime"}, want: setupConfig{target: "web", mode: "normal", assetDir: "artifacts/runtime"}},
		{name: "published host", args: []string{"host", "--published-runtime"}, want: setupConfig{target: "host", publishedRuntime: true}},
		{name: "published full", args: []string{"full", "--published-runtime"}, want: setupConfig{target: "full", mode: "normal", publishedRuntime: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSetupCommandArgs(test.args)
			if err != nil {
				t.Fatalf("parseSetupCommandArgs returned error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("config = %#v, want %#v", got, test.want)
			}
		})
	}

	invalid := [][]string{
		{},
		{"unknown"},
		{"host", "--mode", "worker"},
		{"web", "--mode", "unknown"},
		{"web", "--published-runtime"},
		{"host", "--asset-dir", "artifacts", "--published-runtime"},
		{"full", "extra"},
	}
	for _, args := range invalid {
		if _, err := parseSetupCommandArgs(args); err == nil {
			t.Fatalf("parseSetupCommandArgs(%q) succeeded, want error", args)
		}
	}
}

func TestParseBuildCommandArgs(t *testing.T) {
	tests := []struct {
		args []string
		want workflow.BuildConfig
	}{
		{args: []string{"dev"}, want: workflow.BuildConfig{Target: "dev", Mode: "normal"}},
		{args: []string{"dev", "--mode", "miniprogram"}, want: workflow.BuildConfig{Target: "dev", Mode: "miniprogram"}},
		{args: []string{"web"}, want: workflow.BuildConfig{Target: "web", Mode: "normal"}},
		{args: []string{"web", "--mode", "worker"}, want: workflow.BuildConfig{Target: "web", Mode: "worker"}},
		{args: []string{"editor"}, want: workflow.BuildConfig{Target: "editor"}},
		{args: []string{"desktop"}, want: workflow.BuildConfig{Target: "desktop"}},
		{args: []string{"android"}, want: workflow.BuildConfig{Target: "android"}},
		{args: []string{"ios"}, want: workflow.BuildConfig{Target: "ios"}},
	}
	for _, test := range tests {
		got, err := parseBuildCommandArgs(test.args)
		if err != nil {
			t.Fatalf("parseBuildCommandArgs(%q) returned error: %v", test.args, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("parseBuildCommandArgs(%q) = %#v, want %#v", test.args, got, test.want)
		}
	}

	invalid := [][]string{
		{},
		{"unknown"},
		{"editor", "--mode", "worker"},
		{"desktop", "--mode", "normal"},
		{"android", "--mode", "normal"},
		{"ios", "--mode", "normal"},
		{"web", "--mode", "unknown"},
		{"web", "extra"},
	}
	for _, args := range invalid {
		if _, err := parseBuildCommandArgs(args); err == nil {
			t.Fatalf("parseBuildCommandArgs(%q) succeeded, want error", args)
		}
	}
}

func TestInspectBuildConfiguration(t *testing.T) {
	repoRoot := t.TempDir()
	lockData, err := release.DefaultRuntimeLock().JSON()
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"), lockData)
	moduleSource := filepath.Join(repoRoot, "godot_modules", "spx")
	mustWriteFile(t, filepath.Join(moduleSource, "SCsub"), []byte("# test\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "config.py"), []byte("# test\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "spx_scons_profile.json"), []byte(`{
  "schema": 1,
  "common": [],
  "editor_release": [],
  "template_release": []
}`))

	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("PLATFORM", "")
	t.Setenv("GODOT_SRC", filepath.Join(repoRoot, "missing-godot"))
	t.Setenv("SPX_MODULE_SRC", moduleSource)

	var output bytes.Buffer
	if err := inspectBuildConfiguration(repoRoot, &output); err != nil {
		t.Fatalf("inspectBuildConfiguration returned error: %v", err)
	}
	for _, text := range []string{
		"Repository: " + repoRoot,
		"Runtime lock: " + filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"),
		"Godot source: " + filepath.Join(repoRoot, "missing-godot") + " (missing;",
		"SCons profile: " + filepath.Join(moduleSource, "spx_scons_profile.json") + " (valid)",
		"Toolchain:",
		"scons=",
		"Status: OK",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("doctor output missing %q:\n%s", text, output.String())
		}
	}
}

func TestInspectBuildConfigurationRejectsStaleEmbeddedLock(t *testing.T) {
	repoRoot := t.TempDir()
	lock := release.DefaultRuntimeLock()
	lock.RuntimeABI++
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"), lockData)

	var output bytes.Buffer
	err = inspectBuildConfiguration(repoRoot, &output)
	if err == nil || !strings.Contains(err.Error(), "rebuild buildctl") {
		t.Fatalf("inspectBuildConfiguration error = %v, want stale buildctl error", err)
	}
}

func TestRootCommandEntryPoints(t *testing.T) {
	for _, command := range []string{"setup", "build", "doctor", "env", "tool", "engine", "workflow"} {
		t.Run(command+"/help", func(t *testing.T) {
			if err := run([]string{command, "--help"}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, args := range [][]string{{"setup"}, {"setup", "host", "--invalid"}, {"build"}, {"build", "dev", "--invalid"}, {"doctor", "extra"}} {
		if err := run(args); err == nil {
			t.Fatalf("run(%q) accepted invalid arguments", args)
		}
	}
	for _, args := range [][]string{{"setup", "host"}, {"build", "dev"}, {"doctor"}} {
		t.Run(strings.Join(args, "/")+"/missing-repository", func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := run(args); err == nil || !strings.Contains(err.Error(), "repository root") {
				t.Fatalf("run(%q) = %v", args, err)
			}
		})
	}
	t.Run("build dispatch", func(t *testing.T) {
		root := newBuildctlFixture(t)
		t.Setenv("SPX_MODULE_SRC", filepath.Join(root, "missing-module"))
		if err := run([]string{"build", "dev"}); err == nil || !strings.Contains(err.Error(), "SPX module source") {
			t.Fatalf("build error = %v", err)
		}
	})
	t.Run("main help", func(t *testing.T) {
		oldArgs := os.Args
		os.Args = []string{"buildctl", "--help"}
		defer func() { os.Args = oldArgs }()
		main()
	})
}

func TestDoctorEntryPointFailuresAndPinnedCheckout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX git fixture")
	}
	for _, scenario := range []string{"pinned", "missing lock", "bad lock", "bad environment", "missing module", "bad git", "wrong commit"} {
		t.Run(scenario, func(t *testing.T) {
			root := newBuildctlFixture(t)
			lock := release.DefaultRuntimeLock()
			lockData, err := lock.JSON()
			if err != nil {
				t.Fatal(err)
			}
			lockPath := filepath.Join(root, "internal", "release", "runtime.lock.json")
			mustWriteFile(t, lockPath, lockData)
			mustMkdirAll(t, os.Getenv("GODOT_SRC"))
			bin := filepath.Join(root, "tools")
			writeBuildctlTool(t, filepath.Join(bin, "git"), "printf '%s\\n' '"+lock.Godot.Commit+"'\n")
			t.Setenv("PATH", bin)
			want := ""
			switch scenario {
			case "missing lock":
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
				want = "read runtime lock"
			case "bad lock":
				mustWriteFile(t, lockPath, []byte("invalid"))
				want = "invalid character"
			case "bad environment":
				t.Setenv("PLATFORM", "unsupported")
				want = "unsupported platform"
			case "missing module":
				t.Setenv("SPX_MODULE_SRC", filepath.Join(root, "absent"))
				want = "SPX module source"
			case "bad git":
				writeBuildctlTool(t, filepath.Join(bin, "git"), "exit 23\n")
				want = "inspect Godot source"
			case "wrong commit":
				writeBuildctlTool(t, filepath.Join(bin, "git"), "printf wrong\n")
				want = "runtime.lock.json requires"
			}
			output, err := captureBuildctlOutput(t, func() error { return run([]string{"doctor"}) })
			if want != "" {
				if err == nil || !strings.Contains(err.Error(), want) || output != "" {
					t.Fatalf("doctor = %q, %v; want %q and no success output", output, err, want)
				}
			} else if err != nil || !strings.Contains(output, "pinned commit ready") || !strings.Contains(output, "Status: OK") {
				t.Fatalf("doctor = %q, %v", output, err)
			}
		})
	}
}

type setupTestRunner struct {
	t         *testing.T
	root      string
	scripts   [][]string
	commands  []string
	scriptErr error
}

func (r *setupTestRunner) RepoRootDir() string { return r.root }
func (r *setupTestRunner) RunScript(script string, args ...string) error {
	r.scripts = append(r.scripts, append([]string{script}, args...))
	return r.scriptErr
}
func (r *setupTestRunner) RunCommand(dir, name string, args ...string) error {
	if dir != r.root || name != "go" || len(args) != 5 || args[0] != "run" || args[1] != "./cmd/spx" || args[3] != "--path" {
		r.t.Fatalf("unexpected export command: %s %s %q", dir, name, args)
	}
	r.commands = append(r.commands, args[2])
	dst := filepath.Join(args[4], "project", ".builds")
	switch args[2] {
	case "exportpack":
		mustWriteFile(r.t, filepath.Join(dst, "pc", "gdexport.pck"), []byte("exported pack"))
	case "exporttemplateweb":
		mustWriteFile(r.t, filepath.Join(dst, "webi", "engine.pck"), []byte("web pack"))
		mustWriteFile(r.t, filepath.Join(dst, "webi", "engine.js"), []byte("engine code"))
	default:
		r.t.Fatalf("unexpected export operation: %q", args[2])
	}
	return nil
}

func TestSetupLocalArtifacts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux release fixtures")
	}
	for _, target := range []string{"host", "web", "full"} {
		for _, failed := range []bool{false, true} {
			t.Run(target+map[bool]string{false: "/success", true: "/installer failure"}[failed], func(t *testing.T) {
				root := newBuildctlFixture(t)
				env, err := shared.ResolveBuildEnvironment(root, "linux")
				if err != nil {
					t.Fatal(err)
				}
				assets := filepath.Join(root, "assets")
				files := map[string]map[string]string{
					"linux-" + env.Arch + ".zip":        {"godot.linuxbsd.template_release." + env.Arch: "runtime", "godot.linuxbsd.template_debug." + env.Arch: "debug"},
					"editor-linux-" + env.Arch + ".zip": {"godot.linuxbsd.editor." + env.Arch: "editor"},
					release.RuntimeAssetZipName:         {"gdspxrt.pck": "release pack", "runtime.gdextension": "extension"},
				}
				for archive, contents := range files {
					named := map[string]string{}
					for name, content := range contents {
						path := filepath.Join(root, "zip-source", name)
						mustWriteFile(t, path, []byte(content))
						named[name] = path
					}
					if err := shared.WriteNamedZip(filepath.Join(assets, archive), named); err != nil {
						t.Fatal(err)
					}
				}
				mustWriteFile(t, filepath.Join(assets, "web.zip"), []byte("web template"))
				mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("extension"))
				runner := &setupTestRunner{t: t, root: root}
				failure := errors.New("fixture install failure")
				if failed {
					runner.scriptErr = failure
				}
				err = setupAssets(setupConfig{target: target, mode: "normal", assetDir: assets}, runner)
				if failed {
					if !errors.Is(err, failure) || len(runner.commands) != 0 {
						t.Fatalf("setup failure = %v, commands = %v", err, runner.commands)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				wantScript := []string{filepath.Join("cmd", "spx", "install.sh")}
				if target != "host" {
					wantScript = append(wantScript, "--web")
					if target == "web" {
						wantScript = append(wantScript, "--no-embed-runtime")
					}
				}
				if !reflect.DeepEqual(runner.scripts, [][]string{wantScript}) {
					t.Fatalf("scripts = %v, want %v", runner.scripts, wantScript)
				}
				if !failed && target != "host" {
					path := filepath.Join(env.GoPath, "bin", "gdspxrt"+env.Version+"_webnormal", "engine.js")
					if data, err := os.ReadFile(path); err != nil || string(data) != "var EnginePackMode = 'normal';\nengine code" {
						t.Fatalf("web output = %q, %v", data, err)
					}
				}
			})
		}
	}
}

func TestSetupAndRuntimePackFailures(t *testing.T) {
	root := newBuildctlFixture(t)
	runner := &setupTestRunner{t: t, root: root}
	for _, target := range []string{"host", "web", "full", "unsupported"} {
		if err := setupAssets(setupConfig{target: target, assetDir: filepath.Join(root, "missing")}, runner); err == nil {
			t.Fatalf("%s accepted unavailable assets", target)
		}
	}
	if err := prepareWebAssets("normal", runner, true, filepath.Join(root, "missing")); err == nil {
		t.Fatal("web preparation accepted unavailable assets")
	}
	if len(runner.scripts) != 0 || len(runner.commands) != 0 {
		t.Fatal("failed setup invoked tools")
	}
	if err := run([]string{"setup", "host", "--asset-dir", filepath.Join(root, "missing")}); err == nil {
		t.Fatal("setup command accepted missing assets")
	}
	pack := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+release.DefaultRuntimeLock().RuntimeVersion+".pck")
	mustWriteFile(t, pack, []byte("cached"))
	if err := ensureRuntimePack(runner); err != nil || len(runner.commands) != 0 {
		t.Fatalf("cached pack = %v, commands = %v", err, runner.commands)
	}
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("extension"))
	t.Setenv("SPX_PREPARE_FORCE_REFRESH", "true")
	if err := ensureRuntimePack(runner); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(pack); err != nil || string(data) != "exported pack" {
		t.Fatalf("refreshed pack = %q, %v", data, err)
	}
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", t.TempDir())
	if err := ensureRuntimePack(runner); err == nil {
		t.Fatal("missing GOPATH resolver accepted")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newBuildctlFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for key, value := range map[string]string{
		"HOME": root, "USERPROFILE": root, "APPDATA": filepath.Join(root, "AppData"),
		"GOPATH": filepath.Join(root, "go"), "GODOT_SRC": filepath.Join(root, "godot"),
		"SPX_MODULE_SRC": filepath.Join(root, "module"), "PLATFORM": "", "SPX_PREPARE_FORCE_REFRESH": "false",
	} {
		t.Setenv(key, value)
	}
	mustWriteFile(t, filepath.Join(root, "go.mod"), nil)
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "install.sh"), nil)
	mustMkdirAll(t, filepath.Join(root, "internal", "tools"))
	mustWriteFile(t, filepath.Join(root, "module", "SCsub"), nil)
	mustWriteFile(t, filepath.Join(root, "module", "config.py"), nil)
	mustWriteFile(t, filepath.Join(root, "module", "spx_scons_profile.json"), []byte(`{"schema":1,"common":[],"editor_release":[],"template_release":[]}`))
	t.Chdir(root)
	return root
}

func writeBuildctlTool(t *testing.T, path, body string) {
	t.Helper()
	mustWriteFile(t, path, []byte("#!/bin/sh\n"+body))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func captureBuildctlOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	previous := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = previous }()
	callErr := fn()
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), callErr
}
