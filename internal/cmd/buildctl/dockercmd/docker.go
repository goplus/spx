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

package dockercmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

const dockerImageVersion = "4.x-f36"

const podmanSPXModulePath = "/root/spx-module"

func runDockerBuildImages(cfg dockerBuildImagesConfig) error {
	printLegacyDockerWarning()
	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		return fmt.Errorf("podman needs to be in PATH for this command to work")
	}

	dockerDir := filepath.Join(repoRoot, "internal", "cmd", "buildctl", "docker")
	logsDir := filepath.Join(dockerDir, "logs")
	filesRoot := filepath.Join(dockerDir, "files")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}

	baseArgs := []string{
		"build",
		"--build-arg", "HTTP_PROXY=" + cfg.proxyURL,
		"--build-arg", "HTTPS_PROXY=" + cfg.proxyURL,
		"-t", "godot-fedora:" + dockerImageVersion,
		"-f", "Dockerfile.base",
		".",
	}
	if err := runLoggedCommand(dockerDir, filepath.Join(logsDir, "base.log"), map[string]string{
		"HTTP_PROXY":  cfg.proxyURL,
		"HTTPS_PROXY": cfg.proxyURL,
	}, podmanPath, baseArgs...); err != nil {
		return err
	}

	linuxArgs := []string{
		"build",
		"--build-arg", "HTTP_PROXY=" + cfg.proxyURL,
		"--build-arg", "HTTPS_PROXY=" + cfg.proxyURL,
		"--build-arg", "img_version=" + dockerImageVersion,
		"-v", filesRoot + ":/root/files:z",
		"-t", "godot-linux:" + dockerImageVersion,
		"-f", "Dockerfile.linux",
		".",
	}
	return runLoggedCommand(dockerDir, filepath.Join(logsDir, "linux.log"), map[string]string{
		"HTTP_PROXY":  cfg.proxyURL,
		"HTTPS_PROXY": cfg.proxyURL,
	}, podmanPath, linuxArgs...)
}

func runDockerBuildEngine(cfg dockerBuildEngineConfig) error {
	printLegacyDockerWarning()
	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return err
	}

	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		return fmt.Errorf("podman needs to be in PATH for this command to work")
	}

	godotPath, err := resolveDockerGodotPath(repoRoot, cfg.godotSrc)
	if err != nil {
		return err
	}
	spxModulePath, err := resolveDockerSPXModulePath(repoRoot)
	if err != nil {
		return err
	}
	profile, err := shared.LoadSConsProfile(spxModulePath)
	if err != nil {
		return err
	}
	// Export template variants share one static profile; target=template_debug
	// and target=template_release remain per-build arguments, as in release CI.
	templateProfileArgs := profile.TemplateReleaseArgs()

	logsDir := filepath.Join(repoRoot, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Starting multi-platform Godot builds...")
	fmt.Fprintf(os.Stdout, "Godot source path: %s\n", godotPath)
	fmt.Fprintf(os.Stdout, "SPX module source path: %s\n", spxModulePath)
	fmt.Fprintln(os.Stdout, "----------------------------------------")

	iosEnv := []string{
		"IOS_SDK_PATH='/root/ioscross/arm64/SDK/iPhoneOS17.0.sdk'",
		"IOS_TOOLCHAIN_PATH='/root/ioscross/arm64'",
		"ios_triple='arm-apple-darwin11-'",
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "ios", templateProfileArgs, append([]string{"target=template_debug", "ios_simulator=no"}, iosEnv...)...); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "ios", templateProfileArgs, append([]string{"target=template_release", "ios_simulator=no", "generate_bundle=yes"}, iosEnv...)...); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "ios", templateProfileArgs, append([]string{"target=template_debug", "ios_simulator=yes", "arch=arm64", "generate_bundle=yes"}, iosEnv...)...); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_android_debug_arm32.log"), nil, "scons", buildDockerSConsArgs("android", []string{"target=template_debug", "arch=arm32"}, templateProfileArgs, spxModulePath)...); err != nil {
		return err
	}
	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_android_debug_arm64.log"), nil, "scons", buildDockerSConsArgs("android", []string{"target=template_debug", "arch=arm64"}, templateProfileArgs, spxModulePath)...); err != nil {
		return err
	}
	if err := runLoggedCommand(filepath.Join(godotPath, "platform", "android", "java"), filepath.Join(logsDir, "godot_android_gradle_debug.log"), nil, "./gradlew", "generateGodotTemplates"); err != nil {
		return err
	}

	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "osx", templateProfileArgs, "osxcross_sdk=darwin23", "arch=arm64", "target=template_release"); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_web_init.log"), nil, "./tools/init_web.sh"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "android", templateProfileArgs, "target=template_release", "arch=arm32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "android", templateProfileArgs, "target=template_release", "arch=arm64"); err != nil {
		return err
	}
	if err := runLoggedCommand(filepath.Join(godotPath, "platform", "android", "java"), filepath.Join(logsDir, "godot_android_gradle_release.log"), nil, "./gradlew", "generateGodotTemplates"); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_linux.log"), nil, "scons", buildDockerSConsArgs("linux", []string{"target=template_release"}, templateProfileArgs, spxModulePath)...); err != nil {
		return err
	}

	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "windows", templateProfileArgs, "target=template_debug", "arch=x86_32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "windows", templateProfileArgs, "target=template_release", "arch=x86_32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "windows", templateProfileArgs, "target=template_debug", "arch=x86_64"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, "windows", templateProfileArgs, "target=template_release", "arch=x86_64"); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "All builds completed successfully.")
	fmt.Fprintln(os.Stdout, "Build logs are available in the logs directory.")
	return nil
}

