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

package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/goplus/spx/v4/internal/release"
)

// CurrentEnvMap returns a copy of the process environment keyed by name.
func CurrentEnvMap() map[string]string {
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

// PrependToPath puts dirs before pathValue, omitting empty and duplicate entries.
func PrependToPath(pathValue string, dirs ...string) string {
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

// PrependEnvPath prepends directories to env's PATH and normalizes its key on Windows.
func PrependEnvPath(env map[string]string, dirs ...string) {
	setPathEnv(env, PrependToPath(pathEnvValue(env), dirs...))
}

func CurrentBuildEnv() (map[string]string, error) {
	env := CurrentEnvMap()
	if err := configureCurrentMacOSGoToolchainEnv(env); err != nil {
		return nil, fmt.Errorf("configure macOS Go toolchain: %w", err)
	}
	return env, nil
}

func EnvMapToSlice(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

// ResolveCommandPath finds a command using PATH and PATHEXT from env.
func ResolveCommandPath(name string, env map[string]string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	extensions := append([]string{""}, executableExtensions(env)...)
	for _, dir := range filepath.SplitList(pathEnvValue(env)) {
		if dir == "" {
			continue
		}
		base := filepath.Join(dir, name)
		for _, ext := range extensions {
			candidate := base + ext
			info, err := os.Stat(candidate)
			if err == nil && isCommandFile(info) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}

func (r CommandRunner) RunCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.RepoRoot, workdir)
	}

	env, err := buildctlCommandEnv()
	if err != nil {
		return fmt.Errorf("resolve command environment: %w", err)
	}
	commandName, err := ResolveCommandPath(name, env)
	if err != nil {
		return fmt.Errorf("resolve command path for %s: %w", name, err)
	}

	cmd := exec.Command(commandName, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = EnvMapToSlice(env)
	return cmd.Run()
}

func buildctlCommandEnv() (map[string]string, error) {
	env, err := CurrentBuildEnv()
	if err != nil {
		return nil, err
	}
	goPath, err := EnsureGoPath()
	if err != nil {
		return nil, err
	}
	pathDirs := []string{filepath.Join(goPath, "bin")}
	//lint:ignore SA1019 Locally built buildctl deliberately prefers its build toolchain over Go binaries later in PATH.
	if goRoot := runtime.GOROOT(); goRoot != "" {
		pathDirs = append(pathDirs, filepath.Join(goRoot, "bin"))
	}
	PrependEnvPath(env, pathDirs...)
	env["GOTOOLCHAIN"] = "go" + release.DefaultRuntimeLock().Toolchain.Go
	return env, nil
}

func isCommandFile(info os.FileInfo) bool {
	return isCommandFileForOS(info, runtime.GOOS)
}

func isCommandFileForOS(info os.FileInfo, goos string) bool {
	if info.IsDir() {
		return false
	}
	if goos == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func executableExtensions(env map[string]string) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	pathExt := envValue(env, "PATHEXT")
	parts := filepath.SplitList(strings.ToLower(pathExt))
	exts := make([]string, 0, len(parts))
	for _, ext := range parts {
		if ext == "" {
			continue
		}
		exts = append(exts, ext)
	}
	if len(exts) == 0 {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	return exts
}

func pathEnvValue(env map[string]string) string {
	return envValue(env, "PATH")
}

func envValue(env map[string]string, key string) string {
	if value, ok := env[key]; ok {
		return value
	}
	for envKey, value := range env {
		if strings.EqualFold(envKey, key) {
			return value
		}
	}
	return ""
}

func setPathEnv(env map[string]string, value string) {
	if runtime.GOOS == "windows" {
		for key := range env {
			if key != "PATH" && strings.EqualFold(key, "PATH") {
				delete(env, key)
			}
		}
	}
	env["PATH"] = value
}
