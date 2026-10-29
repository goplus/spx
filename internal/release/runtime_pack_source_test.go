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
	"io/fs"
	"os"
	"os/exec"
	"path"
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
	}
	tree := gitTreeFixture(entries...)

	digest, err := runtimePackSourcePathTreeSHA256(tree)
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
		{"go.mod", true},
		{"go.sum", false},
		{"gox.mod", true},
		{"internal/release/runtime_pack_source.go", false},
		{"internal/release/runtime_manifest.go", false},
		// The release tag only renders project scaffolding. export-pack uses the
		// repository module and is intentionally independent from this selector.
		{"internal/release/current_spx_version.go", false},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", false},
		{"docs/zh/dev/engine/cmd_make.md", false},
		{"cmd/spx/internal/command/export.go", true},
		{"cmd/spx/internal/command/dispatch.go", true},
		{"cmd/spx/internal/command/buildlauncher.go", false},
		{"cmd/spx/internal/command/run.go", false},
		{"cmd/spx/internal/command/export_web.go", false},
		{"cmd/spx/internal/command/export_staging.go", false},
		{"cmd/spx/internal/command/builderai/sync.go", false},
		{"cmd/spx/internal/pack/pack.go", false},
		{"cmd/spx/template/project/project.godot", true},
		{"cmd/spx/template/project/engine/ui/UiAsk.tscn", true},
		{"cmd/spx/template/project/engine/fonts/default.LICENSE.txt", false},
		{"cmd/spx/template/project/engine/fonts/default.NOTICE.txt", false},
		{"cmd/spx/template/project/.godot/gdspx_web_server.py", false},
		{"cmd/spx/template/project/.builds/keep.txt", false},
		{"cmd/spx/template/project/.gitignore", false},
		{"cmd/spx/template/project/go/ios_adapter.go.txt", false},
		{"cmd/spx/template/platform/web/index.html", false},
		{"cmd/spx/install.sh", false},
		{"fs/fsutil/fsutil.go", false},
		{"fs/asset/asset.go", true},
		{"internal/base/licenseheader/licenseheader.go", false},
		{"internal/audio/audio.go", true},
		{"internal/gdengine/binding/web/ffi.go", false},
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

