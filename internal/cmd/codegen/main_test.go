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

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
	"github.com/stretchr/testify/require"
)

func TestResolveSPXModuleSourceDefaultsToRepositoryModule(t *testing.T) {
	repoRoot := t.TempDir()

	got := resolveSPXModuleSource(repoRoot, "")
	want := filepath.Join(repoRoot, filepath.FromSlash(release.DefaultRuntimeLock().Module.Path))
	if got != want {
		t.Fatalf("module source = %s, want %s", got, want)
	}
}

func TestResolveSPXModuleSourceResolvesRelativeOverrideFromRepository(t *testing.T) {
	repoRoot := t.TempDir()

	got := resolveSPXModuleSource(repoRoot, filepath.Join("custom", "spx"))
	want := filepath.Join(repoRoot, "custom", "spx")
	if got != want {
		t.Fatalf("module source = %s, want %s", got, want)
	}
}

func TestResolveSPXModuleSourcePreservesAbsoluteOverride(t *testing.T) {
	repoRoot := t.TempDir()
	override := filepath.Join(t.TempDir(), "spx")

	got := resolveSPXModuleSource(repoRoot, override)
	if got != override {
		t.Fatalf("module source = %s, want %s", got, override)
	}
}

func TestValidateCodegenInputsOnlyRequiresGeneratorSources(t *testing.T) {
	moduleSource := newValidSPXModuleFixture(t)
	if err := validateCodegenInputs(moduleSource); err != nil {
		t.Fatalf("validateCodegenInputs() error = %v", err)
	}
}

func TestValidateCodegenInputsRejectsMissingModuleFiles(t *testing.T) {
	for _, missing := range requiredCodegenModuleFiles {
		t.Run(missing, func(t *testing.T) {
			dir := newValidSPXModuleFixture(t)
			if err := os.Remove(filepath.Join(dir, missing)); err != nil {
				t.Fatal(err)
			}

			err := validateCodegenInputs(dir)
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("validateCodegenInputs() error = %v, want missing %s", err, missing)
			}
		})
	}
}

func TestValidateCodegenInputsRejectsInvalidDirectories(t *testing.T) {
	moduleFile := filepath.Join(t.TempDir(), "module-file")
	writeCodegenFixtureFile(t, filepath.Dir(moduleFile), filepath.Base(moduleFile))
	if err := validateCodegenInputs(moduleFile); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("module file error = %v, want not a directory", err)
	}
	dir := newValidSPXModuleFixture(t)
	header := filepath.Join(dir, "gdextension_interface.h")
	require.NoError(t, os.Remove(header))
	require.NoError(t, os.Mkdir(header, 0o755))
	require.ErrorContains(t, validateCodegenInputs(dir), "not a regular file")
}

func TestRunValidatesInputsBeforeWriting(t *testing.T) {
	codegenDir := t.TempDir()
	writeCodegenFixtureFile(t, codegenDir, "sentinel")
	before := generationFiles(t, codegenDir)
	spxModulePath := filepath.Join(t.TempDir(), "missing-module")
	t.Chdir(codegenDir)
	t.Setenv("SPX_MODULE_SRC", spxModulePath)
	require.ErrorContains(t, run(), "SPX_MODULE_SRC")
	if _, err := os.Stat(spxModulePath); !os.IsNotExist(err) {
		t.Fatalf("invalid module path was modified: %v", err)
	}
	after, err := os.ReadDir(codegenDir)
	require.NoError(t, err)
	require.Len(t, after, 1, "validation must not create files or directories")
	require.Equal(t, before, generationFiles(t, codegenDir), "validation must preserve existing files")
}

func writeCodegenFixtureFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newValidSPXModuleFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range requiredCodegenModuleFiles {
		writeCodegenFixtureFile(t, dir, name)
	}
	return dir
}

