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

package runtimecmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

type recordedCall struct {
	script string
	args   []string
}

type recordedCommand struct {
	dir  string
	name string
	args []string
}

type recordingRunner struct {
	calls       []recordedCall
	commands    []recordedCommand
	repoRoot    string
	commandHook func(workdir string, name string, args ...string) error
	scriptErr   error
}

func TestRuntimeDispatchAndArgumentErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: errUsage},
		{name: "help", args: []string{"help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown runtime command "unknown"`},
		{name: "export-pack help", args: []string{"export-pack", "--help"}},
		{name: "build-wasm help", args: []string{"build-wasm", "--help"}},
		{name: "compress-wasm help", args: []string{"compress-wasm", "--help"}},
		{name: "export-web help", args: []string{"export-web", "--help"}},
		{name: "export-web-template help", args: []string{"export-web-template", "--help"}},
		{name: "build-wasm unknown flag", args: []string{"build-wasm", "--not-a-runtime-option"}, contains: "flag provided but not defined"},
		{name: "compress-wasm unknown flag", args: []string{"compress-wasm", "--not-a-runtime-option"}, contains: "flag provided but not defined"},
		{name: "export-pack unknown flag", args: []string{"export-pack", "--not-a-runtime-option"}, contains: "flag provided but not defined"},
		{name: "export-web unknown flag", args: []string{"export-web", "--not-a-runtime-option"}, contains: "flag provided but not defined"},
		{name: "export-web-template unknown flag", args: []string{"export-web-template", "--not-a-runtime-option"}, contains: "flag provided but not defined"},
		{name: "build-wasm positional", args: []string{"build-wasm", "extra"}, wantErr: errUsage},
		{name: "export-pack positional", args: []string{"export-pack", "extra"}, wantErr: errUsage},
		{name: "export-web invalid mode", args: []string{"export-web", "--mode", "invalid"}, contains: "unsupported web-mode"},
		{name: "compress-wasm positional", args: []string{"compress-wasm", "extra"}, wantErr: errUsage},
		{name: "export-web positional", args: []string{"export-web", "extra"}, wantErr: errUsage},
		{name: "export-web-template invalid mode", args: []string{"export-web-template", "--mode", "invalid"}, contains: "unsupported web-mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Run(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Run(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains == "" {
				if err != nil {
					t.Fatalf("Run(%q) returned error: %v", test.args, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Run(%q) error = %v, want substring %q", test.args, err, test.contains)
			}
		})
	}

}

func TestParseRuntimeBuildWasmArgsDefault(t *testing.T) {
	cfg, err := parseRuntimeBuildWasmArgs(nil)
	if err != nil {
		t.Fatalf("parseRuntimeBuildWasmArgs returned error: %v", err)
	}
	if cfg.Opt {
		t.Fatal("opt should default to false")
	}
}

func TestParseRuntimeExportPackArgs(t *testing.T) {
	cfg, err := parseRuntimeExportPackArgs(nil)
	if err != nil {
		t.Fatalf("parseRuntimeExportPackArgs returned error: %v", err)
	}
	if cfg.engineAssetDir != "" {
		t.Fatalf("default engine asset directory = %q", cfg.engineAssetDir)
	}

	cfg, err = parseRuntimeExportPackArgs([]string{"--engine-asset-dir", "runtime-assets"})
	if err != nil {
		t.Fatalf("parseRuntimeExportPackArgs returned error: %v", err)
	}
	if cfg.engineAssetDir != "runtime-assets" {
		t.Fatalf("engine asset directory = %q", cfg.engineAssetDir)
	}
}

