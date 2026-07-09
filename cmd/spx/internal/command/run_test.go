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
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v2/internal/scaffold"
)

func TestRunWebCommandRunsExportBeforeServer(t *testing.T) {
	var calls []string

	err := runWebCommand(
		func() error {
			calls = append(calls, "export")
			return nil
		},
		func() error {
			calls = append(calls, "serve")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runWebCommand returned error: %v", err)
	}

	want := []string{"export", "serve"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected call order: got %v, want %v", calls, want)
	}
}

func TestRunWebCommandRunsSetupBeforeExportAndServer(t *testing.T) {
	var calls []string

	err := runWebCommandWithSetup(
		func() error {
			calls = append(calls, "setup")
			return nil
		},
		func() error {
			calls = append(calls, "export")
			return nil
		},
		func() error {
			calls = append(calls, "serve")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runWebCommandWithSetup returned error: %v", err)
	}

	want := []string{"setup", "export", "serve"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected call order: got %v, want %v", calls, want)
	}
}

func TestRunWebCommandStopsOnSetupError(t *testing.T) {
	wantErr := errors.New("setup failed")
	exportCalled := false
	serverCalled := false

	err := runWebCommandWithSetup(
		func() error {
			return wantErr
		},
		func() error {
			exportCalled = true
			return nil
		},
		func() error {
			serverCalled = true
			return nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWebCommandWithSetup error = %v, want %v", err, wantErr)
	}
	if exportCalled {
		t.Fatal("export should not start when setup fails")
	}
	if serverCalled {
		t.Fatal("server should not start when setup fails")
	}
}

func TestRunWebCommandStopsOnExportError(t *testing.T) {
	wantErr := errors.New("export failed")
	serverCalled := false

	err := runWebCommand(
		func() error {
			return wantErr
		},
		func() error {
			serverCalled = true
			return nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWebCommand error = %v, want %v", err, wantErr)
	}
	if serverCalled {
		t.Fatal("server should not start when export fails")
	}
}

func writeLocalSpxRepoMarker(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "spx"), 0755); err != nil {
		t.Fatalf("mkdir cmd/spx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/goplus/spx/v2\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "spx", "install.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("write install.sh: %v", err)
	}
}

func TestFindSpxRootFindsCurrentDir(t *testing.T) {
	root := t.TempDir()
	writeLocalSpxRepoMarker(t, root)

	cmd := CmdTool{}
	cmd.TargetAbsDir = root
	got := cmd.findSpxRoot()
	if got != root {
		t.Fatalf("findSpxRoot = %s, want %s", got, root)
	}
}

func TestFindSpxRootFindsParentDir(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "tutorial", "00-Hello")
	if err := os.MkdirAll(startDir, 0755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeLocalSpxRepoMarker(t, root)

	cmd := CmdTool{}
	cmd.TargetAbsDir = startDir
	got := cmd.findSpxRoot()
	if got != root {
		t.Fatalf("findSpxRoot = %s, want %s", got, root)
	}
}

func TestFindSpxRootReturnsEmptyWhenMissing(t *testing.T) {
	root := t.TempDir()

	cmd := CmdTool{}
	cmd.TargetAbsDir = root
	got := cmd.findSpxRoot()
	if got != "" {
		t.Fatalf("findSpxRoot = %s, want empty", got)
	}
}

func TestFindSpxRootIgnoresProjectsThatOnlyReferenceSpx(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "spx"), 0755); err != nil {
		t.Fatalf("mkdir cmd/spx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "gox.mod"), []byte("project main.spx Game github.com/goplus/spx/v2 math\n"), 0644); err != nil {
		t.Fatalf("write gox.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "spx", "install.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("write install.sh: %v", err)
	}

	cmd := CmdTool{TargetAbsDir: root}
	if got := cmd.findSpxRoot(); got != "" {
		t.Fatalf("findSpxRoot = %s, want empty", got)
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

func TestIsRuntimeModeCommand(t *testing.T) {
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
			if got := isRuntimeModeCommand(tt.cmdName); got != tt.want {
				t.Fatalf("isRuntimeModeCommand(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestShouldBuildWasmForCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{name: "buildweb", cmdName: "buildweb", want: true},
		{name: "exportweb", cmdName: "exportweb", want: true},
		{name: "runweb", cmdName: "runweb", want: true},
		{name: "runwebworker", cmdName: "runwebworker", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBuildWasmForCommand(tt.cmdName); got != tt.want {
				t.Fatalf("shouldBuildWasmForCommand(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestBrowserOpenCommandsDarwinPrefersChromeThenDefault(t *testing.T) {
	url := "http://127.0.0.1:8060"

	got := browserOpenCommands("darwin", url)
	want := []browserOpenCommand{
		{name: "open", args: []string{"-a", "Google Chrome", url}},
		{name: "open", args: []string{url}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browserOpenCommands(darwin) = %#v, want %#v", got, want)
	}
}

func TestBrowserOpenCommandsLinuxUsesXdgOpen(t *testing.T) {
	url := "http://127.0.0.1:8060"

	got := browserOpenCommands("linux", url)
	want := []browserOpenCommand{
		{name: "xdg-open", args: []string{url}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browserOpenCommands(linux) = %#v, want %#v", got, want)
	}
}

func TestBrowserOpenCommandsWindowsUsesRundll32(t *testing.T) {
	url := "http://127.0.0.1:8060"

	got := browserOpenCommands("windows", url)
	want := []browserOpenCommand{
		{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browserOpenCommands(windows) = %#v, want %#v", got, want)
	}
}

func TestRunInterpretedCreatesRuntimeExtensionAndCopiesSharedLibrary(t *testing.T) {
	oldPrepareEmbeddedRuntimeAssets := prepareEmbeddedRuntimeAssets
	prepareEmbeddedRuntimeAssets = func(string, ...string) (string, bool, error) {
		return "", false, nil
	}
	t.Cleanup(func() {
		prepareEmbeddedRuntimeAssets = oldPrepareEmbeddedRuntimeAssets
	})

	goBinPath := t.TempDir()
	runtimeTempDir := t.TempDir()
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
		Version:        version,
	}
	cmd.BinPostfix = executableSuffix(runtime.GOOS)

	if err := cmd.RunInterpreted("--path", "ignored"); err != nil {
		t.Fatalf("RunInterpreted returned error: %v", err)
	}

	extensionPath := filepath.Join(runtimeTempDir, "runtime.gdextension")
	if !fileExists(extensionPath) {
		t.Fatalf("runtime.gdextension not created at %s", extensionPath)
	}
	gotExtension, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatalf("read runtime.gdextension: %v", err)
	}
	if string(gotExtension) != scaffold.RuntimeGDExtension() {
		t.Fatalf("runtime.gdextension contents mismatch")
	}

	copiedLibPath := filepath.Join(runtimeTempDir, libName)
	if !fileExists(copiedLibPath) {
		t.Fatalf("shared library not copied to %s", copiedLibPath)
	}

	gotLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	logContent := string(gotLog)
	if !strings.Contains(logContent, "--path\n"+runtimeTempDir+"\n") {
		t.Fatalf("runtime log = %q, want --path followed by %s", logContent, runtimeTempDir)
	}
	if !strings.Contains(logContent, "--gdextpath\n"+extensionPath+"\n") {
		t.Fatalf("runtime log = %q, want --gdextpath followed by %s", logContent, extensionPath)
	}
}

func TestResolveInterpretedRuntimeAssetsPrefersEmbedded(t *testing.T) {
	oldPrepareEmbeddedRuntimeAssets := prepareEmbeddedRuntimeAssets
	embeddedDir := t.TempDir()
	prepareEmbeddedRuntimeAssets = func(string, ...string) (string, bool, error) {
		return embeddedDir, true, nil
	}
	t.Cleanup(func() {
		prepareEmbeddedRuntimeAssets = oldPrepareEmbeddedRuntimeAssets
	})

	goBinPath := t.TempDir()
	version := "9.9.9-test"
	runtimeName := "gdspxrt" + version
	if runtime.GOOS == "windows" {
		runtimeName += ".exe"
	}
	if err := os.WriteFile(filepath.Join(goBinPath, runtimeName), []byte("external runtime"), 0o755); err != nil {
		t.Fatalf("write external runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinPath, runtimePackFileName(runtimeName)), []byte("external pack"), 0o644); err != nil {
		t.Fatalf("write external runtime pack: %v", err)
	}
	libName := libraryFileName(envName, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(goBinPath, libName), []byte("external library"), 0o755); err != nil {
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
	if got, want := runtimePath, filepath.Join(embeddedDir, runtimeName); got != want {
		t.Fatalf("runtime path = %s, want %s", got, want)
	}
	if got, want := libPath, filepath.Join(embeddedDir, libName); got != want {
		t.Fatalf("shared library path = %s, want %s", got, want)
	}
}

func TestResolveInterpretedRuntimeAssetsFallsBackToExternalWhenEmbeddedUnavailable(t *testing.T) {
	oldPrepareEmbeddedRuntimeAssets := prepareEmbeddedRuntimeAssets
	prepareEmbeddedRuntimeAssets = func(string, ...string) (string, bool, error) {
		return "", false, nil
	}
	t.Cleanup(func() {
		prepareEmbeddedRuntimeAssets = oldPrepareEmbeddedRuntimeAssets
	})

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

func TestRunCmdRunHonorsPortableGoEnv(t *testing.T) {
	oldPrepareEmbeddedRuntimeAssets := prepareEmbeddedRuntimeAssets
	prepareEmbeddedRuntimeAssets = func(string, ...string) (string, bool, error) {
		return "", false, nil
	}
	t.Cleanup(func() {
		prepareEmbeddedRuntimeAssets = oldPrepareEmbeddedRuntimeAssets
	})

	targetDir := t.TempDir()
	goEnvDir := filepath.Join(t.TempDir(), "goenv")
	goBinPath := filepath.Join(goEnvDir, "go", "bin")
	goRootBinPath := filepath.Join(goEnvDir, "gotoolchain", "go", "bin")
	logPath := filepath.Join(t.TempDir(), "runtime.log")
	version := "9.9.9-test"

	if err := os.MkdirAll(goBinPath, 0o755); err != nil {
		t.Fatalf("mkdir go/bin: %v", err)
	}
	if err := os.MkdirAll(goRootBinPath, 0o755); err != nil {
		t.Fatalf("mkdir gotoolchain/go/bin: %v", err)
	}
	writeTestRuntimeExecutable(t, filepath.Join(goRootBinPath, goBinaryName(runtime.GOOS)), filepath.Join(t.TempDir(), "go.log"))

	runtimeName := "gdspxrt" + version + executableSuffix(runtime.GOOS)
	writeTestRuntimeExecutable(t, filepath.Join(goBinPath, runtimeName), logPath)
	if err := os.WriteFile(filepath.Join(goBinPath, runtimePackFileName(runtimeName)), []byte("runtime pack"), 0o644); err != nil {
		t.Fatalf("write runtime pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinPath, libraryFileName(envName, runtime.GOOS, runtime.GOARCH)), []byte("shared library"), 0o755); err != nil {
		t.Fatalf("write shared library: %v", err)
	}

	rawArgs := os.Args
	t.Cleanup(func() {
		os.Args = rawArgs
	})
	os.Args = []string{"spx", "run", "--goenv", goEnvDir, "--path", targetDir}

	cmd := CmdTool{}
	if err := cmd.RunCmd("spx", "spx", version, embed.FS{}, "", "project"); err != nil {
		t.Fatalf("RunCmd returned error: %v", err)
	}

	gotLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	if !strings.Contains(string(gotLog), filepath.Join(targetDir, ".temp")) {
		t.Fatalf("runtime log = %q, want runtime temp dir under %s", string(gotLog), targetDir)
	}
}

func TestGetWasmPathsPrefersProjectBuild(t *testing.T) {
	projectDir := t.TempDir()
	cmd := CmdTool{
		ProjectDir: projectDir,
		GoBinPath:  filepath.Join(projectDir, "gobin"),
	}

	projectWasm := filepath.Join(projectDir, ".builds", "web", "ispx.wasm")
	if err := os.MkdirAll(filepath.Dir(projectWasm), 0755); err != nil {
		t.Fatalf("mkdir project wasm dir: %v", err)
	}
	if err := os.WriteFile(projectWasm, []byte("project"), 0644); err != nil {
		t.Fatalf("write project wasm: %v", err)
	}
	if err := os.WriteFile(projectWasm+".br", []byte("project-br"), 0644); err != nil {
		t.Fatalf("write project wasm br: %v", err)
	}

	if err := os.MkdirAll(cmd.GoBinPath, 0755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmd.GoBinPath, "ispx.wasm"), []byte("gobin"), 0644); err != nil {
		t.Fatalf("write gobin wasm: %v", err)
	}

	gotWasm, gotWasmBr := cmd.getWasmPaths()
	if gotWasm != projectWasm {
		t.Fatalf("getWasmPaths wasm = %s, want %s", gotWasm, projectWasm)
	}
	if gotWasmBr != projectWasm+".br" {
		t.Fatalf("getWasmPaths wasm br = %s, want %s", gotWasmBr, projectWasm+".br")
	}
}

func TestGetWasmPathsFallsBackToGoBin(t *testing.T) {
	projectDir := t.TempDir()
	goBinPath := filepath.Join(projectDir, "gobin")
	cmd := CmdTool{
		ProjectDir: projectDir,
		GoBinPath:  goBinPath,
	}

	if err := os.MkdirAll(goBinPath, 0755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	goBinWasm := filepath.Join(goBinPath, "ispx.wasm")
	if err := os.WriteFile(goBinWasm, []byte("gobin"), 0644); err != nil {
		t.Fatalf("write gobin wasm: %v", err)
	}

	gotWasm, gotWasmBr := cmd.getWasmPaths()
	if gotWasm != goBinWasm {
		t.Fatalf("getWasmPaths wasm = %s, want %s", gotWasm, goBinWasm)
	}
	if gotWasmBr != "" {
		t.Fatalf("getWasmPaths wasm br = %s, want empty", gotWasmBr)
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
	restoreDir := t.TempDir()
	if err := os.Chdir(restoreDir); err != nil {
		t.Fatalf("chdir restore dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(restoreDir)
	})
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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
		script = fmt.Sprintf("@echo off\r\ncd > %q\r\nfor %%%%a in (%%*) do @echo %%%%a>>%q\r\n", logPath, logPath)
	} else {
		script = fmt.Sprintf("#!/bin/sh\npwd > %q\nprintf '%%s\\n' \"$@\" >> %q\n", logPath, logPath)
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write runtime executable: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
