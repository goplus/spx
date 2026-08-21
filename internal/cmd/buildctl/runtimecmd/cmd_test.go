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
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

func TestParseRuntimeBuildWasmArgsDefault(t *testing.T) {
	cfg, err := parseRuntimeBuildWasmArgs(nil)
	if err != nil {
		t.Fatalf("parseRuntimeBuildWasmArgs returned error: %v", err)
	}
	if cfg.Opt {
		t.Fatal("opt should default to false")
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
	version := mustDefaultRuntimeVersion(t)
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
	runner := newRuntimeFixtureRunner(t)
	cfg := runtimeExportWebConfig{mode: "worker"}

	if err := exportWebRuntime(cfg, runner); err != nil {
		t.Fatalf("exportWebRuntime returned error: %v", err)
	}

	expectedCalls := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}},
	}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertSingleRuntimeWorkspaceCommand(t, runner, "spx", []string{"exportwebworker"})

	if !shared.FileExists(filepath.Join(runner.repoRoot, "spx_web_worker.zip")) {
		t.Fatalf("expected export zip to exist")
	}
}

func TestRuntimeExportPackSequence(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	version := mustDefaultRuntimeVersion(t)
	spec, err := release.HostRuntimeSpecFor(release.DefaultRuntimeLock(), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	mustWriteFile(t, filepath.Join(gopathBin, spec.RuntimeName), []byte("runtime-engine"))

	if err := ExportPackRuntime(runner); err != nil {
		t.Fatalf("exportPackRuntime returned error: %v", err)
	}

	assertSingleRuntimeWorkspaceCommand(t, runner, "spx", []string{"export"})

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
	version := mustDefaultRuntimeVersion(t)

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

func installFakeBrotli(t *testing.T, root string) {
	t.Helper()

	binDir := filepath.Join(root, "fake-bin")
	mustMkdirAll(t, binDir)
	scriptPath := filepath.Join(binDir, "brotli")
	script := "#!/bin/sh\nif [ \"$1\" = \"-q\" ]; then shift 2; fi\nif [ \"$1\" != \"-o\" ]; then exit 2; fi\nout=\"$2\"\nin=\"$3\"\ncp \"$in\" \"$out\"\n"
	mustWriteFile(t, scriptPath, []byte(script))
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		t.Fatalf("chmod fake brotli: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertSingleRuntimeWorkspaceCommand(t *testing.T, runner *recordingRunner, name string, args []string) {
	t.Helper()
	if len(runner.commands) != 1 {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
	got := runner.commands[0]
	if name != "spx" {
		if got.name != name || !reflect.DeepEqual(got.args, args) {
			t.Fatalf("unexpected command: %#v", got)
		}
		return
	}
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
