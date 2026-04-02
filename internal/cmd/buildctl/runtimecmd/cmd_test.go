package runtimecmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRuntimeExportPackArgsDefault(t *testing.T) {
	if err := parseRuntimeExportPackArgs(nil); err != nil {
		t.Fatalf("parseRuntimeExportPackArgs returned error: %v", err)
	}
}

func TestParseRuntimeBuildWasmArgsDefault(t *testing.T) {
	cfg, err := parseRuntimeBuildWasmArgs(nil)
	if err != nil {
		t.Fatalf("parseRuntimeBuildWasmArgs returned error: %v", err)
	}
	if cfg.opt {
		t.Fatal("opt should default to false")
	}
}

func TestRuntimeBuildWasmSequence(t *testing.T) {
	runner := &recordingRunner{}

	if err := buildWasmRuntime(runtimeBuildWasmConfig{}, runner); err != nil {
		t.Fatalf("buildWasmRuntime returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web"}},
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

	if err := buildWasmRuntime(runtimeBuildWasmConfig{opt: true}, runner); err != nil {
		t.Fatalf("buildWasmRuntime returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: []string{"--web", "--opt"}},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
	if !fileExists(filepath.Join(gopathBin, "ispx.wasm.br")) {
		t.Fatalf("expected compressed ispx.wasm.br to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version+"_webnormal", "engine.wasm.br")) {
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
		{script: "cmd/spx/install.sh", args: []string{"--web", "--opt"}},
	}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertSingleRuntimeWorkspaceCommand(t, runner, "spx", []string{"exportwebworker"})

	if !fileExists(filepath.Join(runner.repoRoot, "spx_web_worker.zip")) {
		t.Fatalf("expected export zip to exist")
	}
}

func TestRuntimeExportPackSequence(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	version := mustDefaultRuntimeVersion(t)

	if err := exportPackRuntime(runner); err != nil {
		t.Fatalf("exportPackRuntime returned error: %v", err)
	}

	assertSingleRuntimeWorkspaceCommand(t, runner, "spx", []string{"export"})

	gopathBin := filepath.Join(os.Getenv("GOPATH"), "bin")
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version+".pck")) {
		t.Fatalf("expected exported pck to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt.pck."+version+".zip")) {
		t.Fatalf("expected exported zip to exist")
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
	if !fileExists(filepath.Join(gopathBin, "ispx.wasm.br")) {
		t.Fatalf("expected compressed ispx.wasm.br to exist")
	}
	if !fileExists(filepath.Join(gopathBin, "gdspxrt"+version+"_web", "engine.wasm.br")) {
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
	if got.name != name || !reflect.DeepEqual(got.args, args) {
		t.Fatalf("unexpected command: %#v", got)
	}
	prefix := filepath.Join(runner.repoRoot, ".tmp", "runtime-")
	if !strings.HasPrefix(got.dir, prefix) {
		t.Fatalf("unexpected runtime workspace dir: %s", got.dir)
	}
}
