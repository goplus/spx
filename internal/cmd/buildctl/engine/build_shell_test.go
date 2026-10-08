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

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestBuildShellPlanShellExportsRoundTrip(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell is unavailable")
	}
	quoted := "space 'single' \"double\" $literal `literal` \\backslash\nnext line"
	for _, tt := range []struct {
		name string
		plan BuildShellPlan
		want map[string]string
	}{
		{
			name: "empty",
			want: map[string]string{},
		},
		{
			name: "empty-command-lists",
			plan: BuildShellPlan{TemplateSConsCommands: []string{}, TemplatePostCommands: []string{}},
			want: map[string]string{},
		},
		{
			name: "editor",
			plan: BuildShellPlan{Target: "editor", Platform: "linux", EditorSource: quoted, EditorDestination: quoted},
			want: map[string]string{
				"EDITOR_SOURCE": quoted, "EDITOR_DESTINATION": quoted, "EDITOR_USE_VSPROJ": "false",
			},
		},
		{
			name: "editor-vsproj",
			plan: BuildShellPlan{Target: "editor", Platform: "windows", EditorSource: quoted, EditorUseVSProj: true},
			want: map[string]string{
				"EDITOR_SOURCE": quoted, "EDITOR_DESTINATION": "", "EDITOR_USE_VSPROJ": "true",
			},
		},
		{
			name: "template-commands",
			plan: BuildShellPlan{
				Target: "template", Platform: "linux", TemplateSConsPlatform: "linuxbsd",
				TemplateSource: quoted, TemplateDestination: quoted, TemplatePostDir: quoted,
				TemplateSConsCommands: []string{quoted, ""}, TemplatePostCommands: []string{"", quoted},
			},
			want: map[string]string{
				"TEMPLATE_SCONS_PLATFORM": "linuxbsd", "TEMPLATE_SOURCE": quoted, "TEMPLATE_DESTINATION": quoted,
				"TEMPLATE_POST_DIR":            quoted,
				"TEMPLATE_SCONS_COMMAND_COUNT": "2", "TEMPLATE_SCONS_COMMAND_1": quoted, "TEMPLATE_SCONS_COMMAND_2": "",
				"TEMPLATE_POST_COMMAND_COUNT": "2", "TEMPLATE_POST_COMMAND_1": "", "TEMPLATE_POST_COMMAND_2": quoted,
			},
		},
		{
			name: "web-normal",
			plan: BuildShellPlan{Target: "template", Platform: "web", WebThreads: "no", WebThreadSuffix: ".nothreads", WebCachedTemplateZip: quoted},
			want: map[string]string{
				"WEB_THREADS": "no", "WEB_THREAD_SUFFIX": ".nothreads", "WEB_CACHED_TEMPLATE_ZIP": quoted, "WEB_PROXY_TO_PTHREAD": "false",
			},
		},
		{
			name: "web-worker",
			plan: BuildShellPlan{Target: "template", Platform: "web", WebThreads: "yes", WebProxyToPThread: true, WebCachedTemplateZip: quoted},
			want: map[string]string{
				"WEB_THREADS": "yes", "WEB_THREAD_SUFFIX": "", "WEB_CACHED_TEMPLATE_ZIP": quoted, "WEB_PROXY_TO_PTHREAD": "true",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := map[string]string{
				"TARGET": tt.plan.Target, "PLATFORM": tt.plan.Platform,
				"TEMPLATE_SCONS_COMMAND_COUNT": "0", "TEMPLATE_POST_COMMAND_COUNT": "0",
			}
			for key, value := range tt.want {
				want[key] = value
			}
			keys := []string{
				"TARGET", "PLATFORM", "EDITOR_SOURCE", "EDITOR_DESTINATION", "EDITOR_USE_VSPROJ",
				"TEMPLATE_SCONS_PLATFORM", "TEMPLATE_SOURCE", "TEMPLATE_DESTINATION", "TEMPLATE_POST_DIR",
				"TEMPLATE_SCONS_COMMAND_COUNT", "TEMPLATE_SCONS_COMMAND_0", "TEMPLATE_SCONS_COMMAND_1", "TEMPLATE_SCONS_COMMAND_2", "TEMPLATE_SCONS_COMMAND_3",
				"TEMPLATE_POST_COMMAND_COUNT", "TEMPLATE_POST_COMMAND_0", "TEMPLATE_POST_COMMAND_1", "TEMPLATE_POST_COMMAND_2", "TEMPLATE_POST_COMMAND_3",
				"WEB_THREADS", "WEB_THREAD_SUFFIX", "WEB_CACHED_TEMPLATE_ZIP", "WEB_PROXY_TO_PTHREAD",
			}
			var probe strings.Builder
			for _, key := range keys {
				// Record presence separately so an unset export cannot pass as an empty value.
				fmt.Fprintf(&probe, "printf '%%s\\000' \"${ENGINE_BUILD_%s+x}\" \"${ENGINE_BUILD_%s-}\"\n", key, key)
			}
			// Read in a child shell to verify these are exports, not just shell assignments.
			cmd := exec.Command(shell, "-c", tt.plan.ShellExports()+"exec \"$1\" -c \"$2\"", "shell-exports-test", shell, probe.String())
			cmd.Env = []string{}
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "ENGINE_BUILD_") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("evaluate shell exports: %v\n%s", err, output)
			}
			fields := strings.Split(string(output), "\x00")
			if len(fields) != 2*len(keys)+1 || fields[len(fields)-1] != "" {
				t.Fatalf("unexpected shell output: %q", output)
			}
			got := map[string]string{}
			for i, key := range keys {
				if fields[2*i] == "x" {
					got[key] = fields[2*i+1]
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("shell exports = %#v, want %#v", got, want)
			}
		})
	}
}

