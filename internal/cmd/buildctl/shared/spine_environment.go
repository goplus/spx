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

package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/internal/runtimevariant"
)

// ValidateSpineEnvironment checks runtime isolation without creating a workspace.
func ValidateSpineEnvironment() error {
	_, err := spineGoPath()
	return err
}

// PrepareSpineEnvironment reserves the workspace before installing or exporting
// local runtime assets. Read-only commands should use validation instead.
func PrepareSpineEnvironment() error {
	goPath, err := spineGoPath()
	if err != nil || goPath == "" {
		return err
	}
	return prepareSpineWorkspace(goPath)
}

// spineGoPath returns the dedicated GOPATH only when Spine is enabled.
func spineGoPath() (string, error) {
	enabled, err := SpineEnabled()
	if err != nil {
		return "", err
	}
	goPath, err := EnsureGoPath()
	if err != nil {
		return "", err
	}
	marked, err := runtimevariant.SpineBinMarked(filepath.Join(goPath, "bin"))
	if err != nil {
		return "", err
	}
	if !enabled {
		if marked {
			return "", fmt.Errorf("GOPATH %s is reserved for Spine builds; set SPX_SPINE=1 and SPX_SPINE_GOPATH, or use your normal GOPATH", goPath)
		}
		return "", nil
	}
	if goBin := os.Getenv("GOBIN"); goBin != "" && filepath.Clean(goBin) != filepath.Join(goPath, "bin") {
		return "", fmt.Errorf("SPX_SPINE=1 requires GOBIN to be unset or equal to the dedicated GOPATH/bin")
	}
	dedicated := os.Getenv("SPX_SPINE_GOPATH")
	if dedicated == "" || !filepath.IsAbs(goPath) || len(filepath.SplitList(goPath)) != 1 || filepath.Clean(dedicated) != filepath.Clean(goPath) {
		return "", fmt.Errorf("SPX_SPINE=1 requires an explicit dedicated absolute GOPATH and matching SPX_SPINE_GOPATH; see docs/en/dev/engine/spine.md")
	}
	cmd := exec.Command("go", "env", "GOPATH")
	env := CurrentEnvMap()
	delete(env, "GOPATH")
	cmd.Env = EnvMapToSlice(env)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve default GOPATH for Spine isolation: %w", err)
	}
	if filepath.Clean(strings.TrimSpace(string(output))) == filepath.Clean(goPath) {
		return "", fmt.Errorf("Spine GOPATH must differ from the default GOPATH %s", goPath)
	}
	return goPath, nil
}

func prepareSpineWorkspace(goPath string) error {
	bin := filepath.Join(goPath, "bin")
	marked, err := runtimevariant.SpineBinMarked(bin)
	if err != nil {
		return err
	}
	if !marked {
		entries, err := os.ReadDir(bin)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("Spine requires a fresh dedicated GOPATH: %s is not empty", bin)
		}
	}
	if err := os.MkdirAll(bin, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bin, runtimevariant.SpineMarker), []byte("Dedicated local Spine runtime; do not install default/published runtime assets here.\n"), 0644); err != nil {
		return err
	}
	// Godot self-contained mode stores export templates beside the editor, rather
	// than in the user's global Godot directory. Buildctl uses unbundled editors.
	return os.WriteFile(filepath.Join(bin, "_sc_"), nil, 0644)
}

// SpineEnabled validates the explicit, default-off native runtime feature switch.
func SpineEnabled() (bool, error) {
	return runtimevariant.SpineEnabled(os.Environ())
}
