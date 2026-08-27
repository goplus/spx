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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	enginepkg "github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
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
	case "exportpack":
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

func TestSetupAssetsHost(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t)
	cfg := setupConfig{target: "host"}
	version := mustDefaultRuntimeVersion(t)

	if err := setupAssets(cfg, runner); err != nil {
		t.Fatalf("setupAssets returned error: %v", err)
	}

	expected := []recordedCall{
		{script: "cmd/spx/install.sh", args: nil},
	}
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exportpack"}},
	})

	if !shared.FileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspx"+version)) {
		t.Fatalf("expected host editor binary to exist")
	}
	if !shared.FileExists(filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+version+".pck")) {
		t.Fatalf("expected runtime pck to exist")
	}
}

func TestSetupAssetsHostUsesSameRunPack(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t)
	assetDir := filepath.Join(runner.repoRoot, "runtime-assets")
	cfg := setupConfig{target: "host", assetDir: assetDir}
	version := mustDefaultRuntimeVersion(t)

	if err := setupAssets(cfg, runner); err != nil {
		t.Fatalf("setupAssets returned error: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("same-run pack should avoid rebuilding runtime assets: %#v", runner.commands)
	}
	packPath := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+version+".pck")
	pack, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(pack) != "same-run-runtime-pack" {
		t.Fatalf("runtime pack = %q", pack)
	}
}

func TestSetupAssetsHostUsesPublishedPack(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t)
	cfg := setupConfig{target: "host", publishedRuntime: true}
	version := mustDefaultRuntimeVersion(t)

	if err := setupAssets(cfg, runner); err != nil {
		t.Fatalf("setupAssets returned error: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("published pack should avoid rebuilding runtime assets: %#v", runner.commands)
	}
	packPath := filepath.Join(os.Getenv("GOPATH"), "bin", "gdspxrt"+version+".pck")
	pack, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(pack) != "runtime-pck" {
		t.Fatalf("runtime pack = %q", pack)
	}
}

func TestSetupAssetsWeb(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t)
	cfg := setupConfig{target: "web", mode: "worker"}
	version := mustDefaultRuntimeVersion(t)

	if err := setupAssets(cfg, runner); err != nil {
		t.Fatalf("setupAssets returned error: %v", err)
	}

	expectedCalls := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web", "--no-embed-runtime"}}}
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

func TestSetupAssetsFull(t *testing.T) {
	runner := newRuntimeFixtureRunner(t)
	installFakeEngineDownload(t)
	cfg := setupConfig{target: "full", mode: "minigame"}

	if err := setupAssets(cfg, runner); err != nil {
		t.Fatalf("setupAssets returned error: %v", err)
	}

	expectedCalls := []recordedCall{{script: "cmd/spx/install.sh", args: []string{"--web"}}}
	if !reflect.DeepEqual(runner.calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}

	assertRuntimeWorkspaceCommands(t, runner.commands, runner.repoRoot, []recordedCommand{
		{name: "spx", args: []string{"exportpack"}},
		{name: "spx", args: []string{"exporttemplateweb"}},
	})
}

func installFakeEngineDownload(t *testing.T) {
	t.Helper()

	oldDownload := downloadEngineAssets
	oldPrepareEditor := prepareHostEditorAsset
	downloadEngineAssets = func(cfg enginepkg.DownloadConfig, repoRoot string) error {
		version := mustDefaultRuntimeVersion(t)
		goBinDir := filepath.Join(os.Getenv("GOPATH"), "bin")
		if err := os.MkdirAll(goBinDir, 0o755); err != nil {
			return err
		}
		if cfg.Runtime {
			if err := os.WriteFile(filepath.Join(goBinDir, "gdspx"+version), []byte("editor"), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(goBinDir, "gdspxrt"+version), []byte("runtime"), 0o755); err != nil {
				return err
			}
			if !cfg.SkipRuntimePack {
				content := "runtime-pck"
				if cfg.AssetDir != "" {
					content = "same-run-runtime-pack"
				}
				return os.WriteFile(filepath.Join(goBinDir, "gdspxrt"+version+".pck"), []byte(content), 0o644)
			}
		}
		if cfg.Platform == "web" {
			templateDir := filepath.Join(repoRoot, "templates")
			if err := os.MkdirAll(templateDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(templateDir, "web_release.zip"), []byte(cfg.Mode), 0o644)
		}
		return nil
	}
	prepareHostEditorAsset = func(repoRoot, assetDir string) error {
		version := mustDefaultRuntimeVersion(t)
		goBinDir := filepath.Join(os.Getenv("GOPATH"), "bin")
		if err := os.MkdirAll(goBinDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(goBinDir, "gdspx"+version), []byte("editor"), 0o755)
	}

	t.Cleanup(func() {
		downloadEngineAssets = oldDownload
		prepareHostEditorAsset = oldPrepareEditor
	})
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