func TestResolveEngineBuildShellPlanDesktopMappings(t *testing.T) {
	repoRoot := t.TempDir()
	goPath := filepath.Join(repoRoot, "go path's")
	t.Setenv("GOPATH", goPath)
	t.Setenv("HOME", repoRoot)
	t.Setenv("USERPROFILE", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	version := release.DefaultRuntimeLock().RuntimeVersion
	arch, ok := map[string]string{"amd64": "x86_64", "386": "x86_32", "arm64": "arm64", "arm": "arm32"}[runtime.GOARCH]
	if !ok {
		t.Skipf("unsupported host architecture: %s", runtime.GOARCH)
	}
	for _, tt := range []struct {
		platform, sconsPlatform, editorSuffix, templateSuffix string
		useVSProj                                             bool
	}{
		{"linux", "linuxbsd", "", "", false},
		{"macos", "macos", "", "", false},
		{"windows", "windows", ".exe", ".exe", true},
	} {
		for _, target := range []string{"editor", "template"} {
			t.Run(tt.platform+"/"+target, func(t *testing.T) {
				plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{Target: target, Platform: tt.platform})
				if err != nil {
					t.Fatal(err)
				}
				want := BuildShellPlan{Target: target, Platform: tt.platform}
				if target == "editor" {
					want.EditorSource = filepath.Join("bin", "godot."+tt.sconsPlatform+".editor.dev."+arch)
					want.EditorDestination = filepath.Join(goPath, "bin", "gdspx"+version+tt.editorSuffix)
					want.EditorUseVSProj = tt.useVSProj
				} else {
					want.TemplateSConsPlatform = tt.sconsPlatform
					want.TemplateSource = filepath.Join("bin", "godot."+tt.sconsPlatform+".template_release."+arch+tt.templateSuffix)
					want.TemplateDestination = filepath.Join(goPath, "bin", "gdspxrt"+version+tt.templateSuffix)
				}
				if !reflect.DeepEqual(plan, want) {
					t.Fatalf("build plan = %#v, want %#v", plan, want)
				}
			})
		}
	}
}

func TestParseEnvExportEngineBuildShellArgsWebDefaultMode(t *testing.T) {
	cfg, err := ParseEnvExportEngineBuildShellArgs([]string{"--target", "template", "--platform", "web"})
	if err != nil {
		t.Fatalf("parseEnvExportEngineBuildShellArgs returned error: %v", err)
	}
	if cfg.Mode != "normal" {
		t.Fatalf("expected normal mode, got %s", cfg.Mode)
	}
}

func TestResolveEngineBuildShellPlanTreatsWebEditorAsTemplate(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "editor",
		Platform: "web",
		Mode:     "normal",
	})
	if err != nil {
		t.Fatalf("resolve Web editor build plan: %v", err)
	}
	if plan.Target != "template" || plan.WebThreads != "no" {
		t.Fatalf("Web editor build plan = %#v, want normal template plan", plan)
	}
}

func TestResolveEngineBuildShellPlanTreatsEnvironmentWebEditorAsTemplate(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("PLATFORM", "web")

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{Target: "editor"})
	if err != nil {
		t.Fatalf("resolve environment-selected Web editor build plan: %v", err)
	}
	if plan.Target != "template" || plan.Platform != "web" || plan.WebThreads != "no" {
		t.Fatalf("environment-selected Web editor build plan = %#v, want normal Web template plan", plan)
	}
}

