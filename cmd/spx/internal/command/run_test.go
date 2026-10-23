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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/goplus/spx/v4/internal/interpruntime"
	"github.com/goplus/spx/v4/internal/scaffold"
)

func TestRunWebCommandOrderAndErrors(t *testing.T) {
	failure := errors.New("step failed")
	for _, tt := range []struct {
		name, failAt string
		want         []string
	}{
		{"setup before export and server", "", []string{"setup", "export", "serve"}},
		{"setup failure", "setup", []string{"setup"}},
		{"export failure", "export", []string{"setup", "export"}},
		{"server failure", "serve", []string{"setup", "export", "serve"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			step := func(name string) func() error {
				return func() error {
					calls = append(calls, name)
					if name == tt.failAt {
						return failure
					}
					return nil
				}
			}
			err := runWebCommand(step("setup"), step("export"), step("serve"))
			if tt.failAt == "" && err != nil || tt.failAt != "" && !errors.Is(err, failure) {
				t.Fatalf("run error = %v, failing step %q", err, tt.failAt)
			}
			if !reflect.DeepEqual(calls, tt.want) {
				t.Fatalf("call order = %v, want %v", calls, tt.want)
			}
		})
	}
}

func TestHasInstalledWebRuntimeAssetsIsModeSpecific(t *testing.T) {
	goBinPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goBinPath, "ispx"), 0755); err != nil {
		t.Fatalf("mkdir ispx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinPath, "ispx.wasm"), []byte("wasm"), 0644); err != nil {
		t.Fatalf("write ispx.wasm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(goBinPath, "gdspxrt2.0.0_webnormal"), 0755); err != nil {
		t.Fatalf("mkdir webnormal template dir: %v", err)
	}

	cmd := CmdTool{GoBinPath: goBinPath, Version: "2.0.0"}
	if !cmd.hasInstalledWebRuntimeAssets(webNormalMode) {
		t.Fatal("hasInstalledWebRuntimeAssets(normal) = false, want true")
	}
	if cmd.hasInstalledWebRuntimeAssets(webWorkerMode) {
		t.Fatal("hasInstalledWebRuntimeAssets(worker) = true, want false")
	}
}

func TestCommandRuntimeMode(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{name: "run interpreted", cmdName: "run", want: false},
		{name: "runnative", cmdName: "runnative", want: true},
		{name: "runweb", cmdName: "runweb", want: true},
		{name: "runwebworker", cmdName: "runwebworker", want: true},
		{name: "exportweb", cmdName: "exportweb", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := findCommand(tt.cmdName)
			if !ok {
				t.Fatalf("command %q not found", tt.cmdName)
			}
			if spec.runtime != tt.want {
				t.Fatalf("command %q runtime = %v, want %v", tt.cmdName, spec.runtime, tt.want)
			}
		})
	}
}

func TestWebCommandSetup(t *testing.T) {
	tests := []struct {
		name, cmdName string
		setup         commandSetup
		build         commandBuild
	}{
		{name: "buildweb", cmdName: "buildweb", setup: projectSetup, build: wasmBuild},
		{name: "exportweb", cmdName: "exportweb", setup: webExportSetup},
		{name: "runweb", cmdName: "runweb", setup: webExportSetup},
		{name: "runwebworker", cmdName: "runwebworker", setup: webExportSetup},
		{name: "exportwebworker", cmdName: "exportwebworker", setup: webExportSetup},
		{name: "exportminigame", cmdName: "exportminigame", setup: webExportSetup},
		{name: "exportminiprogram", cmdName: "exportminiprogram", setup: webExportSetup},
		{name: "exporttemplateweb", cmdName: "exporttemplateweb", setup: projectSetup},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := findCommand(tt.cmdName)
			if !ok {
				t.Fatalf("command %q not found", tt.cmdName)
			}
			if spec.setup != tt.setup || spec.build != tt.build {
				t.Fatalf("command %q setup/build = %v/%v, want %v/%v", tt.cmdName, spec.setup, spec.build, tt.setup, tt.build)
			}
		})
	}
}

