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
	"math/rand"

	"github.com/goplus/spx/v2/internal/engine"
)

func (p *Game) touchingPoint(dst *SpriteImpl, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil {
		return nil
	}
	// Use optimized spatial partitioning version.
	return p.findTouchingSpriteOptimized(dst, name)
}

func (p *Game) objectPos(obj any) (float64, float64) {
	switch v := obj.(type) {
	case SpriteName:
		if sp := p.spriteMgr.findSprite(v); sp != nil {
			return sp.getXY()
		}
		engine.Panic("objectPos: sprite not found - " + v)
	case specialObj:
		if v == Mouse {
			return p.getMousePos()
		}
	case Pos:
		if v == Random {
			worldW, worldH := p.worldSize()
			mx, my := rand.Intn(worldW), rand.Intn(worldH)
			return float64(mx - (worldW >> 1)), float64((worldH >> 1) - my)
		}
	case Sprite:
		return spriteOf(v).getXY()
	}
	engine.Panic("objectPos: unexpected input")
	return 0, 0
}
