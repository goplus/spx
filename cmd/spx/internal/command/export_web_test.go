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
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

//go:embed template/project/* template/project/.godot/* template/platform/web/* template/platform/webnormal/* template/platform/webworker/* template/platform/webminigame/game.json template/platform/webminiprogram/*
var webExportTestFS embed.FS

//go:embed template/platform/web/*
var commonWebPlatformFS embed.FS

//go:embed template/platform/web/* template/platform/webminigame/engine
var conflictingMinigamePlatformFS embed.FS

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

func TestExportWebAcceptsSymlinkProjectPath(t *testing.T) {
	for _, pathKind := range []string{"absolute", "relative"} {
		t.Run(pathKind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			before := workingDirectory(t)
			cmd := newWebExportFixture(t, webNormalMode)
			projectRoot := cmd.TargetDir
			wantFiles := map[string]string{
				"main.spx":          "onStart => {}\n",
				"assets/index.json": `{"backdrops":[{"path":"bg.png"}]}`,
				"assets/bg.png":     "background image",
			}
			for name, content := range wantFiles {
				writeCommandTestFile(t, filepath.Join(projectRoot, name), content)
			}
			projectLink := filepath.Join(before, "linked-project")
			if err := os.Symlink(projectRoot, projectLink); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			projectPath := projectLink
			if pathKind == "relative" {
				projectPath = filepath.Base(projectLink)
			}
			t.Setenv("GOPATH", filepath.Dir(cmd.GoBinPath))
			t.Setenv("GODEBUG", os.Getenv("GODEBUG"))
			setTestCommandLine(t, "spx", "exportweb", "--path", projectPath)

			err := cmd.RunCmd("spx", ".spx", "test", webExportTestFS, "template/project", "project")
			assertWorkingDirectory(t, before)
			if err != nil {
				t.Fatalf("exportweb through project symlink: %v", err)
			}
			if cmd.TargetDir != projectRoot || *cmd.Args.Path != projectRoot {
				t.Fatalf("source paths = %q, %q; want real root %q", cmd.TargetDir, *cmd.Args.Path, projectRoot)
			}
			projectDir := filepath.Join(projectRoot, "project")
			webDir := filepath.Join(projectDir, ".builds", "web")
			if cmd.ProjectDir != projectDir || cmd.WebDir != webDir {
				t.Fatalf("generated paths = %q, %q; want %q, %q", cmd.ProjectDir, cmd.WebDir, projectDir, webDir)
			}

			archive, err := zip.OpenReader(filepath.Join(webDir, "game.zip"))
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			for name, want := range wantFiles {
				entry, err := archive.Open(name)
				if err != nil {
					t.Fatalf("open game.zip entry %s: %v", name, err)
				}
				got, readErr := io.ReadAll(entry)
				closeErr := entry.Close()
				if readErr != nil || closeErr != nil {
					t.Fatalf("read game.zip entry %s: %v", name, errors.Join(readErr, closeErr))
				}
				if string(got) != want {
					t.Fatalf("game.zip entry %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestWebLogicAssetsUseInstalledInterpreter(t *testing.T) {
	for _, mode := range []string{webNormalMode, webWorkerMode, webMinigameMode, webMiniprogramMode} {
		for _, sibling := range []string{"absent", "stale", "invalid"} {
			t.Run(mode+"/"+sibling, func(t *testing.T) {
				cmd := CmdTool{GoBinPath: t.TempDir(), WebDir: t.TempDir()}
				wasmPath := filepath.Join(cmd.GoBinPath, "ispx.wasm")
				writeCommandTestFile(t, wasmPath, "interpreter-wasm")
				switch sibling {
				case "stale":
					writeCommandTestFile(t, wasmPath+".br", "stale-installed-compression")
				case "invalid":
					if err := os.Mkdir(wasmPath+".br", 0o755); err != nil {
						t.Fatal(err)
					}
				}
				writeCommandTestFile(t, filepath.Join(cmd.WebDir, "ispx.wasm"), "project-compiled-wasm")
				compressedPath := filepath.Join(cmd.WebDir, "ispx.wasm.br")
				writeCommandTestFile(t, compressedPath, "stale-export-compression")

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
			writeCommandTestFile(t, filepath.Join(cmd.GoBinPath, "ispx.wasm"), "ispx.wasm")
			for _, name := range []string{"engine.wasm", "game.zip"} {
				writeCommandTestFile(t, filepath.Join(paths.rawWebDir, name), name)
				if tt.staleCompression && strings.HasSuffix(name, ".wasm") {
					writeCommandTestFile(t, filepath.Join(paths.rawWebDir, name+".br"), "stale")
				}
			}
			if tt.staleCompression {
				writeCommandTestFile(t, filepath.Join(cmd.GoBinPath, "ispx.wasm.br"), "stale-installed-compression")
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
	root, destination := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "game[one]")
	if err := cmd.moveFilesByPattern(source, destination, "["); err == nil {
		t.Fatal("invalid asset pattern accepted")
	}
	writeCommandTestFile(t, filepath.Join(source, "engine.js"), "engine();")
	neighbor := filepath.Join(root, "gameo", "engine.js")
	writeCommandTestFile(t, neighbor, "unrelated();")
	if err := cmd.moveFilesByPattern(source, destination, "engine*["); !errors.Is(err, filepath.ErrBadPattern) {
		t.Fatalf("invalid pattern after a matching prefix = %v, want ErrBadPattern", err)
	}
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
	if err := cmd.moveFilesByPattern(source, destination, "*.js"); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(destination, "engine.js")); got != "engine();" {
		t.Fatalf("literal source directory moved %q, want its own engine script", got)
	}
	if got := readTestFile(t, neighbor); got != "unrelated();" {
		t.Fatalf("asset move changed neighboring project: %q", got)
	}
	if _, err := os.Stat(filepath.Join(source, "engine.js")); !os.IsNotExist(err) {
		t.Fatalf("source engine was not moved: %v", err)
	}
	if err := cmd.moveFilesByPattern(filepath.Join(root, "missing"), destination, "*.js"); err != nil {
		t.Fatalf("missing source directory should have no matches: %v", err)
	}
	t.Run("unreadable source", func(t *testing.T) {
		restrictCommandTestPath(t, source, 0)
		if err := cmd.moveFilesByPattern(source, destination, "*.js"); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("source listing error = %v, want permission denied", err)
		}
	})
}

func TestWeChatDevToolsUsesTargetProjectDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake DevTools command uses a POSIX shell script")
	}
	for _, pathKind := range []string{"relative", "absolute", "unset"} {
		t.Run(pathKind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			before := workingDirectory(t)
			projectDir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			toolsDir := filepath.Join(projectDir, "dev tools")
			cli := filepath.Join(toolsDir, "cli")
			writeCommandTestFile(t, cli, "#!/bin/sh\nprintf '%s\\000' \"$PWD\" \"$@\" > \"$SPX_DEVTOOLS_LOG\"\n")
			if err := os.Chmod(cli, 0o755); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(projectDir, "devtools.log")
			t.Setenv("SPX_DEVTOOLS_LOG", logPath)
			toolPath := toolsDir
			if pathKind == "relative" {
				toolPath = filepath.Base(toolsDir)
			} else if pathKind == "unset" {
				toolPath = ""
			}
			t.Setenv("WECHAT_DEV_TOOLS", toolPath)
			cmd := CmdTool{TargetDir: projectDir}
			workDir := filepath.Join(projectDir, "project", ".builds", "web")
			cmd.openWeChatDevTools(workDir)
			assertWorkingDirectory(t, before)
			if pathKind == "unset" {
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Fatalf("unset DevTools path launched a command: %v", err)
				}
				return
			}
			want := projectDir + "\x00open\x00--project\x00" + workDir + "\x00-y\x00"
			if got := readTestFile(t, logPath); got != want {
				t.Fatalf("DevTools invocation = %q, want %q", got, want)
			}
		})
	}
}

func TestWebExportModeWorkflows(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		export func(*CmdTool) error
	}{
		{webNormalMode, (*CmdTool).ExportWeb},
		{webWorkerMode, (*CmdTool).ExportWebWorker},
		{webMinigameMode, (*CmdTool).ExportMinigame},
		{webMiniprogramMode, (*CmdTool).ExportMiniprogram},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Chdir(t.TempDir())
			before := workingDirectory(t)
			mode := tc.mode
			cmd := newWebExportFixture(t, mode)
			devToolsLog := filepath.Join(t.TempDir(), "devtools.log")
			t.Setenv("SPX_DEVTOOLS_LOG", devToolsLog)
			tools := t.TempDir()
			writeCommandTestFile(t, filepath.Join(tools, "cli"), "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SPX_DEVTOOLS_LOG\"\n")
			if err := os.Chmod(filepath.Join(tools, "cli"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("WECHAT_DEV_TOOLS", tools)
			err := tc.export(cmd)
			if err != nil {
				t.Fatal(err)
			}
			assertWorkingDirectory(t, before)
			archivePath := filepath.Join(cmd.WebDir, "game.zip")
			switch mode {
			case webWorkerMode:
				engine := readTestFile(t, filepath.Join(cmd.WebDir, "engine.js"))
				if !strings.Contains(engine, "handleGameRunnerMessage(e.data)") || !strings.Contains(engine, "runtime();") || !strings.Contains(engine, "workerOverlay();") {
					t.Fatalf("worker engine lacks runtime/message handler: %s", engine)
				}
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "__worker")); !os.IsNotExist(err) {
					t.Fatalf("worker staging directory retained: %v", err)
				}
			case webMinigameMode:
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "game.json")); got != "{\"deviceOrientation\":\"landscape\"}\n" {
					t.Fatalf("minigame overlay = %q", got)
				}
				archivePath = filepath.Join(cmd.WebDir, "engine", "game.zip")
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "engine", "engine.wasm")); got != "engine wasm" {
					t.Fatalf("engine wasm = %q", got)
				}
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "engine", "ispx.wasm")); got != "interpreter wasm" {
					t.Fatalf("interpreter wasm = %q", got)
				}
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "js", "engine.js")); !strings.Contains(got, "isWasmCompressed = false") || !strings.HasSuffix(got, "runtime();game();") {
					t.Fatalf("merged minigame JS = %q", got)
				}
				if got := readTestFile(t, devToolsLog); got != "open\n--project\n"+cmd.WebDir+"\n-y\n" {
					t.Fatalf("DevTools arguments = %q", got)
				}
				for _, dir := range []string{cmd.WebDir + "_bck", filepath.Join(cmd.WebDir, "rawWeb")} {
					if _, err := os.Stat(dir); !os.IsNotExist(err) {
						t.Fatalf("temporary minigame assets retained: %s: %v", dir, err)
					}
				}
			case webMiniprogramMode:
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "mini.txt")); got != "miniprogram overlay\n" {
					t.Fatalf("miniprogram overlay = %q", got)
				}
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "lab")); !os.IsNotExist(err) {
					t.Fatalf("miniprogram retained debug lab: %v", err)
				}
			}
			archive, err := zip.OpenReader(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			for name, want := range map[string]string{"main.spx": "onStart => {}\n", "assets/index.json": "{}"} {
				if data, err := fs.ReadFile(archive, name); err != nil || string(data) != want {
					t.Fatalf("exported archive entry %s = %q, %v; want %q", name, data, err, want)
				}
			}
		})
	}
}

