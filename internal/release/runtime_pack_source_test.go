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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimePackSourceTreeSHA256SelectsPackInputs(t *testing.T) {
	t.Parallel()

	entries := []string{
		"100644 blob 1111111111111111111111111111111111111111\tREADME.md",
		"100644 blob 2222222222222222222222222222222222222222\tgame.go",
		"100644 blob 3333333333333333333333333333333333333333\tgame_test.go",
		"100644 blob 4444444444444444444444444444444444444444\tcmd/spx/template/project/runtime.gdextension.txt",
		"100644 blob 5555555555555555555555555555555555555555\tgodot_modules/spx/register_types.cpp",
		"100644 blob 6666666666666666666666666666666666666666\tinternal/release/runtime_pack_source.go",
		"100644 blob 7777777777777777777777777777777777777777\tinternal/scaffold/go.mod.template",
		"100644 blob 8888888888888888888888888888888888888888\tinternal/cmd/buildctl/simple_cmd.go",
		"100644 blob 9999999999999999999999999999999999999999\tinternal/cmd/buildctl/runtimecmd/assets.go",
		"100644 blob aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\tpkg/spx/pkg/engine/engine.go",
		"100644 blob bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\t.github/workflows/release.yml",
	}
	tree := []byte(strings.Join(entries, "\x00") + "\x00")

	digest, err := runtimePackSourceTreeSHA256(tree)
	if err != nil {
		t.Fatal(err)
	}
	hasher := sha256.New()
	for _, index := range []int{1, 3, 6, 9} {
		_, _ = hasher.Write([]byte(entries[index]))
		_, _ = hasher.Write([]byte{0})
	}
	want := hex.EncodeToString(hasher.Sum(nil))
	if digest != want {
		t.Fatalf("runtime pack source SHA-256 = %q, want %q", digest, want)
	}
}

