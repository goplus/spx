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

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

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

func TestSpineAtlasPages(t *testing.T) {
	for _, tc := range []struct {
		name, atlas string
		want        []string
	}{
		{"empty", "", nil},
		{"blank", "\ufeff\r\n \n\t", nil},
		{"metadata only", "scale: 1\ncustom: value\n", nil},
		{"relative page", "./page.png\nsize: 1,1\n", []string{"./page.png"}},
		{"multiple pages", "\ufeffpage.png\r\nsize: 16,16\r\nregion\r\n bounds: 0,0,16,16\r\n\r\ntextures/second.png\r\nsize: 8,8\r\n", []string{"page.png", "textures/second.png"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages, err := SpineAtlasPages(tc.atlas)
			if err != nil || !reflect.DeepEqual(pages, tc.want) {
				t.Fatalf("pages=%v, error=%v; want %v", pages, err, tc.want)
			}
		})
	}
}

func TestSpineAtlasInitialMetadata(t *testing.T) {
	for _, header := range []string{"scale: 1\n", "\nscale:1\n", "scale: 1\ncustom: value\n", "scale: 1\n\n"} {
		pages, err := SpineAtlasPages(header + "page.png\nsize: 16,8\nregion\n bounds: 0,0,16,8\n\nsecond.png\nsize: 1,1\n")
		if err != nil || len(pages) != 2 || pages[0] != "page.png" || pages[1] != "second.png" {
			t.Fatalf("header %q: %v, %v", header, pages, err)
		}
	}
	if pages, err := SpineAtlasPages("page.png\n\nscale: 1\n"); pages != nil || err == nil {
		t.Fatalf("late header was skipped: %v, %v", pages, err)
	}
}

func TestSpineAtlasPagesRejectsUnsafePaths(t *testing.T) {
	for _, page := range []string{"../page.png", "nested/../page.png", "/page.png", "res://page.png", "C:/page.png", `textures\page.png`, "bad\x00.png", "bad:page.png"} {
		for _, prefix := range []string{"", "scale: 1\n\n", "page.png\nsize: 1,1\n\n"} {
			pages, err := SpineAtlasPages(prefix + page + "\nsize: 16,8\n")
			if pages != nil || err == nil || !strings.Contains(err.Error(), "unsafe page "+strconv.Quote(page)) {
				t.Fatalf("prefix %q, page %q: %v, %v", prefix, page, pages, err)
			}
		}
	}
}