func TestRuntimeBuildWasmSequence(t *testing.T) {
	runner := &recordingRunner{}

	if err := BuildWasmRuntime(BuildWasmConfig{}, runner); err != nil {
		t.Fatalf("buildWasmRuntime returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestRuntimeBuildWasmOptSequence(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeBrotli(t, runner.repoRoot)
	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	version := release.DefaultRuntimeLock().RuntimeVersion
	mustWriteFile(t, filepath.Join(gopathBin, "ispx.wasm"), []byte("wasm"))
	mustWriteFile(t, filepath.Join(gopathBin, "gdspxrt"+version+"_webnormal", "engine.wasm"), []byte("engine"))

	if err := BuildWasmRuntime(BuildWasmConfig{Opt: true}, runner); err != nil {
		t.Fatalf("buildWasmRuntime returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
	if !shared.FileExists(filepath.Join(gopathBin, "ispx.wasm.br")) {
		t.Fatalf("expected compressed ispx.wasm.br to exist")
	}
	if !shared.FileExists(filepath.Join(gopathBin, "gdspxrt"+version+"_webnormal", "engine.wasm.br")) {
		t.Fatalf("expected compressed engine.wasm.br to exist")
	}
}

func TestParseRuntimeExportWebArgsDefault(t *testing.T) {
	cfg, err := parseRuntimeExportWebArgs(nil)
	if err != nil {
		t.Fatalf("parseRuntimeExportWebArgs returned error: %v", err)
	}
	if cfg.mode != "normal" {
		t.Fatalf("expected normal mode, got %s", cfg.mode)
	}
}

func TestRuntimeExportWebSequence(t *testing.T) {
	for _, tt := range []struct{ mode, command, output string }{
		{"normal", "exportweb", "spx_web.zip"},
		{"worker", "exportwebworker", "spx_web_worker.zip"},
		{"minigame", "exportminigame", "spx_web_minigame.zip"},
		{"miniprogram", "exportminiprogram", "spx_web_miniprogram.zip"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			if err := exportWebRuntime(runtimeExportWebConfig{mode: tt.mode}, runner); err != nil {
				t.Fatal(err)
			}
			expectedCalls := []recordedCall{
				{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}},
			}
			if !reflect.DeepEqual(runner.calls, expectedCalls) {
				t.Fatalf("unexpected calls: %#v", runner.calls)
			}
			assertSingleRuntimeWorkspaceCommand(t, runner, []string{tt.command})
			if !shared.FileExists(filepath.Join(runner.repoRoot, tt.output)) {
				t.Fatalf("expected export zip %s to exist", tt.output)
			}
		})
	}
}

func TestRuntimeExportWebInvalidModeInstallsToolsBeforeRejecting(t *testing.T) {
	for _, mode := range []string{"", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			runner := &recordingRunner{}
			err := exportWebRuntime(runtimeExportWebConfig{mode: mode}, runner)
			if err == nil || err.Error() != "unsupported web-mode: "+mode {
				t.Fatalf("error = %v", err)
			}
			want := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}}}
			if !reflect.DeepEqual(runner.calls, want) || len(runner.commands) != 0 {
				t.Fatalf("calls = %#v; commands = %#v", runner.calls, runner.commands)
			}
		})
	}
}

func TestRuntimeExportPackSequence(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	version := release.DefaultRuntimeLock().RuntimeVersion
	spec, err := release.HostRuntimeSpecFor(release.DefaultRuntimeLock(), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	mustWriteFile(t, filepath.Join(gopathBin, spec.RuntimeName), []byte("runtime-engine"))

	if err := ExportPackRuntime(runner); err != nil {
		t.Fatalf("exportPackRuntime returned error: %v", err)
	}

	assertSingleRuntimeWorkspaceCommand(t, runner, []string{"exportpack"})

	if !shared.FileExists(filepath.Join(gopathBin, "gdspxrt"+version+".pck")) {
		t.Fatalf("expected exported pck to exist")
	}
	if !shared.FileExists(filepath.Join(gopathBin, release.RuntimeAssetZipName)) {
		t.Fatalf("expected exported zip to exist")
	}
	manifestPath := filepath.Join(runner.repoRoot, ".spx", "runtime", version, spec.GOOS+"-"+spec.GOARCH, "engine-manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read local runtime manifest: %v", err)
	}
	manifest, err := release.ParseLocalRuntimeManifest(manifestData)
	if err != nil {
		t.Fatalf("parse local runtime manifest: %v", err)
	}
	if err := manifest.ValidateForLock(release.DefaultRuntimeLock(), spec.GOOS, spec.GOARCH); err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Dir(manifestPath)
	if err := manifest.VerifyFiles(manifestDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{manifest.Engine.Name, manifest.Pack.Name} {
		if !shared.FileExists(filepath.Join(manifestDir, name)) {
			t.Fatalf("expected published local runtime file %s beside manifest", name)
		}
	}
}

func TestCompressWasmArtifactsLegacyWebDirFallback(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)
	installFakeBrotli(t, root)
	version := release.DefaultRuntimeLock().RuntimeVersion

	gopathBin := filepath.Join(gopath, "bin")
	mustWriteFile(t, filepath.Join(gopathBin, "ispx.wasm"), []byte("wasm"))
	mustWriteFile(t, filepath.Join(gopathBin, "gdspxrt"+version+"_web", "engine.wasm"), []byte("engine"))

	if err := compressWasmArtifacts(); err != nil {
		t.Fatalf("compressWasmArtifacts returned error: %v", err)
	}
	if !shared.FileExists(filepath.Join(gopathBin, "ispx.wasm.br")) {
		t.Fatalf("expected compressed ispx.wasm.br to exist")
	}
	if !shared.FileExists(filepath.Join(gopathBin, "gdspxrt"+version+"_web", "engine.wasm.br")) {
		t.Fatalf("expected compressed engine.wasm.br to exist in legacy dir")
	}
}

