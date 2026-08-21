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

func TestRuntimeBuildRecipePathSeparatesRecipeFromSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".github/actions/deps/action.yml", false},
		{".github/actions/setup-buildctl/action.yml", true},
		{".github/scripts/runtime/build_pack.sh", true},
		{".github/scripts/runtime_build_contract.py", true},
		{".github/scripts/runtime_build_contract_test.py", false},
		{".github/scripts/runtime_lock_snapshot.py", false},
		{"cmd/internal/macos_go_toolchain.sh", true},
		{"internal/cmd/buildctl/shared/macos_go_toolchain.go", true},
		{".github/scripts/release/assemble.sh", false},
		{".github/scripts/runtime/manifest.go", false},
		{".github/scripts/runtime/digest.go", false},
		{".github/workflows/release_runtime_assets.yml", false},
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
		{"internal/cmd/buildctl/shared/build_api.go", false},
		{"internal/cmd/buildctl/shared/command_runner.go", true},
		{"internal/cmd/buildctl/shared/files.go", false},
		{"internal/cmd/buildctl/shared/runtime_api.go", true},
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
	command := exec.Command("go", "list", "-deps", "-f", `{{if .Module}}{{if eq .Module.Path "github.com/goplus/spx/v3"}}{{.Dir}}{{end}}{{end}}`, ".")
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
	workflow, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/release_runtime_assets.yml"))
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
		"100644 blob 2222222222222222222222222222222222222222\t.github/actions/deps/action.yml",
		"100644 blob 3333333333333333333333333333333333333333\t.github/workflows/release_runtime_assets.yml",
	)
	after := gitTreeFixture(
		semanticEntry,
		"100644 blob 4444444444444444444444444444444444444444\t.github/actions/deps/action.yml",
		"100644 blob 5555555555555555555555555555555555555555\t.github/workflows/release_runtime_assets.yml",
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

func gitTreeFixture(entries ...string) []byte {
	return []byte(strings.Join(entries, "\x00") + "\x00")
}
