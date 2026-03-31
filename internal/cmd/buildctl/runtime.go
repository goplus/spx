package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var osStderr = os.Stderr

type scriptRunner interface {
	runScript(relativePath string, args ...string) error
	runCommand(workdir string, name string, args ...string) error
	repoRootDir() string
}

type commandRunner struct {
	repoRoot string
}

func (r commandRunner) runScript(relativePath string, args ...string) error {
	scriptPath := filepath.Join(r.repoRoot, relativePath)
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = r.repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r commandRunner) runCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.repoRoot, workdir)
	}

	env, err := buildctlCommandEnv()
	if err != nil {
		return fmt.Errorf("resolve command environment: %w", err)
	}
	commandName, err := resolveCommandPath(name, env)
	if err != nil {
		return fmt.Errorf("resolve command path for %s: %w", name, err)
	}

	cmd := exec.Command(commandName, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

func (r commandRunner) repoRootDir() string {
	return r.repoRoot
}

func (r commandRunner) listDemoDirs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(r.repoRoot, "tutorial"))
	if err != nil {
		return nil, err
	}

	demos := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			demos = append(demos, filepath.Join("tutorial", entry.Name()))
		}
	}
	sort.Strings(demos)
	return demos, nil
}

func (r commandRunner) stopWebServers() error {
	if runtime.GOOS == "windows" || os.Getenv("OS") == "Windows_NT" {
		_ = exec.Command("taskkill", "/F", "/FI", "IMAGENAME eq python.exe").Run()
		_ = exec.Command("taskkill", "/F", "/FI", "IMAGENAME eq python3.exe").Run()
		return nil
	}

	cmd := exec.Command("pgrep", "-f", "gdspx_web_server.py")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}

	for _, field := range bytes.Fields(output) {
		pid, err := strconv.Atoi(string(field))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = process.Kill()
	}
	return nil
}

func installTools(cfg toolInstallConfig, runner scriptRunner) error {
	args := []string{}
	if cfg.web {
		args = append(args, "--web")
	}
	if cfg.opt {
		args = append(args, "--opt")
	}
	return runner.runScript(filepath.Join("cmd", "gox", "install.sh"), args...)
}

func buildWasmRuntime(cfg runtimeBuildWasmConfig, runner scriptRunner) error {
	if err := installTools(toolInstallConfig{web: true, opt: cfg.opt}, runner); err != nil {
		return err
	}
	if !cfg.opt {
		return nil
	}
	return compressWasmArtifacts()
}

func buildctlCommandEnv() (map[string]string, error) {
	env := currentEnvMap()
	goPath, err := ensureGoPath()
	if err != nil {
		return nil, err
	}
	env["PATH"] = prependToPath(env["PATH"], filepath.Join(goPath, "bin"))
	return env, nil
}

func resolveCommandPath(name string, env map[string]string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	for _, dir := range filepath.SplitList(env["PATH"]) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
		for _, ext := range executableExtensions(env) {
			candidateWithExt := candidate + ext
			info, err := os.Stat(candidateWithExt)
			if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
				return candidateWithExt, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}

func executableExtensions(env map[string]string) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	pathExt := env["PATHEXT"]
	if pathExt == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	parts := filepath.SplitList(strings.ToLower(pathExt))
	if len(parts) == 0 {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
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