func TestRuntimePackSourcePathSeparatesEngineAndOrchestration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"godot_modules/spx/register_types.cpp", false},
		{"internal/release/runtime_pack_source.go", false},
		{"internal/release/runtime_manifest.go", false},
		// The release tag only renders project scaffolding. export-pack uses the
		// repository module and is intentionally independent from this selector.
		{"internal/release/current_spx_version.go", false},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", false},
		{"docs/zh/dev/engine/cmd_make.md", false},
		{"cmd/spx/internal/command/export.go", true},
		{"cmd/spx/template/project/project.godot", true},
		{"fs/fsutil/fsutil.go", false},
		{"fs/asset/asset.go", true},
		{"internal/audio/audio.go", true},
		{"internal/scaffold/gdextension.go", true},
		{"pkg/ispx/runner.go", false},
		{"pkg/spx/pkg/engine/engine.go", true},
		{"tools/godot/addons/spx_tilemap_exporter/export_cli.gd", false},
	}
	for _, test := range tests {
		if got := isRuntimePackSourcePath(test.path); got != test.want {
			t.Errorf("isRuntimePackSourcePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestRuntimeBuildRecipePathSeparatesRecipeFromSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".github/actions/deps/action.yml", false},
		{".github/actions/setup-buildctl/action.yml", true},
		{".github/scripts/build_runtime_pack.sh", true},
		{".github/scripts/runtime_build_contract.py", true},
		{".github/scripts/runtime_build_contract_test.py", false},
		{".github/scripts/runtime_lock_snapshot.py", false},
		{"cmd/internal/macos_go_toolchain.sh", true},
		{"internal/cmd/buildctl/shared/macos_go_toolchain.go", true},
		{".github/scripts/runtime_asset_manifest.go", false},
		{".github/scripts/runtime_digest.go", false},
		{".github/workflows/release_runtime_assets.yml", false},
		{".github/workflows/release.yml", false},
		{"internal/cmd/buildctl/main.go", true},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/prepare.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", true},
		{"internal/cmd/buildctl/runtimecmd/common.go", false},
		{"internal/cmd/buildctl/runtimecmd/compress.go", false},
		{"internal/cmd/buildctl/shared/files.go", true},
		{"internal/cmd/buildctl/shared/env_tools.go", false},
		{"internal/cmd/buildctl/shared/scons_profile.go", false},
		{"internal/cmd/buildctl/engine/api.go", false},
		{"internal/cmd/buildctl/engine/common.go", false},
		{"internal/cmd/buildctl/engine/download.go", true},
		{"internal/cmd/buildctl/engine/build.go", false},
		{"internal/release/runtime_asset.go", true},
		{"internal/release/runtime.lock.json", false},
		{"internal/release/runtime_lock.go", true},
		{"internal/release/runtime_manifest.go", true},
		{"internal/release/runtime_pack_source.go", false},
		{"internal/release/release_meta.go", false},
		{"internal/release/future_mapping.go", false},
		{"cmd/spx/internal/command/export.go", false},
		{"godot_modules/spx/register_types.cpp", false},
		{"docs/zh/dev/engine/cmd_make.md", false},
	}
	for _, test := range tests {
		if got := isRuntimeBuildRecipePath(test.path); got != test.want {
			t.Errorf("isRuntimeBuildRecipePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestRuntimeBuildRecipeFilesExist(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime_pack_source_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	for name := range runtimeBuildRecipeFiles {
		info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("runtime build recipe input %q: %v", name, err)
			continue
		}
		if !info.Mode().IsRegular() {
			t.Errorf("runtime build recipe input %q is not a regular file", name)
		}
	}
	for _, prefix := range runtimeBuildRecipePrefixes {
		directory := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Errorf("runtime build recipe prefix %q: %v", prefix, err)
			continue
		}
		selected := false
		for _, entry := range entries {
			if !entry.IsDir() && isRuntimeBuildRecipePath(prefix+entry.Name()) {
				selected = true
				break
			}
		}
		if !selected {
			t.Errorf("runtime build recipe prefix %q contains no selected files", prefix)
		}
	}
}

func TestRuntimePackWorkflowDelegatesSemanticBuild(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime_pack_source_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	workflow, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/release_runtime_assets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflowText := string(workflow)
	if !strings.Contains(workflowText, "bash .github/scripts/build_runtime_pack.sh") {
		t.Fatal("runtime asset workflow does not call the semantic pack builder")
	}
	for _, duplicatedCommand := range []string{"\"$BUILDCTL\" engine download", "\"$BUILDCTL\" runtime export-pack"} {
		if strings.Contains(workflowText, duplicatedCommand) {
			t.Fatalf("runtime asset workflow duplicates semantic command %q", duplicatedCommand)
		}
	}
}

func TestRuntimeBuildRecipeIgnoresReleaseMappings(t *testing.T) {
	t.Parallel()

	const recipeEntry = "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/build_runtime_pack.sh"
	before := []byte(strings.Join([]string{
		recipeEntry,
		"100644 blob 2222222222222222222222222222222222222222\tinternal/release/release_meta.go",
	}, "\x00") + "\x00")
	after := []byte(strings.Join([]string{
		recipeEntry,
		"100644 blob 3333333333333333333333333333333333333333\tinternal/release/release_meta.go",
	}, "\x00") + "\x00")

	beforeDigest, err := runtimeBuildRecipeTreeSHA256(before)
	if err != nil {
		t.Fatal(err)
	}
	afterDigest, err := runtimeBuildRecipeTreeSHA256(after)
	if err != nil {
		t.Fatal(err)
	}
	if beforeDigest != afterDigest {
		t.Fatalf("release mapping changed runtime build recipe: %s != %s", beforeDigest, afterDigest)
	}
}

func TestRuntimeBuildRecipeIgnoresCITransport(t *testing.T) {
	t.Parallel()

	semanticEntry := "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/build_runtime_pack.sh"
	before := []byte(strings.Join([]string{
		semanticEntry,
		"100644 blob 2222222222222222222222222222222222222222\t.github/actions/deps/action.yml",
		"100644 blob 3333333333333333333333333333333333333333\t.github/workflows/release_runtime_assets.yml",
	}, "\x00") + "\x00")
	after := []byte(strings.Join([]string{
		semanticEntry,
		"100644 blob 4444444444444444444444444444444444444444\t.github/actions/deps/action.yml",
		"100644 blob 5555555555555555555555555555555555555555\t.github/workflows/release_runtime_assets.yml",
	}, "\x00") + "\x00")

	beforeDigest, err := runtimeBuildRecipeTreeSHA256(before)
	if err != nil {
		t.Fatal(err)
	}
	afterDigest, err := runtimeBuildRecipeTreeSHA256(after)
	if err != nil {
		t.Fatal(err)
	}
	if beforeDigest != afterDigest {
		t.Fatalf("CI transport changed runtime build recipe: %s != %s", beforeDigest, afterDigest)
	}
}

func TestRuntimePackSourceTreeSHA256RejectsMalformedTree(t *testing.T) {
	t.Parallel()

	for _, tree := range [][]byte{
		[]byte("not NUL terminated"),
		[]byte("entry-without-tab\x00"),
		[]byte("100644 blob deadbeef\t\x00"),
	} {
		if _, err := runtimePackSourceTreeSHA256(tree); err == nil {
			t.Fatalf("runtimePackSourceTreeSHA256(%q) succeeded", tree)
		}
	}
}

func TestRuntimePackSourceTreeSHA256RejectsEmptySelection(t *testing.T) {
	t.Parallel()

	tree := []byte("100644 blob 1111111111111111111111111111111111111111\tREADME.md\x00")
	if _, err := runtimePackSourceTreeSHA256(tree); err == nil {
		t.Fatal("runtimePackSourceTreeSHA256 accepted a tree without runtime pack inputs")
	}
}
