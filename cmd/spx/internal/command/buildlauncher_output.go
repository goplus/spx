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

package command

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type launcherOutputInputs struct {
	ProjectDir string
	ProjectExt string
	PackDir    string
	Protection launcherOutputProtection
}

type launcherOutputProtection struct {
	Files []string
	Roots []string
}

func resolveLauncherOutput(inputs launcherOutputInputs, requested string) (string, error) {
	projectDir, err := canonicalDirectory(inputs.ProjectDir)
	if err != nil {
		return "", fmt.Errorf("buildlauncher: resolve project directory: %w", err)
	}
	if requested == "" {
		name := filepath.Base(projectDir)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = "spx-launcher"
		}
		requested = filepath.Join(projectDir, ".builds", name+executableSuffix(runtime.GOOS))
	} else if !filepath.IsAbs(requested) {
		requested, err = filepath.Abs(requested)
		if err != nil {
			return "", fmt.Errorf("buildlauncher: resolve output: %w", err)
		}
	}
	requested = filepath.Clean(requested)
	requested, err = canonicalizeLauncherOutputAlias(requested)
	if err != nil {
		return "", err
	}

	packRoot := filepath.Join(projectDir, filepath.FromSlash(inputs.PackDir))
	insidePack, err := launcherPathWithinFilesystem(packRoot, requested)
	if err != nil {
		return "", fmt.Errorf("buildlauncher: compare output with pack directory: %w", err)
	}
	if launcherPathsEqual(projectDir, requested) || insidePack {
		return "", fmt.Errorf("buildlauncher: output must not be the project directory or inside the pack directory: %q", requested)
	}
	insideProject, err := launcherPathWithinFilesystem(projectDir, requested)
	if err != nil {
		return "", fmt.Errorf("buildlauncher: compare output with project directory: %w", err)
	}
	if insideProject && inputs.ProjectExt != "" && filepath.Ext(requested) == inputs.ProjectExt {
		return "", fmt.Errorf("buildlauncher: output must not use project source extension %q", inputs.ProjectExt)
	}
	if err := validateLauncherOutputProtection(requested, inputs.Protection); err != nil {
		return "", err
	}
	if err := validateLauncherOutputParent(filepath.Dir(requested), true); err != nil {
		return "", err
	}
	if err := validateLauncherOutputTarget(requested); err != nil {
		return "", err
	}
	return requested, nil
}

func validateLauncherOutputProtection(output string, protection launcherOutputProtection) error {
	if input, ok := protectedLauncherInput(output, protection.Files); ok {
		return fmt.Errorf("buildlauncher: output must not overwrite build input %q", input)
	}
	for _, root := range protection.Roots {
		if root == "" {
			continue
		}
		inside, err := launcherPathWithinFilesystem(root, output)
		if err != nil {
			return fmt.Errorf("buildlauncher: compare output with build input directory: %w", err)
		}
		if inside {
			return fmt.Errorf("buildlauncher: output must not overwrite build input under %q", root)
		}
	}
	return nil
}

func protectedLauncherInput(output string, paths []string) (string, bool) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		absolute = filepath.Clean(absolute)
		if canonical, canonicalErr := canonicalizeLauncherOutputAlias(absolute); canonicalErr == nil {
			absolute = canonical
		}
		if launcherPathsEqual(output, absolute) {
			return absolute, true
		}
	}
	return "", false
}

func launcherPathsEqual(left, right string) bool {
	if outputPathsEqual(left, right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func launcherPathWithinFilesystem(root, target string) (bool, error) {
	if launcherPathWithin(root, target) {
		return true, nil
	}
	rootInfo, err := os.Stat(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for current := target; ; current = filepath.Dir(current) {
		if info, statErr := os.Stat(current); statErr == nil {
			if os.SameFile(rootInfo, info) {
				return true, nil
			}
		} else if !os.IsNotExist(statErr) {
			return false, statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}

func validateLauncherOutputTarget(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("buildlauncher: output is not a regular file: %q", path)
		}
		if reparse, reparseErr := launcherOutputPathIsReparse(path); reparseErr != nil {
			return fmt.Errorf("buildlauncher: inspect output %q: %w", path, reparseErr)
		} else if reparse {
			return fmt.Errorf("buildlauncher: output is a reparse point: %q", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("buildlauncher: inspect output %q: %w", path, err)
	}
	return nil
}

// canonicalizeLauncherOutputAlias permits only macOS system aliases.
func canonicalizeLauncherOutputAlias(path string) (string, error) {
	parent := filepath.Dir(path)
	existing := parent
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			resolved = filepath.Clean(resolved)
			if outputPathsEqual(existing, resolved) {
				return path, nil
			}
			if !launcherOutputAliasAllowed(existing, resolved) {
				return "", fmt.Errorf("buildlauncher: output parent contains a symlink: %q", existing)
			}
			relative, relErr := filepath.Rel(existing, parent)
			if relErr != nil {
				return "", fmt.Errorf("buildlauncher: resolve output parent %q: %w", parent, relErr)
			}
			return filepath.Join(resolved, relative, filepath.Base(path)), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("buildlauncher: resolve output parent %q: %w", existing, err)
		}
		ancestor := filepath.Dir(existing)
		if ancestor == existing {
			return path, nil
		}
		existing = ancestor
	}
}

// validateLauncherOutputParent rejects symlink and reparse components.
func validateLauncherOutputParent(path string, allowMissing bool) error {
	if path == "" {
		return fmt.Errorf("buildlauncher: output parent is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("buildlauncher: resolve output parent: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if err := validateLauncherExistingPath(absolute, allowMissing); err != nil {
		return err
	}
	return nil
}

func validateLauncherExistingPath(path string, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("buildlauncher: output parent contains a symlink: %q", path)
		}
		if !info.IsDir() {
			return fmt.Errorf("buildlauncher: output parent is not a directory: %q", path)
		}
		if reparse, reparseErr := launcherOutputPathIsReparse(path); reparseErr != nil {
			return fmt.Errorf("buildlauncher: inspect output parent %q: %w", path, reparseErr)
		} else if reparse {
			return fmt.Errorf("buildlauncher: output parent contains a reparse point: %q", path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		return validateLauncherExistingPath(parent, false)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("buildlauncher: inspect output parent %q: %w", path, err)
	}
	if !allowMissing {
		return fmt.Errorf("buildlauncher: output parent does not exist: %q", path)
	}
	parent := filepath.Dir(path)
	if parent == path {
		return nil
	}
	return validateLauncherExistingPath(parent, true)
}
