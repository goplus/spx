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

package project

import (
	"strings"
	"testing"
)

func TestReadSVGLogicalSize(t *testing.T) {
	tests := []struct {
		name        string
		svg         string
		wantWidth   float64
		wantHeight  float64
		wantSuccess bool
	}{
		{
			name:        "fractional dimensions",
			svg:         `<svg width="247.24325" height="476.42829" viewBox="0 0 247.24325 476.42829"></svg>`,
			wantWidth:   247.24325,
			wantHeight:  476.42829,
			wantSuccess: true,
		},
		{
			name:        "pixel suffix",
			svg:         `<svg width="100.5px" height="80.25px"></svg>`,
			wantWidth:   100.5,
			wantHeight:  80.25,
			wantSuccess: true,
		},
		{
			name:        "view box fallback",
			svg:         `<svg viewBox="0, 0, 32.5, 16.25"></svg>`,
			wantWidth:   32.5,
			wantHeight:  16.25,
			wantSuccess: true,
		},
		{
			name:        "height from view box aspect ratio",
			svg:         `<svg width="200" viewBox="0 0 100 50"></svg>`,
			wantWidth:   200,
			wantHeight:  100,
			wantSuccess: true,
		},
		{
			name:        "width from view box aspect ratio",
			svg:         `<svg height="100" viewBox="0 0 100 50"></svg>`,
			wantWidth:   200,
			wantHeight:  100,
			wantSuccess: true,
		},
		{
			name: "unsupported physical units",
			svg:  `<svg width="72pt" height="36pt" viewBox="0 0 720 360"></svg>`,
		},
		{
			name: "missing dimensions",
			svg:  `<svg></svg>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height, ok := readSVGLogicalSize(strings.NewReader(tt.svg))
			if ok != tt.wantSuccess || width != tt.wantWidth || height != tt.wantHeight {
				t.Fatalf(
					"readSVGLogicalSize() = (%v, %v, %v), want (%v, %v, %v)",
					width, height, ok, tt.wantWidth, tt.wantHeight, tt.wantSuccess,
				)
			}
		})
	}
}

func TestLoadSpriteConfigFillsMissingSVGCostumeSize(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "sprites/Level/index.json", `{
		"costumes": [{
			"name": "costume2",
			"path": "costume2.svg",
			"x": 76.0323911558041,
			"y": 208.79720845345383,
			"bitmapResolution": 1
		}]
	}`)
	writeProjectFile(
		t,
		dir,
		"sprites/Level/costume2.svg",
		`<svg width="247.24325" height="476.42829" viewBox="0 0 247.24325 476.42829"></svg>`,
	)

	loaded, err := LoadSpriteConfig(localDir{base: dir}, "Level")
	if err != nil {
		t.Fatal(err)
	}
	costume := loaded.Costumes[0]
	if costume.ImageWidth != 247.24325 || costume.ImageHeight != 476.42829 {
		t.Fatalf(
			"SVG logical size = (%v, %v), want (247.24325, 476.42829)",
			costume.ImageWidth,
			costume.ImageHeight,
		)
	}
}

func TestLoadSpriteConfigLeavesUnsupportedSVGSizeUnresolved(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "sprites/Level/index.json", `{
		"costumes": [{
			"name": "costume",
			"path": "costume.svg",
			"bitmapResolution": 1
		}]
	}`)
	writeProjectFile(
		t,
		dir,
		"sprites/Level/costume.svg",
		`<svg width="72pt" height="36pt" viewBox="0 0 720 360"></svg>`,
	)

	loaded, err := LoadSpriteConfig(localDir{base: dir}, "Level")
	if err != nil {
		t.Fatal(err)
	}
	costume := loaded.Costumes[0]
	if costume.ImageWidth != 0 || costume.ImageHeight != 0 {
		t.Fatalf(
			"unsupported SVG size = (%v, %v), want unresolved size (0, 0)",
			costume.ImageWidth,
			costume.ImageHeight,
		)
	}
}
