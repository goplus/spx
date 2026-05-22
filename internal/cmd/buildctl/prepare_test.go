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
	"archive/zip"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	enginepkg "github.com/goplus/spx/v2/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v2/internal/release"
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
}

func (r *recordingRunner) RunScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return nil
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

	mustMkdirAll(t, filepath.Join(root, "cmd", "spx", "template", "project"))
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

	switch spxArgs[0] {
	case "export":
		dst := filepath.Join(projectDir, "project", ".builds", "pc", "gdexport.pck")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("runtime-pack"), 0o644)
	case "exporttemplateweb":
		dstDir := filepath.Join(projectDir, "project", ".builds", "webi")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, "engine.pck"), []byte("engine-pack"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "engine.js"), []byte("console.log('engine');\n"), 0o644)
	case "exportweb", "exportwebworker", "exportminigame", "exportminiprogram":
		dstDir := filepath.Join(projectDir, "project", ".builds", "web", "subdir")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "project", ".builds", "web", "index.html"), []byte("<html></html>"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "game.js"), []byte("console.log('game');\n"), 0o644)
	default:
		return nil
	}
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
	version := mustDefaultRuntimeVersion(t)

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: nil},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	if !shared.FileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx"+version)) {
		t.Fatalf("expected host editor binary to exist")
	}
}

func TestPrepareAssetsNone(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	cfg := prepareConfig{setupMode: "none", webMode: "normal"}

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("unexpected script calls: %#v", runner.calls)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("unexpected commands: %#v", runner.commands)
	}
}

func TestPrepareAssetsWeb(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t, runner.repoRoot, "linux", "x86_64")
	cfg := prepareConfig{setupMode: "web", webMode: "worker"}
	version := mustDefaultRuntimeVersion(t)

	if err := prepareAssets(cfg, runner); err != nil {
		t.Fatalf("prepareAssets returned error: %v", err)
	}

	expectedCalls := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web"}}}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
	if !shared.FileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx"+version)) {
		t.Fatalf("expected host editor binary to exist")
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	})

	dstDir := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+version+"_webworker")
	if !shared.FileExists(filepath.Join(dstDir, "engine.zip")) {
		t.Fatalf("expected engine.zip in %s", dstDir)
	}
	if !shared.FileExists(filepath.Join(runner.repoRoot, "templates", "web_release.zip")) {
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

	expectedCalls := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web"}}}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exporttemplateweb"}},
	})
}

func installFakeEngineDownload(t *testing.T, repoRoot, defaultPlatform, arch string) {
	t.Helper()

	restoreResolve := enginepkg.SetEngineDownloadResolveEnv(func(repoRootArg, platform string) (enginepkg.DownloadEnv, error) {
		if platform == "" {
			platform = defaultPlatform
		}
		version := mustDefaultRuntimeVersion(t)
		return enginepkg.DownloadEnv{
			RepoRoot:    repoRootArg,
			Version:     version,
			Platform:    platform,
			Arch:        arch,
			GoBinDir:    filepath.Join(os.Getenv("GOPATH"), "bin"),
			TemplateDir: filepath.Join(repoRootArg, "templates"),
			CacheDir:    filepath.Join(repoRootArg, "cache"),
			URLPrefix:   "https://example.invalid/",
		}, nil
	})
	oldFetch := enginepkg.EngineDownloadFetcher
	enginepkg.EngineDownloadFetcher = fakeEngineDownloadFetcher

	t.Cleanup(func() {
		restoreResolve()
		enginepkg.EngineDownloadFetcher = oldFetch
	})
}

func fakeEngineDownloadFetcher(url, dst string) error {
	base := filepath.Base(url)
	runtimePackZip := release.RuntimeAssetZipName

	switch base {
	case "linux-x86_64.zip":
		return writeZipFixture(dst, map[string]string{
			"godot.linuxbsd.template_release.x86_64": "linux-template",
		})
	case "editor-linux-x86_64.zip":
		return writeZipFixture(dst, map[string]string{
			"godot.linuxbsd.editor.x86_64": "linux-editor",
		})
	case "web-worker.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "worker-web-template",
		})
	case "web.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "normal-web-template",
		})
	case "web-minigame.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "minigame-web-template",
		})
	case "web-miniprogram.zip":
		return writeZipFixture(dst, map[string]string{
			"web-template.bin": "miniprogram-web-template",
		})
	case runtimePackZip:
		return writeZipFixture(dst, map[string]string{
			"gdspxrt.pck": "runtime-pck",
		})
	default:
		return errors.New("unexpected download URL: " + url)
	}
}

func writeZipFixture(dst string, files map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			return err
		}
	}
	return writer.Close()
}

func assertRuntimeWorkspaceCommands(t *testing.T, got []recordedCommand, repoRoot string, want []recordedCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected commands: %#v", got)
	}
	for i := range want {
		if want[i].name == "spx" {
			projectDir, spxArgs, ok := simulatedSPXInvocation(got[i].dir, got[i].name, got[i].args...)
			if !ok || !reflect.DeepEqual(spxArgs, want[i].args) {
				t.Fatalf("unexpected command[%d]: %#v", i, got[i])
			}
			prefix := filepath.Join(repoRoot, ".tmp", "runtime-")
			if !strings.HasPrefix(projectDir, prefix) {
				t.Fatalf("unexpected runtime workspace dir[%d]: %s", i, projectDir)
			}
			continue
		}
		if got[i].name != want[i].name || !reflect.DeepEqual(got[i].args, want[i].args) {
			t.Fatalf("unexpected command[%d]: %#v", i, got[i])
		}
		prefix := filepath.Join(repoRoot, ".tmp", "runtime-")
		if !strings.HasPrefix(got[i].dir, prefix) {
			t.Fatalf("unexpected runtime workspace dir[%d]: %s", i, got[i].dir)
		}
	}
}