func TestRuntimeInstallFailureStopsBeforeBuildAndExport(t *testing.T) {
	wantErr := errors.New("install failed")
	for _, tc := range []struct {
		name string
		run  func(*recordingRunner) error
	}{
		{"build", func(r *recordingRunner) error { return BuildWasmRuntime(BuildWasmConfig{}, r) }},
		{"optimized build", func(r *recordingRunner) error { return BuildWasmRuntime(BuildWasmConfig{Opt: true}, r) }},
		{"web export", func(r *recordingRunner) error { return exportWebRuntime(runtimeExportWebConfig{mode: "normal"}, r) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingRunner{repoRoot: t.TempDir(), scriptErr: wantErr}
			if err := tc.run(runner); !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want original install error", err)
			}
			want := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}}}
			if !reflect.DeepEqual(runner.calls, want) || len(runner.commands) != 0 {
				t.Fatalf("calls = %#v, commands = %#v; want one installation and no export", runner.calls, runner.commands)
			}
			entries, err := os.ReadDir(runner.repoRoot)
			if err != nil || len(entries) != 0 {
				t.Fatalf("installation failure created workspace files: %v, %v", entries, err)
			}
		})
	}
}

func TestRuntimeExportCommandFailureCleansWorkspace(t *testing.T) {
	wantErr := errors.New("export command failed")
	version := release.DefaultRuntimeLock().RuntimeVersion
	for _, tc := range []struct {
		name, command, output string
		run                   func(*recordingRunner) error
		wantScripts           int
	}{
		{"pack", "exportpack", filepath.Join("gopath", "bin", "gdspxrt"+version+".pck"), func(r *recordingRunner) error { return ExportPackRuntime(r) }, 0},
		{"web", "exportweb", "spx_web.zip", func(r *recordingRunner) error { return exportWebRuntime(runtimeExportWebConfig{mode: "normal"}, r) }, 1},
		{"web template", "exporttemplateweb", filepath.Join("gopath", "bin", "gdspxrt"+version+"_webnormal", "engine.zip"), func(r *recordingRunner) error { return ExportWebTemplateRuntime("normal", r) }, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			runner.commandHook = func(string, string, ...string) error { return wantErr }
			output := filepath.Join(runner.repoRoot, tc.output)
			mustWriteFile(t, output, []byte("previous export"))
			if err := tc.run(runner); !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want original export error", err)
			}
			assertSingleRuntimeWorkspaceCommand(t, runner, []string{tc.command})
			if len(runner.calls) != tc.wantScripts {
				t.Fatalf("script calls = %d, want %d", len(runner.calls), tc.wantScripts)
			}
			assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
			if got, err := os.ReadFile(output); err != nil || string(got) != "previous export" {
				t.Fatalf("previous export changed: %q, %v", got, err)
			}
		})
	}
}

func TestRuntimeWebTemplateRejectsIncompleteOutputs(t *testing.T) {
	for _, tc := range []struct {
		name, remove, message string
	}{
		{"missing pack", "engine.pck", "web runtime engine pack not found"},
		{"missing script", "engine.js", "engine.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			runner.commandHook = func(dir, name string, args ...string) error {
				if err := simulateRuntimeCommandOutputs(dir, name, args...); err != nil {
					return err
				}
				workspace, _, ok := simulatedSPXInvocation(dir, name, args...)
				if !ok {
					return errors.New("unrecognized SPX invocation")
				}
				return os.Remove(filepath.Join(workspace, "project", ".builds", "webi", tc.remove))
			}
			err := ExportWebTemplateRuntime("normal", runner)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, want missing-output error containing %q", err, tc.message)
			}
			assertSingleRuntimeWorkspaceCommand(t, runner, []string{"exporttemplateweb"})
			assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
		})
	}
}

func TestRuntimeCommandsOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, command := range []string{"build-wasm", "export-pack", "export-web", "export-web-template"} {
		t.Run(command, func(t *testing.T) {
			if err := Run([]string{command}); err == nil || !strings.Contains(err.Error(), "could not locate repository root") {
				t.Fatalf("Run(%q) = %v, want repository lookup failure", command, err)
			}
		})
	}
}