func TestWebExportRejectsMissingModeOverlay(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		export func(*CmdTool) error
	}{
		{webNormalMode, (*CmdTool).ExportWeb},
		{webWorkerMode, (*CmdTool).ExportWebWorker},
		{webMinigameMode, (*CmdTool).ExportMinigame},
		{webMiniprogramMode, (*CmdTool).ExportMiniprogram},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			cmd := newWebExportFixture(t, tc.mode)
			cmd.PlatformFS = commonWebPlatformFS
			t.Setenv("WECHAT_DEV_TOOLS", "")
			err := tc.export(cmd)
			if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "template/platform/web"+tc.mode) {
				t.Fatalf("missing mode overlay = %v, want original missing-source error", err)
			}
			switch tc.mode {
			case webWorkerMode:
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "__worker")); !os.IsNotExist(err) {
					t.Fatalf("failed overlay retained worker staging: %v", err)
				}
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "go.wasm.exec.js")); got != "runtime();" {
					t.Fatalf("failed overlay changed worker runtime: %q", got)
				}
			case webMinigameMode:
				if got := readTestFile(t, filepath.Join(cmd.WebDir+"_bck", "engine.wasm")); got != "engine wasm" {
					t.Fatalf("failed overlay changed raw engine: %q", got)
				}
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "engine")); !os.IsNotExist(err) {
					t.Fatalf("failed overlay continued preparing minigame engine: %v", err)
				}
			case webMiniprogramMode:
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "lab", "debug.txt")); got != "debug only" {
					t.Fatalf("failed overlay continued removing lab assets: %q", got)
				}
			}
			cmd.PlatformFS = webExportTestFS
			if err := tc.export(cmd); err != nil {
				t.Fatalf("retry with complete overlay: %v", err)
			}
			archivePath := filepath.Join(cmd.WebDir, "game.zip")
			if tc.mode == webMinigameMode {
				archivePath = filepath.Join(cmd.WebDir, "engine", "game.zip")
				if _, err := os.Stat(cmd.WebDir + "_bck"); !os.IsNotExist(err) {
					t.Fatalf("successful retry retained backup: %v", err)
				}
			}
			archive, err := zip.OpenReader(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			if data, err := fs.ReadFile(archive, "main.spx"); err != nil || string(data) != "onStart => {}\n" {
				t.Fatalf("retried project archive = %q, %v", data, err)
			}
		})
	}
}

