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
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var engineCommonArgs = []string{
	"optimize=size",
	"use_volk=no",
	"deprecated=no",
	"minizip=yes",
	"openxr=false",
	"vulkan=false",
	"graphite=false",
	"disable_3d_physics=true",
	"module_msdfgen_enabled=false",
	"module_text_server_adv_enabled=true",
	"module_text_server_fb_enabled=false",
	"builtin_harfbuzz=true",
	"modules_enabled_by_default=true",
	"module_gdscript_enabled=true",
	"module_freetype_enabled=true",
	"module_minimp3_enabled=true",
	"module_svg_enabled=true",
	"module_jpg_enabled=true",
	"module_ogg_enabled=true",
	"module_regex_enabled=true",
	"module_zip_enabled=true",
	"module_godot_physics_2d_enabled=true",
}

var engineExtraOptArgs = []string{"disable_3d=true"}

func buildEngine(cfg engineBuildConfig, repoRoot string) error {
	if cfg.target == "editor" && cfg.platform == "web" {
		cfg.target = "template"
	}

	buildEnv, commandEnv, err := prepareEngineBuildEnvironment(repoRoot, cfg.platform)
	if err != nil {
		return err
	}
	plan, err := resolveEngineBuildShellPlan(repoRoot, envExportEngineBuildShellConfig(cfg))
	if err != nil {
		return err
	}

	switch cfg.target {
	case "editor":
		return buildEngineEditor(buildEnv, commandEnv, plan)
	case "template":
		return buildEngineTemplate(buildEnv, commandEnv, plan)
	default:
		return fmt.Errorf("unsupported build target: %s", cfg.target)
	}
}

func prepareEngineBuildEnvironment(repoRoot, requestedPlatform string) (buildEnvironment, map[string]string, error) {
	runOptionalStreamingCommand("", "xgo", "version")
	runOptionalStreamingCommand("", "go", "version")

	buildEnv, err := resolveBuildEnvironment(repoRoot, requestedPlatform)
	if err != nil {
		return buildEnvironment{}, nil, err
	}
	if err := os.MkdirAll(buildEnv.TemplateDir, 0o755); err != nil {
		return buildEnvironment{}, nil, err
	}

	printBuildEnvironmentSummary(buildEnv)

	if err := setupSCons(); err != nil {
		return buildEnvironment{}, nil, err
	}
	if err := ensureEngineSource(repoRoot, func(name string, args ...string) error {
		return buildEnvRunStreaming("", name, args...)
	}); err != nil {
		return buildEnvironment{}, nil, err
	}

	commandEnv := currentEnvMap()
	if runtime.GOOS == "darwin" && buildEnv.Platform == "ios" {
		fmt.Fprintf(os.Stdout, "Installing macOS Vulkan SDK...\n")
		if err := runStreamingCommand(buildEnv.EngineDir, filepath.Join("misc", "scripts", "install_vulkan_sdk_macos.sh")); err != nil {
			return buildEnvironment{}, nil, err
		}
		if homeDir, err := os.UserHomeDir(); err == nil {
			if sdkRoot, err := resolveMacOSVulkanSDKRoot(homeDir, os.Getenv("VULKAN_SDK")); err == nil {
				commandEnv["VULKAN_SDK"] = sdkRoot
				commandEnv["PATH"] = prependToPath(commandEnv["PATH"], filepath.Join(sdkRoot, "bin"))
				fmt.Fprintf(os.Stdout, "Using macOS Vulkan SDK from %s\n", sdkRoot)
				if _, err := runCommandOutputWithEnv("", envMapToSlice(commandEnv), "vulkaninfo", "--summary"); err != nil {
					fmt.Fprintf(os.Stdout, "Warning: vulkaninfo check failed. Continuing with the configured Vulkan SDK.\n")
				}
			} else {
				fmt.Fprintf(os.Stdout, "Warning: vulkaninfo was not found after Vulkan SDK setup. Continuing anyway.\n")
			}
		}
	}

	return buildEnv, commandEnv, nil
}

