package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

const (
	requiredSConsVersion = "4.7.0"
	requiredJDKMajor     = 17
	requiredEMSDKVersion = "3.1.62"
)

type toolSetupSConsConfig struct{}

type toolSetupJDKConfig struct{}

type toolSetupEMSDKConfig struct{}

type emsdkEnvironment struct {
	rootDir string
	repoDir string
}

var (
	buildEnvLookPath           = exec.LookPath
	buildEnvRunStreaming       = runStreamingCommand
	buildEnvRunOutput          = runCommandOutput
	buildEnvRunOutputWithDir   = runCommandOutputWithEnv
	resolveEMSDKShellExportsFn = resolveEMSDKShellExports
)

func parseToolSetupSConsArgs(args []string) (toolSetupSConsConfig, error) {
	fs := flag.NewFlagSet("tool setup-scons", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool setup-scons")
	}
	if err := fs.Parse(args); err != nil {
		return toolSetupSConsConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolSetupSConsConfig{}, errUsage
	}
	return toolSetupSConsConfig{}, nil
}

func parseToolSetupJDKArgs(args []string) (toolSetupJDKConfig, error) {
	fs := flag.NewFlagSet("tool setup-jdk", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool setup-jdk")
	}
	if err := fs.Parse(args); err != nil {
		return toolSetupJDKConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolSetupJDKConfig{}, errUsage
	}
	return toolSetupJDKConfig{}, nil
}

func parseToolSetupEMSDKArgs(args []string) (toolSetupEMSDKConfig, error) {
	fs := flag.NewFlagSet("tool setup-emsdk", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool setup-emsdk")
	}
	if err := fs.Parse(args); err != nil {
		return toolSetupEMSDKConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolSetupEMSDKConfig{}, errUsage
	}
	return toolSetupEMSDKConfig{}, nil
}

func setupSCons() error {
	python, err := detectPythonCommand()
	if err != nil {
		return err
	}
	if err := buildEnvRunStreaming("", python, "-c", "import sys; print(sys.version)"); err != nil {
		return err
	}
	if err := buildEnvRunStreaming("", python, "-m", "pip", "install", "scons=="+requiredSConsVersion, "--break-system-packages"); err != nil {
		return err
	}
	return buildEnvRunStreaming("", "scons", "--version")
}

func setupJDK() error {
	env, err := resolveJDKShellEnvironment()
	if err != nil {
		return err
	}
	if major, ok := detectJavaMajorVersion(env); ok && major == requiredJDKMajor {
		fmt.Fprintf(os.Stdout, "JDK %d is already installed.\n", requiredJDKMajor)
		return nil
	}

	fmt.Fprintf(os.Stdout, "JDK %d not found. Installing...\n", requiredJDKMajor)
	switch runtime.GOOS {
	case "darwin":
		if _, err := buildEnvLookPath("brew"); err != nil {
			return errors.New("Homebrew not found. Please install Homebrew first or install JDK 17 manually")
		}
		if err := buildEnvRunStreaming("", "brew", "install", "openjdk@17"); err != nil {
			return err
		}
	case "linux":
		if _, err := buildEnvLookPath("apt-get"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "apt-get", "update"); err != nil {
				return err
			}
			if err := buildEnvRunStreaming("", "sudo", "apt-get", "install", "-y", "openjdk-17-jdk"); err != nil {
				return err
			}
		} else if _, err := buildEnvLookPath("dnf"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "dnf", "install", "-y", "java-17-openjdk-devel"); err != nil {
				return err
			}
		} else if _, err := buildEnvLookPath("yum"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "yum", "install", "-y", "java-17-openjdk-devel"); err != nil {
				return err
			}
		} else {
			return errors.New("unsupported Linux distribution. Please install JDK 17 manually")
		}
	case "windows":
		return errors.New("on Windows, please install JDK 17 manually and ensure JAVA_HOME is set")
	default:
		return fmt.Errorf("unsupported OS for JDK setup: %s", runtime.GOOS)
	}

	env, err = resolveJDKShellEnvironment()
	if err != nil {
		return err
	}
	if major, ok := detectJavaMajorVersion(env); ok && major == requiredJDKMajor {
		fmt.Fprintf(os.Stdout, "JDK %d installed successfully.\n", requiredJDKMajor)
		return nil
	}
	return errors.New("failed to install JDK 17. Please install it manually")
}

