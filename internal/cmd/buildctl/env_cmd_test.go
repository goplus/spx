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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v4/internal/cmd/buildctl/tool"
	"github.com/goplus/spx/v4/internal/release"
)

func TestRunEnvDispatchAndArgumentErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: shared.ErrUsage},
		{name: "help", args: []string{"help"}},
		{name: "export shell help", args: []string{"export-shell", "--help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown env command "unknown"`},
		{name: "invalid platform", args: []string{"export-shell", "--platform", "plan9"}, contains: "unsupported platform"},
		{name: "extra argument", args: []string{"export-jdk-shell", "extra"}, wantErr: shared.ErrUsage},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := runEnv(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("runEnv(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains != "" {
				if err == nil || !strings.Contains(err.Error(), test.contains) {
					t.Fatalf("runEnv(%q) error = %v, want substring %q", test.args, err, test.contains)
				}
				return
			}
			if err != nil {
				t.Fatalf("runEnv(%q) returned error: %v", test.args, err)
			}
		})
	}
}

func TestParseEnvExportShellArgsDefault(t *testing.T) {
	cfg, err := parseEnvExportShellArgs(nil)
	if err != nil {
		t.Fatalf("parseEnvExportShellArgs returned error: %v", err)
	}
	if cfg.platform != "" {
		t.Fatalf("unexpected platform: %s", cfg.platform)
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

func TestEnvironmentCommandEntryPoints(t *testing.T) {
	commands := []string{"ensure-engine-source", "export-emsdk-shell", "export-engine-build-shell", "export-jdk-shell", "export-shell", "export-macos-vulkan-shell"}
	for _, command := range commands {
		t.Run(command+"/help", func(t *testing.T) {
			if err := run([]string{"env", command, "--help"}); err != nil {
				t.Fatal(err)
			}
		})
		t.Run(command+"/invalid", func(t *testing.T) {
			if err := run([]string{"env", command, "--invalid"}); err == nil {
				t.Fatal("invalid flag was accepted")
			}
		})
	}
	for _, args := range [][]string{{"export-shell", "extra"}, {"export-engine-build-shell", "--target", "editor"}, {"export-shell"}, {"ensure-engine-source"}} {
		t.Run(strings.Join(args, "/")+"/outside-repository", func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := runEnv(args); err == nil {
				t.Fatal("command succeeded outside repository")
			}
		})
	}
	for _, args := range [][]string{{"export-shell"}, {"export-engine-build-shell", "--target", "template"}} {
		t.Run(strings.Join(args, "/")+"/invalid-environment", func(t *testing.T) {
			newBuildctlFixture(t)
			t.Setenv("PLATFORM", "unsupported")
			if err := runEnv(args); err == nil || !strings.Contains(err.Error(), "unsupported platform") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("exports", func(t *testing.T) {
		root := newBuildctlFixture(t)
		t.Setenv("JAVA_HOME", filepath.Join(root, "java's home"))
		for _, tt := range []struct {
			args []string
			want string
		}{
			{[]string{"export-shell", "--platform", "ios"}, "export PLATFORM='ios'"},
			{[]string{"export-engine-build-shell", "--target", "template", "--platform", "web", "--mode", "worker"}, "export ENGINE_BUILD_WEB_PROXY_TO_PTHREAD='true'"},
			{[]string{"export-jdk-shell"}, "export JAVA_HOME=" + shared.ShellQuote(os.Getenv("JAVA_HOME"))},
		} {
			output, err := captureBuildctlOutput(t, func() error { return runEnv(tt.args) })
			if err != nil || !strings.Contains(output, tt.want) {
				t.Fatalf("%v output = %q, %v; want %q", tt.args, output, err, tt.want)
			}
		}
	})
	t.Run("vulkan", func(t *testing.T) {
		root := newBuildctlFixture(t)
		t.Setenv("VULKAN_SDK", filepath.Join(root, "Vulkan SDK"))
		if err := runEnv([]string{"export-macos-vulkan-shell"}); err == nil {
			t.Fatal("missing SDK accepted")
		}
		mustWriteFile(t, filepath.Join(os.Getenv("VULKAN_SDK"), "bin", "vulkaninfo"), nil)
		output, err := captureBuildctlOutput(t, func() error { return runEnv([]string{"export-macos-vulkan-shell"}) })
		if err != nil || !strings.Contains(output, "export VULKAN_SDK="+shared.ShellQuote(os.Getenv("VULKAN_SDK"))) {
			t.Fatalf("Vulkan exports = %q, %v", output, err)
		}
		if runtime.GOOS != "windows" {
			t.Setenv("HOME", "")
			if err := runEnv([]string{"export-macos-vulkan-shell"}); err == nil {
				t.Fatal("missing home accepted")
			}
		}
	})
}

func TestEnvironmentCommandExternalTools(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("tool fixtures require Linux or macOS")
	}
	t.Run("pinned source", func(t *testing.T) {
		root := newBuildctlFixture(t)
		bin := filepath.Join(root, "tools")
		writeBuildctlTool(t, filepath.Join(bin, "git"), "printf '%s\\n' '"+release.DefaultRuntimeLock().Godot.Commit+"'\n")
		t.Setenv("SDKROOT", root)
		t.Setenv("CC", filepath.Join(bin, "git"))
		t.Setenv("CXX", filepath.Join(bin, "git"))
		for _, name := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"} {
			t.Setenv(name, "")
		}
		t.Setenv("PATH", bin)
		// An empty checkout exercises the source acquisition callback, using only fake git.
		if err := runEnv([]string{"ensure-engine-source"}); err != nil {
			t.Fatal(err)
		}
		if err := runEnv([]string{"ensure-engine-source"}); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(os.Getenv("GODOT_SRC")); err != nil || !info.IsDir() {
			t.Fatalf("source = %v, %v", info, err)
		}
	})
	t.Run("emsdk exports", func(t *testing.T) {
		root := newBuildctlFixture(t)
		if err := runEnv([]string{"export-emsdk-shell"}); err == nil {
			t.Fatal("missing SDK accepted")
		}
		sdk := filepath.Join(root, ".local", "share", "emsdk", "emsdk")
		if runtime.GOOS == "darwin" {
			sdk = filepath.Join(root, "Library", "Application Support", "emsdk", "emsdk")
		}
		mustWriteFile(t, filepath.Join(sdk, "emsdk_env.sh"), []byte("export EMSDK="+shared.ShellQuote(sdk)+"\nexport EM_CONFIG="+shared.ShellQuote(filepath.Join(root, "config"))+"\nexport PATH=/usr/bin\n"))
		output, err := captureBuildctlOutput(t, func() error { return runEnv([]string{"export-emsdk-shell"}) })
		want := "export EMSDK=" + shared.ShellQuote(sdk) + "\nexport EM_CONFIG=" + shared.ShellQuote(filepath.Join(root, "config")) + "\nexport PATH='/usr/bin'\n"
		if err != nil || output != want {
			t.Fatalf("EMSDK exports = %q, %v; want %q", output, err, want)
		}
	})
}
