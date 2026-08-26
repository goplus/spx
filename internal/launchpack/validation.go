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

package launchpack

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func (c Config) validate() error {
	for _, item := range []struct{ name, value string }{
		{"project-dir", c.ProjectDir}, {"project-file", c.ProjectFile}, {"output", c.Output},
	} {
		if item.value == "" {
			return fmt.Errorf("launchpack: %s is required", item.name)
		}
	}
	if err := regularPath("project-dir", c.ProjectDir, true); err != nil {
		return err
	}
	if err := regularPath("project-file", c.ProjectFile, false); err != nil {
		return err
	}
	if !pathWithin(c.ProjectDir, c.ProjectFile) {
		return fmt.Errorf("launchpack: project-file must be within project-dir")
	}
	if err := c.validateGraphInputs(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(c.GoCommand)
		if err == nil && info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("launchpack: go-command is not executable: %q", c.GoCommand)
		}
	}
	if c.PackDir == "" || c.PackIndex == "" {
		return fmt.Errorf("launchpack: pack directory and index are required")
	}
	if err := validatePackPath("pack directory", c.PackDir, true); err != nil {
		return err
	}
	if err := validatePackPath("pack index", c.PackIndex, false); err != nil {
		return err
	}
	if !filepath.IsAbs(c.Output) || filepath.Clean(c.Output) != c.Output {
		return fmt.Errorf("launchpack: output must be an absolute clean path")
	}
	packRoot := filepath.Join(c.ProjectDir, filepath.FromSlash(c.PackDir))
	if c.Output == c.ProjectDir || pathWithin(packRoot, c.Output) {
		return fmt.Errorf("launchpack: output must not be inside the pack directory")
	}
	if c.Source.SourceMode {
		if c.BridgePackage == "" {
			return fmt.Errorf("launchpack: bridge package is required")
		}
	} else {
		if err := validatePublishedSource(c.Source); err != nil {
			return err
		}
		if c.BridgePackage != "" {
			return fmt.Errorf("launchpack: published mode must not configure a bridge package")
		}
		if c.RuntimeSourceRoot != "" {
			return fmt.Errorf("launchpack: published mode must not configure a runtime source root")
		}
	}
	if c.RuntimeSourceRoot != "" {
		if err := regularPath("runtime-source-root", c.RuntimeSourceRoot, true); err != nil {
			return err
		}
	}
	if c.DriverAssetDir != "" && (!filepath.IsAbs(c.DriverAssetDir) || filepath.Clean(c.DriverAssetDir) != c.DriverAssetDir) {
		return fmt.Errorf("launchpack: driver asset directory must be an absolute clean path")
	}
	if !c.Source.SourceMode && c.RuntimeAssetDir != "" {
		return fmt.Errorf("launchpack: published mode must not configure a runtime asset directory")
	}
	if !c.Source.SourceMode && c.RuntimeManifestPath != "" {
		return fmt.Errorf("launchpack: published mode must not configure a runtime manifest")
	}
	return nil
}

// validatePackPath uses the slash-separated paths stored in project metadata;
// filepath semantics would reject nested pack directories on Windows.
func validatePackPath(name, value string, directory bool) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "\\\x00:") || path.IsAbs(value) || looksLikeWindowsAbsolutePath(value) || path.Clean(value) != value {
		return fmt.Errorf("launchpack: %s must be a clean relative slash path: %q", name, value)
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("launchpack: %s contains an invalid path component: %q", name, value)
		}
	}
	if !directory && strings.Contains(value, "/") {
		return fmt.Errorf("launchpack: %s must be a plain file name: %q", name, value)
	}
	return nil
}

func looksLikeWindowsAbsolutePath(value string) bool {
	return strings.HasPrefix(value, "//") || len(value) >= 3 && isASCIIAlpha(value[0]) && value[1] == ':' && value[2] == '/'
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func (c Config) validateGraphInputs() error {
	if c.GoCommand == "" || c.WorkDir == "" {
		return fmt.Errorf("launchpack: Go command and work directory are required")
	}
	if err := regularPath("go-command", c.GoCommand, false); err != nil {
		return err
	}
	if err := regularPath("work-dir", c.WorkDir, true); err != nil {
		return err
	}
	if c.GoWork != "" && c.GoWork != "off" {
		if err := regularPath("go-work", c.GoWork, false); err != nil {
			return err
		}
	}
	if err := validateGraphFlags(c.GraphFlags); err != nil {
		return err
	}
	return validateBuildFlags(c.BuildFlags)
}

func validateGraphFlags(flags []string) error {
	for _, flag := range flags {
		if !strings.HasPrefix(flag, "-") || flag == "-" {
			return fmt.Errorf("launchpack: invalid graph flag: %q", flag)
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(flag, "-"), "=")
		switch name {
		case "overlay":
			return fmt.Errorf("launchpack: graph overlay is not supported")
		case "modfile":
			if !hasValue || value == "" {
				return fmt.Errorf("launchpack: graph modfile has no path")
			}
			if err := regularPath("graph modfile", value, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBuildFlags(flags []string) error {
	for _, flag := range flags {
		if !strings.HasPrefix(flag, "-") || flag == "-" {
			return fmt.Errorf("launchpack: invalid build flag: %q", flag)
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(flag, "-"), "=")
		switch name {
		case "v", "x", "work", "trimpath":
			if hasValue && value != "true" && value != "false" {
				return fmt.Errorf("launchpack: build flag -%s requires true or false", name)
			}
		case "buildvcs":
			if !hasValue || value != "auto" && value != "true" && value != "false" {
				return fmt.Errorf("launchpack: build flag -buildvcs requires auto, true, or false")
			}
		default:
			return fmt.Errorf("launchpack: unsupported build flag: %q", flag)
		}
	}
	return nil
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func regularPath(name, value string, directory bool) error {
	info, err := os.Lstat(value)
	if err != nil {
		return fmt.Errorf("launchpack: %s cannot be inspected: %w", name, err)
	}
	if filepath.Clean(value) != value || !filepath.IsAbs(value) {
		return fmt.Errorf("launchpack: %s must be an absolute clean path: %q", name, value)
	}
	if info.Mode()&os.ModeSymlink != 0 || (directory && !info.IsDir()) || (!directory && !info.Mode().IsRegular()) {
		return fmt.Errorf("launchpack: %s has an invalid file type: %q", name, value)
	}
	return nil
}