func setupEMSDK() error {
	env, err := resolveEMSDKEnvironment()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Using emsdk installation directory: %s\n", env.rootDir)
	if err := os.MkdirAll(env.rootDir, 0o755); err != nil {
		return err
	}

	if !fileExists(env.repoDir) {
		fmt.Fprintln(os.Stdout, "emsdk not found in global location, installing emsdk...")
		if err := buildEnvRunStreaming(env.rootDir, "git", "clone", "https://github.com/emscripten-core/emsdk.git"); err != nil {
			return err
		}
		if err := buildEnvRunStreaming(env.repoDir, "./emsdk", "install", requiredEMSDKVersion); err != nil {
			return err
		}
		if err := buildEnvRunStreaming(env.repoDir, "./emsdk", "activate", requiredEMSDKVersion); err != nil {
			return err
		}
		return verifyEMSDK(env)
	}

	currentVersion, ok := detectEMSDKVersion(env)
	if ok {
		fmt.Fprintf(os.Stdout, "current emcc version: %s, target version: %s\n", currentVersion, requiredEMSDKVersion)
	}

	if !ok || currentVersion != requiredEMSDKVersion {
		fmt.Fprintf(os.Stdout, "emcc version mismatch, installing target version %s...\n", requiredEMSDKVersion)
		if err := buildEnvRunStreaming(env.repoDir, "./emsdk", "install", requiredEMSDKVersion); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "emcc version matches, no need to re-install")
	}
	if err := buildEnvRunStreaming(env.repoDir, "./emsdk", "activate", requiredEMSDKVersion); err != nil {
		return err
	}
	return verifyEMSDK(env)
}

func verifyEMSDK(env emsdkEnvironment) error {
	verifyEnv, emppPath, err := resolveEMSDKVerificationEnvironment(env)
	if err != nil {
		return err
	}
	output, err := buildEnvRunOutputWithDir("", envMapToSlice(verifyEnv), emppPath, "--version")
	if err != nil {
		return fmt.Errorf("failed to set up emsdk. Please check the installation: %w", err)
	}
	fmt.Fprintln(os.Stdout, "emsdk is set up successfully:")
	fmt.Fprint(os.Stdout, string(output))
	return nil
}

func resolveEMSDKVerificationEnvironment(env emsdkEnvironment) (map[string]string, string, error) {
	exports, err := resolveEMSDKShellExportsFn()
	if err != nil {
		return nil, "", err
	}

	merged := currentEnvMap()
	for key, value := range exports {
		merged[key] = value
	}

	if _, ok := merged["EM_CONFIG"]; !ok || strings.TrimSpace(merged["EM_CONFIG"]) == "" {
		merged["EM_CONFIG"] = filepath.Join(env.repoDir, ".emscripten")
	}
	if _, ok := merged["EM_CACHE"]; !ok || strings.TrimSpace(merged["EM_CACHE"]) == "" {
		merged["EM_CACHE"] = filepath.Join(env.repoDir, "upstream", "emscripten", "cache")
	}
	if err := os.MkdirAll(merged["EM_CACHE"], 0o755); err != nil {
		return nil, "", err
	}

	emppPath := filepath.Join(env.repoDir, "upstream", "emscripten", emscriptenCPPExecutableName())
	if !fileExists(emppPath) {
		return nil, "", fmt.Errorf("em++ not found at %s", emppPath)
	}
	return merged, emppPath, nil
}

func emscriptenCPPExecutableName() string {
	if runtime.GOOS == "windows" {
		return "em++.bat"
	}
	return "em++"
}

func detectPythonCommand() (string, error) {
	if _, err := buildEnvLookPath("python3"); err == nil {
		return "python3", nil
	}
	if _, err := buildEnvLookPath("python"); err == nil {
		return "python", nil
	}
	return "", errors.New("neither python3 nor python is installed")
}

func resolveJDKShellEnvironment() (map[string]string, error) {
	env := currentEnvMap()
	for _, binDir := range candidateJDKBinDirs() {
		if dirExists(binDir) {
			env["PATH"] = prependToPath(env["PATH"], binDir)
			break
		}
	}

	if javaHome, err := resolveJavaHome(env); err == nil && javaHome != "" {
		env["JAVA_HOME"] = javaHome
		env["PATH"] = prependToPath(env["PATH"], filepath.Join(javaHome, "bin"))
	}
	return env, nil
}

func resolveJavaHome(env map[string]string) (string, error) {
	if javaHome := strings.TrimSpace(env["JAVA_HOME"]); javaHome != "" {
		return javaHome, nil
	}
	if runtime.GOOS == "darwin" {
		for _, binDir := range candidateJDKBinDirs() {
			if dirExists(binDir) {
				prefix := filepath.Dir(binDir)
				home := filepath.Join(prefix, "libexec", "openjdk.jdk", "Contents", "Home")
				if dirExists(home) {
					return home, nil
				}
			}
		}
		output, err := runCommandOutputWithEnv("", envMapToSlice(env), "/usr/libexec/java_home", "-v", strconv.Itoa(requiredJDKMajor))
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}
	return "", errors.New("JAVA_HOME not found")
}

func candidateJDKBinDirs() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return []string{
		"/opt/homebrew/opt/openjdk@17/bin",
		"/usr/local/opt/openjdk@17/bin",
	}
}