func TestRuntimeWorkspacePreparationFailures(t *testing.T) {
	for _, command := range []string{"pack", "web", "template"} {
		for _, failure := range []string{"GOPATH lookup", "blocked temp directory", "blocked bin directory", "missing extension"} {
			if command == "web" && failure == "missing extension" {
				continue
			}
			t.Run(command+"/"+failure, func(t *testing.T) {
				runner := newRuntimeFixtureRunner(t)
				switch failure {
				case "GOPATH lookup":
					t.Setenv("GOPATH", "")
					t.Setenv("PATH", t.TempDir())
				case "blocked temp directory":
					mustWriteFile(t, filepath.Join(runner.repoRoot, ".tmp"), []byte("keep"))
				case "blocked bin directory":
					mustWriteFile(t, filepath.Join(os.Getenv("GOPATH"), "bin"), []byte("keep"))
				case "missing extension":
					if err := os.Remove(filepath.Join(runner.repoRoot, "cmd", "spx", "template", "project", "runtime.gdextension.txt")); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				switch command {
				case "pack":
					err = ExportPackRuntime(runner)
				case "web":
					err = exportWebRuntime(runtimeExportWebConfig{mode: "normal"}, runner)
				case "template":
					err = ExportWebTemplateRuntime("normal", runner)
				}
				if err == nil || len(runner.commands) != 0 {
					t.Fatalf("error = %v, commands = %#v; want preparation failure before export", err, runner.commands)
				}
				if failure == "blocked bin directory" || failure == "missing extension" {
					assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
				}
			})
		}
	}
}

func TestCompressWasmArtifactsFailures(t *testing.T) {
	for _, failure := range []string{"GOPATH lookup", "missing engine", "missing interpreter", "brotli failure"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPATH", filepath.Join(root, "gopath"))
			bin := filepath.Join(os.Getenv("GOPATH"), "bin")
			engine := filepath.Join(bin, "gdspxrt"+release.DefaultRuntimeLock().RuntimeVersion+"_webnormal", "engine.wasm")
			want := ""
			switch failure {
			case "GOPATH lookup":
				t.Setenv("GOPATH", "")
				t.Setenv("PATH", t.TempDir())
				want = "go"
			case "missing engine":
				want = "engine.wasm not found"
			case "missing interpreter", "brotli failure":
				installFakeBrotli(t, root)
				mustWriteFile(t, engine, []byte("engine"))
				want = "ispx.wasm"
				if failure == "brotli failure" {
					mustWriteFile(t, filepath.Join(root, "fake-bin", "brotli"), []byte("#!/bin/sh\nexit 23\n"))
					mustWriteFile(t, filepath.Join(bin, "ispx.wasm"), []byte("interpreter"))
					want = "exit status 23"
				}
			}
			err := Run([]string{"compress-wasm"})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want %q", err, want)
			}
			if shared.FileExists(filepath.Join(bin, "ispx.wasm.br")) {
				t.Fatal("interpreter compressed despite earlier failure")
			}
			if failure == "missing interpreter" {
				if got, err := os.ReadFile(engine + ".br"); err != nil || string(got) != "engine" {
					t.Fatalf("engine compression = %q, %v", got, err)
				}
			}
		})
	}
}

func TestCompressWithBrotliRejectsInvalidPaths(t *testing.T) {
	for _, tc := range []struct{ input, output, message string }{
		{"", "out", "requires input and output"},
		{"in", "", "requires input and output"},
		{filepath.Join(t.TempDir(), "missing"), "out", "does not exist"},
	} {
		if err := compressWithBrotli(tc.input, tc.output); err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("compressWithBrotli(%q, %q) = %v, want %q", tc.input, tc.output, err, tc.message)
		}
	}
}

func TestRuntimeExportRejectsInvalidOutputs(t *testing.T) {
	for _, failure := range []string{"missing pack", "blocked pack destination", "directory extension", "missing extension", "missing template directory", "blocked template rename"} {
		t.Run(failure, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			bin := filepath.Join(os.Getenv("GOPATH"), "bin")
			runner.commandHook = func(dir, name string, args ...string) error {
				if err := simulateRuntimeCommandOutputs(dir, name, args...); err != nil {
					return err
				}
				workspace, _, ok := simulatedSPXInvocation(dir, name, args...)
				if !ok {
					return errors.New("unrecognized SPX command")
				}
				switch failure {
				case "missing pack":
					return os.Remove(filepath.Join(workspace, "project", ".builds", "pc", "gdexport.pck"))
				case "blocked pack destination":
					return os.Mkdir(filepath.Join(bin, "gdspxrt"+release.DefaultRuntimeLock().RuntimeVersion+".pck"), 0o755)
				case "directory extension":
					path := filepath.Join(bin, "runtime.gdextension")
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.Mkdir(path, 0o755)
				case "missing extension":
					return os.Remove(filepath.Join(bin, "runtime.gdextension"))
				case "missing template directory":
					return os.RemoveAll(filepath.Join(workspace, "project", ".builds", "webi"))
				case "blocked template rename":
					return os.Mkdir(filepath.Join(workspace, "project", ".builds", "webi", "engine.zip"), 0o755)
				}
				return nil
			}
			var err error
			if strings.Contains(failure, "template") {
				err = ExportWebTemplateRuntime("normal", runner)
			} else {
				err = ExportPackRuntime(runner)
			}
			if err == nil {
				t.Fatal("export accepted invalid output")
			}
			assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
		})
	}
}

