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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

	if want := release.DefaultRuntimeLock().RuntimeVersion; env.Version != want {
		t.Fatalf("runtime version = %q, want locked version %q", env.Version, want)
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
	env := shared.BuildEnvironment{
		RepoRoot:        "not exported",
		ProjectDir:      "/project with spaces",
		EngineDir:       "/engine's source",
		GodotSrc:        "/godot",
		SPXModuleSrc:    "/module",
		EngineVersion:   "4.5.stable",
		GoPath:          "/go",
		Version:         "v1",
		GodotRepository: "https://example.com/godot",
		GodotRef:        "refs/heads/main",
		GodotCommit:     "abc123",
		TemplateDir:     "",
		Platform:        "linux",
		Arch:            "x86_64",
	}
	const want = `export PROJ_DIR='/project with spaces'
export ENGINE_DIR='/engine'"'"'s source'
export GODOT_SRC='/godot'
export SPX_MODULE_SRC='/module'
export ENGINE_VERSION='4.5.stable'
export GOPATH='/go'
export VERSION='v1'
export GODOT_REPOSITORY='https://example.com/godot'
export GODOT_REF='refs/heads/main'
export GODOT_COMMIT='abc123'
export TEMPLATE_DIR=''
export PLATFORM='linux'
export ARCH='x86_64'
`
	if got := env.ShellExports(); got != want {
		t.Fatalf("ShellExports() = %q, want %q", got, want)
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

func TestBuildEnvironmentRetainsTypeIdentity(t *testing.T) {
	env := shared.BuildEnvironment{}
	if got, want := reflect.TypeOf(env).Name(), "buildEnvironment"; got != want {
		t.Fatalf("type name = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%T", env), "shared.buildEnvironment"; got != want {
		t.Fatalf("formatted type = %q, want %q", got, want)
	}
}

func TestShellQuoteExact(t *testing.T) {
	tests := []struct{ value, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"a b", "'a b'"},
		{"a'b", `'a'"'"'b'`},
		{"$HOME;$(command)", "'$HOME;$(command)'"},
		{"line 1\nline 2", "'line 1\nline 2'"},
	}
	for _, test := range tests {
		if got := shared.ShellQuote(test.value); got != test.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestMacOSVulkanSDKShellExportsExact(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/path with spaces")
	const sdk = "/sdk's root"
	want := "export VULKAN_SDK='/sdk'\"'\"'s root'\n" +
		"export PATH=" + shared.ShellQuote(filepath.Join(sdk, "bin")+":/usr/bin:/path with spaces") + "\n"
	if got := shared.MacOSVulkanSDKShellExports(sdk); got != want {
		t.Fatalf("MacOSVulkanSDKShellExports() = %q, want %q", got, want)
	}
}

func TestCurrentBuildEnvReturnsIndependentCopy(t *testing.T) {
	const key = "SPX_SHARED_ENV_COPY_TEST"
	t.Setenv(key, "original=value")
	// Provide valid Darwin inputs so this test does not depend on xcrun.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SDKROOT", t.TempDir())
	t.Setenv("CC", shared.ShellQuote(executable))
	t.Setenv("CXX", shared.ShellQuote(executable))
	for _, name := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"} {
		t.Setenv(name, "")
	}
	first, err := shared.CurrentBuildEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got := first[key]; got != "original=value" {
		t.Fatalf("environment value = %q, want original=value", got)
	}
	first[key] = "changed"
	if got := os.Getenv(key); got != "original=value" {
		t.Fatalf("process environment was changed to %q", got)
	}
	second, err := shared.CurrentBuildEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got := second[key]; got != "original=value" {
		t.Fatalf("second environment was changed to %q", got)
	}
}

func TestEnvMapToSliceSortsAndCopies(t *testing.T) {
	env := map[string]string{"Z": "last", "A": "first=part", "EMPTY": ""}
	want := []string{"A=first=part", "EMPTY=", "Z=last"}
	got := shared.EnvMapToSlice(env)
	if !slices.Equal(got, want) {
		t.Fatalf("EnvMapToSlice() = %q, want %q", got, want)
	}
	got[0] = "changed"
	if env["A"] != "first=part" {
		t.Fatal("result changed the input map")
	}
	env["Z"] = "changed"
	if got[2] != "Z=last" {
		t.Fatal("input map changed the result")
	}
	if got := shared.EnvMapToSlice(nil); got == nil || len(got) != 0 {
		t.Fatalf("EnvMapToSlice(nil) = %#v, want an empty non-nil slice", got)
	}
}