func TestBrowserOpenCommands(t *testing.T) {
	const url = "http://127.0.0.1:8060"
	for _, tt := range []struct {
		goos string
		want []browserOpenCommand
	}{
		{"darwin", []browserOpenCommand{{name: "open", args: []string{"-a", "Google Chrome", url}}, {name: "open", args: []string{url}}}},
		{"linux", []browserOpenCommand{{name: "xdg-open", args: []string{url}}}},
		{"windows", []browserOpenCommand{{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}}}},
	} {
		t.Run(tt.goos, func(t *testing.T) {
			if got := browserOpenCommands(tt.goos, url); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("browser commands = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSetupInterpretedPathsKeepsWorkingDirectory(t *testing.T) {
	before := workingDirectory(t)
	projectDir := t.TempDir()
	pathArg := projectDir
	cmd := CmdTool{Args: ExtraArgs{Path: &pathArg}}

	if err := cmd.setupInterpretedPaths("project"); err != nil {
		t.Fatal(err)
	}
	assertWorkingDirectory(t, before)
	if cmd.TargetAbsDir != projectDir || cmd.TargetDir != projectDir {
		t.Fatalf("project root = (%q, %q), want %q", cmd.TargetAbsDir, cmd.TargetDir, projectDir)
	}
	if cmd.ProjectDir != filepath.Join(projectDir, "project") {
		t.Fatalf("Engine project dir = %q", cmd.ProjectDir)
	}
}

func TestRunPureEngineBuildsWithTagsAndRunsFromTargetDir(t *testing.T) {
	goDir := t.TempDir()
	targetDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "pure-engine.log")
	writePureEngineTestProject(t, goDir)

	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOPROXY", "off")
	tags := "pure_test_tag"
	cmd := CmdTool{
		GoDir: goDir, TargetDir: targetDir,
		Args: ExtraArgs{Tags: &tags},
	}
	before := workingDirectory(t)
	if err := cmd.RunPureEngine(logPath, "--sample", "value"); err != nil {
		t.Fatalf("RunPureEngine returned error: %v", err)
	}
	assertWorkingDirectory(t, before)

	binaryPath := filepath.Join(goDir, "main"+executableSuffix(runtime.GOOS))
	if !fileExists(binaryPath) {
		t.Fatalf("pure-engine binary not created at %s", binaryPath)
	}
	assertPureEngineLog(t, logPath, targetDir, tags, "--sample", "value")
}

func TestRunPureEngineSupportsConcurrentWorkingDirectories(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOPROXY", "off")
	tags := "pure_test_tag"
	type run struct {
		cmd     CmdTool
		logPath string
	}
	runs := make([]run, 2)
	for i := range runs {
		goDir := t.TempDir()
		writePureEngineTestProject(t, goDir)
		runs[i] = run{
			cmd: CmdTool{
				GoDir: goDir, TargetDir: t.TempDir(),
				Args: ExtraArgs{Tags: &tags},
			},
			logPath: filepath.Join(t.TempDir(), fmt.Sprintf("pure-engine-%d.log", i)),
		}
	}

	before := workingDirectory(t)
	errs := make(chan error, len(runs))
	for _, run := range runs {
		go func() {
			errs <- run.cmd.RunPureEngine(run.logPath)
		}()
	}
	for range runs {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent RunPureEngine returned error: %v", err)
		}
	}
	assertWorkingDirectory(t, before)
	for _, run := range runs {
		assertPureEngineLog(t, run.logPath, run.cmd.TargetDir, tags)
	}
}

func TestRunPureEngineBuildFailureKeepsWorkingDirectory(t *testing.T) {
	goDir := filepath.Join(t.TempDir(), "missing")

	before := workingDirectory(t)
	setTestCommandLine(t, "spx", "runnative", "-tags=pure_engine")
	cmd := &CmdTool{GoDir: goDir, TargetDir: t.TempDir()}
	if err := cmd.parseCommandLineArgs(cmd.initializeFlags()); err != nil {
		t.Fatal(err)
	}
	err := cmd.executeRunNative()
	if err == nil || !strings.Contains(err.Error(), "failed to build Go binary") {
		t.Fatalf("RunPureEngine error = %v, want build failure context", err)
	}
	assertWorkingDirectory(t, before)
}

func TestRunPackModeUsesSessionEnvironment(t *testing.T) {
	projectDir := t.TempDir()
	mustWriteAssetIndex(t, projectDir)
	sessionDir := filepath.Join(projectDir, ".temp")
	generatedDir := filepath.Join(projectDir, "project")
	for _, dir := range []string{sessionDir, generatedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	logPath := filepath.Join(t.TempDir(), "runtime.log")
	runtimePath := filepath.Join(t.TempDir(), "runtime"+executableSuffix(runtime.GOOS))
	writeTestRuntimeExecutable(t, runtimePath, logPath)
	libPath := filepath.Join(generatedDir, libraryFileName(envName, runtime.GOOS, runtime.GOARCH))
	if err := os.WriteFile(libPath, []byte("bridge"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{interpruntime.ProjectDirEnv, interpruntime.AssetDirEnv, interpruntime.SessionDirEnv} {
		t.Setenv(key, "/stale")
	}

	cmd := CmdTool{
		TargetAbsDir: projectDir, ProjectDir: generatedDir, RuntimeTempDir: sessionDir,
		RuntimeCmdPath: runtimePath, LibPath: libPath,
	}
	before := workingDirectory(t)
	if err := cmd.RunPackMode("--path", "ignored"); err != nil {
		t.Fatalf("RunPackMode returned error: %v", err)
	}
	assertWorkingDirectory(t, before)

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	assertSameDirectory(t, strings.SplitN(string(log), "\n", 2)[0], sessionDir)
	for _, want := range []string{
		"--path\n" + sessionDir,
		"SPX_PROJECT_DIR=" + projectDir,
		"SPX_ASSET_DIR=" + filepath.Join(projectDir, "assets"),
		"SPX_SESSION_DIR=" + sessionDir,
	} {
		if !strings.Contains(string(log), want+"\n") {
			t.Fatalf("runtime log = %q, want %q", log, want)
		}
	}

	for file, want := range map[string]string{
		filepath.Join(sessionDir, "runtime.gdextension"):          scaffold.SessionRuntimeGDExtension(),
		filepath.Join(sessionDir, "gdspx.gdextension"):            scaffold.ProjectGDExtension(),
		filepath.Join(sessionDir, ".godot", "extension_list.cfg"): scaffold.SessionExtensionList(),
		filepath.Join(sessionDir, "lib", filepath.Base(libPath)):  "bridge",
	} {
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", file, got, want)
		}
	}
}

func TestRunInterpretedCreatesIsolatedSessionAndCopiesSharedLibrary(t *testing.T) {
	goBinPath := t.TempDir()
	projectDir := t.TempDir()
	mustWriteAssetIndex(t, projectDir)
	runtimeTempDir := filepath.Join(projectDir, ".temp")
	logPath := filepath.Join(t.TempDir(), "runtime.log")
	version := "9.9.9-test"

	runtimeName := "gdspxrt" + version + executableSuffix(runtime.GOOS)
	writeTestRuntimeExecutable(t, filepath.Join(goBinPath, runtimeName), logPath)
	if err := os.WriteFile(filepath.Join(goBinPath, runtimePackFileName(runtimeName)), []byte("runtime pack"), 0o644); err != nil {
		t.Fatalf("write runtime pack: %v", err)
	}

	libName := libraryFileName(envName, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(goBinPath, libName), []byte("shared library"), 0o755); err != nil {
		t.Fatalf("write shared library: %v", err)
	}

	cmd := CmdTool{
		GoBinPath:      goBinPath,
		RuntimeTempDir: runtimeTempDir,
		TargetAbsDir:   projectDir,
		Version:        version,
	}
	cmd.BinPostfix = executableSuffix(runtime.GOOS)

	before := workingDirectory(t)
	if err := cmd.RunInterpreted("--path", "ignored"); err != nil {
		t.Fatalf("RunInterpreted returned error: %v", err)
	}
	assertWorkingDirectory(t, before)
	wantRuntimePath := filepath.Join(goBinPath, runtimeName)
	if cmd.RuntimeCmdPath != wantRuntimePath {
		t.Fatalf("runtime path = %q, want %q", cmd.RuntimeCmdPath, wantRuntimePath)
	}

	extensionPath := filepath.Join(runtimeTempDir, "runtime.gdextension")
	if !fileExists(extensionPath) {
		t.Fatalf("runtime.gdextension not created at %s", extensionPath)
	}
	gotExtension, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatalf("read runtime.gdextension: %v", err)
	}
	if string(gotExtension) != scaffold.SessionRuntimeGDExtension() {
		t.Fatalf("runtime.gdextension contents mismatch")
	}
	projectExtensionPath := filepath.Join(runtimeTempDir, "gdspx.gdextension")
	gotProjectExtension, err := os.ReadFile(projectExtensionPath)
	if err != nil {
		t.Fatalf("read gdspx.gdextension: %v", err)
	}
	if string(gotProjectExtension) != scaffold.ProjectGDExtension() {
		t.Fatalf("gdspx.gdextension contents mismatch")
	}
	extensionListPath := filepath.Join(runtimeTempDir, ".godot", "extension_list.cfg")
	gotExtensionList, err := os.ReadFile(extensionListPath)
	if err != nil {
		t.Fatalf("read extension_list.cfg: %v", err)
	}
	if string(gotExtensionList) != scaffold.SessionExtensionList() {
		t.Fatalf("extension_list.cfg contents mismatch")
	}

	copiedLibPath := filepath.Join(runtimeTempDir, "lib", libName)
	if !fileExists(copiedLibPath) {
		t.Fatalf("shared library not copied to %s", copiedLibPath)
	}

	gotLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	logContent := string(gotLog)
	assertSameDirectory(t, strings.SplitN(logContent, "\n", 2)[0], runtimeTempDir)
	if !strings.Contains(logContent, "--path\n"+runtimeTempDir+"\n") {
		t.Fatalf("runtime log = %q, want --path followed by %s", logContent, runtimeTempDir)
	}
	if strings.Contains(logContent, "--gdextpath") {
		t.Fatalf("runtime log = %q, custom --gdextpath should not be used", logContent)
	}
	for _, want := range []string{
		"SPX_PROJECT_DIR=" + projectDir,
		"SPX_ASSET_DIR=" + filepath.Join(projectDir, "assets"),
		"SPX_SESSION_DIR=" + runtimeTempDir,
	} {
		if !strings.Contains(logContent, want+"\n") {
			t.Fatalf("runtime log = %q, want %q", logContent, want)
		}
	}
}

func TestRunModesKeepDistinctEngineFailureContext(t *testing.T) {
	projectDir := t.TempDir()
	mustWriteAssetIndex(t, projectDir)
	goBinPath := t.TempDir()
	version := "9.9.9-failure-test"
	runtimeName := "gdspxrt" + version + executableSuffix(runtime.GOOS)
	runtimePath := filepath.Join(goBinPath, runtimeName)
	writeCommandTestFile(t, runtimePath, "not an executable")
	writeCommandTestFile(t, filepath.Join(goBinPath, runtimePackFileName(runtimeName)), "runtime pack")
	libPath := filepath.Join(goBinPath, libraryFileName(envName, runtime.GOOS, runtime.GOARCH))
	if err := os.WriteFile(libPath, []byte("bridge"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		wantErr string
		run     func(sessionDir string) error
	}{
		{
			name:    "pack mode",
			wantErr: "native Engine failed",
			run: func(sessionDir string) error {
				cmd := CmdTool{
					TargetAbsDir: projectDir, RuntimeTempDir: sessionDir,
					RuntimeCmdPath: runtimePath, LibPath: libPath,
				}
				return cmd.RunPackMode()
			},
		},
		{
			name:    "interpreted mode",
			wantErr: "interpreted Engine failed",
			run: func(sessionDir string) error {
				cmd := CmdTool{
					TargetAbsDir: projectDir, RuntimeTempDir: sessionDir,
					GoBinPath: goBinPath, Version: version, BinPostfix: executableSuffix(runtime.GOOS),
				}
				return cmd.RunInterpreted()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := workingDirectory(t)
			err := tt.run(filepath.Join(projectDir, ".temp-"+strings.ReplaceAll(tt.name, " ", "-")))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("run error = %v, want context %q", err, tt.wantErr)
			}
			assertWorkingDirectory(t, before)
		})
	}
}

func TestResolveInterpretedRuntimeAssetsFallsBackToExternalWhenEmbeddedUnavailable(t *testing.T) {
	goBinPath := t.TempDir()
	version := "9.9.9-test"
	runtimeName := "gdspxrt" + version
	if runtime.GOOS == "windows" {
		runtimeName += ".exe"
	}
	runtimePathWant := filepath.Join(goBinPath, runtimeName)
	if err := os.WriteFile(runtimePathWant, []byte("external runtime"), 0o755); err != nil {
		t.Fatalf("write external runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinPath, runtimePackFileName(runtimeName)), []byte("external pack"), 0o644); err != nil {
		t.Fatalf("write external runtime pack: %v", err)
	}
	libName := libraryFileName(envName, runtime.GOOS, runtime.GOARCH)
	libPathWant := filepath.Join(goBinPath, libName)
	if err := os.WriteFile(libPathWant, []byte("external library"), 0o755); err != nil {
		t.Fatalf("write external shared library: %v", err)
	}

	cmd := CmdTool{
		GoBinPath: goBinPath,
		Version:   version,
	}

	runtimePath, libPath, err := cmd.resolveInterpretedRuntimeAssets(runtimeName, runtimePackFileName(runtimeName), libName)
	if err != nil {
		t.Fatalf("resolveInterpretedRuntimeAssets returned error: %v", err)
	}
	if runtimePath != runtimePathWant {
		t.Fatalf("runtime path = %s, want %s", runtimePath, runtimePathWant)
	}
	if libPath != libPathWant {
		t.Fatalf("shared library path = %s, want %s", libPath, libPathWant)
	}
}

func mustWriteAssetIndex(t *testing.T, projectDir string) {
	t.Helper()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "index.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write assets/index.json: %v", err)
	}
}

func TestStopWebIgnoresInvalidPIDFile(t *testing.T) {
	targetDir := t.TempDir()
	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir}
	pidFile := filepath.Join(targetDir, ".gdspx_web_server.pid")
	if err := os.WriteFile(pidFile, []byte("invalid"), 0644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	if err := cmd.StopWeb(); err != nil {
		t.Fatalf("StopWeb returned error for invalid pid file: %v", err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}

func TestWebServerPIDPathUsesAbsoluteTargetDir(t *testing.T) {
	targetDir := t.TempDir()
	otherDir := t.TempDir()
	t.Chdir(otherDir)

	cmd := CmdTool{TargetDir: ".", TargetAbsDir: targetDir}
	got := cmd.webServerPIDPath()
	want := filepath.Join(targetDir, ".gdspx_web_server.pid")
	if got != want {
		t.Fatalf("webServerPIDPath = %s, want %s", got, want)
	}
}

func TestParsePIDList(t *testing.T) {
	got := parsePIDList([]byte("123\n456\ninvalid\n"))
	want := []int{123, 456}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePIDList = %v, want %v", got, want)
	}
}

func TestLooksLikeGDSPXWebServerCommandLine(t *testing.T) {
	if !looksLikeGDSPXWebServerCommandLine(`python.exe C:\tmp\gdspx_web_server.py -r build -p 8080`) {
		t.Fatal("expected gdspx web server command line to match")
	}
	if looksLikeGDSPXWebServerCommandLine(`python.exe C:\tmp\other_server.py -p 8080`) {
		t.Fatal("did not expect unrelated python process to match")
	}
}

func TestWindowsProcessCommandLineQuery(t *testing.T) {
	got := windowsProcessCommandLineQuery(321)
	if !strings.Contains(got, "Get-CimInstance Win32_Process") {
		t.Fatalf("windowsProcessCommandLineQuery = %q, want Get-CimInstance query", got)
	}
	if !strings.Contains(got, "ProcessId = 321") {
		t.Fatalf("windowsProcessCommandLineQuery = %q, want pid filter", got)
	}
	if strings.Contains(strings.ToLower(got), "tasklist") {
		t.Fatalf("windowsProcessCommandLineQuery = %q, should not use tasklist", got)
	}
}

func writeTestRuntimeExecutable(t *testing.T, path string, logPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}

	var script string
	if runtime.GOOS == "windows" {
		script = fmt.Sprintf("@echo off\r\ncd > %q\r\nfor %%%%a in (%%*) do @echo %%%%a>>%q\r\necho SPX_PROJECT_DIR=%%SPX_PROJECT_DIR%%>>%q\r\necho SPX_ASSET_DIR=%%SPX_ASSET_DIR%%>>%q\r\necho SPX_SESSION_DIR=%%SPX_SESSION_DIR%%>>%q\r\n", logPath, logPath, logPath, logPath, logPath)
	} else {
		script = fmt.Sprintf("#!/bin/sh\npwd > %q\nprintf '%%s\\n' \"$@\" >> %q\nprintf 'SPX_PROJECT_DIR=%%s\\nSPX_ASSET_DIR=%%s\\nSPX_SESSION_DIR=%%s\\n' \"$SPX_PROJECT_DIR\" \"$SPX_ASSET_DIR\" \"$SPX_SESSION_DIR\" >> %q\n", logPath, logPath, logPath)
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write runtime executable: %v", err)
	}
}

func writePureEngineTestProject(t *testing.T, goDir string) {
	t.Helper()
	for name, content := range map[string]string{
		"go.mod": "module example.com/pureenginetest\n\ngo 1.23\n",
		"main.go": `package main

import (
	"os"
	"strings"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fields := append([]string{cwd, requiredTag}, os.Args[2:]...)
	if err := os.WriteFile(os.Args[1], []byte(strings.Join(fields, "\n")), 0o644); err != nil {
		panic(err)
	}
}
`,
		"tag.go": `//go:build pure_test_tag

package main

const requiredTag = "pure_test_tag"
`,
	} {
		if err := os.WriteFile(filepath.Join(goDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func workingDirectory(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func assertWorkingDirectory(t *testing.T, want string) {
	t.Helper()
	if got := workingDirectory(t); got != want {
		t.Fatalf("working directory changed from %q to %q", want, got)
	}
}

func assertPureEngineLog(t *testing.T, logPath, targetDir, tag string, wantArgs ...string) {
	t.Helper()
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(log), "\n")
	wantLines := append([]string{targetDir, tag}, wantArgs...)
	if len(lines) != len(wantLines) || !reflect.DeepEqual(lines[1:], wantLines[1:]) {
		t.Fatalf("pure-engine log = %q, want %q", log, strings.Join(wantLines, "\n"))
	}
	assertSameDirectory(t, lines[0], targetDir)
}

func assertSameDirectory(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat runtime working directory %q: %v", got, err)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatalf("stat expected runtime working directory %q: %v", want, err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("runtime working directory = %q, want %q", got, want)
	}
}

func TestInterpretedRootsResolveDefaultsAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, tt := range []struct {
		name             string
		cmd              CmdTool
		project, session string
	}{
		{"logical root", CmdTool{TargetAbsDir: root}, root, filepath.Join(root, ".temp")},
		{"relative root", CmdTool{TargetDir: "game"}, filepath.Join(root, "game"), filepath.Join(root, "game", ".temp")},
		{"relative session", CmdTool{TargetAbsDir: root, RuntimeTempDir: "session"}, root, filepath.Join(root, "session")},
		{"explicit session", CmdTool{TargetAbsDir: root, RuntimeTempDir: filepath.Join(root, "other")}, root, filepath.Join(root, "other")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cmd.interpretedRoots()
			if err != nil || got.ProjectDir != tt.project || got.AssetDir != filepath.Join(tt.project, "assets") || got.SessionDir != tt.session {
				t.Fatalf("interpreted roots = %+v, %v; want project %q, session %q", got, err, tt.project, tt.session)
			}
		})
	}
	if _, err := (&CmdTool{}).interpretedRoots(); err == nil || !strings.Contains(err.Error(), "project directory is not configured") {
		t.Fatalf("missing roots error = %v", err)
	}
}

func TestRuntimePackFilenames(t *testing.T) {
	for _, tt := range []struct{ executable, pack string }{
		{"gdspxrt", "gdspxrt.pck"},
		{"gdspxrt.exe", "gdspxrt.pck"},
		{"gdspxrt.exe.extra", "gdspxrt.exe.extra.pck"},
	} {
		if got := runtimePackFileName(tt.executable); got != tt.pack {
			t.Errorf("pack filename for %q = %q, want %q", tt.executable, got, tt.pack)
		}
	}
}

func TestListeningPIDCommands(t *testing.T) {
	for _, tt := range []struct {
		name, tool, script string
		windows            bool
		want               []int
		wantError          bool
	}{
		{"unix parsed", "lsof", `printf '31\ninvalid\n42\n'`, false, []int{31, 42}, false},
		{"unix no listeners", "lsof", "exit 1", false, nil, false},
		{"unix command failed", "lsof", "exit 2", false, nil, true},
		{"unix missing command", "unused", "exit 0", false, nil, true},
		{"windows parsed", "netstat", `printf '%s\n' 'Proto Local Address Foreign Address State PID' 'UDP 0.0.0.0:8060 *:* LISTENING 10' 'TCP 0.0.0.0:8061 *:* LISTENING 11' 'TCP 0.0.0.0:8060 *:* ESTABLISHED 12' 'TCP 0.0.0.0:8060 *:* LISTENING invalid' 'TCP 0.0.0.0:8060 *:* LISTENING 31' 'tcp [::]:8060 *:* listen 42' 'incomplete'`, true, []int{31, 42}, false},
		{"windows command failed", "netstat", "exit 2", true, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installCommandTestTools(t, map[string]string{tt.tool: tt.script})
			var got []int
			var err error
			if tt.windows {
				got, err = listListeningPIDsWindows(8060)
			} else {
				got, err = listListeningPIDsUnix(8060)
			}
			if (err != nil) != tt.wantError || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("listening PIDs = %v, %v; want %v, error %v", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestWebServerProcessIdentification(t *testing.T) {
	for _, tt := range []struct {
		name, script string
		want         bool
	}{
		{"matching server", `printf 'python gdspx_web_server.py -p 8060'`, true},
		{"unrelated python", `printf 'python another-server.py'`, false},
		{"process query failed", "exit 2", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installCommandTestTools(t, map[string]string{"ps": tt.script, "powershell": tt.script})
			if got := looksLikeGDSPXWebServerProcessUnix(37); got != tt.want {
				t.Errorf("Unix process match = %v, want %v", got, tt.want)
			}
			if got := looksLikeGDSPXWebServerProcessWindows(37); got != tt.want {
				t.Errorf("Windows process match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunWebServerLifecycle(t *testing.T) {
	for _, scenario := range []string{"running", "browser failure", "orphaned server", "python3 fallback", "early failure", "missing python", "start failure", "pid write failure"} {
		t.Run(scenario, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "commands.log")
			t.Setenv("SPX_SERVER_LOG", logPath)
			const python = `printf '%s\n' "$*" >> "$SPX_SERVER_LOG"
exec /bin/sleep 30
`
			tools := map[string]string{
				"python":   python,
				"lsof":     `printf '%s\n' "${SPX_ORPHAN_PID:-}"`,
				"ps":       `printf 'python gdspx_web_server.py'`,
				"xdg-open": `printf 'browser %s\n' "$*" >> "$SPX_SERVER_LOG"`,
				"open":     `printf 'browser %s\n' "$*" >> "$SPX_SERVER_LOG"`,
			}
			wantError := ""
			switch scenario {
			case "browser failure":
				tools["xdg-open"], tools["open"] = "exit 2", "exit 2"
			case "python3 fallback":
				delete(tools, "python")
				tools["python3"] = "exit 0"
				wantError = "exited unexpectedly without an error"
			case "early failure":
				tools["python"] = "exit 9"
				wantError = "exited early"
			case "missing python":
				delete(tools, "python")
				wantError = "neither python nor python3"
			case "start failure":
				tools["python"] = "exit 0"
				wantError = "error starting server"
			case "pid write failure":
				wantError = "failed to record web server pid"
			}
			binDir := installCommandTestTools(t, tools)
			if scenario == "start failure" {
				// LookPath succeeds, but the executable's interpreter is missing.
				if err := os.WriteFile(filepath.Join(binDir, "python"), []byte("#!/missing-spx-test-interpreter\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("SPX_ORPHAN_PID", "")
			cmd := &CmdTool{TargetAbsDir: t.TempDir(), ProjectDir: t.TempDir()}
			if scenario == "orphaned server" {
				cmd.ServerPort = 8060
			}
			if scenario == "pid write failure" {
				cmd.TargetAbsDir = filepath.Join(cmd.TargetAbsDir, "missing-parent")
			}
			t.Cleanup(func() {
				if err := cmd.StopWeb(); err != nil {
					t.Errorf("stop test server: %v", err)
				}
			})
			var err error
			if scenario == "python3 fallback" || scenario == "early failure" {
				// Keep the startup grace period frozen while Wait observes the
				// immediately exiting child, regardless of host process startup time.
				synctest.Test(t, func(t *testing.T) { err = cmd.runWebServer() })
			} else {
				err = cmd.runWebServer()
			}
			if wantError != "" {
				if err == nil || !strings.Contains(err.Error(), wantError) {
					t.Fatalf("server error = %v, want %q", err, wantError)
				}
				if _, err := os.Stat(cmd.webServerPIDPath()); !os.IsNotExist(err) {
					t.Fatalf("failed server left pid file: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pid := strings.TrimSpace(readTestFile(t, cmd.webServerPIDPath())); pid == "" {
				t.Fatal("running server has no recorded PID")
			}
			calls := readTestFile(t, logPath)
			wantArgs := filepath.Join(cmd.ProjectDir, ".godot", "gdspx_web_server.py") + " -r " + filepath.Join(cmd.ProjectDir, ".builds", "web") + fmt.Sprintf(" -p %d --no-browser", cmd.ServerPort)
			if !strings.Contains(calls, wantArgs) {
				t.Fatalf("server invocation = %q, want %q", calls, wantArgs)
			}
			if scenario == "running" && (!strings.Contains(calls, "browser ") || !strings.Contains(calls, "http://127.0.0.1:0")) {
				t.Fatalf("browser did not receive local URL: %s", calls)
			}
			if scenario == "orphaned server" {
				// Discover only this test's owned process after its PID file is lost.
				t.Setenv("SPX_ORPHAN_PID", strings.TrimSpace(readTestFile(t, cmd.webServerPIDPath())))
				if err := os.Remove(cmd.webServerPIDPath()); err != nil {
					t.Fatal(err)
				}
			}
			if err := cmd.StopWeb(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(cmd.webServerPIDPath()); !os.IsNotExist(err) {
				t.Fatalf("stopped server retained pid file: %v", err)
			}
		})
	}
}

func TestStopWebHandlesUnreadableAndStalePID(t *testing.T) {
	for _, scenario := range []string{"directory", "negative pid", "unrelated process", "missing pid", "invalid pid"} {
		t.Run(scenario, func(t *testing.T) {
			installCommandTestTools(t, map[string]string{"ps": "exit 1", "lsof": "exit 1"})
			cmd := &CmdTool{TargetDir: t.TempDir(), ServerPort: 8060}
			pidPath := cmd.webServerPIDPath()
			switch scenario {
			case "directory":
				if err := os.Mkdir(pidPath, 0o755); err != nil {
					t.Fatal(err)
				}
			case "negative pid":
				writeCommandTestFile(t, pidPath, "-1")
			case "unrelated process":
				writeCommandTestFile(t, pidPath, "37")
			case "invalid pid":
				writeCommandTestFile(t, pidPath, "invalid")
			}
			err := cmd.StopWeb()
			if scenario == "directory" {
				if err == nil || !strings.Contains(err.Error(), "failed to read web server pid") {
					t.Fatalf("PID read error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
				t.Fatalf("stale PID file retained: %v", err)
			}
		})
	}
}

func TestRunWebExportsBeforeStartingServer(t *testing.T) {
	for _, mode := range []string{webNormalMode, webWorkerMode} {
		t.Run(mode, func(t *testing.T) {
			cmd := newWebExportFixture(t, mode)
			t.Setenv("SPX_SERVER_EXPECTED", filepath.Join(cmd.WebDir, "game.zip"))
			installCommandTestTools(t, map[string]string{
				"go":     `printf '%s\n' "$SPX_WEB_GOROOT"`,
				"python": `test -f "$SPX_SERVER_EXPECTED"; exit 0`,
			})
			// The fake server must finish checking the export before the startup
			// grace period elapses, even when launching a shell takes over 500ms.
			synctest.Test(t, func(t *testing.T) {
				var err error
				if mode == webNormalMode {
					err = cmd.RunWeb()
				} else {
					err = cmd.RunWebWorker()
				}
				if err == nil || !strings.Contains(err.Error(), "exited unexpectedly without an error") {
					t.Fatalf("runweb result = %v, want successful export before test server exits", err)
				}
			})
		})
	}
}

func TestInstallRepoWebRuntimeUsesOnlyMissingAssets(t *testing.T) {
	for _, scenario := range []string{"install", "installer failure", "installed", "external project"} {
		t.Run(scenario, func(t *testing.T) {
			root, bin := t.TempDir(), t.TempDir()
			logPath := filepath.Join(t.TempDir(), "installer.log")
			t.Setenv("SPX_INSTALLER_LOG", logPath)
			script := `printf '%s\n' "$@" > "$SPX_INSTALLER_LOG"`
			if scenario == "installer failure" {
				script += "\nexit 19"
			}
			installCommandTestTools(t, map[string]string{"bash": script})
			cmd := &CmdTool{TargetAbsDir: root, GoBinPath: bin, Version: "test"}
			if scenario != "external project" {
				writeLocalSpxRepoMarker(t, root)
			}
			if scenario == "installed" {
				for _, name := range []string{"ispx", "ispx.wasm", "gdspxrttest_webnormal"} {
					writeCommandTestFile(t, filepath.Join(bin, name), "installed")
				}
			}
			err := cmd.installRepoWebRuntime(webNormalMode)
			if (err != nil) != (scenario == "installer failure") {
				t.Fatalf("installer error = %v, scenario %s", err, scenario)
			}
			if scenario == "installed" || scenario == "external project" {
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Fatalf("unnecessary installer invocation: %v", err)
				}
			} else if got := readTestFile(t, logPath); got != filepath.Join(root, "cmd", "spx", "install.sh")+"\n--web\n" {
				t.Fatalf("installer arguments = %q", got)
			}
		})
	}
}

func TestInterpretedRuntimeReportsMissingAssetsAndInitializesCommand(t *testing.T) {
	for _, kind := range []string{"runtime and library", "runtime pack", "library"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			cmd := &CmdTool{GoBinPath: root, Version: "9.9.9-missing-test"}
			runtimeName, libName := "gdspxrt9.9.9-missing-test", "bridge.so"
			if kind != "runtime and library" {
				writeCommandTestFile(t, filepath.Join(root, runtimeName), "runtime")
			}
			if kind == "runtime pack" {
				writeCommandTestFile(t, filepath.Join(root, libName), "bridge")
			}
			if kind == "library" {
				writeCommandTestFile(t, filepath.Join(root, runtimePackFileName(runtimeName)), "pack")
			}
			runtimePath, libPath, err := cmd.resolveInterpretedRuntimeAssets(runtimeName, runtimePackFileName(runtimeName), libName)
			if err == nil || !strings.Contains(err.Error(), "interpreted runtime assets not available") || runtimePath != "" || libPath != "" {
				t.Fatalf("assets = %q, %q, %v", runtimePath, libPath, err)
			}
			if kind == "runtime pack" && !strings.Contains(err.Error(), "runtime pack") {
				t.Fatalf("missing pack reason = %v", err)
			}
			if kind != "runtime pack" && !strings.Contains(err.Error(), libName) {
				t.Fatalf("missing library reason = %v", err)
			}
		})
	}
	for _, goos := range []string{"", "windows"} {
		t.Run("command "+goos, func(t *testing.T) {
			t.Setenv("GOOS", goos)
			root := t.TempDir()
			setTestCommandLine(t, "spx", "run", "-movie")
			cmd := &CmdTool{TargetDir: root, GoBinPath: filepath.Join(root, "bin"), Version: "9.9.9-missing-test"}
			if err := cmd.parseCommandLineArgs(cmd.initializeFlags()); err != nil {
				t.Fatal(err)
			}
			if err := cmd.handleInterpretedRunCommand(); err == nil || !strings.Contains(err.Error(), "runtime assets not available") {
				t.Fatalf("run error = %v", err)
			}
			suffix := executableSuffix(runtime.GOOS)
			if goos != "" {
				suffix = executableSuffix(goos)
			}
			if !cmd.RuntimeMode || cmd.RuntimeTempDir != filepath.Join(root, ".temp") || cmd.RuntimeCmdPath != filepath.Join(root, "bin", "gdspxrt"+cmd.Version+suffix) {
				t.Fatalf("initialized command = %+v", cmd)
			}
			if info, err := os.Stat(filepath.Join(root, ".temp", "output")); err != nil || !info.IsDir() {
				t.Fatalf("movie directory = %v, %v", info, err)
			}
		})
	}
}

func TestRunPackModeRejectsIncompleteSessionBeforeStartingEngine(t *testing.T) {
	for _, scenario := range []string{"missing project", "missing bridge", "missing engine"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			mustWriteAssetIndex(t, root)
			log := filepath.Join(root, "engine.log")
			bin := installCommandTestTools(t, map[string]string{"engine": `printf started > "$SPX_TEST_ENGINE_LOG"`})
			t.Setenv("SPX_TEST_ENGINE_LOG", log)
			cmd := &CmdTool{TargetAbsDir: root, RuntimeCmdPath: filepath.Join(bin, "engine"), LibPath: filepath.Join(root, "bridge.so")}
			writeCommandTestFile(t, cmd.LibPath, "bridge")
			want := ""
			switch scenario {
			case "missing project":
				cmd.TargetAbsDir, want = "", "project directory is not configured"
			case "missing bridge":
				if err := os.Remove(cmd.LibPath); err != nil {
					t.Fatal(err)
				}
				want = "BridgePath"
			case "missing engine":
				if err := os.Remove(cmd.RuntimeCmdPath); err != nil {
					t.Fatal(err)
				}
				want = "Executable"
			}
			before := workingDirectory(t)
			if err := cmd.RunPackMode(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("session preparation error = %v, want %q", err, want)
			}
			assertWorkingDirectory(t, before)
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Fatalf("engine started after preparation failed: %v", err)
			}
		})
	}
}
