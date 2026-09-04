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
	"path"
	"path/filepath"
	"strings"
)

// RuntimePackSourceSHA256 returns a stable digest of the tracked SPX inputs
// that can affect spx-runtime-assets.zip at revision. It deliberately excludes
// godot_modules/spx: the module tree is an independent Godot-engine input and
// is recorded separately in RuntimeProvenance.ModuleTree. Go constraints use
// the fixed Linux/amd64 CGO pack target.
//
// Git tree entries are used instead of worktree contents so untracked build
// outputs cannot contaminate release provenance.
func RuntimePackSourceSHA256(repoRoot, revision string) (string, error) {
	tree, err := trackedTree(repoRoot, revision, "runtime pack source")
	if err != nil {
		return "", err
	}
	paths, projections, err := runtimePackSourcePaths(repoRoot, tree)
	if err != nil {
		return "", err
	}
	return selectedRuntimePackSourceSHA256(tree, paths, projections)
}

func runtimePackSourcePathTreeSHA256(tree []byte) (string, error) {
	return selectedTreeSHA256(tree, "runtime pack source", isRuntimePackSourcePath)
}

// RuntimeBuildRecipeSHA256 returns a stable digest of the implementation that
// can change runtime-pack bytes. CI transport details such as workflow layout
// and external Action versions are deliberately excluded.
func RuntimeBuildRecipeSHA256(repoRoot, revision string) (string, error) {
	return trackedTreeSHA256(repoRoot, revision, "runtime build recipe", isRuntimeBuildRecipePath)
}

func runtimeBuildRecipeTreeSHA256(tree []byte) (string, error) {
	return selectedTreeSHA256(tree, "runtime build recipe", isRuntimeBuildRecipePath)
}

var runtimeBuildRecipeFiles = map[string]struct{}{
	".github/scripts/runtime/build_pack.sh":               {},
	".github/scripts/runtime_build_contract.py":           {},
	"cmd/internal/macos_go_toolchain.sh":                  {},
	"internal/cmd/buildctl/main.go":                       {},
	"internal/cmd/buildctl/runtime_dispatch.go":           {},
	"internal/cmd/buildctl/engine/download_linux_pack.go": {},
	"internal/cmd/buildctl/engine/download_local.go":      {},
	"internal/cmd/buildctl/engine/download_zip.go":        {},
	"internal/cmd/buildctl/runtimecmd/dispatch.go":        {},
	"internal/cmd/buildctl/runtimecmd/pack.go":            {},
	"internal/cmd/buildctl/runtimecmd/workspace.go":       {},
	"internal/cmd/buildctl/shared/api.go":                 {},
	"internal/cmd/buildctl/shared/build_env.go":           {},
	"internal/cmd/buildctl/shared/command_runner.go":      {},
	"internal/cmd/buildctl/shared/macos_go_toolchain.go":  {},
	"internal/cmd/buildctl/shared/repo.go":                {},
	"internal/cmd/buildctl/shared/runtime_api.go":         {},
	"internal/cmd/buildctl/shared/runtime_files.go":       {},
	"internal/release/runtime_asset.go":                   {},
	"internal/release/runtime_lock.go":                    {},
}

var runtimeBuildRecipePrefixes = []string{
	".github/actions/setup-buildctl/",
	"internal/base/fileutil/",
}

var runtimePackSourceFiles = map[string]struct{}{
	"cmd/spx/appname.txt":                                           {},
	"cmd/spx/main.go":                                               {},
	"cmd/spx/internal/command/ai_module.go":                         {},
	"cmd/spx/internal/command/args.go":                              {},
	"cmd/spx/internal/command/build.go":                             {},
	"cmd/spx/internal/command/cmd.go":                               {},
	"cmd/spx/internal/command/env.go":                               {},
	"cmd/spx/internal/command/export.go":                            {},
	"cmd/spx/internal/command/logging.go":                           {},
	"cmd/spx/internal/command/platform.go":                          {},
	"cmd/spx/internal/command/builderai/project.go":                 {},
	"cmd/spx/template/project/.godot/extension_list.cfg":            {},
	"cmd/spx/template/project/.godot/global_script_class_cache.cfg": {},
	"cmd/spx/template/project/export_presets.cfg":                   {},
	"cmd/spx/template/project/gdspx.gdextension":                    {},
	"cmd/spx/template/project/gdspx.gdextension.uid":                {},
	"cmd/spx/template/project/main.tscn":                            {},
	"cmd/spx/template/project/project.godot":                        {},
	"cmd/spx/template/project/runtime.gdextension.txt":              {},
}

