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

func newWebExportFixture(t *testing.T, mode string) *CmdTool {
	t.Helper()
	root := t.TempDir()
	goRoot := filepath.Join(root, "goroot")
	t.Setenv("SPX_WEB_GOROOT", goRoot)
	installCommandTestTools(t, map[string]string{"go": `test "$*" = 'env GOROOT'; printf '%s\n' "$SPX_WEB_GOROOT"`})
	build := "fast"
	cmd := &CmdTool{
		TargetDir: root, TargetAbsDir: root, ProjectDir: filepath.Join(root, "project"),
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

func TestWebExportModeWorkflows(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		export func(*CmdTool) error
	}{
		{webWorkerMode, (*CmdTool).ExportWebWorker},
		{webMinigameMode, (*CmdTool).ExportMinigame},
		{webMiniprogramMode, (*CmdTool).ExportMiniprogram},
	} {
		t.Run(tc.mode, func(t *testing.T) {
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
			archivePath := filepath.Join(cmd.WebDir, "game.zip")
			switch mode {
			case webWorkerMode:
				engine := readTestFile(t, filepath.Join(cmd.WebDir, "engine.js"))
				if !strings.Contains(engine, "handleGameRunnerMessage(e.data)") || !strings.Contains(engine, "runtime();") {
					t.Fatalf("worker engine lacks runtime/message handler: %s", engine)
				}
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "__worker")); !os.IsNotExist(err) {
					t.Fatalf("worker staging directory retained: %v", err)
				}
			case webMinigameMode:
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
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "lab")); !os.IsNotExist(err) {
					t.Fatalf("miniprogram retained debug lab: %v", err)
				}
			}
			archive, err := zip.OpenReader(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			foundSource := false
			for _, file := range archive.File {
				if file.Name == "main.spx" {
					foundSource = true
				}
			}
			if !foundSource {
				t.Fatal("exported archive omits project source")
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
	for _, failure := range []string{"interpreter", "go env", "wasm runtime", "project assets", "interpreter wasm"} {
		t.Run(failure, func(t *testing.T) {
			cmd := newWebExportFixture(t, webNormalMode)
			want := ""
			switch failure {
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
			if err := cmd.ExportWeb(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("export error = %v, want %q", err, want)
			}
		})
	}
}

func TestWebExportFailureCleanup(t *testing.T) {
	for _, scenario := range []string{"worker bundle", "worker patch", "minigame engine", "minigame JS"} {
		t.Run(scenario, func(t *testing.T) {
			mode := webMinigameMode
			if strings.HasPrefix(scenario, "worker") {
				mode = webWorkerMode
			}
			cmd := newWebExportFixture(t, mode)
			template := filepath.Join(cmd.GoBinPath, "gdspxrttest_web"+mode)
			want := ""
			switch scenario {
			case "worker bundle":
				// A directory cannot be replaced by the runtime script during bundling.
				writeCommandTestFile(t, filepath.Join(template, "__worker", "go.wasm.exec.js", "keep"), "original")
				want = "failed to move go.wasm.exec.js"
			case "worker patch":
				writeCommandTestFile(t, filepath.Join(template, "engine.js"), "unrecognized engine")
				want = "missing worker hook anchor"
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
			} else if scenario == "minigame engine" {
				if _, err := os.Stat(filepath.Join(cmd.WebDir, "rawWeb", "game.zip")); err != nil {
					t.Fatalf("failed engine preparation removed raw project archive: %v", err)
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
	if err := os.Symlink("/dev/full", filepath.Join(dir, "engine_new.js")); err != nil {
		t.Fatal(err)
	}
	if err := (&CmdTool{}).mergeJSFiles(dir, false); err == nil || !strings.Contains(err.Error(), "flushing merged JS output") {
		t.Fatalf("full output error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "engine.js")); !os.IsNotExist(err) {
		t.Fatalf("failed merge published engine.js: %v", err)
	}
}
