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

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/scaffold"
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
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/goplus/spx/v3\n"), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(root, "gox.mod"), []byte("project main.spx Game github.com/goplus/spx/v3 math\n"), 0644); err != nil {
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
	err := (&CmdTool{GoDir: goDir, TargetDir: t.TempDir()}).RunPureEngine()
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
	if err := os.WriteFile(runtimePath, []byte("not an executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goBinPath, runtimePackFileName(runtimeName)), []byte("runtime pack"), 0o644); err != nil {
		t.Fatal(err)
	}
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