func TestRuntimePackSourceDigestIgnoresIndependentInputs(t *testing.T) {
	t.Parallel()

	const selected = "100644 blob 1111111111111111111111111111111111111111\tgame.go"
	for _, path := range []string{
		"cmd/spx/internal/command/buildlauncher.go",
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

func TestRuntimePackSourceDigestTracksExportDispatch(t *testing.T) {
	t.Parallel()

	// ExportPack's setup and build phases live in dispatch.go, outside the
	// runtime library dependencies checked below. A dispatch change must prevent
	// reuse even when the exported runtime library itself is unchanged.
	digest := func(blob string) string {
		t.Helper()
		value, err := runtimePackSourcePathTreeSHA256(gitTreeFixture(
			"100644 blob 1111111111111111111111111111111111111111\tgame.go",
			"100644 blob "+blob+"\tcmd/spx/internal/command/dispatch.go",
		))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if digest(strings.Repeat("2", 40)) == digest(strings.Repeat("3", 40)) {
		t.Fatal("changing export-pack dispatch did not invalidate the runtime digest")
	}
}

func TestRuntimeBuildRecipePathSeparatesRecipeFromSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".github/actions/setup-toolchain/action.yml", false},
		{".github/actions/setup-buildctl/action.yml", true},
		{".github/actions/setup-buildctl/build.sh", true}, // Historical commits.
		{".github/scripts/setup/build_buildctl.sh", true},
		{".github/scripts/setup/install_linux_dependencies.sh", false},
		{".github/scripts/setup/setup_test.py", false},
		{".github/scripts/runtime/build_pack.sh", true},
		{".github/scripts/runtime/build_contract.py", true},
		{".github/scripts/runtime_build_contract.py", false},
		{".github/scripts/runtime/build_contract_test.py", false},
		{".github/scripts/runtime/lock_snapshot.py", false},
		{"cmd/internal/macos_go_toolchain.sh", true},
		{"internal/cmd/buildctl/shared/macos_go_toolchain.go", true},
		{"internal/base/quoted/quoted.go", true},
		{"internal/base/quoted/quoted_test.go", false},
		{"internal/httpclient/http.go", true},
		{"internal/httpclient/http_test.go", false},
		{".github/scripts/release/assemble.py", false},
		{".github/cmd/runtime-manifest/main.go", false},
		{".github/cmd/runtime-digest/main.go", false},
		{".github/workflows/build_runtime_pack.yml", false},
		{".github/workflows/release.yml", false},
		{"internal/cmd/buildctl/main.go", true},
		{"internal/cmd/buildctl/root.go", false},
		{"internal/cmd/buildctl/runtime_dispatch.go", true},
		{"internal/cmd/buildctl/simple_cmd.go", false},
		{"internal/cmd/buildctl/prepare.go", false},
		{"internal/cmd/buildctl/runtimecmd/assets.go", false},
		{"internal/cmd/buildctl/runtimecmd/common.go", false},
		{"internal/cmd/buildctl/runtimecmd/cmd.go", false},
		{"internal/cmd/buildctl/runtimecmd/dispatch.go", true},
		{"internal/cmd/buildctl/runtimecmd/compress.go", false},
		{"internal/cmd/buildctl/runtimecmd/local_manifest.go", false},
		{"internal/cmd/buildctl/runtimecmd/pack.go", true},
		{"internal/cmd/buildctl/runtimecmd/workspace.go", true},
		{"internal/cmd/buildctl/shared/api.go", true},
		{"internal/cmd/buildctl/shared/command_runner.go", true},
		{"internal/cmd/buildctl/shared/files.go", true},
		{"internal/cmd/buildctl/shared/runtime_files.go", true},
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
		{"internal/cmd/buildctl/engine/download_linux_pack.go", true},
		{"internal/cmd/buildctl/engine/download_local.go", true},
		{"internal/cmd/buildctl/engine/download_zip.go", true},
		{"internal/cmd/buildctl/engine/download_http.go", false},
		{"internal/cmd/buildctl/engine/download_mobile.go", false},
		{"internal/cmd/buildctl/engine/download_runtime.go", false},
		{"internal/cmd/buildctl/engine/download_web.go", false},
		{"internal/cmd/buildctl/engine/build.go", false},
		{"internal/release/runtime_asset.go", true},
		{"internal/release/runtime.lock.json", false},
		{"internal/release/runtime_lock.go", true},
		{"internal/release/runtime_manifest.go", false},
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
	for directory := range runtimePackSourceDirectories {
		entries, err := os.ReadDir(filepath.Join(repoRoot, filepath.FromSlash(directory)))
		if err != nil {
			t.Errorf("runtime pack directory %q: %v", directory, err)
			continue
		}
		selected := false
		for _, entry := range entries {
			name := path.Join(directory, entry.Name())
			if !entry.IsDir() && isRuntimePackSourcePath(name) {
				selected = true
				break
			}
		}
		if !selected {
			t.Errorf("runtime pack directory %q contains no selected files", directory)
		}
	}
	for _, prefix := range runtimePackSourcePrefixes {
		directory := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
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
			t.Errorf("runtime pack prefix %q: %v", prefix, err)
			continue
		}
		if !selected {
			t.Errorf("runtime pack prefix %q contains no selected files", prefix)
		}
	}
}

