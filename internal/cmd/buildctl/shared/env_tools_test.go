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

package shared_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

func TestResolveBuildEnvironment(t *testing.T) {
	for _, test := range []struct {
		name, godotSrc, moduleSrc, platform, wantEngine, wantModule, wantErr string
	}{
		{
			name: "Godot source", godotSrc: "./custom-godot",
			wantEngine: "custom-godot", wantModule: filepath.Join("godot_modules", "spx"),
		},
		{
			name: "SPX module source", moduleSrc: filepath.Join("external", "spx"),
			wantEngine: "godot", wantModule: filepath.Join("external", "spx"),
		},
		{
			name: "invalid platform", platform: "plan9", wantErr: "unsupported platform: plan9",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
			t.Setenv("HOME", repoRoot)
			t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
			t.Setenv("GODOT_SRC", test.godotSrc)
			t.Setenv("SPX_MODULE_SRC", test.moduleSrc)
			t.Setenv("PLATFORM", "")

			env, err := shared.ResolveBuildEnvironment(repoRoot, test.platform)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ResolveBuildEnvironment error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveBuildEnvironment returned error: %v", err)
			}
			if want := release.DefaultRuntimeLock().RuntimeVersion; env.Version != want {
				t.Fatalf("runtime version = %q, want locked version %q", env.Version, want)
			}
			wantEngine := filepath.Join(repoRoot, test.wantEngine)
			if env.EngineDir != wantEngine || env.GodotSrc != wantEngine {
				t.Fatalf("engine dir = %q, Godot source = %q, want %q", env.EngineDir, env.GodotSrc, wantEngine)
			}
			if want := filepath.Join(repoRoot, test.wantModule); env.SPXModuleSrc != want {
				t.Fatalf("SPX module source = %q, want %q", env.SPXModuleSrc, want)
			}
		})
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
	if got := reflect.TypeOf(env).Name(); got != "buildEnvironment" {
		t.Fatalf("type name = %q, want buildEnvironment", got)
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

func TestResolveMacOSVulkanSDKRoot(t *testing.T) {
	for _, test := range []struct {
		name, override, want string
	}{
		{"environment override", "custom-sdk", "custom-sdk"},
		{"latest installed version", "", filepath.Join("VulkanSDK", "1.3.296.0", "macOS")},
	} {
		t.Run(test.name, func(t *testing.T) {
			homeDir := t.TempDir()
			for _, sdk := range []string{
				"custom-sdk",
				filepath.Join("VulkanSDK", "1.3.99.0", "macOS"),
				filepath.Join("VulkanSDK", "1.3.296.0", "macOS"),
			} {
				binDir := filepath.Join(homeDir, sdk, "bin")
				if err := os.MkdirAll(binDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(binDir, "vulkaninfo"), []byte("bin"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			override := ""
			if test.override != "" {
				override = filepath.Join(homeDir, test.override)
			}
			got, err := shared.ResolveMacOSVulkanSDKRoot(homeDir, override)
			if err != nil {
				t.Fatalf("ResolveMacOSVulkanSDKRoot returned error: %v", err)
			}
			if want := filepath.Join(homeDir, test.want); got != want {
				t.Fatalf("SDK root = %q, want %q", got, want)
			}
		})
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

func TestEnsureEngineSourceChecksOutLockedCommit(t *testing.T) {
	repoRoot := t.TempDir()
	engineDir := filepath.Join(repoRoot, "custom-godot")
	t.Setenv("GODOT_SRC", "./custom-godot")

	type invocation struct {
		name string
		args []string
	}
	var got []invocation
	err := shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		got = append(got, invocation{name: name, args: append([]string(nil), args...)})
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureEngineSource returned error: %v", err)
	}

	lock := release.DefaultRuntimeLock()
	if len(got) != 5 || len(got[0].args) != 3 {
		t.Fatalf("git invocations = %#v", got)
	}
	stagingDir := got[0].args[1]
	if filepath.Dir(stagingDir) != repoRoot || stagingDir == engineDir {
		t.Fatalf("staging directory = %q", stagingDir)
	}
	want := []invocation{
		{name: "git", args: []string{"-C", stagingDir, "init"}},
		{name: "git", args: []string{"-C", stagingDir, "remote", "add", "origin", lock.Godot.Repository}},
		{name: "git", args: []string{"-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", lock.Godot.Ref}},
		{name: "git", args: []string{"-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", lock.Godot.Commit}},
		{name: "git", args: []string{"-C", stagingDir, "checkout", "--detach", lock.Godot.Commit}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("git invocations = %#v, want %#v", got, want)
	}
	if info, err := os.Stat(engineDir); err != nil || !info.IsDir() {
		t.Fatalf("committed engine directory = %v, %v", info, err)
	}
}

func TestEnsureEngineSourceDeepensLockedRefWhenCommitFetchFails(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GODOT_SRC", filepath.Join(repoRoot, "godot-source"))

	lock := release.DefaultRuntimeLock()
	var got [][]string
	err := shared.EnsureEngineSource(repoRoot, func(_ string, args ...string) error {
		got = append(got, append([]string(nil), args...))
		if len(args) > 0 && args[len(args)-1] == lock.Godot.Commit && slices.Contains(args, "fetch") {
			return errors.New("unadvertised object")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureEngineSource returned error: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("git invocation count = %d, want 6: %#v", len(got), got)
	}
	wantFallback := []string{"-C", got[0][1], "fetch", "--filter=blob:none", "--unshallow", "origin", lock.Godot.Ref}
	if !reflect.DeepEqual(got[4], wantFallback) {
		t.Fatalf("fallback invocation = %#v, want %#v", got[4], wantFallback)
	}
}

func TestEnsureEngineSourceCleansFailedClone(t *testing.T) {
	repoRoot := t.TempDir()
	engineDir := filepath.Join(repoRoot, "godot-source")
	t.Setenv("GODOT_SRC", engineDir)

	wantErr := errors.New("fetch failed")
	err := shared.EnsureEngineSource(repoRoot, func(_ string, args ...string) error {
		for _, arg := range args {
			if arg == "fetch" {
				return wantErr
			}
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("EnsureEngineSource error = %v", err)
	}
	if _, err := os.Stat(engineDir); !os.IsNotExist(err) {
		t.Fatalf("failed clone left target directory: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(repoRoot, ".godot-source.clone-*")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("failed clone left staging directories: %v", matches)
	}
}

func TestEnsureEngineSourceRejectsUnpinnedExistingDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GODOT_SRC", "./custom-godot")
	if err := os.MkdirAll(filepath.Join(repoRoot, "custom-godot"), 0o755); err != nil {
		t.Fatal(err)
	}

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