func TestWebExportMissingPrerequisites(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		export func(*CmdTool) error
	}{
		{webNormalMode, (*CmdTool).ExportWeb},
		{webWorkerMode, (*CmdTool).ExportWebWorker},
		{webMinigameMode, (*CmdTool).ExportMinigame},
		{webMiniprogramMode, (*CmdTool).ExportMiniprogram},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			mode := tc.mode
			cmd := newWebExportFixture(t, mode)
			if err := os.RemoveAll(filepath.Join(cmd.GoBinPath, "gdspxrttest_web"+mode)); err != nil {
				t.Fatal(err)
			}
			err := tc.export(cmd)
			if err == nil || !strings.Contains(err.Error(), "web dir file not found") {
				t.Fatalf("missing runtime error = %v", err)
			}
			if _, err := os.Stat(cmd.WebDir); !os.IsNotExist(err) {
				t.Fatalf("missing runtime produced an export: %v", err)
			}
		})
	}
	for _, failure := range []string{"protected generated directory", "interpreter", "go env", "wasm runtime", "project assets", "interpreter wasm"} {
		t.Run(failure, func(t *testing.T) {
			cmd := newWebExportFixture(t, webNormalMode)
			want := ""
			switch failure {
			case "protected generated directory":
				writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, "keep"), "original export")
				restrictCommandTestPath(t, cmd.ProjectDir, 0o500)
				want = "failed to remove project directory"
			case "interpreter":
				if err := os.RemoveAll(filepath.Join(cmd.GoBinPath, "ispx")); err != nil {
					t.Fatal(err)
				}
				want = "ispx web runtime not found"
			case "go env":
				installCommandTestTools(t, map[string]string{"go": "exit 17"})
				want = "failed to get GOROOT"
			case "wasm runtime":
				if err := os.Remove(filepath.Join(os.Getenv("SPX_WEB_GOROOT"), "lib", "wasm", "wasm_exec.js")); err != nil {
					t.Fatal(err)
				}
				want = "failed to copy wasm_exec.js"
			case "project assets":
				if err := os.Remove(filepath.Join(cmd.TargetDir, "assets", "index.json")); err != nil {
					t.Fatal(err)
				}
				want = "contains neither"
			case "interpreter wasm":
				if err := os.Remove(filepath.Join(cmd.GoBinPath, "ispx.wasm")); err != nil {
					t.Fatal(err)
				}
				want = "failed to copy interpreter wasm"
			}
			err := cmd.ExportWeb()
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("export error = %v, want %q", err, want)
			}
			if failure == "protected generated directory" {
				if !errors.Is(err, os.ErrPermission) {
					t.Fatalf("protected export error = %v, want permission denied", err)
				}
				if got := readTestFile(t, filepath.Join(cmd.ProjectDir, "keep")); got != "original export" {
					t.Fatalf("failed cleanup changed existing export: %q", got)
				}
			}
		})
	}
}

