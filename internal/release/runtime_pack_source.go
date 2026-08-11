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

package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RuntimePackSourceSHA256 returns a stable digest of the tracked SPX inputs
// that can affect spx-runtime-assets.zip at revision. It deliberately excludes
// godot_modules/spx: the module tree is an independent Godot-engine input and
// is recorded separately in RuntimeProvenance.ModuleTree.
//
// Git tree entries are used instead of worktree contents so untracked build
// outputs cannot contaminate release provenance.
func RuntimePackSourceSHA256(repoRoot, revision string) (string, error) {
	return trackedTreeSHA256(repoRoot, revision, "runtime pack source", isRuntimePackSourcePath)
}

func runtimePackSourceTreeSHA256(tree []byte) (string, error) {
	return selectedTreeSHA256(tree, "runtime pack source", isRuntimePackSourcePath)
}

// RuntimeBuildRecipeSHA256 returns a stable digest of the tracked release and
// buildctl inputs that orchestrate runtime pack generation. Recipe inputs are
// kept separate from RuntimePackSourceSHA256 and from the Godot module tree so
// changing orchestration cannot masquerade as an engine source change.
func RuntimeBuildRecipeSHA256(repoRoot, revision string) (string, error) {
	return trackedTreeSHA256(repoRoot, revision, "runtime build recipe", isRuntimeBuildRecipePath)
}

func runtimeBuildRecipeTreeSHA256(tree []byte) (string, error) {
	return selectedTreeSHA256(tree, "runtime build recipe", isRuntimeBuildRecipePath)
}

var runtimeBuildRecipeFiles = map[string]struct{}{
	".github/scripts/runtime_build_contract.py":          {},
	".github/workflows/release_runtime_assets.yml":       {},
	"cmd/internal/macos_go_toolchain.sh":                 {},
	"internal/cmd/buildctl/main.go":                      {},
	"internal/cmd/buildctl/root.go":                      {},
	"internal/cmd/buildctl/engine/cmd.go":                {},
	"internal/cmd/buildctl/engine/download.go":           {},
	"internal/cmd/buildctl/runtimecmd/assets.go":         {},
	"internal/cmd/buildctl/runtimecmd/cmd.go":            {},
	"internal/cmd/buildctl/shared/api.go":                {},
	"internal/cmd/buildctl/shared/build_env.go":          {},
	"internal/cmd/buildctl/shared/files.go":              {},
	"internal/cmd/buildctl/shared/macos_go_toolchain.go": {},
	"internal/cmd/buildctl/shared/module_source.go":      {},
	"internal/cmd/buildctl/shared/repo.go":               {},
	"internal/cmd/buildctl/shared/runner.go":             {},
	"internal/cmd/buildctl/shared/validate.go":           {},
	"internal/release/runtime_lock.go":                   {},
	"internal/release/runtime_manifest.go":               {},
}

var runtimeBuildRecipePrefixes = []string{
	".github/actions/deps/",
	".github/actions/setup-buildctl/",
	"internal/base/fileutil/",
}

func trackedTreeSHA256(repoRoot, revision, label string, include func(string) bool) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("release: %s repository must not be empty", label)
	}
	if strings.TrimSpace(revision) == "" {
		return "", fmt.Errorf("release: %s revision must not be empty", label)
	}

	command := exec.Command("git", "-C", filepath.Clean(repoRoot), "ls-tree", "-r", "-z", "--full-tree", revision)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			detail := strings.TrimSpace(string(exitErr.Stderr))
			if detail != "" {
				return "", fmt.Errorf("release: list %s tree at %q: %s", label, revision, detail)
			}
		}
		return "", fmt.Errorf("release: list %s tree at %q: %w", label, revision, err)
	}
	return selectedTreeSHA256(output, label, include)
}

func selectedTreeSHA256(tree []byte, label string, include func(string) bool) (string, error) {
	hasher := sha256.New()
	count := 0
	for len(tree) != 0 {
		end := bytes.IndexByte(tree, 0)
		if end < 0 {
			return "", fmt.Errorf("release: malformed NUL-delimited git tree output")
		}
		entry := tree[:end]
		tree = tree[end+1:]
		separator := bytes.IndexByte(entry, '\t')
		if separator < 0 || separator == len(entry)-1 {
			return "", fmt.Errorf("release: malformed git tree entry %q", entry)
		}
		name := string(entry[separator+1:])
		if !include(name) {
			continue
		}
		_, _ = hasher.Write(entry)
		_, _ = hasher.Write([]byte{0})
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("release: %s tree contains no tracked inputs", label)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func isRuntimeBuildRecipePath(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	// Keep this at file granularity: buildctl also contains engine compilation,
	// Web compression, and developer orchestration that do not participate in
	// the release_runtime_assets export-pack path. That workflow passes
	// setup-mode=none, so the general setup/prepare path is excluded as well. If
	// the executed call graph starts using a new helper, add it here with a
	// selector test.
	if _, ok := runtimeBuildRecipeFiles[name]; ok {
		return true
	}
	for _, prefix := range runtimeBuildRecipePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// isRuntimePackSourcePath is intentionally a positive list. In particular,
// changes to release orchestration, buildctl commands unrelated to export-pack,
// documentation, or the external Godot module must not invalidate the runtime
// pack source identity.
func isRuntimePackSourcePath(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	switch name {
	case "go.mod", "go.sum", "gox.mod",
		"fs/schema.go":
		return true
	}
	if !strings.ContainsRune(name, '/') && strings.HasSuffix(name, ".go") {
		return true
	}
	for _, prefix := range []string{
		"cmd/spx/",
		"fs/asset/",
		"fs/zip/",
		"internal/animation/",
		"internal/assets/",
		"internal/audio/",
		"internal/base/",
		"internal/core/",
		"internal/coroutine/",
		"internal/debug/",
		"internal/engine/",
		"internal/enginewrap/",
		"internal/gdengine/",
		"internal/input/",
		"internal/log/",
		"internal/scaffold/",
		"internal/text/",
		"internal/tilemap/",
		"internal/time/",
		"internal/tools/",
		"internal/ui/",
		"pkg/spx/",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