func detectJavaMajorVersion(env map[string]string) (int, bool) {
	output, err := runCommandOutputWithEnv("", envMapToSlice(env), "java", "-version")
	if err != nil {
		return 0, false
	}
	return parseJavaMajorVersion(string(output))
}

func parseJavaMajorVersion(output string) (int, bool) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(strings.ToLower(line), "version") {
			continue
		}
		start := strings.Index(line, "\"")
		if start < 0 {
			continue
		}
		rest := line[start+1:]
		end := strings.Index(rest, "\"")
		if end < 0 {
			continue
		}
		version := rest[:end]
		parts := strings.Split(version, ".")
		if len(parts) == 0 {
			continue
		}
		if parts[0] == "1" && len(parts) > 1 {
			major, err := strconv.Atoi(parts[1])
			return major, err == nil
		}
		major, err := strconv.Atoi(parts[0])
		return major, err == nil
	}
	return 0, false
}

func resolveJDKShellExports() (map[string]string, error) {
	env, err := resolveJDKShellEnvironment()
	if err != nil {
		return nil, err
	}
	exports := map[string]string{}
	if javaHome := strings.TrimSpace(env["JAVA_HOME"]); javaHome != "" {
		exports["JAVA_HOME"] = javaHome
		exports["PATH"] = env["PATH"]
	}
	return exports, nil
}

func resolveEMSDKEnvironment() (emsdkEnvironment, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return emsdkEnvironment{}, err
	}
	switch runtime.GOOS {
	case "linux":
		root := filepath.Join(home, ".local", "share", "emsdk")
		return emsdkEnvironment{rootDir: root, repoDir: filepath.Join(root, "emsdk")}, nil
	case "darwin":
		root := filepath.Join(home, "Library", "Application Support", "emsdk")
		return emsdkEnvironment{rootDir: root, repoDir: filepath.Join(root, "emsdk")}, nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return emsdkEnvironment{}, errors.New("APPDATA is not set")
		}
		root := filepath.Join(appData, "emsdk")
		return emsdkEnvironment{rootDir: root, repoDir: filepath.Join(root, "emsdk")}, nil
	default:
		return emsdkEnvironment{}, fmt.Errorf("unsupported OS for emsdk installation: %s", runtime.GOOS)
	}
}

func detectEMSDKVersion(env emsdkEnvironment) (string, bool) {
	output, err := runEMSDKShellOutput(env.repoDir, "source ./emsdk_env.sh >/dev/null 2>&1; emcc --version | head -n 1")
	if err != nil {
		return "", false
	}
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) < 3 {
		return "", false
	}
	return fields[2], true
}

func resolveEMSDKShellExports() (map[string]string, error) {
	env, err := resolveEMSDKEnvironment()
	if err != nil {
		return nil, err
	}
	before := currentEnvMap()
	output, err := runEMSDKShellOutput(env.repoDir, "source ./emsdk_env.sh >/dev/null 2>&1; env -0")
	if err != nil {
		return nil, err
	}
	after := parseNullEnvOutput(output)
	return selectEMSDKExports(before, after), nil
}

func runEMSDKShellOutput(workdir, script string) ([]byte, error) {
	return runCommandOutputWithEnv(workdir, os.Environ(), "bash", "-lc", script)
}

func parseNullEnvOutput(data []byte) map[string]string {
	result := map[string]string{}
	for _, field := range bytes.Split(data, []byte{0}) {
		if len(field) == 0 {
			continue
		}
		parts := bytes.SplitN(field, []byte{'='}, 2)
		if len(parts) != 2 {
			continue
		}
		result[string(parts[0])] = string(parts[1])
	}
	return result
}

func selectEMSDKExports(before, after map[string]string) map[string]string {
	exports := map[string]string{}
	for key, value := range after {
		if before[key] == value {
			continue
		}
		if key == "PATH" || key == "JAVA_HOME" || strings.HasPrefix(key, "EM") || strings.HasPrefix(key, "EMSDK") {
			exports[key] = value
		}
	}
	return exports
}

func prependToPath(pathValue string, dirs ...string) string {
	result := []string{}
	seen := map[string]bool{}
	appendDir := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		result = append(result, dir)
	}
	for _, dir := range dirs {
		appendDir(dir)
	}
	for _, dir := range filepath.SplitList(pathValue) {
		appendDir(dir)
	}
	return strings.Join(result, string(os.PathListSeparator))
}

func currentEnvMap() map[string]string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return env
}

func envMapToSlice(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func runCommandOutput(name string, args ...string) ([]byte, error) {
	return runCommandOutputWithEnv("", os.Environ(), name, args...)
}

func runCommandOutputWithEnv(workdir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	if workdir != "" {
		cmd.Dir = workdir
	}
	return cmd.CombinedOutput()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
