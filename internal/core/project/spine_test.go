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

package project

import "testing"

func TestNormalizeSpineConfigPaths(t *testing.T) {
	for _, tc := range []struct{ atlas, skeleton, wantAtlas, wantSkeleton string }{
		{"spine/hero.atlas", "hero.skel", "sprites/Hero/spine/hero.atlas", "sprites/Hero/hero.skel"},
		{"res://shared/hero.atlas", "/shared/hero.json", "res://shared/hero.atlas", "/shared/hero.json"},
		{"", "", "", ""},
	} {
		config := SpriteConfig{SpineAnim: &SpineAnimConfig{Atlas: tc.atlas, Skeleton: tc.skeleton}}
		normalizeSpriteConfigPaths(&config, "sprites/Hero")
		if config.SpineAnim.Atlas != tc.wantAtlas || config.SpineAnim.Skeleton != tc.wantSkeleton {
			t.Fatalf("normalized: %+v", config.SpineAnim)
		}
	}
}

func TestSpineAtlasPagesAndValidation(t *testing.T) {
	pages := SpineAtlasPages("\ufeffpage.png\r\nsize: 16,16\r\nregion\r\n bounds: 0,0,16,16\r\n\r\ntextures/second.png\r\nsize: 8,8\r\n")
	if len(pages) != 2 || pages[0] != "page.png" || pages[1] != "textures/second.png" {
		t.Fatalf("pages=%v", pages)
	}
	for _, page := range []string{"page.png", "textures/page.png", "./page.png"} {
		if !ValidSpineAtlasPage(page) {
			t.Fatalf("rejected %q", page)
		}
	}
	for _, page := range []string{"", "../page.png", "nested/../page.png", "/page.png", "res://page.png", "C:/page.png", `textures\page.png`, "bad\x00.png"} {
		if ValidSpineAtlasPage(page) {
			t.Fatalf("accepted %q", page)
		}
	}
}

func TestSpineAtlasInitialMetadata(t *testing.T) {
	for _, header := range []string{"scale: 1\n", "\nscale:1\n", "scale: 1\ncustom: value\n", "scale: 1\n\n"} {
		pages := SpineAtlasPages(header + "page.png\nsize: 16,8\nregion\n bounds: 0,0,16,8\n\nsecond.png\nsize: 1,1\n")
		if len(pages) != 2 || pages[0] != "page.png" || pages[1] != "second.png" {
			t.Fatalf("header %q: %v", header, pages)
		}
	}
	for _, unsafe := range []string{"res://page.png", "C:/page.png", "../page.png", "/page.png", "bad:page.png"} {
		pages := SpineAtlasPages(unsafe + "\nsize: 16,8\n")
		if len(pages) != 1 || pages[0] != unsafe || ValidSpineAtlasPage(pages[0]) {
			t.Fatalf("unsafe page %q was skipped: %v", unsafe, pages)
		}
	}
	for _, atlas := range []string{"page.png\n\nscale: 1\n", "scale: 1\n\nres://page.png\n"} {
		pages := SpineAtlasPages(atlas)
		if len(pages) == 0 || ValidSpineAtlasPage(pages[len(pages)-1]) {
			t.Fatalf("late header or unsafe page was skipped: %v", pages)
		}
	}
}