func resolveDockerGodotPath(repoRoot, explicit string) (string, error) {
	godotPath := explicit
	if godotPath == "" {
		godotPath = os.Getenv("GODOT_SRC")
	}
	if godotPath == "" {
		godotPath = filepath.Join(repoRoot, "godot")
	}
	if !filepath.IsAbs(godotPath) {
		godotPath = filepath.Join(repoRoot, godotPath)
	}
	resolved, err := filepath.Abs(godotPath)
	if err != nil {
		return "", err
	}
	if !shared.FileExists(resolved) {
		return "", fmt.Errorf("godot source directory does not exist: %s", resolved)
	}
	return resolved, nil
}

func resolveDockerSPXModulePath(repoRoot string) (string, error) {
	modulePath, err := shared.ResolveSPXModuleSource(repoRoot)
	if err != nil {
		return "", err
	}
	if !shared.FileExists(modulePath) {
		return "", fmt.Errorf("SPX module source directory does not exist: %s", modulePath)
	}
	return modulePath, nil
}

func runPodmanSConsBuild(podmanPath, logsDir, godotPath, spxModulePath, platform string, profileArgs []string, sconsArgs ...string) error {
	fmt.Fprintf(os.Stdout, "Building for platform: %s\n", platform)
	fmt.Fprintf(os.Stdout, "Using image version: %s\n", dockerImageVersion)
	fmt.Fprintf(os.Stdout, "Godot source path: %s\n", godotPath)
	effectiveSConsArgs := buildDockerSConsArgs(platform, sconsArgs, profileArgs, podmanSPXModulePath)
	fmt.Fprintf(os.Stdout, "SCons arguments: %s\n", strings.Join(effectiveSConsArgs, " "))
	fmt.Fprintln(os.Stdout, "----------------------------------------")

	args := buildPodmanSConsArgs(godotPath, spxModulePath, platform, effectiveSConsArgs, stdinHasTTY())
	logPath := filepath.Join(logsDir, fmt.Sprintf("godot_%s.log", platform))
	if err := runLoggedCommand("", logPath, nil, podmanPath, args...); err != nil {
		return fmt.Errorf("build failed for platform %s: %w", platform, err)
	}

	fmt.Fprintf(os.Stdout, "Completed build for %s\n", platform)
	fmt.Fprintln(os.Stdout, "----------------------------------------")
	return nil
}

func buildPodmanSConsArgs(godotPath, spxModulePath, platform string, sconsArgs []string, useTTY bool) []string {
	args := []string{"run", "--rm"}
	if useTTY {
		args = append(args, "-it")
	}
	args = append(args,
		"-w", "/root/godot",
		"-v", godotPath+":/root/godot:z",
		"-v", spxModulePath+":"+podmanSPXModulePath+":z",
		fmt.Sprintf("godot-%s:%s", platform, dockerImageVersion),
		"scons",
	)
	return append(args, sconsArgs...)
}

func buildDockerSConsArgs(platform string, buildArgs, profileArgs []string, spxModulePath string) []string {
	args := make([]string, 0, 2+len(buildArgs)+len(profileArgs))
	args = append(args, profileArgs...)
	args = append(args, "platform="+platform)
	args = append(args, buildArgs...)
	args = append(args, "custom_modules="+spxModulePath)
	return args
}

func runLoggedCommand(dir, logPath string, extraEnv map[string]string, name string, args ...string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	writer := io.MultiWriter(os.Stdout, logFile)
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func stdinHasTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func printLegacyDockerWarning() {
	fmt.Fprintln(os.Stderr, "Warning: buildctl docker is a legacy, unsupported workflow with toolchain pins independent of runtime.lock.json.")
}