var runtimePackSourceDirectories = map[string]struct{}{
	"cmd/spx/internal/util":            {},
	"fs":                               {},
	"fs/asset":                         {},
	"fs/zip":                           {},
	"internal/animation":               {},
	"internal/assets":                  {},
	"internal/audio":                   {},
	"internal/base/collision":          {},
	"internal/base/defaults":           {},
	"internal/base/sliceutil":          {},
	"internal/core/event":              {},
	"internal/core/project":            {},
	"internal/core/runtime":            {},
	"internal/core/state":              {},
	"internal/coroutine":               {},
	"internal/debug":                   {},
	"internal/engine":                  {},
	"internal/engine/platform":         {},
	"internal/engine/profiler":         {},
	"internal/enginewrap":              {},
	"internal/gdengine":                {},
	"internal/gdengine/binding/facade": {},
	"internal/gdengine/binding/native": {},
	"internal/gdengine/impl":           {},
	"internal/input":                   {},
	"internal/input/keycode":           {},
	"internal/log":                     {},
	"internal/scaffold":                {},
	"internal/text":                    {},
	"internal/tilemap":                 {},
	"internal/time":                    {},
	"internal/tools":                   {},
	"internal/ui":                      {},
	"internal/zippreflight":            {},
	"pkg/spx":                          {},
	"pkg/spx/pkg/engine":               {},
	"pkg/spx/pkg/gdspx":                {},
}

var runtimePackSourcePrefixes = []string{"cmd/spx/template/project/engine/"}

func trackedTreeSHA256(repoRoot, revision, label string, include func(string) bool) (string, error) {
	tree, err := trackedTree(repoRoot, revision, label)
	if err != nil {
		return "", err
	}
	return selectedTreeSHA256(tree, label, include)
}

func trackedTree(repoRoot, revision, label string) ([]byte, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("release: %s repository must not be empty", label)
	}
	if strings.TrimSpace(revision) == "" {
		return nil, fmt.Errorf("release: %s revision must not be empty", label)
	}

	command := exec.Command("git", "-C", filepath.Clean(repoRoot), "ls-tree", "-r", "-z", "--full-tree", revision)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			detail := strings.TrimSpace(string(exitErr.Stderr))
			if detail != "" {
				return nil, fmt.Errorf("release: list %s tree at %q: %s", label, revision, detail)
			}
		}
		return nil, fmt.Errorf("release: list %s tree at %q: %w", label, revision, err)
	}
	return output, nil
}

func selectedTreeSHA256(tree []byte, label string, include func(string) bool) (string, error) {
	return transformedTreeSHA256(tree, label, func(entry []byte, name string) ([]byte, bool) {
		return entry, include(name)
	})
}

func selectedRuntimePackSourceSHA256(tree []byte, paths map[string]struct{}, projections map[string][]byte) (string, error) {
	return transformedTreeSHA256(tree, "runtime pack source", func(entry []byte, name string) ([]byte, bool) {
		if projection, ok := projections[name]; ok {
			return append([]byte("projection\t"+name+"\x00"), projection...), true
		}
		_, ok := paths[name]
		return entry, ok
	})
}

func transformedTreeSHA256(tree []byte, label string, selectEntry func([]byte, string) ([]byte, bool)) (string, error) {
	hasher := sha256.New()
	count := 0
	err := walkGitTree(tree, func(entry []byte, name string) error {
		selected, ok := selectEntry(entry, name)
		if !ok {
			return nil
		}
		_, _ = hasher.Write(selected)
		_, _ = hasher.Write([]byte{0})
		count++
		return nil
	})
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", fmt.Errorf("release: %s tree contains no tracked inputs", label)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func walkGitTree(tree []byte, visit func(entry []byte, name string) error) error {
	for len(tree) != 0 {
		end := bytes.IndexByte(tree, 0)
		if end < 0 {
			return fmt.Errorf("release: malformed NUL-delimited git tree output")
		}
		entry := tree[:end]
		tree = tree[end+1:]
		separator := bytes.IndexByte(entry, '\t')
		if separator < 0 || separator == len(entry)-1 {
			return fmt.Errorf("release: malformed git tree entry %q", entry)
		}
		if err := visit(entry, string(entry[separator+1:])); err != nil {
			return err
		}
	}
	return nil
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
	if strings.HasPrefix(name, "cmd/spx/template/project/engine/") &&
		(strings.HasSuffix(name, ".LICENSE.txt") || strings.HasSuffix(name, ".NOTICE.txt")) {
		return false
	}
	switch name {
	case "go.mod", "gox.mod":
		return true
	}
	if !strings.ContainsRune(name, '/') && strings.HasSuffix(name, ".go") {
		return true
	}
	if _, ok := runtimePackSourceFiles[name]; ok {
		return true
	}
	if _, ok := runtimePackSourceDirectories[path.Dir(name)]; ok {
		return true
	}
	for _, prefix := range runtimePackSourcePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
