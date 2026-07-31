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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

func getRenderOffset(p *SpriteImpl) (float64, float64) {
	return getCostumeRenderOffset(p.currentCostume(), p.getPivot(), p.runtimeState.Scale, p.runtimeState.Scale)
}

func getCostumeRenderOffset(c *costume, pivot mathf.Vec2, scaleX, scaleY float64) (float64, float64) {
	anchor := c.renderAnchorInSPX().Add(pivot)
	return -anchor.X * scaleX, -anchor.Y * scaleY
}

func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getWorldRenderOffset(p)
	*cx += x
	*cy += y
}

// getWorldRenderOffset applies the sprite root's flip and rotation to the
// local render offset. Godot performs the same transform through RenderRoot.
func getWorldRenderOffset(p *SpriteImpl) (float64, float64) {
	x, y := getRenderOffset(p)
	rotation, scaleX, scaleY := getRenderRotationAndScale(p)
	x *= scaleX
	y *= scaleY

	sin, cos := math.Sincos(-engine.DegToRad(rotation))
	return x*cos - y*sin, x*sin + y*cos
}

func getRenderRotationAndScale(p *SpriteImpl) (rotation, scaleX, scaleY float64) {
	transform := p.transform()
	if transform.rotationStyle == None {
		return 0, 1.0, 1.0
	}
	cs := p.costumes[p.costumeIndex]
	rotation = p.Heading() + cs.faceRight
	rotation -= 90
	scaleX = 1.0
	scaleY = 1.0
	if transform.rotationStyle == LeftRight {
		rotation = 0
		if transform.direction < 0 {
			scaleX = -1.0
		}
	}
	return rotation, scaleX, scaleY
}

func syncGetCostumeBoundByAlpha(p *SpriteImpl) (mathf.Vec2, mathf.Vec2) {
	return getCostumeBoundByAlpha(p, true)
}

func getCostumeBoundByAlpha(p *SpriteImpl, isSync bool) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex]
	var rect mathf.Rect2
	if cs.isAtlas() {
		rect = p.getCostumeAtlasRegion()
		rect.Position.X = 0
		rect.Position.Y = 0
	} else {
		if cache, ok := cachedBounds[cs.path]; ok {
			rect = cache
		} else {
			assetPath := cs.getAssetPath()
			if isSync {
				rect = engine.Managers().ResMgr.GetBoundFromAlpha(assetPath)
			} else {
				rect = p.engine().ResMgr.GetBoundFromAlpha(assetPath)
			}
		}
		cachedBounds[cs.path] = rect
	}
	scale := 1 / float64(cs.bitmapResolution)
	posX := float64(rect.Position.X) * scale
	posY := float64(rect.Position.Y) * scale
	sizeX := float64(rect.Size.X) * scale
	sizeY := float64(rect.Size.Y) * scale

	w, h := p.getCostumeSize()
	offsetX := float64(posX + sizeX/2 - w/2)
	offsetY := -float64(posY + sizeY/2 - h/2)

	center := mathf.NewVec2(offsetX, offsetY)
	size := mathf.NewVec2(sizeX, sizeY)
	return center, size
}
