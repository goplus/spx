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
	"path/filepath"
	"runtime"
	"strings"
)

const EngineBuildVersion = "4.4.1.stable"

type buildEnvironment struct {
	RepoRoot      string
	ProjectDir    string
	EngineDir     string
	GodotSrc      string
	GoPath        string
	Version       string
	EngineGitTag  string
	EngineVersion string
	TemplateDir   string
	Platform      string
	Arch          string
}

func resolveBuildEnvironment(repoRoot string, requestedPlatform string) (buildEnvironment, error) {
	version, err := defaultRuntimeVersion()
	if err != nil {
		return buildEnvironment{}, err
	}
	goPath, err := ensureGoPath()
	if err != nil {
		return buildEnvironment{}, err
	}
	engineDir, err := resolveGodotSrc(repoRoot)
	if err != nil {
		return buildEnvironment{}, err
	}
	templateDir, err := detectGodotTemplateDir()
	if err != nil {
		return buildEnvironment{}, err
	}
	arch, err := detectBuildArch()
	if err != nil {
		return buildEnvironment{}, err
	}

	platform := requestedPlatform
	if platform == "" {
		platform = strings.TrimSpace(os.Getenv("PLATFORM"))
	}
	if platform == "" {
		platform, err = detectBuildPlatform()
		if err != nil {
			return buildEnvironment{}, err
		}
	}
	if err := validateOptionalPlatform(platform); err != nil {
		return buildEnvironment{}, err
	}

	return buildEnvironment{
		RepoRoot:      repoRoot,
		ProjectDir:    repoRoot,
		EngineDir:     engineDir,
		GodotSrc:      engineDir,
		GoPath:        goPath,
		Version:       version,
		EngineGitTag:  "spx" + version,
		EngineVersion: EngineBuildVersion,
		TemplateDir:   templateDir,
		Platform:      platform,
		Arch:          arch,
	}, nil
}

func resolveGodotSrc(repoRoot string) (string, error) {
	rawPath := strings.TrimSpace(os.Getenv("GODOT_SRC"))
	if rawPath == "" {
		rawPath = filepath.Join(repoRoot, "godot")
	}
	if !filepath.IsAbs(rawPath) {
		rawPath = filepath.Join(repoRoot, rawPath)
	}
	return filepath.Clean(rawPath), nil
}

func detectBuildPlatform() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "linux", nil
	case "darwin":
		return "macos", nil
	case "windows":
		return "windows", nil
	default:
		return "", fmt.Errorf("unsupported host OS: %s", runtime.GOOS)
	}
}

func detectBuildArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "386":
		return "x86_32", nil
	case "arm64":
		return "arm64", nil
	case "arm":
		return "arm32", nil
	default:
		return "", fmt.Errorf("unsupported host architecture: %s", runtime.GOARCH)
	}
}

func detectGodotTemplateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".local", "share", "godot", "export_templates", EngineBuildVersion), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Godot", "export_templates", EngineBuildVersion), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("missing APPDATA")
		}
		return filepath.Join(appData, "Godot", "export_templates", EngineBuildVersion), nil
	default:
		return "", fmt.Errorf("unsupported host OS: %s", runtime.GOOS)
	}
}

func (env buildEnvironment) shellExports() string {
	lines := []string{
		"export PROJ_DIR=" + shellQuote(env.ProjectDir),
		"export ENGINE_DIR=" + shellQuote(env.EngineDir),
		"export GODOT_SRC=" + shellQuote(env.GodotSrc),
		"export ENGINE_VERSION=" + shellQuote(env.EngineVersion),
		"export GOPATH=" + shellQuote(env.GoPath),
		"export VERSION=" + shellQuote(env.Version),
		"export ENGINE_GIT_TAG=" + shellQuote(env.EngineGitTag),
		"export TEMPLATE_DIR=" + shellQuote(env.TemplateDir),
		"export PLATFORM=" + shellQuote(env.Platform),
		"export ARCH=" + shellQuote(env.Arch),
	}
	return strings.Join(lines, "\n") + "\n"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