func TestRuntimePackSourceDirectoriesCoverRuntimeDependencies(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime_pack_source_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	command := exec.Command("go", "list", "-deps", "-f", `{{if .Module}}{{if eq .Module.Path "github.com/goplus/spx/v4"}}{{.Dir}}{{end}}{{end}}`, ".")
	command.Dir = repoRoot
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range strings.Fields(string(output)) {
		rel, err := filepath.Rel(repoRoot, directory)
		if err != nil {
			t.Fatal(err)
		}
		if rel == "." {
			continue
		}
		probe := path.Join(filepath.ToSlash(rel), "runtime_pack_dependency.go")
		if !isRuntimePackSourcePath(probe) {
			t.Errorf("runtime dependency %q is outside the source selector", filepath.ToSlash(rel))
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

func TestRuntimeBuildRecipeTracksBuildInputs(t *testing.T) {
	t.Parallel()

	const recipeEntry = "100644 blob 1111111111111111111111111111111111111111\t.github/scripts/runtime/build_pack.sh"
	for _, name := range []string{
		".github/scripts/runtime/build_contract.py",
		".github/scripts/setup/build_buildctl.sh",
		"internal/cmd/buildctl/shared/files.go",
		"internal/httpclient/http.go",
	} {
		t.Run(name, func(t *testing.T) {
			before, err := runtimeBuildRecipeTreeSHA256(gitTreeFixture(
				recipeEntry,
				"100644 blob 2222222222222222222222222222222222222222\t"+name,
			))
			if err != nil {
				t.Fatal(err)
			}
			after, err := runtimeBuildRecipeTreeSHA256(gitTreeFixture(
				recipeEntry,
				"100644 blob 3333333333333333333333333333333333333333\t"+name,
			))
			if err != nil {
				t.Fatal(err)
			}
			if before == after {
				t.Fatalf("change to %q did not change the runtime build recipe", name)
			}
		})
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
		"internal/cmd/buildctl/root.go",
		"internal/cmd/buildctl/engine/download.go",
		"internal/cmd/buildctl/engine/download_desktop.go",
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
		tree := gitTreeFixture("100644 blob abc\tinternal/cmd/buildctl/runtimecmd/pack.go")
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
		{"invalid tree entry", "100644 tree abc\tgame.go", "", "invalid git blob entry"},
		{"invalid Go constraint", "100644 blob abc\tgame.go", "//go:build !\n\npackage spx\n", "match runtime pack source"},
		{"missing runtime preset", "100644 blob abc\t" + runtimePackExportPresets, "[preset.0]\nname=\"Other\"\n", "project runtime pack preset"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installReleaseGitFixture(t, fmt.Sprintf("abc blob %d\n%s\n", len(tt.content), tt.content))
			if _, _, err := runtimePackSourcePaths(t.TempDir(), gitTreeFixture(tt.header)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("source selection = %v, want %q", err, tt.want)
			}
		})
	}
	t.Run("public source error", func(t *testing.T) {
		installReleaseGitFixture(t, "100644 tree abc\\tgame.go\\000")
		if _, err := RuntimePackSourceSHA256(t.TempDir(), "HEAD"); err == nil || !strings.Contains(err.Error(), "invalid git blob entry") {
			t.Fatalf("public source digest = %v", err)
		}
	})
}

func TestReadGitBlobsRejectsMalformedProtocol(t *testing.T) {
	for _, tt := range []struct{ name, response, want string }{
		{"missing header", "abc", "malformed git cat-file"},
		{"wrong hash", "other blob 0\n\n", "invalid git cat-file"},
		{"wrong type", "abc tree 0\n\n", "invalid git cat-file"},
		{"invalid size", "abc blob no\n\n", "invalid git blob size"},
		{"negative size", "abc blob -1\n\n", "invalid git blob size"},
		{"short contents", "abc blob 4\nx\n", "invalid git blob size"},
		{"trailing output", "abc blob 1\nx\nextra", "unexpected trailing"},
		{"command failure", "", "read runtime pack source blobs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installReleaseGitFixture(t, tt.response)
			if tt.name == "command failure" {
				t.Setenv("SPX_GIT_STATUS", "9")
			}
			if _, err := readGitBlobs(t.TempDir(), []runtimePackBlob{{name: "game.go", hash: "abc"}}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("read Git blobs = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRuntimeSourceSelectionWithoutGoAndFailedFetch(t *testing.T) {
	paths, projections, err := runtimePackSourcePaths(t.TempDir(), gitTreeFixture("100644 blob abc\tgo.mod", "100644 blob def\tREADME.md"))
	if err != nil || len(paths) != 1 || len(projections) != 0 {
		t.Fatalf("non-Go source selection = %v, %v, %v", paths, projections, err)
	}
	if _, ok := paths["go.mod"]; !ok {
		t.Fatal("module file omitted")
	}
	installReleaseGitFixture(t, "")
	t.Setenv("SPX_GIT_STATUS", "4")
	if _, _, err := runtimePackSourcePaths(t.TempDir(), gitTreeFixture("100644 blob abc\tgame.go")); err == nil || !strings.Contains(err.Error(), "read runtime pack source blobs") {
		t.Fatalf("source blob failure = %v", err)
	}
}

func TestMatchesRuntimePackTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"game.go", "package spx\n", true},
		{"runtime_capture_js.go", "package spx\n", false},
		{"runtime_capture_windows.go", "package spx\n", false},
		{"runtime_capture_linux.go", "package spx\n", true},
		{"open_mobile.go", "//go:build android || ios\n\npackage asset\n", false},
		{"path_packmode.go", "//go:build packmode\n\npackage engine\n", false},
		{"facade_web.go", "//go:build js\n\npackage facade\n", false},
		{"macro_on.go", "//go:build profiler\n\npackage profiler\n", false},
		{"macro_off.go", "//go:build !profiler\n\npackage profiler\n", true},
		{"ffi.go", "//go:build cgo && !js\n\npackage native\n", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := matchesRuntimePackTarget(test.name, []byte(test.content))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("matchesRuntimePackTarget() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRuntimePackSourceSHA256HonorsBuildConstraints(t *testing.T) {
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
	write("game.go", "package spx\n")
	write("runtime_capture_js.go", "package spx\n")
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
	baseline, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	write("runtime_capture_js.go", "package spx\n\nconst changed = true\n")
	commit("change inactive source")
	inactive, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if inactive != baseline {
		t.Fatal("inactive JS source changed Linux runtime pack digest")
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

	write("game.go", "package spx\n\nconst changed = true\n")
	commit("change active source")
	active, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if active == baseline {
		t.Fatal("active Linux source did not change runtime pack digest")
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