func TestLocalRuntimeManifestFailures(t *testing.T) {
	spec, err := release.HostRuntimeSpecFor(release.DefaultRuntimeLock(), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"missing pack", "blocked manifest directory"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			mustWriteFile(t, filepath.Join(bin, spec.RuntimeName), []byte("engine"))
			if failure == "blocked manifest directory" {
				mustWriteFile(t, filepath.Join(bin, spec.PackName), []byte("pack"))
				mustWriteFile(t, filepath.Join(root, ".spx"), []byte("keep"))
			}
			if err := writeLocalRuntimeManifestIfComplete(root, bin); err == nil {
				t.Fatal("published incomplete or unwritable manifest")
			}
			got, err := os.ReadFile(filepath.Join(bin, spec.RuntimeName))
			if err != nil || string(got) != "engine" {
				t.Fatalf("source engine changed: %q, %v", got, err)
			}
		})
	}
}

func TestFindExportedPack(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		wantFound    bool
	}{
		{"direct pack", "gdexport.pck", true},
		{"legacy app output", filepath.Join("gdexport.app", "Contents", "Resources", "gdexport.pck"), false},
		{"missing output", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			packDir := filepath.Join(root, "project", ".builds", "pc")
			if tc.output != "" {
				mustWriteFile(t, filepath.Join(packDir, tc.output), []byte("pack"))
			}
			got, err := findExportedPack(root)
			want := filepath.Join(packDir, "gdexport.pck")
			if tc.wantFound {
				if err != nil || got != want {
					t.Fatalf("findExportedPack = %q, %v; want %q", got, err, want)
				}
			} else if got != "" || err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("findExportedPack = %q, %v; want missing expected pack %q", got, err, want)
			}
		})
	}
}

func TestRuntimeWebTemplateModes(t *testing.T) {
	for _, mode := range []string{"normal", "worker", "minigame", "miniprogram"} {
		t.Run(mode, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			dst := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+release.DefaultRuntimeLock().RuntimeVersion+"_web"+mode)
			mustWriteFile(t, filepath.Join(dst, "stale.js"), []byte("previous export"))
			if err := ExportWebTemplateRuntime(mode, runner); err != nil {
				t.Fatal(err)
			}
			assertSingleRuntimeWorkspaceCommand(t, runner, []string{"exporttemplateweb"})
			for name, want := range map[string]string{
				"engine.js":  "var EnginePackMode = '" + mode + "';\nconsole.log('engine');\n",
				"engine.zip": "engine-pack",
			} {
				if got, err := os.ReadFile(filepath.Join(dst, name)); err != nil || string(got) != want {
					t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
				}
			}
			for _, name := range []string{"engine.pck", "stale.js"} {
				if _, err := os.Lstat(filepath.Join(dst, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("obsolete %s remained: %v", name, err)
				}
			}
			assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
		})
	}
	t.Run("invalid mode", func(t *testing.T) {
		runner := newRuntimeFixtureRunner(t)
		if err := ExportWebTemplateRuntime("unknown", runner); err == nil || !strings.Contains(err.Error(), "unsupported web-mode") {
			t.Fatalf("invalid mode error = %v", err)
		}
		if len(runner.commands) != 0 || len(runner.calls) != 0 {
			t.Fatalf("invalid mode ran commands: %#v, %#v", runner.commands, runner.calls)
		}
		if _, err := os.Stat(filepath.Join(runner.repoRoot, ".tmp")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid mode created workspace: %v", err)
		}
	})
}

func TestRuntimeCommandsPropagateRunnerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures")
	}
	for _, command := range []string{"build-wasm", "export-web", "export-pack", "export-web-template"} {
		t.Run(command, func(t *testing.T) {
			runner := newRuntimeFixtureRunner(t)
			t.Chdir(runner.repoRoot)
			mustWriteFile(t, filepath.Join(runner.repoRoot, "go.mod"), []byte("module fixture\n"))
			mustMkdirAll(t, filepath.Join(runner.repoRoot, "internal", "tools"))
			log := filepath.Join(runner.repoRoot, "command.log")
			t.Setenv("RUNTIME_COMMAND_LOG", log)
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$RUNTIME_COMMAND_LOG\"\nexit 23\n"
			mustWriteExecutable(t, filepath.Join(runner.repoRoot, "cmd", "spx", "install.sh"), script)
			goCommand := filepath.Join(os.Getenv("GOPATH"), "bin", "go")
			mustWriteExecutable(t, goCommand, script)
			// Keep command environment setup independent of installed macOS SDKs.
			t.Setenv("SDKROOT", runner.repoRoot)
			t.Setenv("CC", goCommand)
			t.Setenv("CXX", goCommand)
			for _, name := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"} {
				t.Setenv(name, "")
			}
			err := Run([]string{command})
			if err == nil || !strings.Contains(err.Error(), "exit status 23") {
				t.Fatalf("Run(%q) = %v, want command failure", command, err)
			}
			got, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if command == "build-wasm" || command == "export-web" {
				if string(got) != "--web --no-embed-runtime\n" {
					t.Fatalf("installation arguments = %q", got)
				}
			} else {
				export := "exportpack"
				if command == "export-web-template" {
					export = "exporttemplateweb"
				}
				wantPrefix := "run ./cmd/spx " + export + " --path " + filepath.Join(runner.repoRoot, ".tmp", "runtime-workspace-")
				if !strings.HasPrefix(string(got), wantPrefix) {
					t.Fatalf("export arguments = %q; want prefix %q", got, wantPrefix)
				}
				assertRuntimeWorkspacesCleaned(t, runner.repoRoot)
			}
		})
	}
}

func TestRuntimeExportPackRejectsInvalidLocalAssets(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	assets := filepath.Join(runner.repoRoot, "missing-assets")
	if err := exportPackRuntime(runtimeExportPackConfig{engineAssetDir: assets}, runner); err == nil {
		t.Fatal("missing local engine assets were accepted")
	}
	if len(runner.commands) != 0 {
		t.Fatalf("invalid local assets ran export: %#v", runner.commands)
	}
	if _, err := os.Stat(filepath.Join(runner.repoRoot, ".tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid local assets created workspace: %v", err)
	}
}

func TestLocalRuntimeManifestMissingEngineDoesNotPublish(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := writeLocalRuntimeManifestIfComplete(root, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spx")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing engine published metadata: %v", err)
	}
}

func TestCompressWasmArtifactsInstallsBrotli(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		osRelease, err := os.ReadFile("/etc/os-release")
		if err != nil {
			t.Skipf("cannot identify Linux package manager: %v", err)
		}
		if lower := strings.ToLower(string(osRelease)); !strings.Contains(lower, "id=ubuntu") && !strings.Contains(lower, "id=debian") {
			t.Skip("APT installation fixture requires Ubuntu or Debian")
		}
	case "darwin":
	default:
		t.Skip("package manager fixture requires Linux or macOS")
	}
	copyCommand, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, failure, wantErr, wantCalls string
	}{
		{"install and compress", "", "", "apt-get update\napt-get install -y brotli\n"},
		{"update fails", "update", "exit status 41", "apt-get update\n"},
		{"install fails", "install", "exit status 42", "apt-get update\napt-get install -y brotli\n"},
		{"install succeeds without binary", "missing binary", "brotli installation failed", "apt-get update\napt-get install -y brotli\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "darwin" {
				if tc.failure == "update" {
					t.Skip("Homebrew installation has no separate update command")
				}
				tc.wantCalls = "install brotli\n"
			}
			root := t.TempDir()
			bin := filepath.Join(root, "commands")
			log := filepath.Join(root, "install.log")
			brotliFixture := filepath.Join(root, "brotli-fixture")
			t.Setenv("GOPATH", filepath.Join(root, "gopath"))
			t.Setenv("PATH", bin)
			t.Setenv("BROTLI_TEST_LOG", log)
			t.Setenv("BROTLI_TEST_FAILURE", tc.failure)
			t.Setenv("BROTLI_TEST_COPY", copyCommand)
			t.Setenv("BROTLI_TEST_SOURCE", brotliFixture)
			t.Setenv("BROTLI_TEST_DEST", filepath.Join(bin, "brotli"))
			// PATH contains only local fixtures, so no package manager or real sudo can run.
			mustWriteExecutable(t, filepath.Join(bin, "sudo"), `#!/bin/sh
printf '%s\n' "$*" >> "$BROTLI_TEST_LOG"
[ "$1" = apt-get ] || exit 90
case "$2" in
  update)
    [ "$BROTLI_TEST_FAILURE" != update ] || exit 41
    ;;
  install)
    [ "$*" = 'apt-get install -y brotli' ] || exit 91
    [ "$BROTLI_TEST_FAILURE" != install ] || exit 42
    [ "$BROTLI_TEST_FAILURE" != 'missing binary' ] || exit 0
    exec "$BROTLI_TEST_COPY" "$BROTLI_TEST_SOURCE" "$BROTLI_TEST_DEST"
    ;;
  *) exit 92 ;;
esac
`)
			mustWriteExecutable(t, filepath.Join(bin, "brew"), `#!/bin/sh
printf '%s\n' "$*" >> "$BROTLI_TEST_LOG"
[ "$*" = 'install brotli' ] || exit 90
[ "$BROTLI_TEST_FAILURE" != install ] || exit 42
[ "$BROTLI_TEST_FAILURE" != 'missing binary' ] || exit 0
exec "$BROTLI_TEST_COPY" "$BROTLI_TEST_SOURCE" "$BROTLI_TEST_DEST"
`)
			mustWriteExecutable(t, brotliFixture, `#!/bin/sh
[ "$1" = -q ] && [ "$2" = 11 ] && [ "$3" = -o ] || exit 93
exec "$BROTLI_TEST_COPY" "$5" "$4"
`)
			goBin := filepath.Join(os.Getenv("GOPATH"), "bin")
			engine := filepath.Join(goBin, "gdspxrt"+release.DefaultRuntimeLock().RuntimeVersion+"_webnormal", "engine.wasm")
			interpreter := filepath.Join(goBin, "ispx.wasm")
			for _, path := range []string{engine, interpreter} {
				mustWriteFile(t, path, []byte(filepath.Base(path)))
				mustWriteFile(t, path+".br", []byte("previous compressed file"))
			}
			err := compressWasmArtifacts()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				// A second availability check must reuse the installed tool.
				if err := ensureBrotliAvailable(); err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("compression error = %v, want %q", err, tc.wantErr)
			}
			if got, err := os.ReadFile(log); err != nil || string(got) != tc.wantCalls {
				t.Fatalf("installer calls = %q, %v; want %q", got, err, tc.wantCalls)
			}
			for _, path := range []string{engine, interpreter} {
				want := "previous compressed file"
				if tc.wantErr == "" {
					want = filepath.Base(path)
				}
				if got, err := os.ReadFile(path + ".br"); err != nil || string(got) != want {
					t.Fatalf("compressed %s = %q, %v; want %q", path, got, err, want)
				}
			}
		})
	}
}