func TestGenerateCodeFailurePreservesPublishedOutputs(t *testing.T) {
	for _, test := range []struct {
		name, errorText string
		prepare         func(string, *codegenConfig) error
	}{
		{"parser", "parse GDExtension interface", func(root string, _ *codegenConfig) error {
			return os.WriteFile(filepath.Join(root, "internal/gdengine/binding/native/gdextension_spx_pre_define.h"), []byte("invalid C declaration !!!"), 0o600)
		}},
		{"preprocessor", "error preprocessing", func(root string, _ *codegenConfig) error {
			return os.WriteFile(filepath.Join(root, "internal/gdengine/binding/native/gdextension_spx_pre_define.h"), []byte("#ifdef\n"), 0o600)
		}},
		{"web cache validation", "cache function has no matching API", func(root string, _ *codegenConfig) error {
			return os.WriteFile(filepath.Join(root, "internal/gdengine/binding/web/invalid_cache.go"), []byte("package webffi\nfunc CachedMissingAPI(fallback func() bool) bool { return fallback() }\n"), 0o600)
		}},
		{"invalid web cache declaration", "generate manager wrapper", func(root string, _ *codegenConfig) error {
			return os.WriteFile(filepath.Join(root, "internal/gdengine/binding/web/invalid_cache.go"), []byte("package webffi\nfunc CachedInvalid("), 0o600)
		}},
		{"missing array start marker", "missing array types start marker", func(_ string, config *codegenConfig) error {
			return os.WriteFile(filepath.Join(config.spxModulePath, "web/js/engine/gdspx.util.js"), []byte("// END GENERATED ARRAY TYPES\n"), 0o600)
		}},
		{"missing array end marker", "missing array types end marker", func(_ string, config *codegenConfig) error {
			return os.WriteFile(filepath.Join(config.spxModulePath, "web/js/engine/gdspx.util.js"), []byte("// BEGIN GENERATED ARRAY TYPES\n"), 0o600)
		}},
		{"missing parser include", "parse GDExtension interface", func(root string, _ *codegenConfig) error {
			return os.Remove(filepath.Join(root, "internal/gdengine/binding/native/gdextension_spx_pre_define.h"))
		}},
		{"manager header is a directory", "prepare GDExtension headers: read SPX manager header", func(_ string, config *codegenConfig) error {
			return os.Mkdir(filepath.Join(config.spxModulePath, "spx_unreadable_mgr.h"), 0o755)
		}},
		{"blocked publication directory", "publish generated bindings: create generated file directory", func(_ string, config *codegenConfig) error {
			// The existing diagnostic file blocks the first output's parent.
			// No binding may be published before Commit reaches this error.
			config.parsedASTPath = filepath.Join(config.parsedASTPath, "parsed.json")
			return nil
		}},
		{"native output conflict", "generate native bindings: generate native callbacks", func(root string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(root, "internal/gdengine/binding/native/callbacks.gen.go")
			return nil
		}},
		{"synchronized API output conflict", "generate sync_pure.gen.go", func(root string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(root, "internal/enginewrap/sync_pure.gen.go")
			return nil
		}},
		{"Sprite output conflict", "generate Sprite manager implementation", func(root string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(root, "pkg/spx/pkg/engine/sprite_pure.gen.go")
			return nil
		}},
		{"callback defaults output conflict", "generate GDExtension sources: duplicate generated file", func(_ string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(config.spxModulePath, "spx_callback_defaults.gen.h")
			return nil
		}},
		{"extension output conflict", "generate GDExtension sources: duplicate generated file", func(_ string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(config.spxModulePath, "gdextension_spx_ext.cpp")
			return nil
		}},
		{"extension header output conflict", "generate GDExtension sources: duplicate generated file", func(_ string, config *codegenConfig) error {
			config.parsedASTPath = filepath.Join(config.spxModulePath, "gdextension_spx_ext.h")
			return nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, config := newGenerationFixture(t)
			// These outputs must survive even when the native phase has finished
			// rendering and a later Web phase rejects an input.
			for _, name := range []string{"ffi_wrapper.gen.h", "ffi_wrapper.gen.go", "callbacks.gen.go"} {
				require.NoError(t, os.WriteFile(filepath.Join(root, "internal/gdengine/binding/native", name), []byte("previous generation\n"), 0o600))
			}
			config.parsedASTPath = filepath.Join(root, "parsed.json")
			require.NoError(t, os.WriteFile(config.parsedASTPath, []byte("previous diagnostic\n"), 0o600))
			require.NoError(t, test.prepare(root, &config))
			before := generationFiles(t, root)
			require.ErrorContains(t, generateCode(config), test.errorText)
			require.Equal(t, before, generationFiles(t, root), "failed generation must not publish or create outputs")
		})
	}
}

