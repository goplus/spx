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

package tool

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type emsdkEnvironment struct {
	rootDir string
	repoDir string
}

var (
	buildEnvLookPath           = exec.LookPath
	buildEnvRunStreaming       = shared.RunStreamingCommand
	buildEnvRunOutputWithDir   = runCommandOutputWithEnv
	resolveEMSDKShellExportsFn = ResolveEMSDKShellExports
)

func setupSCons() error {
	_, err := EnsureSCons()
	return err
}

func EnsureSCons() (string, error) {
	python, err := detectPythonCommand()
	if err != nil {
		return "", err
	}
	if err := buildEnvRunStreaming("", python, "-c", "import sys; print(sys.version)"); err != nil {
		return "", err
	}

	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		return "", err
	}
	venvDir := filepath.Join(repoRoot, ".bin", "scons-"+requiredSConsVersion)
	venvPython, sconsCommand := sconsEnvironmentCommands(venvDir)
	if shared.FileExists(venvPython) && shared.FileExists(sconsCommand) {
		output, versionErr := buildEnvRunOutputWithDir("", os.Environ(), venvPython, "-c", "import SCons; print(SCons.__version__)")
		if versionErr == nil && strings.TrimSpace(string(output)) == requiredSConsVersion {
			if err := buildEnvRunStreaming("", sconsCommand, "--version"); err != nil {
				return "", err
			}
			return sconsCommand, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(venvDir), 0o755); err != nil {
		return "", err
	}
	if err := buildEnvRunStreaming("", python, "-m", "venv", venvDir); err != nil {
		return "", err
	}
	if err := buildEnvRunStreaming("", venvPython, "-m", "pip", "install", "scons=="+requiredSConsVersion); err != nil {
		return "", err
	}
	if err := buildEnvRunStreaming("", sconsCommand, "--version"); err != nil {
		return "", err
	}
	return sconsCommand, nil
}

func sconsEnvironmentCommands(venvDir string) (pythonCommand, sconsCommand string) {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe"), filepath.Join(venvDir, "Scripts", "scons.exe")
	}
	return filepath.Join(venvDir, "bin", "python"), filepath.Join(venvDir, "bin", "scons")
}

func SetupJDK() error {
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
			return fmt.Errorf("homebrew not found; install Homebrew first or install JDK %d manually", requiredJDKMajor)
		}
		if err := buildEnvRunStreaming("", "brew", "install", fmt.Sprintf("openjdk@%d", requiredJDKMajor)); err != nil {
			return err
		}
	case "linux":
		if _, err := buildEnvLookPath("apt-get"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "apt-get", "update"); err != nil {
				return err
			}
			if err := buildEnvRunStreaming("", "sudo", "apt-get", "install", "-y", fmt.Sprintf("openjdk-%d-jdk", requiredJDKMajor)); err != nil {
				return err
			}
		} else if _, err := buildEnvLookPath("dnf"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "dnf", "install", "-y", fmt.Sprintf("java-%d-openjdk-devel", requiredJDKMajor)); err != nil {
				return err
			}
		} else if _, err := buildEnvLookPath("yum"); err == nil {
			if err := buildEnvRunStreaming("", "sudo", "yum", "install", "-y", fmt.Sprintf("java-%d-openjdk-devel", requiredJDKMajor)); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported Linux distribution; install JDK %d manually", requiredJDKMajor)
		}
	case "windows":
		return fmt.Errorf("on Windows, install JDK %d manually and ensure JAVA_HOME is set", requiredJDKMajor)
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
	return fmt.Errorf("failed to install JDK %d; install it manually", requiredJDKMajor)
}

func SetupEMSDK() error {
	env, err := resolveEMSDKEnvironment()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "EMSDK installation directory: %s\n", env.rootDir)
	if err := os.MkdirAll(env.rootDir, 0o755); err != nil {
		return err
	}

	if !shared.FileExists(env.repoDir) {
		fmt.Fprintln(os.Stdout, "EMSDK not found in the global location. Installing...")
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
		fmt.Fprintf(os.Stdout, "Current emcc version: %s; target version: %s\n", currentVersion, requiredEMSDKVersion)
	}

	if !ok || currentVersion != requiredEMSDKVersion {
		fmt.Fprintf(os.Stdout, "Installing target emcc version %s...\n", requiredEMSDKVersion)
		if err := buildEnvRunStreaming(env.repoDir, "./emsdk", "install", requiredEMSDKVersion); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "Current emcc version matches the target version. No reinstall needed.")
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
	fmt.Fprintln(os.Stdout, "EMSDK setup completed:")
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
	if !shared.FileExists(emppPath) {
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
	return "", errors.New("missing JAVA_HOME")
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

func ResolveJDKShellExports() (map[string]string, error) {
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
			return emsdkEnvironment{}, errors.New("missing APPDATA")
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

func ResolveEMSDKShellExports() (map[string]string, error) {
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
