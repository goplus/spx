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
	"io/fs"
	"os"
	"os/exec"
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
		"100644 blob cccccccccccccccccccccccccccccccccccccccc\tcmd/spx/internal/command/buildlauncher.go",
		"100644 blob dddddddddddddddddddddddddddddddddddddddd\tinternal/base/defaults.go",
		"100644 blob eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee\tinternal/base/fileutil/zip_preflight.go",
		"100644 blob ffffffffffffffffffffffffffffffffffffffff\tinternal/base/log/log.go",
		"100644 blob 1234567890123456789012345678901234567890\tinternal/log/log.go",
	}
	tree := gitTreeFixture(entries...)

	digest, err := runtimePackSourcePathTreeSHA256(tree)
	if err != nil {
		t.Fatal(err)
	}
	hasher := sha256.New()
	for _, index := range []int{3} {
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
		{"go.mod", false},
		{"go.sum", false},
		{"gox.mod", false},
		{"internal/release/runtime_pack_source.go", false},
		{"internal/release/runtime_manifest.go", false},
		// The pack command copies resources without rendering Go scaffolding.
		{"internal/release/current_spx_version.go", false},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", false},
		{"docs/zh/dev/engine/cmd_make.md", false},
		{"cmd/spx/internal/command/export.go", false},
		{"cmd/spx/internal/command/dispatch.go", false},
		{"cmd/spx/internal/command/build.go", false},
		{"cmd/spx/internal/command/ai_module.go", false},
		{"cmd/spx/internal/command/builderai/project.go", false},
		{"cmd/spx/internal/util/io.go", false},
		{"cmd/spx/internal/util/cmd.go", false},
		{"cmd/spx/internal/logutil/logutil.go", false},
		{"cmd/spx/internal/command/buildlauncher.go", false},
		{"cmd/spx/internal/command/run.go", false},
		{"cmd/spx/internal/command/export_web.go", false},
		{"cmd/spx/internal/command/export_staging.go", false},
		{"cmd/spx/internal/command/builderai/sync.go", false},
		{"cmd/spx/internal/pack/pack.go", false},
		{"cmd/spx/template/project/project.godot", true},
		{"cmd/spx/template/project/engine/ui/UiAsk.tscn", true},
		{"cmd/spx/template/project/engine/fonts/default.ttf.import", true},
		{"cmd/spx/template/project/engine/textures/monitor/slider_thumb.svg.import", true},
		{"cmd/spx/template/project/engine/fonts/default.LICENSE.txt", false},
		{"cmd/spx/template/project/engine/fonts/default.NOTICE.txt", false},
		{"cmd/spx/template/project/.godot/gdspx_web_server.py", false},
		{"cmd/spx/template/project/.builds/keep.txt", false},
		{"cmd/spx/template/project/.gitignore", false},
		{"cmd/spx/template/project/go/ios_adapter.go.txt", false},
		{"cmd/spx/template/platform/web/index.html", false},
		{"cmd/spx/install.sh", false},
		{"fs/fsutil/fsutil.go", false},
		{"fs/asset/asset.go", false},
		{"internal/base/defaults.go", false},
		{"internal/base/slice.go", false},
		{"internal/base/slice_test.go", false},
		{"internal/base/fileutil/fileutil.go", false},
		{"internal/base/fileutil/path.go", false},
		{"internal/base/fileutil/stable_read.go", false},
		{"internal/base/fileutil/zip_preflight.go", false},
		{"internal/base/fileutil/zip_preflight_test.go", false},
		{"internal/base/httpclient/http.go", false},
		{"internal/base/strictjson/decode.go", false},
		{"internal/base/quoted/quoted.go", false},
		{"internal/base/licenseheader/licenseheader.go", false},
		{"internal/base/log/log.go", false},
		{"internal/base/log/log_test.go", false},
		{"internal/log/log.go", false},
		{"internal/log/log_test.go", false},
		{"internal/core/clock/clock.go", false},
		{"internal/core/clock/clock_test.go", false},
		{"internal/core/collision/collision.go", false},
		{"internal/core/collision/collision_test.go", false},
		{"internal/core/scratch/list.go", false},
		{"internal/core/scratch/list_test.go", false},
		{"internal/core/input/replay.go", false},
		{"internal/core/input/replay_test.go", false},
		{"internal/core/input/keycode/keycode.go", false},
		{"internal/core/input/keycode/parse.go", false},
		{"internal/core/input/keycode/parse_test.go", false},
		{"internal/core/project/asset_index.go", false},
		{"internal/core/project/asset_index_test.go", false},
		{"internal/core/tilemap/load.go", false},
		{"internal/core/tilemap/tilemap.go", false},
		{"internal/core/tilemap/load_test.go", false},
		{"internal/engine/coroutine/scheduler.go", false},
		{"internal/engine/coroutine/scheduler_test.go", false},
		{"internal/engine/ui/ui_ask.go", false},
		{"internal/engine/ui/text_wrap.go", false},
		{"internal/engine/ui/ui_lifecycle_test.go", false},
		{"sound_manager.go", false},
		{"monitor.go", false},
		{"internal/core/project/member_eval.go", false},
		{"internal/core/scratch/monitor.go", false},
		{"internal/engine/ui/ui_monitor.go", false},
		{"sound_test.go", false},
		{"internal/gdengine/binding/web/ffi.go", false},
		{"internal/scaffold/gdextension.go", false},
		{"pkg/ispx/runner.go", false},
		{"pkg/spx/pkg/engine/engine.go", false},
		{"tools/godot/addons/spx_tilemap_exporter/export_cli.gd", false},
	}
	for _, test := range tests {
		if got := isRuntimePackSourcePath(test.path); got != test.want {
			t.Errorf("isRuntimePackSourcePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestRuntimePackSourceDigestIgnoresIndependentInputs(t *testing.T) {
	t.Parallel()

	const selected = "100644 blob 1111111111111111111111111111111111111111\tcmd/spx/template/project/main.tscn"
	for _, path := range []string{
		"cmd/spx/internal/command/buildlauncher.go",
		"cmd/spx/internal/logutil/logutil.go",
		"internal/base/log/log.go",
		"cmd/spx/internal/command/export.go",
		"cmd/spx/internal/command/export_staging.go",
		"cmd/spx/internal/pack/pack.go",
		"cmd/spx/template/project/.godot/gdspx_web_server.py",
		"cmd/spx/template/project/go/ios_adapter.go.txt",
	} {
		before, err := runtimePackSourcePathTreeSHA256(gitTreeFixture(
			selected,
			"100644 blob 2222222222222222222222222222222222222222\t"+path,
		))
		if err != nil {
			t.Fatal(err)
		}
		after, err := runtimePackSourcePathTreeSHA256(gitTreeFixture(
			selected,
			"100644 blob 3333333333333333333333333333333333333333\t"+path,
		))
		if err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Errorf("independent input %q changed runtime pack digest", path)
		}
	}
}

func TestRuntimeBuildRecipePathSeparatesRecipeFromSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".github/actions/setup-toolchain/action.yml", false},
		{".github/actions/setup-buildctl/action.yml", false},
		{".github/actions/setup-buildctl/build.sh", false}, // Historical commits.
		{".github/scripts/setup/build_buildctl.sh", false},
		{".github/scripts/setup/install_linux_dependencies.sh", false},
		{".github/scripts/setup/setup_test.py", false},
		{".github/scripts/runtime/build_pack.sh", true},
		{".github/scripts/runtime/build_contract.py", false},
		{".github/scripts/runtime_build_contract.py", false},
		{".github/scripts/runtime/build_contract_test.py", false},
		{".github/scripts/runtime/lock_snapshot.py", false},
		{"cmd/internal/macos_go_toolchain.sh", false},
		{"internal/cmd/buildctl/shared/macos_go_toolchain.go", false},
		{"internal/base/quoted/quoted.go", false},
		{"internal/base/quoted/quoted_test.go", false},
		{"internal/base/httpclient/http.go", false},
		{"internal/base/httpclient/http_test.go", false},
		{"internal/base/fileutil/fileutil.go", false},
		{"internal/base/fileutil/zip_writer.go", true},
		{"internal/base/fileutil/fileutil_test.go", false},
		{"internal/base/fileutil/path.go", false},
		{"internal/base/fileutil/stable_read.go", false},
		{"internal/base/fileutil/zip_preflight.go", false},
		{".github/scripts/release/assemble.py", false},
		{".github/cmd/runtime-manifest/main.go", false},
		{".github/cmd/runtime-digest/main.go", false},
		{".github/workflows/build_runtime_pack.yml", false},
		{".github/workflows/release.yml", false},
		{"internal/cmd/buildctl/main.go", false},
		{"internal/cmd/buildctl/root.go", false},
		{"internal/cmd/buildctl/runtime_dispatch.go", false},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/prepare.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", false},
		{"internal/cmd/buildctl/runtimecmd/common.go", false},
		{"internal/cmd/buildctl/runtimecmd/cmd.go", false},
		{"internal/cmd/buildctl/runtimecmd/dispatch.go", false},
		{"internal/cmd/buildctl/runtimecmd/compress.go", false},
		{"internal/cmd/buildctl/runtimecmd/local_manifest.go", false},
		{"internal/cmd/buildctl/runtimecmd/pack.go", true},
		{"internal/cmd/buildctl/runtimecmd/workspace.go", true},
		{"internal/cmd/buildctl/shared/api.go", false},
		{"internal/cmd/buildctl/shared/command_runner.go", false},
		{"internal/cmd/buildctl/shared/files.go", false},
		{"internal/cmd/buildctl/shared/gopath.go", false},
		{"internal/cmd/buildctl/shared/gopath_test.go", false},
		{"internal/cmd/buildctl/shared/runtime_files.go", false}, // Historical revisions.
		{"internal/cmd/buildctl/shared/runtime_files_test.go", false},
		{"internal/cmd/buildctl/shared/runner.go", false},
		{"internal/cmd/buildctl/shared/validate.go", false},
		{"internal/cmd/buildctl/shared/workflow_runner.go", false},
		{"internal/cmd/buildctl/shared/env_tools.go", false},
		{"internal/cmd/buildctl/shared/module_source.go", false},
		{"internal/cmd/buildctl/shared/scons_profile.go", false},
		{"internal/cmd/buildctl/engine/api.go", false},
		{"internal/cmd/buildctl/engine/cmd.go", false},
		{"internal/cmd/buildctl/engine/common.go", false},
		{"internal/cmd/buildctl/engine/download.go", false},
		{"internal/cmd/buildctl/engine/download_desktop.go", false},
		{"internal/cmd/buildctl/engine/download_env.go", false},
		{"internal/cmd/buildctl/engine/download_local.go", false},
		{"internal/cmd/buildctl/engine/download_zip.go", false},
		{"internal/cmd/buildctl/engine/download_http.go", false},
		{"internal/cmd/buildctl/engine/download_mobile.go", false},
		{"internal/cmd/buildctl/engine/download_runtime.go", false},
		{"internal/cmd/buildctl/engine/download_web.go", false},
		{"internal/cmd/buildctl/engine/build.go", false},
		{"internal/release/runtime_asset.go", false},
		{"internal/release/runtime.lock.json", false},
		{"internal/release/runtime_lock.go", false},
		{"internal/release/runtime_manifest.go", false},
		{"internal/release/runtime_pack_source.go", false},
		{"internal/release/release_meta.go", false},
		{"internal/release/future_mapping.go", false},
		{"cmd/spx/internal/command/export.go", true},
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

}

