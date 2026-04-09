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
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v2/internal/cmd/buildctl/tool"
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
}

func TestBuildEnvironmentShellExports(t *testing.T) {
	version := mustDefaultRuntimeVersion(t)
	env := shared.BuildEnvironment{
		ProjectDir:    "/repo",
		EngineDir:     "/repo/godot src",
		GodotSrc:      "/repo/godot src",
		EngineVersion: shared.EngineBuildVersion,
		GoPath:        "/tmp/go path",
		Version:       version,
		EngineGitTag:  "spx" + version,
		TemplateDir:   "/tmp/templates",
		Platform:      "linux",
		Arch:          "x86_64",
	}

	out := env.ShellExports()
	for _, key := range []string{
		"export PROJ_DIR=",
		"export ENGINE_DIR=",
		"export GODOT_SRC=",
		"export ENGINE_VERSION=",
		"export GOPATH=",
		"export VERSION=",
		"export ENGINE_GIT_TAG=",
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

	var gotName string
	var gotArgs []string
	err := shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureEngineSource returned error: %v", err)
	}
	if gotName != "git" {
		t.Fatalf("command = %s, want git", gotName)
	}
	wantDst := filepath.Join(repoRoot, "custom-godot")
	if gotArgs[len(gotArgs)-1] != wantDst {
		t.Fatalf("clone dst = %s, want %s", gotArgs[len(gotArgs)-1], wantDst)
	}
}

func TestEnsureEngineSourceSkipsCloneWhenPresent(t *testing.T) {
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
	if err != nil {
		t.Fatalf("EnsureEngineSource returned error: %v", err)
	}
	if called {
		t.Fatal("git clone should not be called when engine dir already exists")
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
