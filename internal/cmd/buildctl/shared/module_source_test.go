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

package shared

import (
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestResolveSPXModuleSourcePathDefaultsToRepositoryModule(t *testing.T) {
	repoRoot := t.TempDir()

	got, err := resolveSPXModuleSourcePath(repoRoot, "")
	if err != nil {
		t.Fatalf("resolveSPXModuleSourcePath returned error: %v", err)
	}
	want := filepath.Join(repoRoot, filepath.FromSlash(release.DefaultRuntimeLock().Module.Path))
	if got != want {
		t.Fatalf("module source = %s, want %s", got, want)
	}
}

func TestResolveSPXModuleSourcePathResolvesRelativeOverrideFromRepository(t *testing.T) {
	repoRoot := t.TempDir()

	got, err := resolveSPXModuleSourcePath(repoRoot, filepath.Join("custom", "spx"))
	if err != nil {
		t.Fatalf("resolveSPXModuleSourcePath returned error: %v", err)
	}
	want := filepath.Join(repoRoot, "custom", "spx")
	if got != want {
		t.Fatalf("module source = %s, want %s", got, want)
	}
}

func TestResolveSPXModuleSourcePathPreservesAbsoluteOverride(t *testing.T) {
	repoRoot := t.TempDir()
	override := filepath.Join(t.TempDir(), "spx")

	got, err := resolveSPXModuleSourcePath(repoRoot, override)
	if err != nil {
		t.Fatalf("resolveSPXModuleSourcePath returned error: %v", err)
	}
	if got != override {
		t.Fatalf("module source = %s, want %s", got, override)
	}
}