func TestRuntimePackSourceSelectorsExist(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime_pack_source_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	for name := range runtimePackSourceFiles {
		info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("runtime pack input %q: %v", name, err)
			continue
		}
		if !info.Mode().IsRegular() {
			t.Errorf("runtime pack input %q is not a regular file", name)
		}
	}
	directory := filepath.Join(repoRoot, filepath.FromSlash(runtimePackResourcesPrefix))
	selected := false
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		if isRuntimePackSourcePath(filepath.ToSlash(rel)) {
			selected = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !selected {
		t.Errorf("runtime pack resources %q contain no selected files", runtimePackResourcesPrefix)
	}
}

func TestRuntimePackWorkflowDelegatesSemanticBuild(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime_pack_source_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	workflow, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/build_runtime_pack.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflowText := string(workflow)
	if !strings.Contains(workflowText, "bash .github/scripts/runtime/build_pack.sh") {
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

	const recipeEntry = "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/runtime/build_pack.sh"
	before := gitTreeFixture(
		recipeEntry,
		"100644 blob 2222222222222222222222222222222222222222\tinternal/release/release_meta.go",
	)
	after := gitTreeFixture(
		recipeEntry,
		"100644 blob 3333333333333333333333333333333333333333\tinternal/release/release_meta.go",
	)

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

	semanticEntry := "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/runtime/build_pack.sh"
	before := gitTreeFixture(
		semanticEntry,
		"100644 blob 2222222222222222222222222222222222222222\t.github/actions/setup-toolchain/action.yml",
		"100644 blob 3333333333333333333333333333333333333333\t.github/workflows/build_runtime_pack.yml",
	)
	after := gitTreeFixture(
		semanticEntry,
		"100644 blob 4444444444444444444444444444444444444444\t.github/actions/setup-toolchain/action.yml",
		"100644 blob 5555555555555555555555555555555555555555\t.github/workflows/build_runtime_pack.yml",
	)

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

func TestRuntimeBuildRecipeIgnoresIndependentBuildPaths(t *testing.T) {
	t.Parallel()

	const selected = "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/runtime/build_pack.sh"
	for _, name := range []string{
		".github/scripts/runtime_build_contract.py",
		".github/scripts/setup/install_linux_dependencies.sh",
		".github/scripts/setup/setup_test.py",
		"internal/base/fileutil/path.go",
		"internal/base/fileutil/stable_read.go",
		"internal/base/fileutil/zip_preflight.go",
		"internal/cmd/buildctl/root.go",
		"internal/cmd/buildctl/engine/download.go",
		"internal/cmd/buildctl/engine/download_desktop.go",
		"internal/cmd/buildctl/engine/download_env.go",
		"internal/cmd/buildctl/engine/download_http.go",
		"internal/cmd/buildctl/engine/download_mobile.go",
		"internal/cmd/buildctl/engine/download_runtime.go",
		"internal/cmd/buildctl/engine/download_web.go",
		"internal/cmd/buildctl/runtimecmd/cmd.go",
	} {
		before, err := runtimeBuildRecipeTreeSHA256(gitTreeFixture(
			selected,
			"100644 blob 2222222222222222222222222222222222222222\t"+name,
		))
		if err != nil {
			t.Fatal(err)
		}
		after, err := runtimeBuildRecipeTreeSHA256(gitTreeFixture(
			selected,
			"100644 blob 3333333333333333333333333333333333333333\t"+name,
		))
		if err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Errorf("independent build path %q changed runtime recipe", name)
		}
	}
}

func TestRuntimePackSourceTreeSHA256RejectsMalformedTree(t *testing.T) {
	t.Parallel()

	for _, tree := range [][]byte{
		[]byte("not NUL terminated"),
		[]byte("entry-without-tab\x00"),
		[]byte("100644 blob deadbeef\t\x00"),
	} {
		if _, err := runtimePackSourcePathTreeSHA256(tree); err == nil {
			t.Fatalf("runtimePackSourcePathTreeSHA256(%q) succeeded", tree)
		}
	}
}

func TestRuntimePackSourceTreeSHA256RejectsEmptySelection(t *testing.T) {
	t.Parallel()

	tree := []byte("100644 blob 1111111111111111111111111111111111111111\tREADME.md\x00")
	if _, err := runtimePackSourcePathTreeSHA256(tree); err == nil {
		t.Fatal("runtimePackSourcePathTreeSHA256 accepted a tree without runtime pack inputs")
	}
}

func TestTrackedRuntimeDigestsRejectInvalidRepositoryResponses(t *testing.T) {
	for _, digest := range []struct {
		name string
		run  func(string, string) (string, error)
	}{
		{"pack", RuntimePackSourceSHA256}, {"recipe", RuntimeBuildRecipeSHA256},
	} {
		t.Run(digest.name, func(t *testing.T) {
			installReleaseGitFixture(t, "")
			for _, args := range [][2]string{{" ", "HEAD"}, {t.TempDir(), " "}} {
				if _, err := digest.run(args[0], args[1]); err == nil || !strings.Contains(err.Error(), "must not be empty") {
					t.Fatalf("empty Git input = %v", err)
				}
			}
			for _, stderr := range []string{"", "revision unavailable"} {
				t.Setenv("SPX_GIT_STATUS", "7")
				t.Setenv("SPX_GIT_ERROR", stderr)
				if _, err := digest.run(t.TempDir(), "bad-revision"); err == nil || !strings.Contains(err.Error(), "list runtime") || (stderr != "" && !strings.Contains(err.Error(), stderr)) {
					t.Fatalf("Git failure = %v", err)
				}
			}
		})
	}
	t.Run("recipe success", func(t *testing.T) {
		tree := gitTreeFixture("100644 blob abc\t.github/scripts/runtime/build_pack.sh")
		installReleaseGitFixture(t, strings.ReplaceAll(string(tree), "\x00", "\\000"))
		want, err := runtimeBuildRecipeTreeSHA256(tree)
		if err != nil {
			t.Fatal(err)
		}
		got, err := RuntimeBuildRecipeSHA256(t.TempDir(), "HEAD")
		if err != nil || got != want {
			t.Fatalf("recipe digest = %q, %v, want %q", got, err, want)
		}
	})
}

func TestRuntimeSourceSelectionRejectsMalformedBlobs(t *testing.T) {
	for _, tt := range []struct{ name, header, content, want string }{
		{"invalid tree entry", "100644 tree abc\t" + runtimePackExportPresets, "", "invalid git blob entry"},
		{"missing runtime preset", "100644 blob abc\t" + runtimePackExportPresets, "[preset.0]\nname=\"Other\"\n", "project runtime pack preset"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installReleaseGitFixture(t, tt.content)
			tree, err := parseGitTree(t.TempDir(), gitTreeFixture(tt.header))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tree.packSourceSHA256(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("source selection = %v, want %q", err, tt.want)
			}
		})
	}
	t.Run("public source error", func(t *testing.T) {
		installReleaseGitFixture(t, "100644 tree abc\\t"+runtimePackExportPresets+"\\000")
		if _, err := RuntimePackSourceSHA256(t.TempDir(), "HEAD"); err == nil || !strings.Contains(err.Error(), "invalid git blob entry") {
			t.Fatalf("public source digest = %v", err)
		}
	})
}