func TestGenerateCodeRejectsMismatchedInputAndOutputRoots(t *testing.T) {
	root, config := newGenerationFixture(t)
	config.codegenDir = filepath.Join(config.codegenDir, "nested")
	before := generationFiles(t, root)
	require.ErrorContains(t, generateCode(config), "gdextension_spx_codegen_header.h")
	require.Equal(t, before, generationFiles(t, root), "a wrong codegen directory must not read ancestor inputs and publish elsewhere")
}

func TestGenerateCodeMatchesPublishedBindingsAndIsDeterministic(t *testing.T) {
	root, config := newGenerationFixture(t)
	inputs := generationFiles(t, root)
	// Newly generated files use the same permissions as ordinary source files,
	// including Web JS. A reference accounts for the host platform and umask.
	reference := filepath.Join(t.TempDir(), "reference")
	require.NoError(t, os.WriteFile(reference, nil, 0o644))
	wantMode, err := os.Stat(reference)
	require.NoError(t, err)
	require.NoError(t, generateCode(config))
	first := generationFiles(t, root)
	require.Greater(t, len(first), len(inputs)+15, "exercise all three generators")
	for _, name := range []string{"callbacks.gen.go", "callbacks.gen.h"} {
		require.Contains(t, first, filepath.Join("internal", "gdengine", "binding", "native", name))
	}
	for path, got := range first {
		if _, isInput := inputs[path]; !isInput {
			info, err := os.Stat(filepath.Join(root, path))
			require.NoError(t, err)
			require.Equal(t, wantMode.Mode().Perm(), info.Mode().Perm(), "%s permissions", path)
		}
		want, err := os.ReadFile(filepath.Join("..", "..", "..", path))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got), "%s differs from the published binding", path)
	}
	require.NoError(t, generateCode(config))
	require.Equal(t, first, generationFiles(t, root), "second generation must be byte-for-byte identical")
}

func newGenerationFixture(t *testing.T) (string, codegenConfig) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "checkout[one]")
	repo := filepath.Join("..", "..", "..")
	for _, pattern := range []string{
		"godot_modules/spx/spx*mgr.h",
		"godot_modules/spx/gdextension_interface.h",
		"godot_modules/spx/web/js/engine/gdspx.util.js",
		"internal/gdengine/binding/native/gdextension_spx_codegen_header.h",
		"internal/gdengine/binding/native/gdextension_spx_pre_define.h",
		"internal/gdengine/binding/native/gdextension_spx_ext.h",
		"internal/gdengine/binding/web/*_cache.go",
	} {
		paths, err := filepath.Glob(filepath.Join(repo, pattern))
		require.NoError(t, err)
		require.NotEmpty(t, paths, pattern)
		for _, path := range paths {
			relative, err := filepath.Rel(repo, path)
			require.NoError(t, err)
			destination := filepath.Join(root, relative)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0o755))
			require.NoError(t, os.WriteFile(destination, data, 0o600))
		}
	}
	return root, codegenConfig{
		codegenDir:    filepath.Join(root, "internal", "cmd", "codegen"),
		spxModulePath: filepath.Join(root, "godot_modules", "spx"),
	}
}

func generationFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative], err = os.ReadFile(path)
		return err
	}))
	return files
}