func TestResolveEngineBuildShellPlanIOSMatrix(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "template",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	if len(plan.TemplateSConsCommands) != 6 {
		t.Fatalf("ios template command count = %d, want 6", len(plan.TemplateSConsCommands))
	}
	if got := plan.TemplateSConsCommands[0]; got != "platform=ios target=template_debug ios_simulator=yes arch=arm64" {
		t.Fatalf("unexpected first ios command: %s", got)
	}
	if got := plan.TemplateSConsCommands[5]; got != "platform=ios target=template_release ios_simulator=no generate_bundle=yes" {
		t.Fatalf("unexpected last ios command: %s", got)
	}
	for _, command := range plan.TemplateSConsCommands {
		if strings.Contains(strings.ToLower(command), "vulkan=") {
			t.Fatalf("iOS command must inherit vulkan=false from the shared profile: %s", command)
		}
	}
}

func TestResolveEngineBuildShellPlanAndroidMatrix(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "template",
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	if len(plan.TemplateSConsCommands) != 4 {
		t.Fatalf("android template command count = %d, want 4", len(plan.TemplateSConsCommands))
	}
	if plan.TemplatePostDir != filepath.Join("platform", "android", "java") {
		t.Fatalf("android post dir = %s", plan.TemplatePostDir)
	}
	if len(plan.TemplatePostCommands) != 1 || plan.TemplatePostCommands[0] != "./gradlew generateGodotTemplates" {
		t.Fatalf("unexpected android post commands: %#v", plan.TemplatePostCommands)
	}
}

func TestResolveEngineBuildShellPlanEditorUsesHostArtifactNames(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	version := release.DefaultRuntimeLock().RuntimeVersion

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{Target: "editor"})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(plan.EditorSource, "godot.macos.editor.dev.") {
			t.Fatalf("unexpected editor source: %s", plan.EditorSource)
		}
		if !strings.HasSuffix(plan.EditorDestination, "gdspx"+version) {
			t.Fatalf("unexpected editor destination: %s", plan.EditorDestination)
		}
		if plan.EditorUseVSProj {
			t.Fatal("vsproj should be disabled on darwin")
		}
	case "linux":
		if !strings.Contains(plan.EditorSource, "godot.linuxbsd.editor.dev.") {
			t.Fatalf("unexpected editor source: %s", plan.EditorSource)
		}
		if !strings.HasSuffix(plan.EditorDestination, "gdspx"+version) {
			t.Fatalf("unexpected editor destination: %s", plan.EditorDestination)
		}
		if plan.EditorUseVSProj {
			t.Fatal("vsproj should be disabled on linux")
		}
	case "windows":
		if !strings.Contains(plan.EditorSource, "godot.windows.editor.dev.") {
			t.Fatalf("unexpected editor source: %s", plan.EditorSource)
		}
		if !strings.HasSuffix(plan.EditorDestination, "gdspx"+version+".exe") {
			t.Fatalf("unexpected editor destination: %s", plan.EditorDestination)
		}
		if !plan.EditorUseVSProj {
			t.Fatal("vsproj should be enabled on windows")
		}
	}
}

func TestWebBuildModeDefaultsAndThreading(t *testing.T) {
	for _, tt := range []struct {
		mode, threads, suffix string
		proxy                 bool
	}{
		{"", "no", ".nothreads", false},
		{"normal", "no", ".nothreads", false},
		{"worker", "yes", "", true},
		{"minigame", "no", ".nothreads", false},
		{"miniprogram", "no", ".nothreads", false},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			plan, err := resolveEngineBuildShellPlan(buildEnvironment{GoPath: "go", Version: "test"}, BuildConfig{
				Target: "template", Platform: "web", Mode: tt.mode,
			})
			if err != nil {
				t.Fatal(err)
			}
			if plan.WebThreads != tt.threads || plan.WebProxyToPThread != tt.proxy || plan.WebThreadSuffix != tt.suffix {
				t.Fatalf("unexpected threading: %#v", plan)
			}
			if want := filepath.Join("go", "bin", "gdspxtest_webpack.zip"); plan.WebCachedTemplateZip != want {
				t.Fatalf("cached template = %q, want %q", plan.WebCachedTemplateZip, want)
			}
		})
	}
	_, err := resolveEngineBuildShellPlan(buildEnvironment{}, BuildConfig{Target: "template", Platform: "web", Mode: "unknown"})
	if err == nil || err.Error() != "unsupported web-mode: unknown" {
		t.Fatalf("error = %v", err)
	}
}
