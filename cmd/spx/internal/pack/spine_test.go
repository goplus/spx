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

package pack

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPackProjectIncludesSpineResources(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(root, "Game")
			resourceDir := filepath.Join(project, "assets", "sprites", "Hero")
			prefix := ""
			zipPrefix := "assets/sprites/Hero/"
			if external {
				resourceDir = filepath.Join(root, "shared")
				prefix = "../../../../shared/"
				zipPrefix = "shared/"
			}
			writeTestFile(t, filepath.Join(project, "assets", "index_pack.json"), `{"sprites":{"Hero":{"spineAnim":{"atlas":"`+prefix+`hero.atlas","skeleton":"`+prefix+`hero.skel"}}}}`)
			for name, contents := range map[string]string{"hero.atlas": "scale: 1\npage.png\nsize: 1,1\nregion\n bounds: 0,0,1,1\n\nsecond.png\nsize: 1,1\n", "hero.skel": "skeleton", "page.png": "page1", "second.png": "page2"} {
				writeTestFile(t, filepath.Join(resourceDir, name), contents)
			}
			output := filepath.Join(root, "game.zip")
			if err := PackProject(project, output); err != nil {
				t.Fatal(err)
			}
			snapshot := readZipSnapshot(t, output)
			for _, name := range []string{"hero.atlas", "hero.skel", "page.png", "second.png"} {
				if snapshot.counts[zipPrefix+name] != 1 {
					t.Fatalf("missing %s: %v", zipPrefix+name, snapshot.counts)
				}
			}
		})
	}
}

func TestPackProjectRejectsEscapingSpinePage(t *testing.T) {
	for _, page := range []string{"../page.png", "nested/../page.png", "/page.png", "res://page.png", "C:/page.png", `nested\page.png`, "page.png\nsize: 1,1\n\n../outside.png"} {
		root := t.TempDir()
		project := filepath.Join(root, "Game")
		writeTestFile(t, filepath.Join(project, "assets", "index_pack.json"), `{"sprites":{"Hero":{"spineAnim":{"atlas":"hero.atlas","skeleton":"hero.skel"}}}}`)
		writeTestFile(t, filepath.Join(project, "assets", "sprites", "Hero", "hero.atlas"), page+"\nsize: 1,1\n")
		writeTestFile(t, filepath.Join(project, "assets", "sprites", "Hero", "hero.skel"), "skeleton")
		if err := PackProject(project, filepath.Join(root, "game.zip")); err == nil || !strings.Contains(err.Error(), "unsafe page") || !strings.Contains(err.Error(), "hero.atlas") {
			t.Fatalf("page %q: %v", page, err)
		}
	}
}