func TestWebExportCommonFailuresStopBeforeBuildAndPack(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		cause      error
		prepare    func(*testing.T, *CmdTool, string)
	}{
		{"export directory", "failed to create web export directory", os.ErrPermission, func(t *testing.T, cmd *CmdTool, template string) {
			parent := t.TempDir()
			cmd.ProjectDir = filepath.Join(parent, "project")
			cmd.WebDir = filepath.Join(cmd.ProjectDir, ".builds", "web")
			restrictCommandTestPath(t, parent, 0o500)
		}},
		{"runtime directory is file", "failed to copy web runtime", nil, func(t *testing.T, cmd *CmdTool, template string) {
			if err := os.RemoveAll(template); err != nil {
				t.Fatal(err)
			}
			writeCommandTestFile(t, template, "not a directory")
		}},
		{"project template", "failed to copy web project template", fs.ErrNotExist, func(t *testing.T, cmd *CmdTool, template string) {
			cmd.ProjectFS = embed.FS{}
		}},
		{"missing HTML", "failed to rename web entry page", fs.ErrNotExist, func(t *testing.T, cmd *CmdTool, template string) {
			if err := os.Remove(filepath.Join(template, "godot.editor.html")); err != nil {
				t.Fatal(err)
			}
		}},
		{"HTML destination is directory", "failed to rename web entry page", nil, func(t *testing.T, cmd *CmdTool, template string) {
			writeCommandTestFile(t, filepath.Join(template, "index.html", "keep"), "original")
		}},
		{"interpreter destination conflict", "failed to copy interpreter web runtime", nil, func(t *testing.T, cmd *CmdTool, template string) {
			writeCommandTestFile(t, filepath.Join(cmd.GoBinPath, "ispx", "engine.js", "keep"), "conflict")
		}},
		{"common overlay", "failed to copy web overlay template/platform/web", fs.ErrNotExist, func(t *testing.T, cmd *CmdTool, template string) {
			cmd.PlatformFS = embed.FS{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newWebExportFixture(t, webNormalMode)
			goLog := filepath.Join(t.TempDir(), "go.log")
			t.Setenv("SPX_WEB_GO_LOG", goLog)
			installCommandTestTools(t, map[string]string{"go": `printf invoked > "$SPX_WEB_GO_LOG"; exit 37`})
			template := filepath.Join(cmd.GoBinPath, "gdspxrttest_webnormal")
			tc.prepare(t, cmd, template)
			err := cmd.ExportWeb()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("export error = %v, want %q", err, tc.want)
			}
			if tc.cause != nil {
				if !errors.Is(err, tc.cause) {
					t.Fatalf("export lost original cause %v: %v", tc.cause, err)
				}
			} else {
				var pathErr *os.PathError
				var linkErr *os.LinkError
				if !errors.As(err, &pathErr) && !errors.As(err, &linkErr) {
					t.Fatalf("export lost filesystem error: %v", err)
				}
			}
			for _, path := range []string{goLog, filepath.Join(cmd.WebDir, "game.zip"), filepath.Join(cmd.WebDir, "ispx.wasm")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("failed preparation reached later export step %s: %v", path, err)
				}
			}
			if tc.name == "HTML destination is directory" {
				if got := readTestFile(t, filepath.Join(cmd.WebDir, "index.html", "keep")); got != "original" {
					t.Fatalf("failed rename changed destination: %q", got)
				}
			}
		})
	}
}

