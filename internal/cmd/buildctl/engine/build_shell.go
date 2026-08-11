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
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type BuildShellPlan struct {
	Target                string
	Platform              string
	EditorSource          string
	EditorDestination     string
	EditorUseVSProj       bool
	TemplateSConsPlatform string
	TemplateSource        string
	TemplateDestination   string
	TemplateSConsCommands []string
	TemplatePostDir       string
	TemplatePostCommands  []string
	WebThreads            string
	WebProxyToPThread     bool
	WebThreadSuffix       string
	WebCachedTemplateZip  string
}

func ParseEnvExportEngineBuildShellArgs(args []string) (BuildConfig, error) {
	cfg := BuildConfig{}

	fs := flag.NewFlagSet("env export-engine-build-shell", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.Target, "target", "", "engine build target: editor or template")
	fs.StringVar(&cfg.Platform, "platform", "", "build platform: android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.Mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl env export-engine-build-shell --target editor|template [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return BuildConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return BuildConfig{}, errUsage
	}
	switch cfg.Target {
	case "editor", "template":
	default:
		return BuildConfig{}, fmt.Errorf("unsupported build target: %s", cfg.Target)
	}
	if err := shared.ValidateOptionalPlatform(cfg.Platform); err != nil {
		return BuildConfig{}, err
	}
	if cfg.Platform == "web" && cfg.Mode == "" {
		cfg.Mode = "normal"
	}
	if cfg.Mode != "" {
		if cfg.Platform != "web" {
			return BuildConfig{}, fmt.Errorf("--mode requires --platform web")
		}
		if err := shared.ValidateWebMode(cfg.Mode); err != nil {
			return BuildConfig{}, err
		}
	}
	return cfg, nil
}

func ResolveEngineBuildShellPlan(repoRoot string, cfg BuildConfig) (BuildShellPlan, error) {
	env, err := shared.ResolveBuildEnvironment(repoRoot, cfg.Platform)
	if err != nil {
		return BuildShellPlan{}, err
	}
	return resolveEngineBuildShellPlan(env, cfg)
}

func resolveEngineBuildShellPlan(env buildEnvironment, cfg BuildConfig) (BuildShellPlan, error) {
	effectivePlatform := cfg.Platform
	if effectivePlatform == "" {
		effectivePlatform = env.Platform
	}
	effectiveTarget := cfg.Target
	if effectiveTarget == "editor" && effectivePlatform == "web" {
		effectiveTarget = "template"
	}

	plan := BuildShellPlan{
		Target:   effectiveTarget,
		Platform: effectivePlatform,
	}

	switch effectiveTarget {
	case "editor":
		editorSource, editorDestination, editorUseVSProj, err := resolveEditorBuildPaths(env)
		if err != nil {
			return BuildShellPlan{}, err
		}
		plan.EditorSource = editorSource
		plan.EditorDestination = editorDestination
		plan.EditorUseVSProj = editorUseVSProj
	case "template":
		if isDesktopPlatform(effectivePlatform) {
			sconsPlatform, templateSource, templateDestination, err := resolveDesktopTemplateBuildPaths(env, effectivePlatform)
			if err != nil {
				return BuildShellPlan{}, err
			}
			plan.TemplateSConsPlatform = sconsPlatform
			plan.TemplateSource = templateSource
			plan.TemplateDestination = templateDestination
		}
		if effectivePlatform == "web" {
			webThreads, webProxy, webThreadSuffix, err := resolveWebTemplateBuildConfig(cfg.Mode)
			if err != nil {
				return BuildShellPlan{}, err
			}
			plan.WebThreads = webThreads
			plan.WebProxyToPThread = webProxy
			plan.WebThreadSuffix = webThreadSuffix
			plan.WebCachedTemplateZip = filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspx%s_webpack.zip", env.Version))
		}
		if effectivePlatform == "ios" {
			plan.TemplateSConsCommands = iosTemplateBuildCommands()
		}
		if effectivePlatform == "android" {
			plan.TemplateSConsCommands = androidTemplateBuildCommands()
			plan.TemplatePostDir = filepath.Join("platform", "android", "java")
			plan.TemplatePostCommands = []string{"./gradlew generateGodotTemplates"}
		}
	default:
		return BuildShellPlan{}, fmt.Errorf("unsupported build target: %s", effectiveTarget)
	}

	return plan, nil
}