func TestBrotliLinuxInstallRejectsMissingReleaseAndTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("package manager fixtures use POSIX shell scripts")
	}
	if _, err := os.Stat("/etc/os-release"); !errors.Is(err, os.ErrNotExist) {
		t.Skip("fixture requires a host without /etc/os-release")
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "installer-ran")
	t.Setenv("PATH", bin)
	t.Setenv("BROTLI_TEST_LOG", marker)
	mustWriteExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\n: > \"$BROTLI_TEST_LOG\"\nexit 99\n")
	const want = "unable to install brotli automatically on linux: no supported package manager found"
	if err := installBrotliLinux(); err == nil || err.Error() != want {
		t.Fatalf("unsupported Linux installation = %v, want %q", err, want)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported Linux installation ran an installer: %v", err)
	}
}

func TestBrotliLinuxInstallCommands(t *testing.T) {
	apt := [][]string{{"apt-get", "update"}, {"apt-get", "install", "-y", "brotli"}}
	dnf := [][]string{{"dnf", "install", "-y", "brotli"}}
	yum := [][]string{{"yum", "install", "-y", "brotli"}}
	for _, tt := range []struct {
		name, release string
		available     []string
		want          [][]string
	}{
		{"Ubuntu uses apt even when not found on PATH", "ID=ubuntu\n", nil, apt},
		{"Debian prefers apt", "ID=debian\n", []string{"dnf", "yum"}, apt},
		{"distribution matching ignores case", "ID=UBUNTU\n", nil, apt},
		{"Fedora prefers dnf", "ID=fedora\n", []string{"apt-get", "dnf", "yum"}, dnf},
		{"RHEL falls back to yum", "ID=rhel\n", []string{"apt-get", "yum"}, yum},
		{"CentOS leaves missing yum to command execution", "ID=centos\n", nil, yum},
		{"unknown distribution prefers apt", "ID=custom\n", []string{"apt-get", "dnf", "yum"}, apt},
		{"unknown distribution prefers dnf over yum", "ID=custom\n", []string{"dnf", "yum"}, dnf},
		{"unknown distribution uses yum", "ID=custom\n", []string{"yum"}, yum},
		{"missing release uses available manager", "", []string{"dnf"}, dnf},
		{"missing release and tools", "", nil, nil},
		{"unsupported package manager", "ID=arch\n", []string{"pacman"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			available := make(map[string]bool)
			for _, name := range tt.available {
				available[name] = true
			}
			commands, err := brotliLinuxInstallCommands(tt.release, available)
			if tt.want == nil {
				const wantError = "unable to install brotli automatically on linux: no supported package manager found"
				if err == nil || err.Error() != wantError || len(commands) != 0 {
					t.Fatalf("unsupported installer = %v, %v; want no commands and %q", commands, err, wantError)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(commands, tt.want) {
				t.Fatalf("installation commands = %v, %v; want %v", commands, err, tt.want)
			}
		})
	}
}

func (r *recordingRunner) RunScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return r.scriptErr
}

func (r *recordingRunner) RunCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if r.repoRoot != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(r.repoRoot, dir)
	}
	r.commands = append(r.commands, recordedCommand{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	if r.commandHook != nil {
		return r.commandHook(dir, name, args...)
	}
	return nil
}

func (r *recordingRunner) RepoRootDir() string {
	if r.repoRoot == "" {
		return "."
	}
	return r.repoRoot
}

func newRuntimeFixtureRunner(t *testing.T) *recordingRunner {
	t.Helper()

	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("runtime extension"))

	return &recordingRunner{
		repoRoot:    root,
		commandHook: simulateRuntimeCommandOutputs,
	}
}

