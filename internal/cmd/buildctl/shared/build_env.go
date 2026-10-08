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

	"github.com/goplus/spx/v3/internal/release"
)

type BuildEnvironment = buildEnvironment

type buildEnvironment struct {
	RepoRoot        string
	ProjectDir      string
	EngineDir       string
	GodotSrc        string
	SPXModuleSrc    string
	GoPath          string
	Version         string
	GodotRepository string
	GodotRef        string
	GodotCommit     string
	EngineVersion   string
	TemplateDir     string
	Platform        string
	Arch            string
}

func (env BuildEnvironment) ShellExports() string {
	lines := []string{
		"export PROJ_DIR=" + ShellQuote(env.ProjectDir),
		"export ENGINE_DIR=" + ShellQuote(env.EngineDir),
		"export GODOT_SRC=" + ShellQuote(env.GodotSrc),
		"export SPX_MODULE_SRC=" + ShellQuote(env.SPXModuleSrc),
		"export ENGINE_VERSION=" + ShellQuote(env.EngineVersion),
		"export GOPATH=" + ShellQuote(env.GoPath),
		"export VERSION=" + ShellQuote(env.Version),
		"export GODOT_REPOSITORY=" + ShellQuote(env.GodotRepository),
		"export GODOT_REF=" + ShellQuote(env.GodotRef),
		"export GODOT_COMMIT=" + ShellQuote(env.GodotCommit),
		"export TEMPLATE_DIR=" + ShellQuote(env.TemplateDir),
		"export PLATFORM=" + ShellQuote(env.Platform),
		"export ARCH=" + ShellQuote(env.Arch),
	}
	return strings.Join(lines, "\n") + "\n"
}

func ResolveBuildEnvironment(repoRoot string, requestedPlatform string) (BuildEnvironment, error) {
	runtimeLock := release.DefaultRuntimeLock()
	goPath, err := EnsureGoPath()
	if err != nil {
		return buildEnvironment{}, err
	}
	engineDir, err := resolveGodotSrc(repoRoot)
	if err != nil {
		return buildEnvironment{}, err
	}
	spxModuleSrc, err := ResolveSPXModuleSource(repoRoot)
	if err != nil {
		return buildEnvironment{}, err
	}
	templateDir, err := detectGodotTemplateDir(runtimeLock.Godot.Version)
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
	if err := ValidateOptionalPlatform(platform); err != nil {
		return buildEnvironment{}, err
	}

	return buildEnvironment{
		RepoRoot:        repoRoot,
		ProjectDir:      repoRoot,
		EngineDir:       engineDir,
		GodotSrc:        engineDir,
		SPXModuleSrc:    spxModuleSrc,
		GoPath:          goPath,
		Version:         runtimeLock.RuntimeVersion,
		GodotRepository: runtimeLock.Godot.Repository,
		GodotRef:        runtimeLock.Godot.Ref,
		GodotCommit:     runtimeLock.Godot.Commit,
		EngineVersion:   runtimeLock.Godot.Version,
		TemplateDir:     templateDir,
		Platform:        platform,
		Arch:            arch,
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

func detectGodotTemplateDir(engineVersion string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".local", "share", "godot", "export_templates", engineVersion), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Godot", "export_templates", engineVersion), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("missing APPDATA")
		}
		return filepath.Join(appData, "Godot", "export_templates", engineVersion), nil
	default:
		return "", fmt.Errorf("unsupported host OS: %s", runtime.GOOS)
	}
}

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