func TestWebExportFailureCleanup(t *testing.T) {
	for _, scenario := range []string{"worker overlay", "worker bundle", "worker patch", "minigame overlay", "minigame workspace", "minigame engine", "minigame JS"} {
		t.Run(scenario, func(t *testing.T) {
			mode := webMinigameMode
			if strings.HasPrefix(scenario, "worker") {
				mode = webWorkerMode
			}
			cmd := newWebExportFixture(t, mode)
			template := filepath.Join(cmd.GoBinPath, "gdspxrttest_web"+mode)
			want := ""
			switch scenario {
			case "worker overlay":
				writeCommandTestFile(t, filepath.Join(template, "__worker", "worker.js", "keep"), "conflicting directory")
				want = "failed to copy web overlay"
			case "worker bundle":
				// A directory cannot be replaced by the runtime script during bundling.
				writeCommandTestFile(t, filepath.Join(template, "__worker", "go.wasm.exec.js", "keep"), "original")
				want = "failed to move go.wasm.exec.js"
			case "worker patch":
				writeCommandTestFile(t, filepath.Join(template, "engine.js"), "unrecognized engine")
				want = "missing worker hook anchor"
			case "minigame workspace":
				cmd.PlatformFS = conflictingMinigamePlatformFS
				want = "failed to create " + filepath.Join(cmd.WebDir, "engine")
			case "minigame overlay":
				cmd.PlatformFS = commonWebPlatformFS
				want = "failed to copy web overlay"
			case "minigame engine":
				if err := os.Remove(filepath.Join(template, "engine.wasm")); err != nil {
					t.Fatal(err)
				}
				want = "failed to move "
			case "minigame JS":
				writeCommandTestFile(t, filepath.Join(template, "header.js", "keep"), "invalid script directory")
				want = "failed to merge JS files"
			}
			var err error
			if mode == webWorkerMode {
				err = cmd.ExportWebWorker()
			} else {
				err = cmd.ExportMinigame()
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("export error = %v, want %q", err, want)
			}
			if mode == webWorkerMode {
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "__worker")); !os.IsNotExist(err) {
					t.Fatalf("failed worker export retained staging directory: %v", err)
				}
				if scenario == "worker patch" && readTestFile(t, filepath.Join(cmd.WebDir, "engine.js")) != "unrecognized engine" {
					t.Fatal("failed patch changed engine source")
				}
				if scenario == "worker overlay" {
					var pathErr *os.PathError
					if !errors.As(err, &pathErr) {
						t.Fatalf("worker cleanup lost the copy error: %v", err)
					}
					if got := readTestFile(t, filepath.Join(cmd.WebDir, "engine.js")); got != readTestFile(t, filepath.Join(template, "engine.js")) {
						t.Fatalf("failed overlay changed engine source: %q", got)
					}
					if got := readTestFile(t, filepath.Join(cmd.WebDir, "go.wasm.exec.js")); got != "runtime();" {
						t.Fatalf("failed overlay moved worker runtime: %q", got)
					}
				}
			} else if scenario == "minigame workspace" || scenario == "minigame overlay" {
				backup := cmd.WebDir + "_bck"
				if got := readTestFile(t, filepath.Join(backup, "engine.wasm")); got != "engine wasm" {
					t.Fatalf("failed workspace preparation changed raw engine: %q", got)
				}
				if len(readTestFile(t, filepath.Join(backup, "game.zip"))) == 0 {
					t.Fatal("failed workspace preparation lost the project archive")
				}
				if scenario == "minigame workspace" {
					if got := readTestFile(t, filepath.Join(cmd.WebDir, "engine")); got != "conflicting engine file\n" {
						t.Fatalf("failed workspace preparation overwrote conflicting template: %q", got)
					}
				}
			} else if scenario == "minigame engine" {
				backup := cmd.WebDir + "_bck"
				if len(readTestFile(t, filepath.Join(backup, "game.zip"))) == 0 {
					t.Fatal("failed engine preparation lost the project archive")
				}
				if got := readTestFile(t, filepath.Join(backup, "game.js")); got != "game();" {
					t.Fatalf("failed engine preparation changed raw JS: %q", got)
				}
				writeCommandTestFile(t, filepath.Join(template, "engine.wasm"), "engine wasm")
				t.Setenv("WECHAT_DEV_TOOLS", "")
				if err := cmd.ExportMinigame(); err != nil {
					t.Fatalf("retry after restoring missing engine: %v", err)
				}
				archive, err := zip.OpenReader(filepath.Join(cmd.WebDir, "engine", "game.zip"))
				if err != nil {
					t.Fatal(err)
				}
				defer archive.Close()
				for name, want := range map[string]string{"main.spx": "onStart => {}\n", "assets/index.json": "{}"} {
					if data, err := fs.ReadFile(archive, name); err != nil || string(data) != want {
						t.Fatalf("retried archive entry %s = %q, %v; want %q", name, data, err, want)
					}
				}
				if _, err := os.Stat(backup); !os.IsNotExist(err) {
					t.Fatalf("successful retry retained raw backup: %v", err)
				}
			}
		})
	}
}

