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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSPXModuleFixture(t *testing.T, root string) string {
	t.Helper()
	source := filepath.Join(root, "godot_modules", "spx")
	for _, name := range []string{"SCsub", "config.py"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(source, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile := []byte(`{
  "schema": 1,
  "common": ["optimize=size"],
  "editor_release": ["debug_symbols=true"],
  "template_release": ["debug_symbols=false"]
}`)
	if err := os.WriteFile(filepath.Join(source, SConsProfileFilename), profile, 0o600); err != nil {
		t.Fatal(err)
	}
	return source
}

func TestResolveSPXModuleKeepsSourceAndProfileTogether(t *testing.T) {
	repoRoot := t.TempDir()
	wantSource := writeSPXModuleFixture(t, repoRoot)

	module, err := ResolveSPXModule(repoRoot)
	if err != nil {
		t.Fatalf("ResolveSPXModule returned error: %v", err)
	}
	if module.Source() != wantSource {
		t.Fatalf("module source = %s, want %s", module.Source(), wantSource)
	}
	wantLast := "custom_modules=" + wantSource
	args := module.TemplateBuildArgs("platform=web", "target=template_release")
	if args[len(args)-1] != wantLast {
		t.Fatalf("last SCons argument = %q, want %q", args[len(args)-1], wantLast)
	}
}

func TestLoadSPXModuleRejectsIncompleteBuildContract(t *testing.T) {
	for _, missing := range []string{"SCsub", "config.py", SConsProfileFilename} {
		t.Run(missing, func(t *testing.T) {
			repoRoot := t.TempDir()
			source := writeSPXModuleFixture(t, repoRoot)
			if err := os.Remove(filepath.Join(source, missing)); err != nil {
				t.Fatal(err)
			}

			_, err := LoadSPXModule(source)
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("LoadSPXModule error = %v, want missing %s", err, missing)
			}
		})
	}
}

func TestLoadSPXModuleRejectsEmptySource(t *testing.T) {
	if _, err := LoadSPXModule("  "); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("LoadSPXModule empty source error = %v", err)
	}
}
