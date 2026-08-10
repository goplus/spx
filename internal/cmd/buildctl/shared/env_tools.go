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
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func ensureEngineSource(repoRoot string, run func(name string, args ...string) error) error {
	env, err := resolveBuildEnvironment(repoRoot, "")
	if err != nil {
		return err
	}
	if fileExists(env.EngineDir) {
		head, err := gitHead(env.EngineDir)
		if err != nil {
			return fmt.Errorf("inspect existing Godot source %s: %w", env.EngineDir, err)
		}
		if head != env.GodotCommit {
			return fmt.Errorf("Godot source %s is at %s, but runtime.lock.json requires %s; update GODOT_SRC or the runtime lock", env.EngineDir, head, env.GodotCommit)
		}
		fmt.Fprintf(os.Stdout, "Using pinned Godot source: %s (%s)\n", env.EngineDir, head)
		return nil
	}

	fmt.Fprintf(os.Stdout, "Godot directory not found. Cloning to %s...\n", env.EngineDir)
	engineParent := filepath.Dir(env.EngineDir)
	if err := os.MkdirAll(engineParent, 0o755); err != nil {
		return err
	}
	stagingDir, err := os.MkdirTemp(engineParent, "."+filepath.Base(env.EngineDir)+".clone-*")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stagingDir)
		}
	}()
	if err := run("git", "-C", stagingDir, "init"); err != nil {
		return err
	}
	if err := run("git", "-C", stagingDir, "remote", "add", "origin", env.GodotRepository); err != nil {
		return err
	}
	if err := run("git", "-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", env.GodotRef); err != nil {
		return err
	}
	if err := run("git", "-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", env.GodotCommit); err != nil {
		// Some Git servers reject direct fetches of unadvertised commit IDs.
		// The lock also carries a reachable ref so the pinned commit can still
		// be obtained by deepening that ref without checking out a moving tip.
		if fallbackErr := run("git", "-C", stagingDir, "fetch", "--filter=blob:none", "--unshallow", "origin", env.GodotRef); fallbackErr != nil {
			return fmt.Errorf("fetch pinned Godot commit %s: %w (ref fallback failed: %v)", env.GodotCommit, err, fallbackErr)
		}
	}
	if err := run("git", "-C", stagingDir, "checkout", "--detach", env.GodotCommit); err != nil {
		return err
	}
	if err := os.Rename(stagingDir, env.EngineDir); err != nil {
		return err
	}
	committed = true
	fmt.Fprintf(os.Stdout, "Checked out pinned Godot commit %s\n", env.GodotCommit)
	return nil
}

func gitHead(dir string) (string, error) {
	output, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func resolveMacOSVulkanSDKRoot(homeDir string, envSDK string) (string, error) {
	if envSDK != "" && fileExists(filepath.Join(envSDK, "bin", "vulkaninfo")) {
		return envSDK, nil
	}

	sdkParent := filepath.Join(homeDir, "VulkanSDK")
	entries, err := os.ReadDir(sdkParent)
	if err != nil {
		return "", err
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	slices.SortFunc(versions, compareVersionish)
	slices.Reverse(versions)

	for _, version := range versions {
		candidate := filepath.Join(sdkParent, version, "macOS")
		if fileExists(filepath.Join(candidate, "bin", "vulkaninfo")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("macOS Vulkan SDK not found under %s", sdkParent)
}

func macOSVulkanSDKShellExports(sdkRoot string) string {
	return strings.Join([]string{
		"export VULKAN_SDK=" + shellQuote(sdkRoot),
		"export PATH=" + shellQuote(filepath.Join(sdkRoot, "bin")+":"+os.Getenv("PATH")),
	}, "\n") + "\n"
}

func compareVersionish(a, b string) int {
	ap := splitVersionish(a)
	bp := splitVersionish(b)
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av := 0
		bv := 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return strings.Compare(a, b)
}

func splitVersionish(value string) []int {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}