func printBuildEnvironmentSummary(buildEnv buildEnvironment) {
	fmt.Fprintf(os.Stdout, "PROJ_DIR=%s\n", buildEnv.ProjectDir)
	fmt.Fprintf(os.Stdout, "ENGINE_DIR=%s\n", buildEnv.EngineDir)
	fmt.Fprintf(os.Stdout, "GODOT_SRC=%s\n", buildEnv.GodotSrc)
	fmt.Fprintf(os.Stdout, "ENGINE_VERSION=%s\n", buildEnv.EngineVersion)
	fmt.Fprintf(os.Stdout, "GOPATH=%s\n", buildEnv.GoPath)
	fmt.Fprintf(os.Stdout, "VERSION=%s\n", buildEnv.Version)
	fmt.Fprintf(os.Stdout, "Platform: %s\n", buildEnv.Platform)
	fmt.Fprintf(os.Stdout, "Architecture: %s\n", buildEnv.Arch)
	fmt.Fprintf(os.Stdout, "Destination directory: %s\n", buildEnv.TemplateDir)
	fmt.Fprintf(os.Stdout, "Source tag: %s\n", buildEnv.EngineGitTag)
}

func buildEngineEditor(buildEnv buildEnvironment, commandEnv map[string]string, plan engineBuildShellPlan) error {
	fmt.Fprintf(os.Stdout, "scons target=editor dev_build=yes %s\n", strings.Join(engineCommonArgs, " "))
	args := append([]string{"target=editor", "dev_build=yes"}, engineCommonArgs...)
	if plan.EditorUseVSProj {
		args = append(args, "vsproj=yes")
	}
	if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, "scons", args...); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Destination binary path: %s\n", plan.EditorDestination)
	return copyFile(filepath.Join(buildEnv.EngineDir, plan.EditorSource), plan.EditorDestination)
}

func buildEngineTemplate(buildEnv buildEnvironment, commandEnv map[string]string, plan engineBuildShellPlan) error {
	fmt.Fprintf(os.Stdout, "Output directory: %s\n", buildEnv.TemplateDir)
	fmt.Fprintf(os.Stdout, "Destination binary path: %s\n", filepath.Join(buildEnv.GoPath, "bin", "gdspxrt"+buildEnv.Version))

	switch plan.Platform {
	case "linux", "windows", "macos":
		args := append([]string{"platform=" + plan.TemplateSConsPlatform, "target=template_release"}, engineCommonArgs...)
		if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, "scons", args...); err != nil {
			return err
		}
		return copyFile(filepath.Join(buildEnv.EngineDir, plan.TemplateSource), plan.TemplateDestination)
	case "ios":
		if err := runLockedEngineScriptWithEnv(buildEnv.EngineDir, commandEnv, sconsScript(plan.TemplateSConsCommands)); err != nil {
			return err
		}
		return copyFile(filepath.Join(buildEnv.EngineDir, "bin", "godot_ios.zip"), filepath.Join(buildEnv.TemplateDir, "ios.zip"))
	case "android":
		if err := setupJDK(); err != nil {
			return err
		}
		if jdkExports, err := resolveJDKShellExports(); err == nil {
			commandEnv = mergeStringMaps(commandEnv, jdkExports)
		}
		if err := runLockedEngineScriptWithEnv(buildEnv.EngineDir, commandEnv, sconsScript(plan.TemplateSConsCommands)); err != nil {
			return err
		}
		if len(plan.TemplatePostCommands) > 0 {
			if err := runLockedEngineScriptWithEnv(filepath.Join(buildEnv.EngineDir, plan.TemplatePostDir), commandEnv, strings.Join(plan.TemplatePostCommands, "\n")); err != nil {
				return err
			}
		}
		if err := copyGlobMatches(filepath.Join(buildEnv.EngineDir, "bin", "android*.apk"), buildEnv.TemplateDir); err != nil {
			return err
		}
		return copyFile(filepath.Join(buildEnv.EngineDir, "bin", "android_source.zip"), filepath.Join(buildEnv.TemplateDir, "android_source.zip"))
	case "web":
		if err := setupEMSDK(); err != nil {
			return err
		}
		if emsdkExports, err := resolveEMSDKShellExports(); err == nil {
			commandEnv = mergeStringMaps(commandEnv, emsdkExports)
		}
		webArgs := []string{"platform=web", "target=template_release"}
		webArgs = append(webArgs, engineCommonArgs...)
		webArgs = append(webArgs, engineExtraOptArgs...)
		webArgs = append(webArgs, "threads="+plan.WebThreads)
		if plan.WebProxyToPThread {
			webArgs = append(webArgs, "proxy_to_pthread=true")
		}
		fmt.Fprintf(os.Stdout, "scons %s\n", strings.Join(webArgs, " "))
		if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, "scons", webArgs...); err != nil {
			return err
		}
		srcZip := filepath.Join(buildEnv.EngineDir, "bin", "godot.web.template_release.wasm32"+plan.WebThreadSuffix+".zip")
		if err := waitForNonEmptyFile(srcZip, 5*time.Second); err != nil {
			return err
		}
		if err := copyFile(srcZip, plan.WebCachedTemplateZip); err != nil {
			return err
		}
		return populateWebTemplateCopies(srcZip, buildEnv.TemplateDir)
	default:
		return fmt.Errorf("unknown platform: %s", plan.Platform)
	}
}

