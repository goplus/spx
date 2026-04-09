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
		fmt.Fprintf(os.Stdout, "Godot directory already exists: %s\n", env.EngineDir)
		return nil
	}

	fmt.Fprintf(os.Stdout, "Godot directory not found. Cloning to %s ...\n", env.EngineDir)
	if err := os.MkdirAll(filepath.Dir(env.EngineDir), 0o755); err != nil {
		return err
	}
	return run("git", "clone", "--depth", "1", "--branch", env.EngineGitTag, "https://github.com/goplus/godot.git", env.EngineDir)
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
