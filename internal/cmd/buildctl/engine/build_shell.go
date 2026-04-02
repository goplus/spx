package engine

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
)

type envExportEngineBuildShellConfig struct {
	target   string
	platform string
	mode     string
}

type engineBuildShellPlan struct {
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

func parseEnvExportEngineBuildShellArgs(args []string) (envExportEngineBuildShellConfig, error) {
	cfg := envExportEngineBuildShellConfig{}

	fs := flag.NewFlagSet("env export-engine-build-shell", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.target, "target", "", "engine build target: editor or template")
	fs.StringVar(&cfg.platform, "platform", "", "build platform: android, ios, web, linux, windows, or macos")
	fs.StringVar(&cfg.mode, "mode", "", "web mode: normal, worker, minigame, or miniprogram")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl env export-engine-build-shell --target editor|template [--platform android|ios|web|linux|windows|macos] [--mode normal|worker|minigame|miniprogram]")
	}

	if err := fs.Parse(args); err != nil {
		return envExportEngineBuildShellConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return envExportEngineBuildShellConfig{}, errUsage
	}
	switch cfg.target {
	case "editor", "template":
	default:
		return envExportEngineBuildShellConfig{}, fmt.Errorf("unsupported build target: %s", cfg.target)
	}
	if err := validateOptionalPlatform(cfg.platform); err != nil {
		return envExportEngineBuildShellConfig{}, err
	}
	if cfg.platform == "web" && cfg.mode == "" {
		cfg.mode = "normal"
	}
	if cfg.mode != "" {
		if cfg.platform != "web" {
			return envExportEngineBuildShellConfig{}, fmt.Errorf("--mode requires --platform web")
		}
		if err := validateWebMode(cfg.mode); err != nil {
			return envExportEngineBuildShellConfig{}, err
		}
	}
	return cfg, nil
}

func resolveEngineBuildShellPlan(repoRoot string, cfg envExportEngineBuildShellConfig) (engineBuildShellPlan, error) {
	env, err := resolveBuildEnvironment(repoRoot, "")
	if err != nil {
		return engineBuildShellPlan{}, err
	}

	effectivePlatform := cfg.platform
	if effectivePlatform == "" {
		effectivePlatform = env.Platform
	}

	plan := engineBuildShellPlan{
		Target:   cfg.target,
		Platform: effectivePlatform,
	}

	switch cfg.target {
	case "editor":
		editorSource, editorDestination, editorUseVSProj, err := resolveEditorBuildPaths(env)
		if err != nil {
			return engineBuildShellPlan{}, err
		}
		plan.EditorSource = editorSource
		plan.EditorDestination = editorDestination
		plan.EditorUseVSProj = editorUseVSProj
	case "template":
		if isDesktopPlatform(effectivePlatform) {
			sconsPlatform, templateSource, templateDestination, err := resolveDesktopTemplateBuildPaths(env, effectivePlatform)
			if err != nil {
				return engineBuildShellPlan{}, err
			}
			plan.TemplateSConsPlatform = sconsPlatform
			plan.TemplateSource = templateSource
			plan.TemplateDestination = templateDestination
		}
		if effectivePlatform == "web" {
			webThreads, webProxy, webThreadSuffix, err := resolveWebTemplateBuildConfig(cfg.mode)
			if err != nil {
				return engineBuildShellPlan{}, err
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
	if err := validateWebMode(mode); err != nil {
		return "", false, "", err
	}
	switch mode {
	case "normal":
		return "yes", false, "", nil
	case "worker":
		return "yes", true, "", nil
	case "minigame", "miniprogram":
		return "no", false, ".nothreads", nil
	default:
		return "", false, "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func (plan engineBuildShellPlan) shellExports() string {
	lines := []string{
		"export ENGINE_BUILD_TARGET=" + shellQuote(plan.Target),
		"export ENGINE_BUILD_PLATFORM=" + shellQuote(plan.Platform),
	}
	if plan.EditorSource != "" {
		lines = append(lines,
			"export ENGINE_BUILD_EDITOR_SOURCE="+shellQuote(plan.EditorSource),
			"export ENGINE_BUILD_EDITOR_DESTINATION="+shellQuote(plan.EditorDestination),
		)
		if plan.EditorUseVSProj {
			lines = append(lines, "export ENGINE_BUILD_EDITOR_USE_VSPROJ='true'")
		} else {
			lines = append(lines, "export ENGINE_BUILD_EDITOR_USE_VSPROJ='false'")
		}
	}
	if plan.TemplateSConsPlatform != "" {
		lines = append(lines,
			"export ENGINE_BUILD_TEMPLATE_SCONS_PLATFORM="+shellQuote(plan.TemplateSConsPlatform),
			"export ENGINE_BUILD_TEMPLATE_SOURCE="+shellQuote(plan.TemplateSource),
			"export ENGINE_BUILD_TEMPLATE_DESTINATION="+shellQuote(plan.TemplateDestination),
		)
	}
	lines = appendIndexedShellExports(lines, "ENGINE_BUILD_TEMPLATE_SCONS_COMMAND", plan.TemplateSConsCommands)
	if plan.TemplatePostDir != "" {
		lines = append(lines, "export ENGINE_BUILD_TEMPLATE_POST_DIR="+shellQuote(plan.TemplatePostDir))
	}
	lines = appendIndexedShellExports(lines, "ENGINE_BUILD_TEMPLATE_POST_COMMAND", plan.TemplatePostCommands)
	if plan.WebThreads != "" {
		lines = append(lines,
			"export ENGINE_BUILD_WEB_THREADS="+shellQuote(plan.WebThreads),
			"export ENGINE_BUILD_WEB_THREAD_SUFFIX="+shellQuote(plan.WebThreadSuffix),
			"export ENGINE_BUILD_WEB_CACHED_TEMPLATE_ZIP="+shellQuote(plan.WebCachedTemplateZip),
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
		lines = append(lines, fmt.Sprintf("export %s_%d=%s", prefix, i+1, shellQuote(value)))
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
		"platform=ios vulkan=True target=template_debug ios_simulator=yes arch=arm64",
		"platform=ios vulkan=True target=template_debug ios_simulator=yes arch=x86_64",
		"platform=ios vulkan=True target=template_release ios_simulator=yes arch=arm64",
		"platform=ios vulkan=True target=template_release ios_simulator=yes arch=x86_64 generate_bundle=yes",
		"platform=ios vulkan=True target=template_debug ios_simulator=no",
		"platform=ios vulkan=True target=template_release ios_simulator=no generate_bundle=yes",
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
