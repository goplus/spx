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

package spx

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
)

func TestAnimationPayloadBoundsError(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		costumes []*costume
		want     string
	}{
		{"empty", -1, 0, nil, "createAnimation: no costumes configured"},
		{"negative from", -1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=-1, to=0, costumes=1)"},
		{"negative to", 0, -1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=-1, costumes=1)"},
		{"from past end", 1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=1, to=0, costumes=1)"},
		{"to past end", 0, 1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=1, costumes=1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, isAtlas := range []bool{false, true} {
				got, bitmap, err := buildAnimationPayloadJSON(test.costumes, test.from, test.to, isAtlas)
				if err == nil || err.Error() != test.want || got != "" || bitmap != 0 {
					t.Fatalf("buildAnimationPayloadJSON(atlas=%v) = (%q, %d, %v), want empty/0/%q", isAtlas, got, bitmap, err, test.want)
				}
			}
		})
	}
}

func TestAnimationPayloadAtlasIgnoresNormalMetadata(t *testing.T) {
	costumes := []*costume{
		{path: "sprites/base.png", frame: costumeFrame{Width: 90, Height: 100}},
		{
			path: "sprites/other.png",
			frame: costumeFrame{
				PosX: 10, PosY: 20, Width: 30, Height: 40,
				BitmapResolution: 8, Center: mathf.NewVec2(math.NaN(), 0), ImageSize: mathf.NewVec2(0, math.Inf(1)),
			},
		},
		{path: "sprites/another.png", frame: costumeFrame{PosX: 50, PosY: 60, Width: 70, Height: 80}},
	}
	const base = `{"x":0,"y":0,"w":90,"h":100,"offset":[0,0]}`
	const first = `{"x":10,"y":20,"w":30,"h":40,"offset":[0,0]}`
	const second = `{"x":50,"y":60,"w":70,"h":80,"offset":[0,0]}`
	tests := []struct {
		name     string
		from, to int
		frames   string
	}{
		{"single", 1, 1, "[" + first + "]"},
		{"forward", 1, 2, "[" + first + "," + second + "]"},
		{"reverse", 2, 1, "[" + second + "," + first + "]"},
		{"reverse through zero", 2, 0, "[" + second + "," + first + "," + base + "]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, true)
			want := `{"base_path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"frames":` + test.frames + `,"max_bitmap":1}`
			if err != nil || got != want || bitmap != 1 {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, 1, nil)", got, bitmap, err, want)
			}
		})
	}
}

func TestAnimationPayloadRejectsNonFiniteNormalOffset(t *testing.T) {
	got, bitmap, err := buildAnimationPayloadJSON([]*costume{{frame: costumeFrame{Center: mathf.NewVec2(math.NaN(), 0)}}}, 0, 0, false)
	var unsupported *json.UnsupportedValueError
	if !errors.As(err, &unsupported) || !strings.HasPrefix(err.Error(), "createAnimation: failed to marshal animation payload:") || got != "" || bitmap != 0 {
		t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want empty/0/wrapped JSON error", got, bitmap, err)
	}
}

func TestAnimationPayloadNormalPreservesDescriptorGeometry(t *testing.T) {
	costumes := []*costume{
		{
			path: "sprites/fractional.svg",
			frame: costumeFrame{
				ImageSize: mathf.NewVec2(55.75, 54.25), Center: mathf.NewVec2(52, 89),
				// Normal payloads use logical ImageSize, not atlas-only geometry.
				Width: 999, Height: 999, PosX: 10, PosY: 20,
				AtlasUVRect: mathf.NewVec4(math.NaN(), 0, 0, 0),
			},
		},
		{
			path: "sprites/centered.png",
			frame: costumeFrame{
				BitmapResolution: 2, ImageSize: mathf.NewVec2(20, 30), Center: mathf.NewVec2(10, 15),
			},
		},
	}
	first := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"offset":[24.125,-61.875],"bitmap":1}`
	second := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[1].path)) + `,"offset":[0,-0],"bitmap":2}`
	for _, test := range []struct {
		name     string
		from, to int
		frames   string
		bitmap   int
	}{
		{"single", 0, 0, "[" + first + "]", 1},
		{"forward", 0, 1, "[" + first + "," + second + "]", 2},
		{"reverse", 1, 0, "[" + second + "," + first + "]", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, false)
			want := `{"frames":` + test.frames + `,"max_bitmap":` + strconv.Itoa(test.bitmap) + `}`
			if err != nil || got != want || bitmap != test.bitmap {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, %d, nil)", got, bitmap, err, want, test.bitmap)
			}
		})
	}
}