func TestMinigameEngineAssetsRejectIncompletePublication(t *testing.T) {
	for _, scenario := range []string{"missing engine", "missing interpreter", "missing brotli", "compressed destination", "archive destination"} {
		t.Run(scenario, func(t *testing.T) {
			paths := minigamePaths{rawWebDir: t.TempDir(), engineDir: t.TempDir()}
			for _, name := range []string{"engine.wasm", "ispx.wasm", "game.zip"} {
				writeCommandTestFile(t, filepath.Join(paths.rawWebDir, name), name)
			}
			build, want := "fast", "failed to move "
			switch scenario {
			case "missing engine", "missing interpreter":
				name := "engine.wasm"
				if scenario == "missing interpreter" {
					name = "ispx.wasm"
				}
				if err := os.Remove(filepath.Join(paths.rawWebDir, name)); err != nil {
					t.Fatal(err)
				}
				want += filepath.Join(paths.rawWebDir, name)
			case "missing brotli":
				t.Setenv("PATH", t.TempDir())
				build, want = "normal", "brotli is not installed"
			case "compressed destination":
				installCommandTestTools(t, map[string]string{"brotli": `printf compressed > "$4.br"`})
				if err := os.Mkdir(filepath.Join(paths.engineDir, "engine.wasm.br"), 0o755); err != nil {
					t.Fatal(err)
				}
				build, want = "normal", "failed to move br files"
			case "archive destination":
				if err := os.Mkdir(filepath.Join(paths.engineDir, "game.zip"), 0o755); err != nil {
					t.Fatal(err)
				}
				want = "failed to move zip files"
			}
			if err := (&CmdTool{}).prepareMinigameEngineAssets(paths, build); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("asset preparation error = %v, want %q", err, want)
			}
			if got := readTestFile(t, filepath.Join(paths.rawWebDir, "game.zip")); got != "game.zip" {
				t.Fatalf("failed preparation changed raw archive: %q", got)
			}
		})
	}
}