func simulateRuntimeCommandOutputs(workdir string, name string, args ...string) error {
	projectDir, spxArgs, ok := simulatedSPXInvocation(workdir, name, args...)
	if !ok || len(spxArgs) == 0 {
		return nil
	}

	type outputFile struct{ name, content string }
	var files []outputFile
	switch spxArgs[0] {
	case "exportpack":
		files = []outputFile{{"pc/gdexport.pck", "runtime-pack"}}
	case "exporttemplateweb":
		files = []outputFile{
			{"webi/engine.pck", "engine-pack"},
			{"webi/engine.js", "console.log('engine');\n"},
		}
	case "exportweb", "exportwebworker", "exportminigame", "exportminiprogram":
		files = []outputFile{
			{"web/index.html", "<html></html>"},
			{"web/subdir/game.js", "console.log('game');\n"},
		}
	}
	for _, file := range files {
		dst := filepath.Join(projectDir, "project", ".builds", filepath.FromSlash(file.name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(file.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func simulatedSPXInvocation(workdir string, name string, args ...string) (string, []string, bool) {
	switch name {
	case "spx":
		return workdir, append([]string(nil), args...), len(args) > 0
	case "go":
		if len(args) < 4 || args[0] != "run" || args[1] != "./cmd/spx" {
			return "", nil, false
		}
		spxArgs := append([]string(nil), args[2:]...)
		projectDir := workdir
		for i := 0; i+1 < len(spxArgs); i++ {
			if spxArgs[i] == "--path" {
				projectDir = spxArgs[i+1]
				spxArgs = append(spxArgs[:i], spxArgs[i+2:]...)
				break
			}
		}
		return projectDir, spxArgs, len(spxArgs) > 0
	default:
		return "", nil, false
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

func installFakeBrotli(t *testing.T, root string) {
	t.Helper()

	binDir := filepath.Join(root, "fake-bin")
	scriptPath := filepath.Join(binDir, "brotli")
	script := "#!/bin/sh\nif [ \"$1\" = \"-q\" ]; then shift 2; fi\nif [ \"$1\" != \"-o\" ]; then exit 2; fi\nout=\"$2\"\nin=\"$3\"\ncp \"$in\" \"$out\"\n"
	mustWriteExecutable(t, scriptPath, script)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertSingleRuntimeWorkspaceCommand(t *testing.T, runner *recordingRunner, args []string) {
	t.Helper()
	if len(runner.commands) != 1 {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	got := runner.commands[0]
	projectDir, spxArgs, ok := simulatedSPXInvocation(got.dir, got.name, got.args...)
	if !ok || !reflect.DeepEqual(spxArgs, args) {
		t.Fatalf("unexpected command: %#v", got)
	}
	prefix := filepath.Join(runner.repoRoot, ".tmp", "runtime-workspace-")
	if !strings.HasPrefix(projectDir, prefix) {
		t.Fatalf("unexpected runtime workspace dir: %s", projectDir)
	}
	if got := filepath.Base(projectDir); got != runtimeWorkspaceProjectName {
		t.Fatalf("runtime workspace project name = %q, want %q", got, runtimeWorkspaceProjectName)
	}
}

func mustWriteExecutable(t *testing.T, path, script string) {
	t.Helper()
	mustWriteFile(t, path, []byte(script))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertRuntimeWorkspacesCleaned(t *testing.T, repoRoot string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, ".tmp"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("runtime workspaces remain: %v, %v", entries, err)
	}
}
