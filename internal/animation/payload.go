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
	"fmt"

	assetutil "github.com/goplus/spx/v4/internal/assets"
	"github.com/goplus/spx/v4/internal/engine"
)

type Config struct {
	FrameFrom int
	FrameTo   int
}

type FrameSource struct {
	Path  string
	Frame assetutil.FrameDescriptor
}

type frameNormal struct {
	Path   string     `json:"path"`
	Offset [2]float64 `json:"offset"`
	Bitmap int64      `json:"bitmap"`
}

type frameAtlas struct {
	X      int64      `json:"x"`
	Y      int64      `json:"y"`
	W      int64      `json:"w"`
	H      int64      `json:"h"`
	Offset [2]float64 `json:"offset"`
}

type payload struct {
	BasePath  string `json:"base_path,omitempty"`
	Frames    []any  `json:"frames"`
	MaxBitmap int64  `json:"max_bitmap"`
}

func BuildPayloadJSON(cfg Config, costumes []FrameSource, isAtlas bool) (string, int, error) {
	if len(costumes) == 0 {
		return "", 0, fmt.Errorf("createAnimation: no costumes configured")
	}
	if cfg.FrameFrom < 0 || cfg.FrameFrom >= len(costumes) ||
		cfg.FrameTo < 0 || cfg.FrameTo >= len(costumes) {
		return "", 0, fmt.Errorf(
			"createAnimation: frame index out of bounds (from=%d, to=%d, costumes=%d)",
			cfg.FrameFrom, cfg.FrameTo, len(costumes),
		)
	}

	payload := buildPayload(cfg, costumes, isAtlas)
	bin, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("createAnimation: failed to marshal animation payload: %w", err)
	}
	return string(bin), int(payload.MaxBitmap), nil
}

func buildPayload(cfg Config, costumes []FrameSource, isAtlas bool) payload {
	result := payload{}
	if isAtlas {
		result.BasePath = engine.ToAssetPath(costumes[0].Path)
		result.MaxBitmap = 1
	}
	step := 1
	if cfg.FrameTo < cfg.FrameFrom {
		step = -1
	}
	frameCount := (cfg.FrameTo-cfg.FrameFrom)*step + 1
	result.Frames = make([]any, 0, frameCount)
	for i := cfg.FrameFrom; i != cfg.FrameTo+step; i += step {
		c := costumes[i]
		frame := c.Frame
		if isAtlas {
			result.Frames = append(result.Frames, frameAtlas{
				X: int64(frame.PosX),
				Y: int64(frame.PosY),
				W: int64(frame.Width),
				H: int64(frame.Height),
			})
			continue
		}
		b := int64(assetutil.ToBitmapResolution(frame.BitmapResolution))
		result.MaxBitmap = max(result.MaxBitmap, b)
		half := frame.ImageSize.Mulf(0.5)
		result.Frames = append(result.Frames, frameNormal{
			Path: engine.ToAssetPath(c.Path),
			Offset: [2]float64{
				frame.Center.X - half.X,
				-(frame.Center.Y - half.Y),
			},
			Bitmap: b,
		})
	}
	return result
}