func TestWebLogicAssetsPreservesBlockedStaleCompression(t *testing.T) {
	cmd := &CmdTool{GoBinPath: t.TempDir(), WebDir: t.TempDir()}
	writeCommandTestFile(t, filepath.Join(cmd.GoBinPath, "ispx.wasm"), "interpreter")
	blocked := filepath.Join(cmd.WebDir, "ispx.wasm.br", "keep")
	writeCommandTestFile(t, blocked, "unrelated content")
	if err := cmd.writeWebLogicAssets(webMinigameMode); err == nil || !strings.Contains(err.Error(), "failed to remove stale compressed wasm") {
		t.Fatalf("stale compression error = %v", err)
	}
	if got := readTestFile(t, blocked); got != "unrelated content" {
		t.Fatalf("stale compression cleanup removed unrelated content: %q", got)
	}
}

func TestMinigameWorkspacePreservesExistingBackup(t *testing.T) {
	cmd := &CmdTool{WebDir: filepath.Join(t.TempDir(), "web")}
	for _, dir := range []string{cmd.WebDir, cmd.WebDir + "_bck"} {
		writeCommandTestFile(t, filepath.Join(dir, "keep"), dir)
	}
	if _, err := cmd.prepareMinigameWorkspace(); err == nil || !strings.Contains(err.Error(), "failed to backup web directory") {
		t.Fatalf("backup collision error = %v", err)
	}
	for _, dir := range []string{cmd.WebDir, cmd.WebDir + "_bck"} {
		if got := readTestFile(t, filepath.Join(dir, "keep")); got != dir {
			t.Fatalf("failed backup changed %s: %q", dir, got)
		}
	}
}

func TestWebWorkerReportsProtectedInputs(t *testing.T) {
	for _, scenario := range []string{"bundle directory", "bundle script", "engine output"} {
		t.Run(scenario, func(t *testing.T) {
			cmd := &CmdTool{WebDir: t.TempDir()}
			ext := filepath.Join(cmd.WebDir, "worker")
			writeCommandTestFile(t, filepath.Join(cmd.WebDir, "go.wasm.exec.js"), "runtime();")
			writeCommandTestFile(t, filepath.Join(ext, "worker.js"), "worker();")
			engine := filepath.Join(cmd.WebDir, "engine.js")
			const original = "{if(initializedJS){checkMailbox()}};throw ex}}self.onmessage=handleMessage}"
			writeCommandTestFile(t, engine, original)
			var err error
			switch scenario {
			case "bundle directory":
				// Moving the runtime is allowed, but listing the directory is not.
				restrictCommandTestPath(t, ext, 0o300)
				_, err = cmd.readWebWorkerBundle(ext)
			case "bundle script":
				restrictCommandTestPath(t, filepath.Join(ext, "worker.js"), 0)
				_, err = cmd.readWebWorkerBundle(ext)
			case "engine output":
				restrictCommandTestPath(t, engine, 0o400)
				err = cmd.patchWebWorkerEngine("worker();")
				if got := readTestFile(t, engine); got != original {
					t.Fatalf("failed engine write changed source: %q", got)
				}
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("protected input error = %v, want permission denied", err)
			}
		})
	}
}

