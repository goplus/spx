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
	"archive/zip"
	"bytes"
	"embed"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

//go:embed template/project/* template/project/.godot/* template/platform/web/* template/platform/webnormal/*
var webExportTestFS embed.FS

func TestExportWebUsesInstalledRuntimeWithoutProjectBuild(t *testing.T) {
	projectRoot := t.TempDir()
	t.Chdir(projectRoot)
	if err := os.MkdirAll(filepath.Join(projectRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"main.spx":          "onStart => {}\n",
		"assets/index.json": "{}\n",
		"go.sum":            "existing sums\n",
	} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".temp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".temp", "keep"), []byte("session"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "project", "old.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	goPath := t.TempDir()
	binDir := filepath.Join(goPath, "bin")
	runtimeDir := filepath.Join(binDir, "gdspxrttest_webnormal")
	for _, dir := range []string{runtimeDir, filepath.Join(binDir, "ispx")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{
		filepath.Join(runtimeDir, "godot.editor.html"): "<html>runtime</html>",
		filepath.Join(binDir, "ispx", "runner.js"):     "interpreter runtime",
		filepath.Join(binDir, "ispx.wasm"):             "interpreter wasm",
	} {
		if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOPATH", goPath)
	t.Setenv("GODEBUG", os.Getenv("GODEBUG"))

	previousFlags, previousArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"spx", "exportweb", "--path", projectRoot}
	t.Cleanup(func() { flag.CommandLine, os.Args = previousFlags, previousArgs })

	cmd := &CmdTool{PlatformFS: webExportTestFS}
	if err := cmd.RunCmd("spx", ".spx", "test", webExportTestFS, "template/project", "project"); err != nil {
		t.Fatalf("exportweb: %v", err)
	}
	webDir := filepath.Join(projectRoot, "project", ".builds", "web")
	for name, want := range map[string]string{
		"index.html": "<html>runtime</html>",
		"ispx.wasm":  "interpreter wasm",
		"base.txt":   "web test base\n",
		"normal.txt": "web test normal\n",
		"runner.js":  "interpreter runtime",
	} {
		got, err := os.ReadFile(filepath.Join(webDir, name))
		if err != nil || string(got) != want {
			t.Fatalf("exported %s = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "project", ".godot", "gdspx_web_server.py")); err != nil {
		t.Fatalf("web server template is missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "project", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale web project file remains: %v", err)
	}
	for _, name := range []string{"go.sum", ".temp"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("transient %s remains: %v", name, err)
		}
	}

	archive, err := zip.OpenReader(filepath.Join(webDir, "game.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	found := map[string]bool{}
	for _, entry := range archive.File {
		found[entry.Name] = true
	}
	for _, name := range []string{"main.spx", "assets/index.json"} {
		if !found[name] {
			t.Fatalf("game.zip is missing %s", name)
		}
	}
	for _, name := range []string{"go.sum", ".temp/", ".temp/keep", "project/old.txt"} {
		if found[name] {
			t.Fatalf("game.zip contains transient %s", name)
		}
	}
}

func TestWebLogicAssetsUseInterpreterRuntime(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}

	projectWasm := []byte("project-compiled-wasm")
	projectWasmPath := filepath.Join(webDir, "ispx.wasm")
	if err := os.MkdirAll(filepath.Dir(projectWasmPath), 0o755); err != nil {
		t.Fatalf("mkdir project wasm dir: %v", err)
	}
	if err := os.WriteFile(projectWasmPath, projectWasm, 0o644); err != nil {
		t.Fatalf("write project wasm: %v", err)
	}
	wantWasm := []byte("interpreter-wasm")
	wantWasmBr := []byte("compressed-interpreter-wasm")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), wantWasm, 0o644); err != nil {
		t.Fatalf("write interpreter wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm.br"), wantWasmBr, 0o644); err != nil {
		t.Fatalf("write compressed interpreter wasm: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir exported web dir: %v", err)
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}

	gotWasm, err := os.ReadFile(filepath.Join(webDir, "ispx.wasm"))
	if err != nil {
		t.Fatalf("read exported project wasm: %v", err)
	}
	if !bytes.Equal(gotWasm, wantWasm) {
		t.Fatalf("exported wasm = %q, want %q", gotWasm, wantWasm)
	}
	gotWasmBr, err := os.ReadFile(filepath.Join(webDir, "ispx.wasm.br"))
	if err != nil {
		t.Fatalf("read compressed exported interpreter wasm: %v", err)
	}
	if !bytes.Equal(gotWasmBr, wantWasmBr) {
		t.Fatalf("compressed exported wasm = %q, want %q", gotWasmBr, wantWasmBr)
	}
}

func TestWebLogicAssetsRemoveStaleCompression(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), []byte("interpreter-wasm"), 0o644); err != nil {
		t.Fatalf("write fallback wasm: %v", err)
	}

	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "ispx.wasm.br"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale compressed wasm: %v", err)
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}
	if _, err := os.Stat(filepath.Join(webDir, "ispx.wasm.br")); !os.IsNotExist(err) {
		t.Fatalf("stale compressed wasm still exists: %v", err)
	}
}

func TestWebLogicAssetsCopyCompressedInterpreter(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	wantWasm := []byte("interpreter-wasm")
	wantWasmBr := []byte("compressed-interpreter-wasm")
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), wantWasm, 0o644); err != nil {
		t.Fatalf("write fallback wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm.br"), wantWasmBr, 0o644); err != nil {
		t.Fatalf("write compressed fallback wasm: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web dir: %v", err)
	}

	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}

	for name, want := range map[string][]byte{
		"ispx.wasm":    wantWasm,
		"ispx.wasm.br": wantWasmBr,
	} {
		got, err := os.ReadFile(filepath.Join(webDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}
