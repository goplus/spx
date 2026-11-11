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

package assets

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCollectSpineResources(t *testing.T) {
	for _, packed := range []bool{false, true} {
		t.Run(map[bool]string{true: "packed", false: "source"}[packed], func(t *testing.T) {
			dir := t.TempDir()
			sprite := `{"spineAnim":{"atlas":"res://shared/hero.atlas","skeleton":"res://shared/hero.skel"}}`
			if packed {
				writeAssetTestFile(t, dir, "assets/index_pack.json", `{"sprites":{"Hero":`+sprite+`}}`)
			} else {
				writeAssetTestFile(t, dir, "assets/index.json", `{}`)
				writeAssetTestFile(t, dir, "assets/sprites/Hero/index.json", sprite)
			}
			writeAssetTestFile(t, dir, "shared/hero.atlas", "\r\nscale: 1\r\npage.png\r\nsize: 32,32\r\nregion\r\n  bounds: 0,0,32,32\r\n\r\npages/other.png\r\nsize: 16,16\r\n")
			writeAssetTestFile(t, dir, "shared/hero.skel", "skeleton")
			writeAssetTestFile(t, dir, "shared/page.png", "page")
			writeAssetTestFile(t, dir, "shared/pages/other.png", "page")
			files, err := Collect(Config{ProjectDir: dir, PackDir: "assets", PackIndex: "index.json"})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"shared/hero.atlas", "shared/hero.skel", "shared/page.png", "shared/pages/other.png"}
			if !reflect.DeepEqual(files, want) {
				t.Fatalf("files=%v", files)
			}
		})
	}
}

func TestCollectSpineRejectsMissingOrEscapingPage(t *testing.T) {
	for _, page := range []string{"missing.png", "../outside.png", "nested/../page.png", "/page.png", "res://shared/page.png", `nested\page.png`, "page.png\nsize: 1,1\n\n../outside.png"} {
		dir := t.TempDir()
		writeAssetTestFile(t, dir, "assets/index_pack.json", `{"sprites":{"Hero":{"spineAnim":{"atlas":"res://shared/hero.atlas","skeleton":"res://shared/hero.json"}}}}`)
		writeAssetTestFile(t, dir, "shared/hero.atlas", page+"\nsize: 1,1\n")
		writeAssetTestFile(t, dir, "shared/hero.json", `{}`)
		_, err := Collect(Config{ProjectDir: dir, PackDir: "assets", PackIndex: "index.json"})
		wantError := "unsafe page"
		if page == "missing.png" {
			wantError = "missing"
		}
		if err == nil || !strings.Contains(err.Error(), "shared/hero.atlas") || !strings.Contains(err.Error(), wantError) {
			t.Fatalf("page %s: %v", page, err)
		}
	}
}

func TestCollectSpineRejectsInvalidResources(t *testing.T) {
	for _, tc := range []struct{ name, config, want string }{
		{"missing atlas field", `{"skeleton":"hero.skel"}`, "requires atlas and skeleton"},
		{"missing skeleton field", `{"atlas":"hero.atlas"}`, "requires atlas and skeleton"},
		{"missing atlas file", `{"atlas":"missing.atlas","skeleton":"hero.skel"}`, "missing.atlas"},
		{"missing skeleton file", `{"atlas":"hero.atlas","skeleton":"missing.skel"}`, "missing.skel"},
		{"oversized atlas", `{"atlas":"hero.atlas","skeleton":"hero.skel"}`, "exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAssetTestFile(t, dir, "assets/index_pack.json", `{"sprites":{"Hero":{"spineAnim":`+tc.config+`}}}`)
			const atlas = "assets/sprites/Hero/hero.atlas"
			writeAssetTestFile(t, dir, atlas, "page.png\nsize: 1,1\n")
			writeAssetTestFile(t, dir, "assets/sprites/Hero/hero.skel", "skeleton")
			if tc.name == "oversized atlas" {
				if err := os.Truncate(filepath.Join(dir, filepath.FromSlash(atlas)), maxConfigBytes+1); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Collect(Config{ProjectDir: dir, PackDir: "assets", PackIndex: "index.json"}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Collect() error = %v, want %q", err, tc.want)
			}
		})
	}
}
