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
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
	"github.com/goplus/spx/v3/internal/release"
)

func TestParseEnvExportShellArgsDefault(t *testing.T) {
	cfg, err := parseEnvExportShellArgs(nil)
	if err != nil {
		t.Fatalf("parseEnvExportShellArgs returned error: %v", err)
	}
	if cfg.platform != "" {
		t.Fatalf("unexpected platform: %s", cfg.platform)
	}
}

func TestResolveBuildEnvironmentUsesGodotSrcOverride(t *testing.T) {
	repoRoot := t.TempDir()

	goPath := filepath.Join(repoRoot, "gopath")
	t.Setenv("GOPATH", goPath)
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("GODOT_SRC", "./custom-godot")

	env, err := shared.ResolveBuildEnvironment(repoRoot, "")
	if err != nil {
		t.Fatalf("ResolveBuildEnvironment returned error: %v", err)
	}

	wantEngineDir := filepath.Join(repoRoot, "custom-godot")
	if env.EngineDir != wantEngineDir {
		t.Fatalf("unexpected engine dir: got %s want %s", env.EngineDir, wantEngineDir)
	}
	if env.GodotSrc != wantEngineDir {
		t.Fatalf("unexpected godot src: got %s want %s", env.GodotSrc, wantEngineDir)
	}
	wantModuleSource := filepath.Join(repoRoot, "godot_modules", "spx")
	if env.SPXModuleSrc != wantModuleSource {
		t.Fatalf("unexpected SPX module source: got %s want %s", env.SPXModuleSrc, wantModuleSource)
	}
}

func TestResolveBuildEnvironmentUsesSPXModuleSourceOverride(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("SPX_MODULE_SRC", filepath.Join("external", "spx"))

	env, err := shared.ResolveBuildEnvironment(repoRoot, "")
	if err != nil {
		t.Fatalf("ResolveBuildEnvironment returned error: %v", err)
	}

	want := filepath.Join(repoRoot, "external", "spx")
	if env.SPXModuleSrc != want {
		t.Fatalf("unexpected SPX module source: got %s want %s", env.SPXModuleSrc, want)
	}
}

func TestBuildEnvironmentShellExports(t *testing.T) {
	version := mustDefaultRuntimeVersion(t)
	runtimeLock := release.DefaultRuntimeLock()
	env := shared.BuildEnvironment{
		ProjectDir:      "/repo",
		EngineDir:       "/repo/godot src",
		GodotSrc:        "/repo/godot src",
		SPXModuleSrc:    "/repo/custom modules/spx",
		EngineVersion:   runtimeLock.Godot.Version,
		GoPath:          "/tmp/go path",
		Version:         version,
		GodotRepository: runtimeLock.Godot.Repository,
		GodotRef:        runtimeLock.Godot.Ref,
		GodotCommit:     runtimeLock.Godot.Commit,
		TemplateDir:     "/tmp/templates",
		Platform:        "linux",
		Arch:            "x86_64",
	}

	out := env.ShellExports()
	for _, key := range []string{
		"export PROJ_DIR=",
		"export ENGINE_DIR=",
		"export GODOT_SRC=",
		"export SPX_MODULE_SRC=",
		"export ENGINE_VERSION=",
		"export GOPATH=",
		"export VERSION=",
		"export GODOT_REPOSITORY=",
		"export GODOT_REF=",
		"export GODOT_COMMIT=",
		"export TEMPLATE_DIR=",
		"export PLATFORM=",
		"export ARCH=",
	} {
		if !strings.Contains(out, key) {
			t.Fatalf("missing key %s in shell exports: %s", key, out)
		}
	}
}

func TestResolveMacOSVulkanSDKRootPrefersEnvOverride(t *testing.T) {
	homeDir := t.TempDir()
	override := filepath.Join(homeDir, "custom-sdk")
	mustWriteFile(t, filepath.Join(override, "bin", "vulkaninfo"), []byte("bin"))

	got, err := shared.ResolveMacOSVulkanSDKRoot(homeDir, override)
	if err != nil {
		t.Fatalf("ResolveMacOSVulkanSDKRoot returned error: %v", err)
	}
	if got != override {
		t.Fatalf("sdk root = %s, want %s", got, override)
	}
}

func TestResolveMacOSVulkanSDKRootSelectsLatestInstalledVersion(t *testing.T) {
	homeDir := t.TempDir()
	mustWriteFile(t, filepath.Join(homeDir, "VulkanSDK", "1.3.99.0", "macOS", "bin", "vulkaninfo"), []byte("old"))
	mustWriteFile(t, filepath.Join(homeDir, "VulkanSDK", "1.3.296.0", "macOS", "bin", "vulkaninfo"), []byte("new"))

	got, err := shared.ResolveMacOSVulkanSDKRoot(homeDir, "")
	if err != nil {
		t.Fatalf("ResolveMacOSVulkanSDKRoot returned error: %v", err)
	}
	want := filepath.Join(homeDir, "VulkanSDK", "1.3.296.0", "macOS")
	if got != want {
		t.Fatalf("sdk root = %s, want %s", got, want)
	}
}

func TestEnsureEngineSourceRunsCloneWhenMissing(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("GODOT_SRC", "./custom-godot")

	type invocation struct {
		name string
		args []string
	}
	var calls []invocation
	err := shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		calls = append(calls, invocation{name: name, args: append([]string(nil), args...)})
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureEngineSource returned error: %v", err)
	}
	if len(calls) != 5 {
		t.Fatalf("command count = %d, want 5: %#v", len(calls), calls)
	}
	wantDst := filepath.Join(repoRoot, "custom-godot")
	stagingDir := calls[0].args[1]
	if calls[0].name != "git" || calls[0].args[2] != "init" || filepath.Dir(stagingDir) != repoRoot {
		t.Fatalf("init call = %#v, want sibling staging directory for %s", calls[0], wantDst)
	}
	if got := calls[4].args; len(got) < 2 || got[len(got)-2] != "--detach" {
		t.Fatalf("checkout call = %#v, want detached checkout", calls[4])
	}
	if info, err := os.Stat(wantDst); err != nil || !info.IsDir() {
		t.Fatalf("committed engine directory = %v, %v", info, err)
	}
}

func TestEnsureEngineSourceRejectsUnpinnedExistingDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("GODOT_SRC", "./custom-godot")
	mustMkdirAll(t, filepath.Join(repoRoot, "custom-godot"))

	called := false
	err := shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("EnsureEngineSource should reject an existing directory that is not the pinned Git checkout")
	}
	if !strings.Contains(err.Error(), "inspect existing Godot source") {
		t.Fatalf("EnsureEngineSource error = %v, want source inspection error", err)
	}
	if called {
		t.Fatal("clone callback should not run for an existing engine directory")
	}
}

func TestResolveJDKShellExportsIncludesPATHWhenJavaHomeExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin")

	binDir := filepath.Join(home, "custom-jdk", "bin")
	mustWriteFile(t, filepath.Join(binDir, "java"), []byte("bin"))
	t.Setenv("JAVA_HOME", filepath.Join(home, "custom-jdk"))

	exports, err := toolpkg.ResolveJDKShellExports()
	if err != nil {
		t.Fatalf("ResolveJDKShellExports returned error: %v", err)
	}
	if exports["JAVA_HOME"] == "" {
		t.Fatalf("missing JAVA_HOME export: %#v", exports)
	}
	if !strings.Contains(exports["PATH"], binDir) {
		t.Fatalf("PATH export does not include java bin: %s", exports["PATH"])
	}
}
