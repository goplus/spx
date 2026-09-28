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

package animation

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

type decodedPayload struct {
	BasePath  string           `json:"base_path"`
	Frames    []map[string]any `json:"frames"`
	MaxBitmap float64          `json:"max_bitmap"`
}

func TestBuildPayloadJSONNormal(t *testing.T) {
	got, maxBitmap, err := BuildPayloadJSON(
		Config{FrameFrom: 0, FrameTo: 1},
		[]FrameSource{
			{
				Path:             "sprites/cat/a.webp",
				BitmapResolution: 2,
				Center:           mathf.NewVec2(8, 12),
				ImageSize:        mathf.NewVec2(20, 30),
			},
			{
				Path:             "sprites/cat/b.png",
				BitmapResolution: 4,
				Center:           mathf.NewVec2(10, 15),
				ImageSize:        mathf.NewVec2(20, 30),
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("BuildPayloadJSON(normal) error: %v", err)
	}
	if maxBitmap != 4 {
		t.Fatalf("maxBitmap = %d, want 4", maxBitmap)
	}

	var payload decodedPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if payload.BasePath != "" {
		t.Fatalf("BasePath = %q, want empty", payload.BasePath)
	}
	if payload.MaxBitmap != 4 {
		t.Fatalf("MaxBitmap = %v, want 4", payload.MaxBitmap)
	}
	if len(payload.Frames) != 2 {
		t.Fatalf("len(Frames) = %d, want 2", len(payload.Frames))
	}
	if payload.Frames[0]["path"] != engine.ToAssetPath("sprites/cat/a.webp") {
		t.Fatalf("frame[0].path = %v, want %q", payload.Frames[0]["path"], engine.ToAssetPath("sprites/cat/a.webp"))
	}
	if payload.Frames[1]["path"] != engine.ToAssetPath("sprites/cat/b.png") {
		t.Fatalf("frame[1].path = %v, want %q", payload.Frames[1]["path"], engine.ToAssetPath("sprites/cat/b.png"))
	}
	if payload.Frames[0]["bitmap"] != float64(2) {
		t.Fatalf("frame[0].bitmap = %v, want 2", payload.Frames[0]["bitmap"])
	}
	offset := payload.Frames[0]["offset"].([]any)
	if offset[0] != float64(-2) || offset[1] != float64(3) {
		t.Fatalf("frame[0].offset = %#v, want [-2 3]", offset)
	}
}

func TestBuildPayloadJSONAtlasReverse(t *testing.T) {
	got, maxBitmap, err := BuildPayloadJSON(
		Config{FrameFrom: 2, FrameTo: 0},
		[]FrameSource{
			{Path: "sprites/cat/atlas.png", PosX: 0, PosY: 0, Width: 10, Height: 20},
			{Path: "sprites/cat/atlas.png", PosX: 10, PosY: 0, Width: 10, Height: 20},
			{Path: "sprites/cat/atlas.png", PosX: 20, PosY: 0, Width: 10, Height: 20},
		},
		true,
	)
	if err != nil {
		t.Fatalf("BuildPayloadJSON(atlas) error: %v", err)
	}
	if maxBitmap != 1 {
		t.Fatalf("maxBitmap = %d, want 1", maxBitmap)
	}

	var payload decodedPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if payload.BasePath != engine.ToAssetPath("sprites/cat/atlas.png") {
		t.Fatalf("BasePath = %q, want %q", payload.BasePath, engine.ToAssetPath("sprites/cat/atlas.png"))
	}
	if len(payload.Frames) != 3 {
		t.Fatalf("len(Frames) = %d, want 3", len(payload.Frames))
	}
	if payload.Frames[0]["x"] != float64(20) || payload.Frames[2]["x"] != float64(0) {
		t.Fatalf("atlas frame order = %#v", payload.Frames)
	}
}

func TestBuildPayloadJSONNormalReverse(t *testing.T) {
	got, _, err := BuildPayloadJSON(
		Config{FrameFrom: 1, FrameTo: 0},
		[]FrameSource{
			{
				Path:             "sprites/cat/a.png",
				BitmapResolution: 1,
				Center:           mathf.NewVec2(5, 5),
				ImageSize:        mathf.NewVec2(10, 10),
			},
			{
				Path:             "sprites/cat/b.png",
				BitmapResolution: 1,
				Center:           mathf.NewVec2(5, 5),
				ImageSize:        mathf.NewVec2(10, 10),
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("BuildPayloadJSON(reverse normal) error: %v", err)
	}

	var payload decodedPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if len(payload.Frames) != 2 {
		t.Fatalf("len(Frames) = %d, want 2", len(payload.Frames))
	}
	if payload.Frames[0]["path"] != engine.ToAssetPath("sprites/cat/b.png") {
		t.Fatalf("frame[0].path = %v, want %q", payload.Frames[0]["path"], engine.ToAssetPath("sprites/cat/b.png"))
	}
}

func TestBuildPayloadJSONBoundsError(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		costumes []FrameSource
		want     string
	}{
		{"empty", Config{FrameFrom: -1}, nil, "createAnimation: no costumes configured"},
		{"negative from", Config{FrameFrom: -1}, []FrameSource{{}}, "createAnimation: frame index out of bounds (from=-1, to=0, costumes=1)"},
		{"negative to", Config{FrameTo: -1}, []FrameSource{{}}, "createAnimation: frame index out of bounds (from=0, to=-1, costumes=1)"},
		{"from past end", Config{FrameFrom: 1}, []FrameSource{{}}, "createAnimation: frame index out of bounds (from=1, to=0, costumes=1)"},
		{"to past end", Config{FrameTo: 1}, []FrameSource{{}}, "createAnimation: frame index out of bounds (from=0, to=1, costumes=1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, isAtlas := range []bool{false, true} {
				got, bitmap, err := BuildPayloadJSON(test.config, test.costumes, isAtlas)
				if err == nil || err.Error() != test.want || got != "" || bitmap != 0 {
					t.Fatalf("BuildPayloadJSON(atlas=%v) = (%q, %d, %v), want empty/0/%q", isAtlas, got, bitmap, err, test.want)
				}
			}
		})
	}
}

func TestBuildPayloadJSONAtlasIgnoresNormalMetadata(t *testing.T) {
	costumes := []FrameSource{
		{Path: "sprites/base.png"},
		{
			Path: "sprites/other.png", PosX: 10, PosY: 20, Width: 30, Height: 40,
			BitmapResolution: 8, Center: mathf.NewVec2(math.NaN(), 0), ImageSize: mathf.NewVec2(0, math.Inf(1)),
		},
		{Path: "sprites/another.png", PosX: 50, PosY: 60, Width: 70, Height: 80},
	}
	const first = `{"x":10,"y":20,"w":30,"h":40,"offset":[0,0]}`
	const second = `{"x":50,"y":60,"w":70,"h":80,"offset":[0,0]}`
	tests := []struct {
		name   string
		config Config
		frames string
	}{
		{"single", Config{FrameFrom: 1, FrameTo: 1}, "[" + first + "]"},
		{"forward", Config{FrameFrom: 1, FrameTo: 2}, "[" + first + "," + second + "]"},
		{"reverse", Config{FrameFrom: 2, FrameTo: 1}, "[" + second + "," + first + "]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := BuildPayloadJSON(test.config, costumes, true)
			want := `{"base_path":` + strconv.Quote(engine.ToAssetPath(costumes[0].Path)) + `,"frames":` + test.frames + `,"max_bitmap":1}`
			if err != nil || got != want || bitmap != 1 {
				t.Fatalf("BuildPayloadJSON() = (%q, %d, %v), want (%q, 1, nil)", got, bitmap, err, want)
			}
		})
	}
}

func TestBuildPayloadJSONRejectsNonFiniteNormalOffset(t *testing.T) {
	got, bitmap, err := BuildPayloadJSON(Config{}, []FrameSource{{Center: mathf.NewVec2(math.NaN(), 0)}}, false)
	var unsupported *json.UnsupportedValueError
	if !errors.As(err, &unsupported) || !strings.HasPrefix(err.Error(), "createAnimation: failed to marshal animation payload:") || got != "" || bitmap != 0 {
		t.Fatalf("BuildPayloadJSON() = (%q, %d, %v), want empty/0/wrapped JSON error", got, bitmap, err)
	}
}
