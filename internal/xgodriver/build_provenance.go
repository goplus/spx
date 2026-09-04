/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package xgodriver

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimedebug "runtime/debug"
)

type moduleRole uint8

const (
	moduleMain moduleRole = iota
	moduleDependency
)

func verifyBuiltSPXOrigin(ctx context.Context, name string, want ModuleOrigin, role moduleRole, cfg Config, baseEnv []string) error {
	info, err := buildinfo.ReadFile(name)
	if err != nil {
		return err
	}
	replacementPath, err := effectiveLocalReplacementPath(ctx, cfg, baseEnv, want)
	if err != nil {
		return err
	}
	return verifyBuildInfoOrigin(info, want, replacementPath, role)
}

func effectiveLocalReplacementPath(ctx context.Context, cfg Config, baseEnv []string, want ModuleOrigin) (string, error) {
	if want.Replace == nil || want.Replace.Version != "" {
		return "", nil
	}
	if err := cfg.validateGraphInputs(); err != nil {
		return "", err
	}
	args := append([]string{"list"}, cfg.graphFlagsForCommand()...)
	args = append(args, "-m", "-json", want.Selected.Path)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = hostGoEnv(cfg, baseEnv)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("xgodriver: resolve effective module for build provenance: %w", err)
	}
	var listed listedModule
	if err := json.Unmarshal(output, &listed); err != nil {
		return "", fmt.Errorf("xgodriver: decode effective module for build provenance: %w", err)
	}
	got, err := normalizeListedOrigin(&listed)
	if err != nil {
		return "", fmt.Errorf("xgodriver: invalid effective module for build provenance: %w", err)
	}
	if !got.Equal(want) {
		return "", fmt.Errorf("xgodriver: effective module does not match resolved driver provenance")
	}
	if listed.Replace == nil || listed.Replace.Version != "" || listed.Replace.Path == "" {
		return "", fmt.Errorf("xgodriver: effective module is missing its local replacement path")
	}
	return listed.Replace.Path, nil
}

func verifyBuildInfoOrigin(info *runtimedebug.BuildInfo, want ModuleOrigin, effectiveReplacementPath string, role moduleRole) error {
	if info == nil {
		return fmt.Errorf("missing Go build info")
	}
	var got *runtimedebug.Module
	switch role {
	case moduleMain:
		if info.Main.Path != want.Selected.Path {
			return fmt.Errorf("built artifact main module is %q, want %q", info.Main.Path, want.Selected.Path)
		}
		got = &info.Main
	case moduleDependency:
		for _, dependency := range info.Deps {
			if dependency != nil && dependency.Path == want.Selected.Path {
				got = dependency
				break
			}
		}
		if got == nil {
			return fmt.Errorf("built artifact dependencies do not contain module %q", want.Selected.Path)
		}
	default:
		return fmt.Errorf("invalid module role %d", role)
	}
	if want.Main && role == moduleDependency && !isLocalBuildVersion(got.Version) {
		return fmt.Errorf("built workspace module dependency has version %q", got.Version)
	}
	if !want.Main && got.Version != want.Selected.Version {
		return fmt.Errorf("selected module version is %q, want %q", got.Version, want.Selected.Version)
	}
	if want.Replace == nil {
		if got.Replace != nil {
			return fmt.Errorf("built module has unexpected replacement %s@%s", got.Replace.Path, got.Replace.Version)
		}
		return nil
	}
	if got.Replace == nil {
		return fmt.Errorf("built module is missing its resolved replacement")
	}
	if want.Replace.Version != "" {
		if got.Replace.Path != want.Replace.Path || got.Replace.Version != want.Replace.Version {
			return fmt.Errorf("built module replacement is %s@%s, want %s@%s", got.Replace.Path, got.Replace.Version, want.Replace.Path, want.Replace.Version)
		}
		return nil
	}
	if !isLocalBuildVersion(got.Replace.Version) {
		return fmt.Errorf("built local replacement unexpectedly has version %q", got.Replace.Version)
	}
	if filepath.IsAbs(got.Replace.Path) {
		if !sameExistingPath(got.Replace.Path, want.Replace.Dir) {
			return fmt.Errorf("built local replacement %q does not match %q", got.Replace.Path, want.Replace.Dir)
		}
		return nil
	}
	if got.Replace.Path != effectiveReplacementPath {
		return fmt.Errorf("built relative local replacement is %q, want effective graph path %q", got.Replace.Path, effectiveReplacementPath)
	}
	return nil
}

func isLocalBuildVersion(version string) bool {
	return version == "" || version == "(devel)"
}

func sameExistingPath(first, second string) bool {
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}
