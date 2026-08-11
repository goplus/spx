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
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
)

type buildEnvironment = shared.BuildEnvironment

var buildEnvRunStreaming = shared.RunStreamingCommand

func BuildEngine(cfg BuildConfig, repoRoot string) error {
	return buildEngineWithEnvironmentPreparer(cfg, repoRoot, prepareEngineBuildEnvironment)
}

func buildEngineWithEnvironmentPreparer(cfg BuildConfig, repoRoot string, prepare func(string, string) (buildEnvironment, map[string]string, string, error)) error {
	if cfg.Target == "editor" && cfg.Platform == "web" {
		cfg.Target = "template"
	}

	spxModuleSource, err := shared.ResolveSPXModuleSource(repoRoot)
	if err != nil {
		return err
	}
	profile, err := loadSConsProfile(spxModuleSource)
	if err != nil {
		return err
	}
	buildEnv, commandEnv, sconsCommand, err := prepare(repoRoot, cfg.Platform)
	if err != nil {
		return err
	}
	plan, err := ResolveEngineBuildShellPlan(repoRoot, cfg)
	if err != nil {
		return err
	}

	switch cfg.Target {
	case "editor":
		return buildEngineEditor(buildEnv, commandEnv, sconsCommand, profile.EditorReleaseArgs(), plan)
	case "template":
		return buildEngineTemplate(buildEnv, commandEnv, sconsCommand, profile.TemplateReleaseArgs(), plan)
	default:
		return fmt.Errorf("unsupported build target: %s", cfg.Target)
	}
}

func prepareEngineBuildEnvironment(repoRoot, requestedPlatform string) (buildEnvironment, map[string]string, string, error) {
	runOptionalStreamingCommand("", "xgo", "version")
	runOptionalStreamingCommand("", "go", "version")

	buildEnv, err := shared.ResolveBuildEnvironment(repoRoot, requestedPlatform)
	if err != nil {
		return buildEnvironment{}, nil, "", err
	}
	if err := os.MkdirAll(buildEnv.TemplateDir, 0o755); err != nil {
		return buildEnvironment{}, nil, "", err
	}

	printBuildEnvironmentSummary(buildEnv)

	sconsCommand, err := toolpkg.EnsureSCons()
	if err != nil {
		return buildEnvironment{}, nil, "", err
	}
	if err := shared.EnsureEngineSource(repoRoot, func(name string, args ...string) error {
		return buildEnvRunStreaming("", name, args...)
	}); err != nil {
		return buildEnvironment{}, nil, "", err
	}

	commandEnv, err := shared.CurrentBuildEnv()
	if err != nil {
		return buildEnvironment{}, nil, "", err
	}
	return buildEnv, commandEnv, sconsCommand, nil
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
	fmt.Fprintf(os.Stdout, "Godot source: %s@%s (%s)\n", buildEnv.GodotRepository, buildEnv.GodotCommit, buildEnv.GodotRef)
	fmt.Fprintf(os.Stdout, "SPX module source: %s\n", buildEnv.SPXModuleSrc)
}

func buildEngineEditor(buildEnv buildEnvironment, commandEnv map[string]string, sconsCommand string, profileArgs []string, plan BuildShellPlan) error {
	args := sconsBuildArgs([]string{"target=editor", "dev_build=yes"}, profileArgs, buildEnv.SPXModuleSrc)
	if plan.EditorUseVSProj {
		args = append(args, "vsproj=yes")
	}
	fmt.Fprintf(os.Stdout, "scons %s\n", shellJoin(args))
	if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, sconsCommand, args...); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Destination binary path: %s\n", plan.EditorDestination)
	return shared.CopyFile(filepath.Join(buildEnv.EngineDir, plan.EditorSource), plan.EditorDestination)
}

