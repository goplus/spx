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

var runtimeBuildRecipeFiles = map[string]struct{}{
	".github/scripts/runtime/build_pack.sh":         {},
	"cmd/spx/internal/command/args.go":              {},
	"cmd/spx/internal/command/cmd.go":               {},
	"cmd/spx/internal/command/dispatch.go":          {},
	"cmd/spx/internal/command/env.go":               {},
	"cmd/spx/internal/command/export.go":            {},
	"cmd/spx/internal/command/platform.go":          {},
	"cmd/spx/internal/util/io.go":                   {},
	"cmd/spx/main.go":                               {},
	"internal/base/fileutil/zip_writer.go":          {},
	"internal/cmd/buildctl/runtimecmd/pack.go":      {},
	"internal/cmd/buildctl/runtimecmd/workspace.go": {},
}

var runtimePackSourceFiles = map[string]struct{}{
	"cmd/spx/appname.txt":                                           {},
	"cmd/spx/template/project/.godot/extension_list.cfg":            {},
	"cmd/spx/template/project/.godot/global_script_class_cache.cfg": {},
	"cmd/spx/template/project/export_presets.cfg":                   {},
	"cmd/spx/template/project/gdspx.gdextension":                    {},
	"cmd/spx/template/project/gdspx.gdextension.uid":                {},
	"cmd/spx/template/project/main.tscn":                            {},
	"cmd/spx/template/project/project.godot":                        {},
	"cmd/spx/template/project/runtime.gdextension.txt":              {},
}

const runtimePackResourcesPrefix = "cmd/spx/template/project/engine/"

// RuntimePackSourceSHA256 returns a stable digest of the tracked SPX inputs
// that can affect spx-runtime-assets.zip at revision. It deliberately excludes
// godot_modules/spx: the module tree is an independent Godot-engine input and
// is recorded separately in RuntimeProvenance.ModuleTree. The dedicated pack
// command copies resources and imports/exports them in recovery mode without
// generating, compiling, or loading the game's Go code. Only the Linux export
// preset contributes to the release pack identity.
//
// Git tree entries are used instead of worktree contents so untracked build
// outputs cannot contaminate release provenance.
func RuntimePackSourceSHA256(repoRoot, revision string) (string, error) {
	tree, err := trackedTree(repoRoot, revision, "runtime pack source")
	if err != nil {
		return "", err
	}
	return transformedTreeSHA256(tree, "runtime pack source", func(entry []byte, name string) ([]byte, error) {
		return runtimePackSourceEntry(repoRoot, entry, name)
	})
}

func runtimePackSourceEntry(repoRoot string, entry []byte, name string) ([]byte, error) {
	if !isRuntimePackSourcePath(name) {
		return nil, nil
	}
	if name != runtimePackExportPresets {
		return entry, nil
	}
	fields := strings.Fields(string(entry[:bytes.IndexByte(entry, '\t')]))
	if len(fields) != 3 || fields[1] != "blob" {
		return nil, fmt.Errorf("release: invalid git blob entry %q", entry)
	}

	command := exec.Command("git", "-C", filepath.Clean(repoRoot), "cat-file", "blob", fields[2])
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("release: read runtime pack preset: %w", err)
	}
	projection, err := projectGodotPreset(output, runtimePackExportPresetName)
	if err != nil {
		return nil, fmt.Errorf("release: project runtime pack preset: %w", err)
	}
	return append([]byte("projection\t"+name+"\x00"), projection...), nil
}

// RuntimeBuildRecipeSHA256 returns a stable digest of the implementation that
// can change runtime-pack bytes. CI transport details such as workflow layout
// and external Action versions are deliberately excluded.
func RuntimeBuildRecipeSHA256(repoRoot, revision string) (string, error) {
	return trackedTreeSHA256(repoRoot, revision, "runtime build recipe", isRuntimeBuildRecipePath)
}

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
	return transformedTreeSHA256(tree, label, func(entry []byte, name string) ([]byte, error) {
		if !include(name) {
			return nil, nil
		}
		return entry, nil
	})
}

func transformedTreeSHA256(tree []byte, label string, selectEntry func([]byte, string) ([]byte, error)) (string, error) {
	hasher := sha256.New()
	count := 0
	err := walkGitTree(tree, func(entry []byte, name string) error {
		selected, err := selectEntry(entry, name)
		if err != nil {
			return err
		}
		if selected == nil {
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
	_, ok := runtimeBuildRecipeFiles[name]
	return ok
}

// Pack identity describes resource data and export configuration, not the Go
// executable that copies it. Semantic export/packaging steps are tracked
// separately by RuntimeBuildRecipeSHA256. Game logic, logging, dependency
// metadata and download transport are not resource inputs.
func isRuntimePackSourcePath(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	if _, ok := runtimePackSourceFiles[name]; ok {
		return true
	}
	return strings.HasPrefix(name, runtimePackResourcesPrefix) &&
		!strings.HasSuffix(name, ".LICENSE.txt") && !strings.HasSuffix(name, ".NOTICE.txt")
}