func resolveEditorBuildPaths(env buildEnvironment) (source string, destination string, useVSProj bool, err error) {
	switch env.Platform {
	case "linux":
		return filepath.Join("bin", fmt.Sprintf("godot.linuxbsd.editor.dev.%s", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspx%s", env.Version)),
			false,
			nil
	case "macos":
		return filepath.Join("bin", fmt.Sprintf("godot.macos.editor.dev.%s", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspx%s", env.Version)),
			false,
			nil
	case "windows":
		return filepath.Join("bin", fmt.Sprintf("godot.windows.editor.dev.%s", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspx%s.exe", env.Version)),
			true,
			nil
	default:
		return "", "", false, fmt.Errorf("unsupported editor host platform: %s", env.Platform)
	}
}

func resolveDesktopTemplateBuildPaths(env buildEnvironment, platform string) (sconsPlatform string, source string, destination string, err error) {
	switch platform {
	case "linux":
		return "linuxbsd",
			filepath.Join("bin", fmt.Sprintf("godot.linuxbsd.template_release.%s", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspxrt%s", env.Version)),
			nil
	case "macos":
		return "macos",
			filepath.Join("bin", fmt.Sprintf("godot.macos.template_release.%s", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspxrt%s", env.Version)),
			nil
	case "windows":
		return "windows",
			filepath.Join("bin", fmt.Sprintf("godot.windows.template_release.%s.exe", env.Arch)),
			filepath.Join(env.GoPath, "bin", fmt.Sprintf("gdspxrt%s.exe", env.Version)),
			nil
	default:
		return "", "", "", fmt.Errorf("unsupported desktop template platform: %s", platform)
	}
}

func resolveWebTemplateBuildConfig(mode string) (threads string, proxyToPThread bool, threadSuffix string, err error) {
	if mode == "" {
		mode = "normal"
	}
	if err := shared.ValidateWebMode(mode); err != nil {
		return "", false, "", err
	}
	switch mode {
	case "normal", "minigame", "miniprogram":
		return "no", false, ".nothreads", nil
	case "worker":
		return "yes", true, "", nil
	default:
		return "", false, "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func (plan BuildShellPlan) ShellExports() string {
	lines := []string{
		"export ENGINE_BUILD_TARGET=" + shared.ShellQuote(plan.Target),
		"export ENGINE_BUILD_PLATFORM=" + shared.ShellQuote(plan.Platform),
	}
	if plan.EditorSource != "" {
		lines = append(lines,
			"export ENGINE_BUILD_EDITOR_SOURCE="+shared.ShellQuote(plan.EditorSource),
			"export ENGINE_BUILD_EDITOR_DESTINATION="+shared.ShellQuote(plan.EditorDestination),
		)
		if plan.EditorUseVSProj {
			lines = append(lines, "export ENGINE_BUILD_EDITOR_USE_VSPROJ='true'")
		} else {
			lines = append(lines, "export ENGINE_BUILD_EDITOR_USE_VSPROJ='false'")
		}
	}
	if plan.TemplateSConsPlatform != "" {
		lines = append(lines,
			"export ENGINE_BUILD_TEMPLATE_SCONS_PLATFORM="+shared.ShellQuote(plan.TemplateSConsPlatform),
			"export ENGINE_BUILD_TEMPLATE_SOURCE="+shared.ShellQuote(plan.TemplateSource),
			"export ENGINE_BUILD_TEMPLATE_DESTINATION="+shared.ShellQuote(plan.TemplateDestination),
		)
	}
	lines = appendIndexedShellExports(lines, "ENGINE_BUILD_TEMPLATE_SCONS_COMMAND", plan.TemplateSConsCommands)
	if plan.TemplatePostDir != "" {
		lines = append(lines, "export ENGINE_BUILD_TEMPLATE_POST_DIR="+shared.ShellQuote(plan.TemplatePostDir))
	}
	lines = appendIndexedShellExports(lines, "ENGINE_BUILD_TEMPLATE_POST_COMMAND", plan.TemplatePostCommands)
	if plan.WebThreads != "" {
		lines = append(lines,
			"export ENGINE_BUILD_WEB_THREADS="+shared.ShellQuote(plan.WebThreads),
			"export ENGINE_BUILD_WEB_THREAD_SUFFIX="+shared.ShellQuote(plan.WebThreadSuffix),
			"export ENGINE_BUILD_WEB_CACHED_TEMPLATE_ZIP="+shared.ShellQuote(plan.WebCachedTemplateZip),
		)
		if plan.WebProxyToPThread {
			lines = append(lines, "export ENGINE_BUILD_WEB_PROXY_TO_PTHREAD='true'")
		} else {
			lines = append(lines, "export ENGINE_BUILD_WEB_PROXY_TO_PTHREAD='false'")
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendIndexedShellExports(lines []string, prefix string, values []string) []string {
	lines = append(lines, fmt.Sprintf("export %s_COUNT='%d'", prefix, len(values)))
	for i, value := range values {
		lines = append(lines, fmt.Sprintf("export %s_%d=%s", prefix, i+1, shared.ShellQuote(value)))
	}
	return lines
}

func isDesktopPlatform(platform string) bool {
	switch platform {
	case "linux", "windows", "macos":
		return true
	default:
		return false
	}
}

func iosTemplateBuildCommands() []string {
	return []string{
		"platform=ios target=template_debug ios_simulator=yes arch=arm64",
		"platform=ios target=template_debug ios_simulator=yes arch=x86_64",
		"platform=ios target=template_release ios_simulator=yes arch=arm64",
		"platform=ios target=template_release ios_simulator=yes arch=x86_64 generate_bundle=yes",
		"platform=ios target=template_debug ios_simulator=no",
		"platform=ios target=template_release ios_simulator=no generate_bundle=yes",
	}
}

func androidTemplateBuildCommands() []string {
	return []string{
		"platform=android target=template_debug arch=arm32",
		"platform=android target=template_debug arch=arm64",
		"platform=android target=template_release arch=arm32",
		"platform=android target=template_release arch=arm64",
	}
}
