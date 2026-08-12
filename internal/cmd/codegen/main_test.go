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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
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

func writeCodegenFixtureFile(t *testing.T, dir, name string) {
	t.Helper()
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
}

func TestGenerateCodeValidatesBeforeWriting(t *testing.T) {
	oldPackagePath := packagePath
	oldSPXModulePath := spxModulePath
	t.Cleanup(func() {
		packagePath = oldPackagePath
		spxModulePath = oldSPXModulePath
	})

	packagePath = t.TempDir()
	sentinelPath := filepath.Join(packagePath, "sentinel")
	writeCodegenFixtureFile(t, packagePath, filepath.Base(sentinelPath))
	sentinelBefore, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	spxModulePath = filepath.Join(t.TempDir(), "missing-module")
	if err := generateCode(); err == nil {
		t.Fatal("generateCode() error = nil, want invalid input error")
	}
	if _, err := os.Stat(spxModulePath); !os.IsNotExist(err) {
		t.Fatalf("invalid module path was modified: %v", err)
	}
	after, err := os.ReadDir(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) || after[0].Name() != before[0].Name() {
		t.Fatalf("package output changed before validation: before=%v after=%v", before, after)
	}
	sentinelAfter, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(sentinelAfter) != string(sentinelBefore) {
		t.Fatalf("sentinel changed before validation: before=%q after=%q", sentinelBefore, sentinelAfter)
	}
}
