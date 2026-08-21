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
	"testing"

	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func TestProjectBundleIncludesAllTopLevelSourcesAndPack(t *testing.T) {
	projectDir := t.TempDir()
	writeProjectTestFile(t, filepath.Join(projectDir, "main.spx"), "main")
	writeProjectTestFile(t, filepath.Join(projectDir, "Hero.spx"), "hero")
	writeProjectTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}")
	writeProjectTestFile(t, filepath.Join(projectDir, "assets", "hero.png"), "asset")
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
		t.Fatalf("PackDir = %q", bundle.PackDir)
	}
	got := map[string]bool{}
	for _, name := range bundle.ProjectFiles {
		got[name] = true
	}
	if !got["main.spx"] || !got["Hero.spx"] {
		t.Fatalf("project files = %#v", bundle.ProjectFiles)
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