func sconsScript(commands []string) string {
	lines := make([]string, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		lines = append(lines, "scons "+strings.Join(append(append([]string{}, engineCommonArgs...), strings.Fields(command)...), " "))
	}
	return strings.Join(lines, "\n")
}

func runLockedEngineCommandWithEnv(workdir string, env map[string]string, name string, args ...string) error {
	lockDir := filepath.Join(filepath.Dir(workdir), ".spx_build_lock")
	if filepath.Base(workdir) == "godot" {
		lockDir = filepath.Join(workdir, ".spx_build_lock")
	}
	return withEngineBuildLock(lockDir, func() error {
		return runTrackedEngineCommandWithEnv(workdir, env, name, args...)
	})
}

func runLockedEngineScriptWithEnv(workdir string, env map[string]string, script string) error {
	lockDir := filepath.Join(filepath.Dir(workdir), ".spx_build_lock")
	if filepath.Base(workdir) == "godot" {
		lockDir = filepath.Join(workdir, ".spx_build_lock")
	}
	return withEngineBuildLock(lockDir, func() error {
		return runTrackedEngineCommandWithEnv(workdir, env, "bash", "-lc", script)
	})
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func copyGlobMatches(pattern, dstDir string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := copyFile(match, filepath.Join(dstDir, filepath.Base(match))); err != nil {
			return err
		}
	}
	return nil
}

func populateWebTemplateCopies(srcZip, templateDir string) error {
	if matches, err := filepath.Glob(filepath.Join(templateDir, "web_*.zip")); err == nil {
		for _, match := range matches {
			if filepath.Clean(match) == filepath.Clean(srcZip) {
				continue
			}
			if err := os.RemoveAll(match); err != nil {
				return err
			}
		}
	}

	names := []string{
		"web_dlink_nothreads_debug.zip",
		"web_dlink_nothreads_release.zip",
		"web_nothreads_debug.zip",
		"web_nothreads_release.zip",
		"web_dlink_debug.zip",
		"web_dlink_release.zip",
		"web_debug.zip",
		"web_release.zip",
	}
	for _, name := range names {
		if err := copyFile(srcZip, filepath.Join(templateDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func runOptionalStreamingCommand(workdir, name string, args ...string) {
	_ = buildEnvRunStreaming(workdir, name, args...)
}

func waitForNonEmptyFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("waiting for %s: %w", path, err)
			}
			return fmt.Errorf("waiting for %s: file is still empty", path)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
