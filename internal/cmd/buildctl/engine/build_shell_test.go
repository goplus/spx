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

func TestResolveEngineBuildShellPlanWebWorker(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	version := mustDefaultRuntimeVersion(t)

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "template",
		Platform: "web",
		Mode:     "worker",
	})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	if plan.WebThreads != "yes" {
		t.Fatalf("web threads = %s, want yes", plan.WebThreads)
	}
	if !plan.WebProxyToPThread {
		t.Fatal("expected proxy_to_pthread to be enabled for worker mode")
	}
	if plan.WebThreadSuffix != "" {
		t.Fatalf("web thread suffix = %s, want empty", plan.WebThreadSuffix)
	}
	if got, want := plan.WebCachedTemplateZip, filepath.Join(repoRoot, "gopath", "bin", "gdspx"+version+"_webpack.zip"); got != want {
		t.Fatalf("cached template zip = %s, want %s", got, want)
	}
}

func TestResolveEngineBuildShellPlanWebNormal(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	version := mustDefaultRuntimeVersion(t)

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "template",
		Platform: "web",
		Mode:     "normal",
	})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	if plan.WebThreads != "no" {
		t.Fatalf("web threads = %s, want no", plan.WebThreads)
	}
	if plan.WebProxyToPThread {
		t.Fatal("proxy_to_pthread should be disabled for normal mode")
	}
	if plan.WebThreadSuffix != ".nothreads" {
		t.Fatalf("web thread suffix = %s, want .nothreads", plan.WebThreadSuffix)
	}
	if got, want := plan.WebCachedTemplateZip, filepath.Join(repoRoot, "gopath", "bin", "gdspx"+version+"_webpack.zip"); got != want {
		t.Fatalf("cached template zip = %s, want %s", got, want)
	}
}

func TestResolveEngineBuildShellPlanWebMiniGame(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))

	plan, err := ResolveEngineBuildShellPlan(repoRoot, BuildConfig{
		Target:   "template",
		Platform: "web",
		Mode:     "minigame",
	})
	if err != nil {
		t.Fatalf("resolveEngineBuildShellPlan returned error: %v", err)
	}

	if plan.WebThreads != "no" {
		t.Fatalf("web threads = %s, want no", plan.WebThreads)
	}
	if plan.WebProxyToPThread {
		t.Fatal("proxy_to_pthread should be disabled for minigame mode")
	}
	if plan.WebThreadSuffix != ".nothreads" {
		t.Fatalf("web thread suffix = %s, want .nothreads", plan.WebThreadSuffix)
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