func buildEngineTemplate(buildEnv buildEnvironment, commandEnv map[string]string, sconsCommand string, profileArgs []string, plan BuildShellPlan) error {
	fmt.Fprintf(os.Stdout, "Output directory: %s\n", buildEnv.TemplateDir)
	fmt.Fprintf(os.Stdout, "Destination binary path: %s\n", filepath.Join(buildEnv.GoPath, "bin", "gdspxrt"+buildEnv.Version))

	switch plan.Platform {
	case "linux", "windows", "macos":
		args := sconsBuildArgs([]string{"platform=" + plan.TemplateSConsPlatform, "target=template_release"}, profileArgs, buildEnv.SPXModuleSrc)
		if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, sconsCommand, args...); err != nil {
			return err
		}
		return shared.CopyFile(filepath.Join(buildEnv.EngineDir, plan.TemplateSource), plan.TemplateDestination)
	case "ios":
		if err := runLockedEngineScriptWithEnv(buildEnv.EngineDir, commandEnv, sconsBuildScript(sconsCommand, profileArgs, buildEnv.SPXModuleSrc, plan.TemplateSConsCommands)); err != nil {
			return err
		}
		return shared.CopyFile(filepath.Join(buildEnv.EngineDir, "bin", "godot_ios.zip"), filepath.Join(buildEnv.TemplateDir, "ios.zip"))
	case "android":
		if err := toolpkg.SetupJDK(); err != nil {
			return err
		}
		jdkExports, err := toolpkg.ResolveJDKShellExports()
		if err != nil {
			return fmt.Errorf("resolve JDK build environment: %w", err)
		}
		commandEnv = mergeStringMaps(commandEnv, jdkExports)
		if err := runLockedEngineScriptWithEnv(buildEnv.EngineDir, commandEnv, sconsBuildScript(sconsCommand, profileArgs, buildEnv.SPXModuleSrc, plan.TemplateSConsCommands)); err != nil {
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
		return shared.CopyFile(filepath.Join(buildEnv.EngineDir, "bin", "android_source.zip"), filepath.Join(buildEnv.TemplateDir, "android_source.zip"))
	case "web":
		if err := toolpkg.SetupEMSDK(); err != nil {
			return err
		}
		emsdkExports, err := toolpkg.ResolveEMSDKShellExports()
		if err != nil {
			return fmt.Errorf("resolve Emscripten build environment: %w", err)
		}
		commandEnv = mergeStringMaps(commandEnv, emsdkExports)
		webArgs := []string{"platform=web", "target=template_release"}
		webArgs = append(webArgs, "threads="+plan.WebThreads)
		if plan.WebProxyToPThread {
			webArgs = append(webArgs, "proxy_to_pthread=true")
		}
		webArgs = sconsBuildArgs(webArgs, profileArgs, buildEnv.SPXModuleSrc)
		fmt.Fprintf(os.Stdout, "scons %s\n", shellJoin(webArgs))
		if err := runLockedEngineCommandWithEnv(buildEnv.EngineDir, commandEnv, sconsCommand, webArgs...); err != nil {
			return err
		}
		srcZip := filepath.Join(buildEnv.EngineDir, "bin", "godot.web.template_release.wasm32"+plan.WebThreadSuffix+".zip")
		if err := waitForNonEmptyFile(srcZip, 5*time.Second); err != nil {
			return err
		}
		if err := shared.CopyFile(srcZip, plan.WebCachedTemplateZip); err != nil {
			return err
		}
		return populateWebTemplateCopies(srcZip, buildEnv.TemplateDir)
	default:
		return fmt.Errorf("unknown platform: %s", plan.Platform)
	}
}

func sconsScript(commands []string) string {
	return sconsScriptWithCommand("scons", commands)
}

func sconsScriptWithCommand(sconsCommand string, commands []string) string {
	return sconsBuildScript(sconsCommand, nil, "", commands)
}

func sconsBuildScript(sconsCommand string, profileArgs []string, spxModuleSource string, commands []string) string {
	lines := make([]string, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		args := sconsBuildArgs(strings.Fields(command), profileArgs, spxModuleSource)
		lines = append(lines, shellJoin(append([]string{sconsCommand}, args...)))
	}
	return strings.Join(lines, "\n")
}

func sconsBuildArgs(dynamicArgs, profileArgs []string, spxModuleSource string) []string {
	args := make([]string, 0, len(dynamicArgs)+len(profileArgs)+1)
	// Apply the shared profile first and the explicit build axes second. SCons
	// keeps the last value for a repeated key, so this order makes any
	// platform/target override deliberate and consistent across local builds.
	args = append(args, profileArgs...)
	args = append(args, dynamicArgs...)
	if spxModuleSource != "" {
		args = append(args, "custom_modules="+spxModuleSource)
	}
	return args
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = shared.ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
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
		if err := shared.CopyFile(match, filepath.Join(dstDir, filepath.Base(match))); err != nil {
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
		if err := shared.CopyFile(srcZip, filepath.Join(templateDir, name)); err != nil {
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