func TestRuntimeSourceSelectionReadsOnlyExportPreset(t *testing.T) {
	installReleaseGitFixture(t, "")
	t.Setenv("SPX_GIT_STATUS", "4")
	const scene = "100644 blob abc\tcmd/spx/template/project/main.tscn"
	tree, err := parseGitTree(t.TempDir(), gitTreeFixture(scene, "100644 blob def\tREADME.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(scene + "\x00"))
	if got, err := tree.packSourceSHA256(); err != nil || got != hex.EncodeToString(want[:]) {
		t.Fatalf("pack source digest = %q, %v", got, err)
	}
	tree, err = parseGitTree(t.TempDir(), gitTreeFixture("100644 blob abc\t"+runtimePackExportPresets))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.packSourceSHA256(); err == nil || !strings.Contains(err.Error(), "read runtime pack preset") {
		t.Fatalf("preset blob failure = %v", err)
	}
}

func TestRuntimePackDigestsSeparateResourcesAndRecipe(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=SPX Test", "GIT_AUTHOR_EMAIL=spx@example.com",
			"GIT_COMMITTER_NAME=SPX Test", "GIT_COMMITTER_EMAIL=spx@example.com",
		)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) {
		t.Helper()
		runGit("add", ".")
		runGit("commit", "-q", "-m", message)
	}

	runGit("init", "-q")
	write("monitor.go", "package spx\n")
	write("cmd/spx/internal/command/export.go", "package command\n")
	write("capture_js.go", "package spx\n")
	write(runtimePackExportPresets, `[preset.0]
name="Android"
[preset.0.options]
version/code=1
[preset.1]
name="Linux"
[preset.1.options]
binary_format/architecture="x86_64"
`)
	commit("initial")
	runGit("tag", "baseline")
	baseline, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	write("capture_js.go", "package spx\n\nconst changed = true\n")
	write("monitor.go", "package spx\n\nconst changed = true\n")
	write("internal/core/project/member_eval.go", "package project\n")
	write("internal/core/scratch/monitor.go", "package scratch\n")
	write("internal/engine/ui/ui_monitor.go", "package ui\n")
	write("go.mod", "module example.com/game\n\ngo 1.26.0\n")
	write("gox.mod", "project spx\n")
	commit("change Go game logic and its dependencies")
	inactive, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if inactive != baseline {
		t.Fatal("Go game logic changed resource-only runtime pack digest")
	}

	presetsPath := filepath.Join(repo, filepath.FromSlash(runtimePackExportPresets))
	presets, err := os.ReadFile(presetsPath)
	if err != nil {
		t.Fatal(err)
	}
	write(runtimePackExportPresets, strings.Replace(string(presets), "version/code=1", "version/code=2", 1))
	commit("change Android preset")
	android, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if android != baseline {
		t.Fatal("Android preset changed Linux runtime pack digest")
	}

	write("cmd/spx/internal/command/export.go", "package command\n\nconst changed = true\n")
	commit("change export command")
	active, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if active != baseline {
		t.Fatal("Go command changed resource-only runtime pack digest")
	}
	recipe, err := RuntimeBuildRecipeSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		path   string
		repack bool
	}{
		{"internal/base/fileutil/zip_writer.go", true},
		{"internal/base/fileutil/fileutil.go", false},
		{"cmd/spx/internal/logutil/logutil.go", false},
	} {
		write(change.path, "updated implementation\n")
		commit("change " + change.path)
		pack, err := RuntimePackSourceSHA256(repo, "HEAD")
		if err != nil || pack != baseline {
			t.Fatalf("%s changed resource identity: %s, %v", change.path, pack, err)
		}
		next, err := RuntimeBuildRecipeSHA256(repo, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if (next != recipe) != change.repack {
			t.Fatalf("%s requires repack = %t, want %t", change.path, next != recipe, change.repack)
		}
		recipe = next
	}
	previous := active
	for _, resource := range []string{
		"cmd/spx/template/project/engine/ui/UiMonitor.tscn",
		"cmd/spx/template/project/engine/fonts/default.ttf.import",
		"cmd/spx/template/project/runtime.gdextension.txt",
	} {
		write(resource, "updated resource contents\n")
		commit("change " + resource)
		updated, err := RuntimePackSourceSHA256(repo, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if updated == previous {
			t.Fatalf("resource %s did not change runtime pack digest", resource)
		}
		previous = updated
	}
	write(runtimePackExportPresets, strings.Replace(string(presets), "x86_64", "arm64", 1))
	commit("change Linux export preset")
	linux, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if linux == previous {
		t.Fatal("Linux preset did not change runtime pack digest")
	}

	// Both HEAD and the dirty worktree differ from the requested revision.
	write(runtimePackExportPresets, "invalid uncommitted preset\n")
	historical, err := RuntimePackSourceSHA256(repo, "baseline")
	if err != nil {
		t.Fatal(err)
	}
	if historical != baseline {
		t.Fatalf("historical pack digest = %s, want %s", historical, baseline)
	}
}

func TestProjectGodotPreset(t *testing.T) {
	content := []byte(`[preset.0]
name="Android"
runnable=true

[preset.0.options]
architectures/arm64-v8a=true

[preset.7]
name="Linux"
runnable=true

[preset.7.options]
custom_template/debug=""
binary_format/architecture="x86_64"
`)
	got, err := projectGodotPreset(content, "Linux")
	if err != nil {
		t.Fatal(err)
	}
	want := "[preset]\nname=\"Linux\"\nrunnable=true\n[preset.options]\ncustom_template/debug=\"\"\nbinary_format/architecture=\"x86_64\"\n"
	if string(got) != want {
		t.Fatalf("projectGodotPreset() = %q, want %q", got, want)
	}
}

func TestProjectGodotPresetRejectsMissingOrDuplicate(t *testing.T) {
	for _, content := range []string{
		"[preset.0]\nname=\"Web\"\n",
		"[preset.0]\nname=\"Linux\"\n[preset.0.options]\na=1\n[preset.1]\nname=\"Linux\"\n[preset.1.options]\na=2\n",
	} {
		if _, err := projectGodotPreset([]byte(content), "Linux"); err == nil {
			t.Fatalf("projectGodotPreset(%q) succeeded", content)
		}
	}
}

func TestProjectGodotPresetIgnoresOtherPresets(t *testing.T) {
	content := `[preset.0]
name="Android"
[preset.0.options]
version/code=1
[preset.1]
name="Linux"
[preset.1.options]
binary_format/architecture="x86_64"
`
	baseline, err := projectGodotPreset([]byte(content), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	android, err := projectGodotPreset([]byte(strings.Replace(content, "version/code=1", "version/code=2", 1)), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(android, baseline) {
		t.Fatal("Android-only change affected Linux preset")
	}
	linux, err := projectGodotPreset([]byte(strings.Replace(content, "x86_64", "arm64", 1)), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(linux, baseline) {
		t.Fatal("Linux change did not affect projected preset")
	}
}

func TestProjectGodotPresetRequiresOptions(t *testing.T) {
	if _, err := projectGodotPreset([]byte("[preset.0]\nname=\"Linux\"\n"), "Linux"); err == nil || !strings.Contains(err.Error(), "has no options") {
		t.Fatalf("missing preset options = %v", err)
	}
}

func runtimePackSourcePathTreeSHA256(data []byte) (string, error) {
	tree, err := parseGitTree("", data)
	if err != nil {
		return "", err
	}
	return tree.digest("runtime pack source", isRuntimePackSourcePath, nil)
}

func runtimeBuildRecipeTreeSHA256(data []byte) (string, error) {
	tree, err := parseGitTree("", data)
	if err != nil {
		return "", err
	}
	return tree.buildRecipeSHA256()
}

func gitTreeFixture(entries ...string) []byte {
	return []byte(strings.Join(entries, "\x00") + "\x00")
}

// Fake Git serves local protocol responses; PATH excludes the host Git binary.
func installReleaseGitFixture(t *testing.T, output string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixture")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '%b' \"$SPX_GIT_OUTPUT\"\nprintf '%s' \"$SPX_GIT_ERROR\" >&2\nexit \"$SPX_GIT_STATUS\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("SPX_GIT_OUTPUT", output)
	t.Setenv("SPX_GIT_ERROR", "")
	t.Setenv("SPX_GIT_STATUS", "0")
}
