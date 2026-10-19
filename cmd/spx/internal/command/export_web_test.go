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
	"embed"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

//go:embed template/project/* template/project/.godot/* template/platform/web/* template/platform/webnormal/*
var webExportTestFS embed.FS

func TestExportWebUsesInstalledRuntimeWithoutProjectBuild(t *testing.T) {
	projectRoot := t.TempDir()
	t.Chdir(projectRoot)
	for name, data := range map[string]string{
		"main.spx":          "onStart => {}\n",
		"assets/index.json": "{}\n",
		"go.sum":            "existing sums\n",
		".temp/keep":        "session",
		"project/old.txt":   "stale",
	} {
		writeCommandTestFile(t, filepath.Join(projectRoot, name), data)
	}

	goPath := t.TempDir()
	binDir := filepath.Join(goPath, "bin")
	runtimeDir := filepath.Join(binDir, "gdspxrttest_webnormal")
	for name, data := range map[string]string{
		filepath.Join(runtimeDir, "godot.editor.html"): "<html>runtime</html>",
		filepath.Join(binDir, "ispx", "runner.js"):     "interpreter runtime",
		filepath.Join(binDir, "ispx.wasm"):             "interpreter wasm",
	} {
		writeCommandTestFile(t, name, data)
	}
	t.Setenv("GOPATH", goPath)
	t.Setenv("GODEBUG", os.Getenv("GODEBUG"))

	setTestCommandLine(t, "spx", "exportweb", "--path", projectRoot)

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

func TestWebLogicAssetsUseInstalledInterpreter(t *testing.T) {
	for _, mode := range []string{webNormalMode, webWorkerMode, webMinigameMode, webMiniprogramMode} {
		for _, sibling := range []string{"absent", "stale", "invalid"} {
			t.Run(mode+"/"+sibling, func(t *testing.T) {
				cmd := CmdTool{GoBinPath: t.TempDir(), WebDir: t.TempDir()}
				wasmPath := filepath.Join(cmd.GoBinPath, "ispx.wasm")
				if err := os.WriteFile(wasmPath, []byte("interpreter-wasm"), 0o644); err != nil {
					t.Fatal(err)
				}
				switch sibling {
				case "stale":
					if err := os.WriteFile(wasmPath+".br", []byte("stale-installed-compression"), 0o644); err != nil {
						t.Fatal(err)
					}
				case "invalid":
					if err := os.Mkdir(wasmPath+".br", 0o755); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(cmd.WebDir, "ispx.wasm"), []byte("project-compiled-wasm"), 0o644); err != nil {
					t.Fatal(err)
				}
				compressedPath := filepath.Join(cmd.WebDir, "ispx.wasm.br")
				if err := os.WriteFile(compressedPath, []byte("stale-export-compression"), 0o644); err != nil {
					t.Fatal(err)
				}

				err := cmd.writeWebLogicAssets(mode)
				if sibling == "invalid" && mode != webMinigameMode {
					if err == nil || !strings.Contains(err.Error(), "failed to copy compressed ispx wasm") {
						t.Fatalf("writeWebLogicAssets error = %v, want compressed copy error", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("writeWebLogicAssets: %v", err)
				}
				if got, err := os.ReadFile(filepath.Join(cmd.WebDir, "ispx.wasm")); err != nil || string(got) != "interpreter-wasm" {
					t.Fatalf("exported wasm = %q, %v", got, err)
				}
				if mode == webMinigameMode || sibling == "absent" {
					if _, err := os.Stat(compressedPath); !os.IsNotExist(err) {
						t.Fatalf("unused compressed wasm remains: %v", err)
					}
				} else if got, err := os.ReadFile(compressedPath); err != nil || string(got) != "stale-installed-compression" {
					t.Fatalf("exported compressed wasm = %q, %v", got, err)
				}
			})
		}
	}
}

func TestMinigameEngineAssetsCompression(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake brotli uses a shell script")
	}
	for _, tt := range []struct {
		name, buildMode, failFile string
		staleCompression          bool
	}{
		{name: "fast", buildMode: "fast"},
		{name: "fast-stale", buildMode: "fast", staleCompression: true},
		{name: "normal", buildMode: "normal"},
		{name: "normal-stale", buildMode: "normal", staleCompression: true},
		{name: "engine-failure", buildMode: "normal", failFile: "engine.wasm", staleCompression: true},
		{name: "interpreter-failure", buildMode: "normal", failFile: "ispx.wasm", staleCompression: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			paths := minigamePaths{rawWebDir: t.TempDir(), engineDir: t.TempDir()}
			cmd := CmdTool{GoBinPath: t.TempDir(), WebDir: paths.rawWebDir}
			if err := os.WriteFile(filepath.Join(cmd.GoBinPath, "ispx.wasm"), []byte("ispx.wasm"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"engine.wasm", "game.zip"} {
				if err := os.WriteFile(filepath.Join(paths.rawWebDir, name), []byte(name), 0o644); err != nil {
					t.Fatal(err)
				}
				if tt.staleCompression && strings.HasSuffix(name, ".wasm") {
					if err := os.WriteFile(filepath.Join(paths.rawWebDir, name+".br"), []byte("stale"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tt.staleCompression {
				if err := os.WriteFile(filepath.Join(cmd.GoBinPath, "ispx.wasm.br"), []byte("stale-installed-compression"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := cmd.writeWebLogicAssets(webMinigameMode); err != nil {
				t.Fatal(err)
			}
			binDir := t.TempDir()
			logPath := filepath.Join(t.TempDir(), "brotli.log")
			const script = `#!/bin/sh
if [ "$#" != 4 ] || [ "$1 $2 $3" != "-f -q 11" ]; then exit 2; fi
printf '%s\n' "${4##*/}" >> "$SPX_TEST_BROTLI_LOG"
if [ "${4##*/}" = "$SPX_TEST_BROTLI_FAIL" ]; then exit 1; fi
printf 'compressed ' > "$4.br"
cat "$4" >> "$4.br"
`
			if err := os.WriteFile(filepath.Join(binDir, "brotli"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SPX_TEST_BROTLI_LOG", logPath)
			t.Setenv("SPX_TEST_BROTLI_FAIL", tt.failFile)

			err := cmd.prepareMinigameEngineAssets(paths, tt.buildMode)
			if tt.failFile != "" {
				if err == nil || !strings.Contains(err.Error(), "failed to compress "+filepath.Join(paths.rawWebDir, tt.failFile)) {
					t.Fatalf("prepareMinigameEngineAssets error = %v, want compression failure", err)
				}
				wantCalls := "engine.wasm\n"
				if tt.failFile == "ispx.wasm" {
					wantCalls += "ispx.wasm\n"
				}
				if got, err := os.ReadFile(logPath); err != nil || string(got) != wantCalls {
					t.Fatalf("brotli calls after failure = %q, %v; want %q", got, err, wantCalls)
				}
				if entries, err := os.ReadDir(paths.engineDir); err != nil || len(entries) != 0 {
					t.Fatalf("failed compression published engine assets: %v, %v", entries, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("prepareMinigameEngineAssets: %v", err)
			}
			if tt.buildMode == "fast" {
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Fatalf("fast build invoked brotli: %v", err)
				}
			} else if got, err := os.ReadFile(logPath); err != nil || string(got) != "engine.wasm\nispx.wasm\n" {
				t.Fatalf("brotli calls = %q, %v", got, err)
			}
			for _, name := range []string{"engine.wasm", "ispx.wasm", "game.zip"} {
				want := name
				if tt.buildMode != "fast" && strings.HasSuffix(name, ".wasm") {
					want = "compressed " + name
					name += ".br"
				}
				if got, err := os.ReadFile(filepath.Join(paths.engineDir, name)); err != nil || string(got) != want {
					t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
				}
			}
			if tt.buildMode == "fast" {
				if matches, err := filepath.Glob(filepath.Join(paths.engineDir, "*.br")); err != nil || len(matches) != 0 {
					t.Fatalf("fast build exported compressed wasm: %v, %v", matches, err)
				}
			}
		})
	}
}

func TestWebWorkerBundleAndEnginePatch(t *testing.T) {
	cmd := &CmdTool{WebDir: t.TempDir()}
	ext := filepath.Join(cmd.WebDir, "worker")
	for name, content := range map[string]string{
		"go.wasm.exec.js":         "runtime();",
		"worker/a.js":             "first();",
		"worker/z.js":             "last();",
		"worker/readme":           "ignored text",
		"worker/sub.js/nested.js": "ignored nested code",
	} {
		writeCommandTestFile(t, filepath.Join(cmd.WebDir, name), content)
	}
	bundle, err := cmd.readWebWorkerBundle(ext)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\n\n\nfirst();\n\n\nruntime();\n\n\nlast();"; bundle != want {
		t.Fatalf("worker bundle = %q, want %q", bundle, want)
	}
	if _, err := os.Stat(filepath.Join(cmd.WebDir, "go.wasm.exec.js")); !os.IsNotExist(err) {
		t.Fatalf("runtime script was not moved to bundle directory: %v", err)
	}
	const hook = "{if(initializedJS){checkMailbox()}}"
	const end = ";throw ex}}self.onmessage=handleMessage}"
	engine := filepath.Join(cmd.WebDir, "engine.js")
	writeCommandTestFile(t, engine, "before"+hook+"middle"+end+"after")
	if err := cmd.patchWebWorkerEngine(bundle); err != nil {
		t.Fatal(err)
	}
	want := "before" + hook + "else if(e.data._gameRunnerMessageId) {handleGameRunnerMessage(e.data);}" + "middle" + end + bundle + "after"
	if got := readTestFile(t, engine); got != want {
		t.Fatalf("patched worker = %q, want %q", got, want)
	}
}

func TestWebWorkerReportsMissingRuntimeAndAnchors(t *testing.T) {
	cmd := &CmdTool{WebDir: t.TempDir()}
	if _, err := cmd.readWebWorkerBundle(t.TempDir()); err == nil || !strings.Contains(err.Error(), "failed to move go.wasm.exec.js") {
		t.Fatalf("missing runtime error = %v", err)
	}
	if err := cmd.patchWebWorkerEngine(""); err == nil || !strings.Contains(err.Error(), "failed to read engine.js") {
		t.Fatalf("missing engine error = %v", err)
	}
	for _, tt := range []struct{ name, content, want string }{
		{"message anchor", "original engine", "missing worker hook anchor"},
		{"bundle anchor", "{if(initializedJS){checkMailbox()}}", "missing worker bundle anchor"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			engine := filepath.Join(cmd.WebDir, "engine.js")
			writeCommandTestFile(t, engine, tt.content)
			if err := cmd.patchWebWorkerEngine("injected();"); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("invalid engine error = %v, want %q", err, tt.want)
			}
			if got := readTestFile(t, engine); got != tt.content {
				t.Fatalf("failed patch changed engine: %q", got)
			}
		})
	}
}

func TestFinalizeMinigameJS(t *testing.T) {
	for _, mode := range []string{"raw", "compressed"} {
		t.Run(mode, func(t *testing.T) {
			paths := minigamePaths{rawWebDir: t.TempDir(), jsDir: t.TempDir()}
			for name, content := range map[string]string{
				"header.js": "header();", "engine.js": "engine();", "go.wasm.exec.js": "runtime();", "game.js": "game();",
			} {
				writeCommandTestFile(t, filepath.Join(paths.rawWebDir, name), content)
			}
			cmd := &CmdTool{}
			compressed := mode == "compressed"
			if err := cmd.finalizeMinigameJS(paths, compressed); err != nil {
				t.Fatal(err)
			}
			flag := "false"
			if compressed {
				flag = "true"
			}
			want := "globalThis['FFI'] = null;\nconst isWasmCompressed = " + flag + ";\n\nheader();engine();runtime();game();"
			if got := readTestFile(t, filepath.Join(paths.jsDir, "engine.js")); got != want {
				t.Fatalf("merged script = %q, want %q", got, want)
			}
			for _, file := range []string{paths.rawWebDir, filepath.Join(paths.jsDir, "header.js"), filepath.Join(paths.jsDir, "go.wasm.exec.js"), filepath.Join(paths.jsDir, "game.js"), filepath.Join(paths.jsDir, "engine_new.js")} {
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					t.Errorf("temporary input/output remains: %s: %v", file, err)
				}
			}
		})
	}
}

func TestWebAssetMoveAndMergeFailures(t *testing.T) {
	cmd := &CmdTool{}
	source, destination := t.TempDir(), t.TempDir()
	if err := cmd.moveFilesByPattern(source, destination, "["); err == nil {
		t.Fatal("invalid asset pattern accepted")
	}
	writeCommandTestFile(t, filepath.Join(source, "engine.js"), "engine();")
	missing := filepath.Join(destination, "missing")
	if err := cmd.moveFilesByPattern(source, missing, "*.js"); err == nil {
		t.Fatal("asset moved into a missing directory")
	}
	if got := readTestFile(t, filepath.Join(source, "engine.js")); got != "engine();" {
		t.Fatalf("failed move changed source: %q", got)
	}
	if err := cmd.mergeJSFiles(missing, false); err == nil {
		t.Fatal("merge accepted a missing output directory")
	}
	if err := os.Mkdir(filepath.Join(destination, "header.js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmd.mergeJSFiles(destination, false); err == nil {
		t.Fatal("merge accepted a directory in place of a script")
	}
}
