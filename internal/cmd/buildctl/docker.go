package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const dockerImageVersion = "4.x-f36"

func runDockerBuildImages(cfg dockerBuildImagesConfig) error {
	repoRoot, err := findRepoRoot()
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
	repoRoot, err := findRepoRoot()
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

	logsDir := filepath.Join(repoRoot, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Starting multi-platform Godot builds...")
	fmt.Fprintf(os.Stdout, "Godot source path: %s\n", godotPath)
	fmt.Fprintln(os.Stdout, "----------------------------------------")

	iosEnv := []string{
		"IOS_SDK_PATH='/root/ioscross/arm64/SDK/iPhoneOS17.0.sdk'",
		"IOS_TOOLCHAIN_PATH='/root/ioscross/arm64'",
		"ios_triple='arm-apple-darwin11-'",
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "ios", append([]string{"target=template_debug", "ios_simulator=no"}, iosEnv...)...); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "ios", append([]string{"target=template_release", "ios_simulator=no", "generate_bundle=yes"}, iosEnv...)...); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "ios", append([]string{"target=template_debug", "ios_simulator=yes", "arch=arm64", "generate_bundle=yes"}, iosEnv...)...); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_android_debug_arm32.log"), nil, "scons", "platform=android", "target=template_debug", "arch=arm32"); err != nil {
		return err
	}
	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_android_debug_arm64.log"), nil, "scons", "platform=android", "target=template_debug", "arch=arm64"); err != nil {
		return err
	}
	if err := runLoggedCommand(filepath.Join(godotPath, "platform", "android", "java"), filepath.Join(logsDir, "godot_android_gradle_debug.log"), nil, "./gradlew", "generateGodotTemplates"); err != nil {
		return err
	}

	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "osx", "osxcross_sdk=darwin23", "arch=arm64", "vulkan=false", "target=template_release"); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_web_init.log"), nil, "./tools/init_web.sh"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "android", "target=template_release", "arch=arm32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "android", "target=template_release", "arch=arm64"); err != nil {
		return err
	}
	if err := runLoggedCommand(filepath.Join(godotPath, "platform", "android", "java"), filepath.Join(logsDir, "godot_android_gradle_release.log"), nil, "./gradlew", "generateGodotTemplates"); err != nil {
		return err
	}

	if err := runLoggedCommand(godotPath, filepath.Join(logsDir, "godot_linux.log"), nil, "scons", "platform=linux", "target=template_release"); err != nil {
		return err
	}

	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "windows", "target=template_debug", "arch=x86_32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "windows", "target=template_release", "arch=x86_32"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "windows", "target=template_debug", "arch=x86_64"); err != nil {
		return err
	}
	if err := runPodmanSConsBuild(podmanPath, logsDir, godotPath, "windows", "target=template_release", "arch=x86_64"); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "All builds completed successfully!")
	fmt.Fprintln(os.Stdout, "Build logs can be found in the logs directory")
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
	if !fileExists(resolved) {
		return "", fmt.Errorf("godot source directory does not exist: %s", resolved)
	}
	return resolved, nil
}

func runPodmanSConsBuild(podmanPath, logsDir, godotPath, platform string, sconsArgs ...string) error {
	fmt.Fprintf(os.Stdout, "Building for platform: %s\n", platform)
	fmt.Fprintf(os.Stdout, "Build arguments: %s\n", strings.Join(sconsArgs, " "))
	fmt.Fprintf(os.Stdout, "Platform: %s\n", platform)
	fmt.Fprintf(os.Stdout, "Using image version: %s\n", dockerImageVersion)
	fmt.Fprintf(os.Stdout, "Godot source path: %s\n", godotPath)
	fmt.Fprintf(os.Stdout, "Scons arguments: %s\n", strings.Join(sconsArgs, " "))
	fmt.Fprintln(os.Stdout, "----------------------------------------")

	args := buildPodmanSConsArgs(godotPath, platform, sconsArgs, stdinHasTTY())
	logPath := filepath.Join(logsDir, fmt.Sprintf("godot_%s.log", platform))
	if err := runLoggedCommand("", logPath, nil, podmanPath, args...); err != nil {
		return fmt.Errorf("build failed for platform %s: %w", platform, err)
	}

	fmt.Fprintf(os.Stdout, "Build completed successfully for %s\n", platform)
	fmt.Fprintln(os.Stdout, "----------------------------------------")
	return nil
}

func buildPodmanSConsArgs(godotPath, platform string, sconsArgs []string, useTTY bool) []string {
	args := []string{"run", "--rm"}
	if useTTY {
		args = append(args, "-it")
	}
	args = append(args,
		"-w", "/root/godot",
		"-v", godotPath+":/root/godot:z",
		fmt.Sprintf("godot-%s:%s", platform, dockerImageVersion),
		"scons", "platform="+platform,
	)
	return append(args, sconsArgs...)
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
