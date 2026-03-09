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
	"fmt"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v2/internal/assets"
	"github.com/goplus/spx/v2/internal/engine"
)

// createAnimation creates an animation from configuration and costume data.
func createAnimation(
	engineMgr *engineManagers,
	spriteName string,
	animName string,
	cfg *aniConfig,
	costumes []*costume,
	isAtlas bool,
) {
	if cfg.IFrameFrom < 0 || cfg.IFrameFrom >= len(costumes) ||
		cfg.IFrameTo < 0 || cfg.IFrameTo >= len(costumes) {
		panic(fmt.Sprintf(
			"createAnimation: frame index out of bounds (from=%d, to=%d, costumes=%d)",
			cfg.IFrameFrom, cfg.IFrameTo, len(costumes),
		))
	}
	payload := buildAnimPayload(cfg, costumes, isAtlas)
	cfg.AdaptAnimBitmapResolution = int(payload.MaxBitmap)
	bin, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("createAnimation: failed to marshal animation payload: %v", err))
	}
	engineMgr.ResMgr.CreateAnimation(
		spriteName,
		animName,
		string(bin),
		int64(cfg.FrameFps),
		isAtlas,
	)
}

func buildAnimPayload(cfg *aniConfig, costumes []*costume, isAtlas bool) animPayload {
	if isAtlas {
		return buildAtlasPayload(cfg, costumes)
	}
	return buildNormalPayload(cfg, costumes)
}

func buildNormalPayload(cfg *aniConfig, costumes []*costume) animPayload {
	maxBitmap := 0
	frameCount := cfg.IFrameTo - cfg.IFrameFrom + 1
	frames := make([]any, 0, frameCount)
	for i := cfg.IFrameFrom; i <= cfg.IFrameTo; i++ {
		c := costumes[i]
		b := assetutil.ToBitmapResolution(c.bitmapResolution)
		if b > maxBitmap {
			maxBitmap = b
		}
		path := engine.ToAssetPath(c.path)
		half := mathf.Vec2.Mulf(c.imageSize, 0.5)
		frames = append(frames, frameNormal{
			Path: path,
			Offset: [2]float64{
				c.center.X - half.X,
				-(c.center.Y - half.Y),
			},
			Bitmap: int64(b),
		})
	}
	return animPayload{
		Frames:    frames,
		MaxBitmap: int64(maxBitmap),
	}
}

func buildAtlasPayload(cfg *aniConfig, costumes []*costume) animPayload {
	base := engine.ToAssetPath(costumes[0].path)
	step := 1
	if cfg.IFrameTo < cfg.IFrameFrom {
		step = -1
	}
	frameCount := (cfg.IFrameTo-cfg.IFrameFrom)*step + 1
	frames := make([]any, 0, frameCount)
	for i := cfg.IFrameFrom; i != cfg.IFrameTo+step; i += step {
		c := costumes[i]
		frames = append(frames, frameAtlas{
			X:      int64(c.posX),
			Y:      int64(c.posY),
			W:      int64(c.width),
			H:      int64(c.height),
			Offset: [2]float64{0, 0},
		})
	}
	return animPayload{
		BasePath:  base,
		Frames:    frames,
		MaxBitmap: 1,
	}
}