func TestMinigameJSReportsFilesystemFailures(t *testing.T) {
	for _, scenario := range []string{"move script", "open script", "remove script", "remove raw assets"} {
		t.Run(scenario, func(t *testing.T) {
			paths := minigamePaths{rawWebDir: t.TempDir(), jsDir: t.TempDir()}
			cmd := &CmdTool{}
			var err error
			switch scenario {
			case "move script":
				writeCommandTestFile(t, filepath.Join(paths.rawWebDir, "engine.js"), "engine();")
				restrictCommandTestPath(t, paths.jsDir, 0o500)
				err = cmd.finalizeMinigameJS(paths, false)
			case "open script":
				file := filepath.Join(paths.jsDir, "header.js")
				writeCommandTestFile(t, file, "header();")
				restrictCommandTestPath(t, file, 0)
				err = cmd.mergeJSFiles(paths.jsDir, false)
			case "remove script":
				writeCommandTestFile(t, filepath.Join(paths.jsDir, "header.js"), "header();")
				writeCommandTestFile(t, filepath.Join(paths.jsDir, "engine_new.js"), "")
				restrictCommandTestPath(t, paths.jsDir, 0o500)
				err = cmd.mergeJSFiles(paths.jsDir, false)
			case "remove raw assets":
				writeCommandTestFile(t, filepath.Join(paths.rawWebDir, "keep.txt"), "raw asset")
				restrictCommandTestPath(t, paths.rawWebDir, 0o500)
				err = cmd.finalizeMinigameJS(paths, false)
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("JS preparation error = %v, want permission denied", err)
			}
		})
	}
}

func TestMergeJSFilesReportsFullOutputDevice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux /dev/full")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skipf("/dev/full unavailable: %v", err)
	}
	dir := t.TempDir()
	sources := map[string]string{"header.js": "header();", "engine.js": "engine();", "game.js": "game();"}
	for name, content := range sources {
		writeCommandTestFile(t, filepath.Join(dir, name), content)
	}
	output := filepath.Join(dir, "engine_new.js")
	if err := os.Symlink("/dev/full", output); err != nil {
		t.Fatal(err)
	}
	if err := (&CmdTool{}).mergeJSFiles(dir, false); err == nil || !strings.Contains(err.Error(), "write merged JS output "+output) {
		t.Fatalf("full output error = %v", err)
	}
	for name, want := range sources {
		if got := readTestFile(t, filepath.Join(dir, name)); got != want {
			t.Fatalf("failed merge changed source %s: %q, want %q", name, got, want)
		}
	}
}

func newWebExportFixture(t *testing.T, mode string) *CmdTool {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "game[one]")
	goRoot := filepath.Join(root, "goroot")
	t.Setenv("SPX_WEB_GOROOT", goRoot)
	t.Setenv("SPX_WEB_PROJECT_DIR", root)
	installCommandTestTools(t, map[string]string{"go": `test "$*" = 'env GOROOT'; test "$(pwd -P)" = "$SPX_WEB_PROJECT_DIR"; printf '%s\n' "$SPX_WEB_GOROOT"`})
	build := "fast"
	cmd := &CmdTool{
		TargetDir: root, ProjectDir: filepath.Join(root, "project"),
		WebDir: filepath.Join(root, "project", ".builds", "web"), GoBinPath: filepath.Join(root, "bin"), Version: "test",
		ProjectFS: webExportTestFS, PlatformFS: webExportTestFS, Args: ExtraArgs{Build: &build},
	}
	templateDir := filepath.Join(cmd.GoBinPath, "gdspxrttest_web"+mode)
	for name, content := range map[string]string{
		filepath.Join(root, "main.spx"):                          "onStart => {}\n",
		filepath.Join(root, "assets", "index.json"):              "{}",
		filepath.Join(templateDir, "godot.editor.html"):          "web entry",
		filepath.Join(templateDir, "engine.wasm"):                "engine wasm",
		filepath.Join(templateDir, "engine.js"):                  "{if(initializedJS){checkMailbox()}};throw ex}}self.onmessage=handleMessage}",
		filepath.Join(cmd.GoBinPath, "ispx", "game.js"):          "game();",
		filepath.Join(cmd.GoBinPath, "ispx", "lab", "debug.txt"): "debug only",
		filepath.Join(cmd.GoBinPath, "ispx.wasm"):                "interpreter wasm",
		filepath.Join(goRoot, "lib", "wasm", "wasm_exec.js"):     "runtime();",
	} {
		writeCommandTestFile(t, name, content)
	}
	return cmd
}
