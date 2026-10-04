/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package launchpack

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func TestProjectBundleIncludesSortedUniqueSourcesAndResources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index string
		want  []string
	}{
		{"no references", `{}`, []string{"Hero.spx", "main.spx"}},
		{"only source references", `{"backdrops":[{"path":"res://main.spx"},{"path":"res://Hero.spx"}]}`, []string{"Hero.spx", "main.spx"}},
		{"mixed references", `{"backdrops":[{"path":"res://main.spx"},{"path":"res://z.png"},{"path":"res://Hero.spx"},{"path":"res://a.png"}]}`, []string{"Hero.spx", "a.png", "main.spx", "z.png"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			for _, name := range []string{"main.spx", "Hero.spx", "a.png", "z.png", "assets/hero.png"} {
				writeProjectTestFile(t, filepath.Join(projectDir, name), name)
			}
			writeProjectTestFile(t, filepath.Join(projectDir, "assets", "index.json"), tc.index)
			snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{ProjectDir: projectDir, ProjectFile: filepath.Join(projectDir, "main.spx"), ProjectExt: ".spx", PackDir: "assets", PackIndex: "index.json"}
			bundle, err := prepareProjectBundleConfig(cfg, snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if bundle.PackDir != "assets" {
				t.Fatalf("PackDir = %q, want assets", bundle.PackDir)
			}
			if !slices.Equal(bundle.ProjectFiles, tc.want) {
				t.Fatalf("project files = %q, want %q", bundle.ProjectFiles, tc.want)
			}
		})
	}
}

func TestProjectBundleRejectsNestedProjectFile(t *testing.T) {
	projectDir := t.TempDir()
	writeProjectTestFile(t, filepath.Join(projectDir, "main.spx"), "main")
	writeProjectTestFile(t, filepath.Join(projectDir, "nested", "main.spx"), "nested")
	cfg := Config{
		ProjectDir: projectDir, ProjectFile: filepath.Join(projectDir, "nested", "main.spx"),
		ProjectExt: ".spx", PackDir: "assets", PackIndex: "index.json",
	}
	if _, err := collectProjectAllowlist(cfg); err == nil {
		t.Fatal("collectProjectAllowlist accepted a nested project file")
	}
}

func writeProjectTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
