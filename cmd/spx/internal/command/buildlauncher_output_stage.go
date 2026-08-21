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
	"strings"
)

func stageLauncherOutput(final string) (string, func(), error) {
	final, err := canonicalizeLauncherOutputAlias(filepath.Clean(final))
	if err != nil {
		return "", nil, err
	}
	parent := filepath.Dir(final)
	if err := validateLauncherOutputParent(parent, true); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, fmt.Errorf("buildlauncher: create output directory: %w", err)
	}
	if err := validateLauncherOutputParent(parent, false); err != nil {
		return "", nil, err
	}
	if err := validateLauncherOutputTarget(final); err != nil {
		return "", nil, err
	}
	stageDir, err := os.MkdirTemp(parent, "."+filepath.Base(final)+".spx-launchpack-*")
	if err != nil {
		return "", nil, fmt.Errorf("buildlauncher: create staging directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(stageDir) }
	if err := os.Chmod(stageDir, 0o700); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("buildlauncher: secure staging directory: %w", err)
	}
	if err := validateLauncherOutputParent(stageDir, false); err != nil {
		cleanup()
		return "", nil, err
	}
	return filepath.Join(stageDir, "."+filepath.Base(final)+".spx-launchpack-stage"), cleanup, nil
}

func commitLauncherOutput(stage, final string, protection launcherOutputProtection) error {
	stage = filepath.Clean(stage)
	final, err := canonicalizeLauncherOutputAlias(filepath.Clean(final))
	if err != nil {
		return err
	}
	if err := validateLauncherStage(stage); err != nil {
		return err
	}
	if err := validateLauncherOutputParent(filepath.Dir(final), false); err != nil {
		return err
	}
	if err := validateLauncherOutputProtection(final, protection); err != nil {
		return err
	}
	if err := validateLauncherOutputTarget(final); err != nil {
		return err
	}
	if err := commitLauncherOutputPlatform(stage, final); err != nil {
		return fmt.Errorf("buildlauncher: install output: %w", err)
	}
	if err := removeLauncherStageDir(stage, final); err != nil {
		return fmt.Errorf("buildlauncher: clean staging directory: %w", err)
	}
	return nil
}

func validateLauncherStage(stage string) error {
	if err := validateLauncherOutputParent(filepath.Dir(stage), false); err != nil {
		return err
	}
	info, err := os.Lstat(stage)
	if err != nil {
		return fmt.Errorf("buildlauncher: inspect staged output %q: %w", stage, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("buildlauncher: staged output is not a regular file: %q", stage)
	}
	if !launcherStageIsExecutable(info) {
		return fmt.Errorf("buildlauncher: staged output is not executable: %q", stage)
	}
	return nil
}

func removeLauncherStageDir(stage, final string) error {
	dir := filepath.Dir(stage)
	if !outputPathsEqual(filepath.Dir(dir), filepath.Dir(final)) ||
		!strings.HasPrefix(filepath.Base(dir), "."+filepath.Base(final)+".spx-launchpack-") {
		return nil
	}
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("staging directory is not private: %q", dir)
	}
	return os.RemoveAll(dir)
}
