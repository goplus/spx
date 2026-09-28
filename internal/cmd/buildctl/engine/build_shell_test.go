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
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
	version := mustDefaultRuntimeVersion(t)

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
