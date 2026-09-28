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

// resolveTargetPosition distinguishes a missing target from one at the origin.
func (p *Game) resolveTargetPosition(target Target) (float64, float64, bool) {
	var targetSprite *SpriteImpl
	switch target := target.(type) {
	case SpriteName:
		targetSprite = p.shapeMgr.findSprite(target)
	case Sprite:
		targetSprite = spriteOf(target)
	case specialObj:
		if target == Mouse {
			x, y := p.getMousePos()
			return x, y, true
		}
	case Pos:
		if target == Random {
			worldW, worldH := p.worldSize()
			randomX, randomY := randomIntn(worldW), randomIntn(worldH)
			return float64(randomX - (worldW >> 1)), float64((worldH >> 1) - randomY), true
		}
	}
	if targetSprite == nil {
		return 0, 0, false
	}
	x, y := targetSprite.getXY()
	return x, y, true
}
