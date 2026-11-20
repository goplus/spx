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
	"fmt"
	"strings"
)

const runtimePackResourcesPrefix = "cmd/spx/template/project/engine/"

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
	tree, err := readGitTree(repoRoot, revision, "runtime pack source")
	if err != nil {
		return "", err
	}
	return tree.packSourceSHA256()
}

// RuntimeBuildRecipeSHA256 returns a stable digest of the implementation that
// can change runtime-pack bytes. CI transport details such as workflow layout
// and external Action versions are deliberately excluded.
func RuntimeBuildRecipeSHA256(repoRoot, revision string) (string, error) {
	tree, err := readGitTree(repoRoot, revision, "runtime build recipe")
	if err != nil {
		return "", err
	}
	return tree.buildRecipeSHA256()
}

func (tree gitTree) packSourceSHA256() (string, error) {
	return tree.digest("runtime pack source", isRuntimePackSourcePath, tree.packSourceRecord)
}

func (tree gitTree) packSourceRecord(entry gitTreeEntry) ([]byte, error) {
	if entry.path != runtimePackExportPresets {
		return entry.record, nil
	}
	output, err := tree.readBlob(entry.path)
	if err != nil {
		return nil, fmt.Errorf("release: read runtime pack preset: %w", err)
	}
	projection, err := projectGodotPreset(output, runtimePackExportPresetName)
	if err != nil {
		return nil, fmt.Errorf("release: project runtime pack preset: %w", err)
	}
	return append([]byte("projection\t"+entry.path+"\x00"), projection...), nil
}

func (tree gitTree) buildRecipeSHA256() (string, error) {
	return tree.digest("runtime build recipe", isRuntimeBuildRecipePath, nil)
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
