package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type recordedCall struct {
	script string
	args   []string
}

type recordingRunner struct {
	calls       []recordedCall
	commands    []recordedCommand
	repoRoot    string
	commandHook func(workdir string, name string, args ...string) error
}

func (r *recordingRunner) runScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return nil
}

func (r *recordingRunner) runCommand(workdir string, name string, args ...string) error {
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

func (r *recordingRunner) repoRootDir() string {
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

	mustMkdirAll(t, filepath.Join(root, "cmd", "gox", "template", "project"))
	mustWriteFile(t, filepath.Join(root, "cmd", "gox", "template", "version"), []byte("2.1.44"))
	mustWriteFile(t, filepath.Join(root, "cmd", "gox", "template", "project", "runtime.gdextension.txt"), []byte("runtime extension"))

	return &recordingRunner{
		repoRoot:    root,
		commandHook: simulateRuntimeCommandOutputs,
	}
}

func simulateRuntimeCommandOutputs(workdir string, name string, args ...string) error {
	if name != "spx" || len(args) == 0 {
		return nil
	}

	switch args[0] {
	case "export":
		dst := filepath.Join(workdir, "project", ".builds", "pc", "gdexport.pck")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("runtime-pack"), 0o644)
	case "exporttemplateweb":
		dstDir := filepath.Join(workdir, "project", ".builds", "webi")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, "engine.pck"), []byte("engine-pack"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "engine.js"), []byte("console.log('engine');\n"), 0o644)
	case "exportweb", "exportwebworker", "exportminigame", "exportminiprogram":
		dstDir := filepath.Join(workdir, "project", ".builds", "web", "subdir")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(workdir, "project", ".builds", "web", "index.html"), []byte("<html></html>"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "game.js"), []byte("console.log('game');\n"), 0o644)
	default:
		return nil
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

func TestParsePrepareArgsDefaults(t *testing.T) {
	cfg, err := parsePrepareArgs(nil)
	if err != nil {
		t.Fatalf("parsePrepareArgs returned error: %v", err)
	}

	if cfg.setupMode != "runtime" {
		t.Fatalf("unexpected setupMode: %s", cfg.setupMode)
	}
	if cfg.webMode != "normal" {
		t.Fatalf("unexpected webMode: %s", cfg.webMode)
	}
}

func TestParsePrepareArgsHelp(t *testing.T) {
	_, err := parsePrepareArgs([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestPrepareAssetsRuntime(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	cfg := prepareConfig{setupMode: "runtime", webMode: "normal"}

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/gox/install.sh", args: nil},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	if !fileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx2.1.44")) {
		t.Fatalf("expected host editor binary to exist")
	}
}

func TestPrepareAssetsWeb(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	cfg := prepareConfig{setupMode: "web", webMode: "worker"}

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	expectedCalls := []recordedCall{{script: "cmd/gox/install.sh", args: []string{"--web"}}}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
	if !fileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx2.1.44")) {
		t.Fatalf("expected host editor binary to exist")
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	})

	dstDir := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt2.1.44_webworker")
	if !fileExists(filepath.Join(dstDir, "engine.zip")) {
		t.Fatalf("expected engine.zip in %s", dstDir)
	}
	if !fileExists(filepath.Join(runner.repoRoot, "templates", "web_release.zip")) {
		t.Fatalf("expected downloaded web template in template dir")
	}
}

func TestPrepareAssetsFull(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	cfg := prepareConfig{setupMode: "full", webMode: "minigame"}

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	expectedCalls := []recordedCall{{script: "cmd/gox/install.sh", args: []string{"--web"}}}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	})
}

func assertRuntimeWorkspaceCommands(t *testing.T, got []recordedCommand, repoRoot string, want []recordedCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected commands: %#v", got)
	}
	for i := range want {
		if got[i].name != want[i].name || !reflect.DeepEqual(got[i].args, want[i].args) {
			t.Fatalf("unexpected command[%d]: %#v", i, got[i])
		}
		prefix := filepath.Join(repoRoot, ".tmp", "runtime-")
		if !strings.HasPrefix(got[i].dir, prefix) {
			t.Fatalf("unexpected runtime workspace dir[%d]: %s", i, got[i].dir)
		}
	}
}
