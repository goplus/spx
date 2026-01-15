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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Physics Detection and Collision

func (p *Game) checkCollision(ary any) []Sprite {
	spriteIdAry := ary.([]engine.Object)
	sprites := make([]Sprite, 0, len(spriteIdAry))
	slices.Sort(spriteIdAry)
	for _, item := range spriteIdAry {
		sprite := engine.GetSprite(item)
		if sprite != nil {
			impl := sprite.Target.(*SpriteImpl)
			if impl != nil {
				sprites = append(sprites, impl.sprite)
			} else {
				spxlog.Warn("Collision object is not a Sprite: %v", item)
			}
		}
	}
	return sprites
}

func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite {
	ary := physicsMgr.CheckCollisionRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), -1)
	return p.checkCollision(ary)
}

func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite {
	ary := physicsMgr.CheckCollisionCircle(mathf.NewVec2(posX, posY), radius, -1)
	return p.checkCollision(ary)
}

func (p *Game) Raycast__0(fromX, fromY, toX, toY float64, ignoreSprites []Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	from := mathf.NewVec2(fromX, fromY)
	to := mathf.NewVec2(toX, toY)
	ignoreSpritesIds := make([]int64, 0)
	for _, item := range ignoreSprites {
		if item == nil {
			continue
		}
		impl := spriteOf(item)
		if impl != nil {
			ignoreSpritesIds = append(ignoreSpritesIds, impl.getSpriteId())
		}
	}
	result := raycast(from, to, ignoreSpritesIds, -1)
	if result == nil {
		return false, nil, 0, 0
	}
	var target Sprite = nil
	if result.Hited {
		sprite := engine.GetSprite(result.SpriteId)
		if sprite != nil {
			impl := sprite.Target.(*SpriteImpl)
			if impl != nil {
				target = impl.sprite
			}
		}
	}
	return result.Hited, target, result.PosX, result.PosY
}

func (p *Game) Raycast__1(fromX, fromY, toX, toY float64, ignoreSprite Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{ignoreSprite})
}

func (p *Game) Raycast__2(fromX, fromY, toX, toY float64) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{})
}

// -----------------------------------------------------------------------------
// Debug Drawing

func (p *Game) DebugDrawRect(posX, posY, width, height float64, color Color) {
	debugMgr.DebugDrawRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), toMathfColor(color))
}

func (p *Game) DebugDrawCircle(posX, posY, radius float64, color Color) {
	debugMgr.DebugDrawCircle(mathf.NewVec2(posX, posY), radius, toMathfColor(color))
}

func (p *Game) DebugDrawLine(fromX, fromY, toX, toY float64, color Color) {
	debugMgr.DebugDrawLine(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), toMathfColor(color))
}

func (p *Game) DebugDrawLines(points []float64, color Color) {
	if len(points) < 4 || len(points)%2 != 0 {
		return
	}

	for i := 0; i < len(points)-2; i += 2 {
		from := mathf.NewVec2(points[i], points[i+1])
		to := mathf.NewVec2(points[i+2], points[i+3])
		debugMgr.DebugDrawLine(from, to, toMathfColor(color))
	}
}
