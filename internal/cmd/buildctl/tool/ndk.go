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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

type toolSetupNDKConfig struct {
	manualInstall    bool
	ndkPath          string
	skipVerification bool
}

type androidNDKEnv struct {
	archiveName string
	downloadURL string
	sdkRoot     string
	ndkRoot     string
	cacheDir    string
	shellConfig string
}

var androidNDKResolveEnv = resolveAndroidNDKEnv
var androidNDKFetcher = shared.FetchURLToFile

func parseToolSetupNDKArgs(args []string) (toolSetupNDKConfig, error) {
	cfg := toolSetupNDKConfig{}

	fs := flag.NewFlagSet("tool setup-ndk", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.manualInstall, "manual-install", false, "skip download and use an existing NDK zip")
	fs.StringVar(&cfg.ndkPath, "ndk-path", "", "path to an existing Android NDK archive zip")
	fs.BoolVar(&cfg.skipVerification, "skip-verification", false, "skip archive validation checks")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool setup-ndk [--manual-install --ndk-path PATH] [--skip-verification]")
	}

	if err := fs.Parse(args); err != nil {
		return toolSetupNDKConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolSetupNDKConfig{}, errUsage
	}
	if cfg.manualInstall && cfg.ndkPath == "" {
		return toolSetupNDKConfig{}, errors.New("--manual-install requires --ndk-path")
	}
	if !cfg.manualInstall && cfg.ndkPath != "" {
		return toolSetupNDKConfig{}, errors.New("--ndk-path requires --manual-install")
	}
	return cfg, nil
}

func setupAndroidNDK(cfg toolSetupNDKConfig) error {
	env, err := androidNDKResolveEnv()
	if err != nil {
		return err
	}

	if shared.FileExists(env.ndkRoot) {
		fmt.Fprintf(os.Stdout, "Android NDK %s is already installed at %s\n", androidNDKVersion, env.ndkRoot)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(env.ndkRoot), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "android-ndk-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	archivePath, err := resolveNDKArchivePath(cfg, env, tempDir)
	if err != nil {
		return err
	}
	if !cfg.skipVerification && !cfg.manualInstall {
		if err := verifyNDKArchive(archivePath); err != nil {
			return err
		}
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := shared.ExtractZip(archivePath, extractDir); err != nil {
		return err
	}

	ndkSource, err := locateExtractedNDK(extractDir)
	if err != nil {
		return err
	}
	if err := shared.CopyDir(ndkSource, env.ndkRoot); err != nil {
		return err
	}

	if err := updateNDKShellConfig(env); err != nil {
		return err
	}
	_ = os.Setenv("ANDROID_SDK_ROOT", env.sdkRoot)
	_ = os.Setenv("ANDROID_NDK_ROOT", env.ndkRoot)

	fmt.Fprintf(os.Stdout, "Android NDK %s installed at %s\n", androidNDKVersion, env.ndkRoot)
	if env.shellConfig != "" {
		fmt.Fprintf(os.Stdout, "Environment exports appended to %s\n", env.shellConfig)
	}
	return nil
}

func resolveAndroidNDKEnv() (androidNDKEnv, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return androidNDKEnv{}, err
	}
	androidNDKRelease, err := androidNDKReleaseForVersion(androidNDKVersion)
	if err != nil {
		return androidNDKEnv{}, err
	}

	sdkRoot := os.Getenv("ANDROID_SDK_ROOT")
	if sdkRoot == "" {
		switch runtime.GOOS {
		case "darwin":
			sdkRoot = filepath.Join(home, "Library", "Android", "sdk")
		case "linux":
			sdkRoot = filepath.Join(home, "Android", "Sdk")
		case "windows":
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData == "" {
				localAppData = filepath.Join(home, "AppData", "Local")
			}
			sdkRoot = filepath.Join(localAppData, "Android", "Sdk")
		default:
			return androidNDKEnv{}, fmt.Errorf("unsupported host OS for Android NDK setup: %s", runtime.GOOS)
		}
	}

	archiveOS := runtime.GOOS
	switch runtime.GOOS {
	case "darwin":
		archiveOS = "darwin"
	case "linux":
		archiveOS = "linux"
	case "windows":
		archiveOS = "windows"
	default:
		return androidNDKEnv{}, fmt.Errorf("unsupported host OS for Android NDK setup: %s", runtime.GOOS)
	}

	archiveName := fmt.Sprintf("android-ndk-%s-%s.zip", androidNDKRelease, archiveOS)
	return androidNDKEnv{
		archiveName: archiveName,
		downloadURL: "https://dl.google.com/android/repository/" + archiveName,
		sdkRoot:     sdkRoot,
		ndkRoot:     filepath.Join(sdkRoot, "ndk", androidNDKVersion),
		cacheDir:    filepath.Join(home, ".android_ndk_cache"),
		shellConfig: detectShellConfigFile(home),
	}, nil
}

func resolveNDKArchivePath(cfg toolSetupNDKConfig, env androidNDKEnv, tempDir string) (string, error) {
	if cfg.manualInstall {
		if !shared.FileExists(cfg.ndkPath) {
			return "", fmt.Errorf("ndk archive does not exist: %s", cfg.ndkPath)
		}
		return cfg.ndkPath, nil
	}

	cachedArchive := filepath.Join(env.cacheDir, env.archiveName)
	if shared.FileExists(cachedArchive) {
		return cachedArchive, nil
	}

	tempArchive := filepath.Join(tempDir, env.archiveName)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := androidNDKFetcher(env.downloadURL, tempArchive); err == nil {
			if !cfg.skipVerification {
				if err := verifyNDKArchive(tempArchive); err != nil {
					lastErr = err
				} else {
					return persistNDKArchive(tempArchive, cachedArchive)
				}
			} else {
				return persistNDKArchive(tempArchive, cachedArchive)
			}
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr == nil {
		lastErr = errors.New("failed to download Android NDK archive")
	}
	return "", lastErr
}

func persistNDKArchive(src, dst string) (string, error) {
	if err := shared.CopyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func verifyNDKArchive(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() < 900_000_000 {
		return fmt.Errorf("archive looks incomplete: %s (%d bytes)", path, info.Size())
	}
	return nil
}

func locateExtractedNDK(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "android-ndk-") {
			return filepath.Join(extractDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("extracted Android NDK directory not found in %s", extractDir)
}

func updateNDKShellConfig(env androidNDKEnv) error {
	if env.shellConfig == "" {
		return nil
	}

	current, _ := os.ReadFile(env.shellConfig)
	exportLine := "export ANDROID_NDK_ROOT=" + shared.ShellQuote(env.ndkRoot)
	if strings.Contains(string(current), exportLine) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(env.shellConfig), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(env.shellConfig, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	block := fmt.Sprintf("\n# Android NDK environment variables - added by buildctl\nexport ANDROID_SDK_ROOT=%s\nexport ANDROID_NDK_ROOT=%s\nexport PATH=\"$ANDROID_NDK_ROOT:$PATH\"\n", shared.ShellQuote(env.sdkRoot), shared.ShellQuote(env.ndkRoot))
	_, err = file.WriteString(block)
	return err
}

func detectShellConfigFile(home string) string {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		bashProfile := filepath.Join(home, ".bash_profile")
		if shared.FileExists(bashProfile) {
			return bashProfile
		}
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".bash_profile")
	}
}
