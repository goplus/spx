package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func compressWasmArtifacts(repoRoot string) error {
	version, err := readSPXVersion(repoRoot)
	if err != nil {
		return err
	}
	goPath, err := ensureGoPath()
	if err != nil {
		return err
	}

	goBinDir := filepath.Join(goPath, "bin")
	runtimeWebDir, err := resolveRuntimeWebDir(goBinDir, version)
	if err != nil {
		return err
	}

	if err := ensureBrotliAvailable(); err != nil {
		return err
	}

	if err := compressWithBrotli(filepath.Join(runtimeWebDir, "engine.wasm"), filepath.Join(runtimeWebDir, "engine.wasm.br")); err != nil {
		return err
	}
	return compressWithBrotli(filepath.Join(goBinDir, "ispx.wasm"), filepath.Join(goBinDir, "ispx.wasm.br"))
}

func resolveRuntimeWebDir(goBinDir, version string) (string, error) {
	candidates := []string{
		filepath.Join(goBinDir, fmt.Sprintf("gdspxrt%s_webnormal", version)),
		filepath.Join(goBinDir, fmt.Sprintf("gdspxrt%s_web", version)),
	}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, "engine.wasm")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("web runtime engine.wasm not found in %s", strings.Join(candidates, ", "))
}

func ensureBrotliAvailable() error {
	if _, err := exec.LookPath("brotli"); err == nil {
		return nil
	}

	fmt.Fprintln(os.Stdout, "brotli not detected, trying to install...")
	switch runtime.GOOS {
	case "linux":
		if err := installBrotliLinux(); err != nil {
			return err
		}
	case "darwin":
		if err := runStreamingCommand("", "brew", "install", "brotli"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("brotli is not installed and automatic installation is unsupported on %s", runtime.GOOS)
	}

	if _, err := exec.LookPath("brotli"); err != nil {
		return fmt.Errorf("brotli installation failed")
	}
	return nil
}

func installBrotliLinux() error {
	switch {
	case fileExists("/etc/os-release"):
		content, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(content))
		switch {
		case strings.Contains(lower, "id=ubuntu"), strings.Contains(lower, "id=debian"):
			if err := runStreamingCommand("", "sudo", "apt-get", "update"); err != nil {
				return err
			}
			return runStreamingCommand("", "sudo", "apt-get", "install", "-y", "brotli")
		case strings.Contains(lower, "id=fedora"), strings.Contains(lower, "id=rhel"), strings.Contains(lower, "id=centos"):
			if _, err := exec.LookPath("dnf"); err == nil {
				return runStreamingCommand("", "sudo", "dnf", "install", "-y", "brotli")
			}
			return runStreamingCommand("", "sudo", "yum", "install", "-y", "brotli")
		}
	}

	if _, err := exec.LookPath("apt-get"); err == nil {
		if err := runStreamingCommand("", "sudo", "apt-get", "update"); err != nil {
			return err
		}
		return runStreamingCommand("", "sudo", "apt-get", "install", "-y", "brotli")
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return runStreamingCommand("", "sudo", "dnf", "install", "-y", "brotli")
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return runStreamingCommand("", "sudo", "yum", "install", "-y", "brotli")
	}
	return fmt.Errorf("unable to install brotli automatically on linux: no supported package manager found")
}

func compressWithBrotli(inputFile, outputFile string) error {
	if inputFile == "" || outputFile == "" {
		return fmt.Errorf("compressWithBrotli requires input and output file parameters")
	}
	if !fileExists(inputFile) {
		return fmt.Errorf("input file %s does not exist", inputFile)
	}
	if err := os.RemoveAll(outputFile); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Compressing %s with brotli...\n", inputFile)
	if err := runStreamingCommand("", "brotli", "-q", "11", "-o", outputFile, inputFile); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s has been created\n", outputFile)
	return nil
}

func runStreamingCommand(workdir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
