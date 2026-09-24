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

	"github.com/goplus/spx/v3/internal/base/collision"
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil || dst.runtimeState.SyncSprite == nil {
		return nil
	}
	pixel := !p.physicsEnabled
	dstBox, dstBounded := dst.queryBounds(pixel)
	for _, sp := range p.shapeMgr.spritesNamed(name) {
		if sp == dst || !sp.spriteState.IsVisible || sp.spriteState.IsDying || sp.runtimeState.SyncSprite == nil {
			continue
		}
		if box, bounded := sp.queryBounds(pixel); dstBounded && bounded && !dstBox.Intersects(box) {
			continue
		}
		if sp.touchingSprite(dst) {
			return sp
		}
	}
	return nil
}

func (sp *SpriteImpl) queryBounds(pixel bool) (collision.AABB, bool) {
	cfg := &sp.physics().collisionInfo
	if pixel || cfg.Type == physicsColliderAuto {
		bounds, ok := sp.renderBounds()
		if !ok {
			return collision.AABB{}, false
		}
		return collision.AABB{
			MinX: bounds.Position.X,
			MinY: bounds.Position.Y,
			MaxX: bounds.Position.X + bounds.Size.X,
			MaxY: bounds.Position.Y + bounds.Size.Y,
		}, true
	}

	var minX, minY, maxX, maxY float64
	switch cfg.Type {
	case physicsColliderRect, physicsColliderCircle, physicsColliderCapsule:
		width, height := cfg.getDimensions()
		if width <= 0 || height <= 0 {
			return collision.AABB{}, false
		}
		if cfg.Type == physicsColliderCapsule && height < width {
			height = width
		}
		minX, maxX = -width/2, width/2
		minY, maxY = -height/2, height/2
	case physicsColliderPolygon:
		if len(cfg.Params) < 6 || len(cfg.Params)%2 != 0 {
			return collision.AABB{}, false
		}
		minX, maxX = cfg.Params[0], cfg.Params[0]
		minY, maxY = cfg.Params[1], cfg.Params[1]
		for i := 2; i < len(cfg.Params); i += 2 {
			minX, maxX = math.Min(minX, cfg.Params[i]), math.Max(maxX, cfg.Params[i])
			minY, maxY = math.Min(minY, cfg.Params[i+1]), math.Max(maxY, cfg.Params[i+1])
		}
	default:
		return collision.AABB{}, false
	}
	if !sp.hasCostume() {
		return collision.AABB{}, false
	}

	rotation, flipX, flipY := getRenderRotationAndScale(sp)
	sin, cos := math.Sincos(-engine.DegToRad(rotation))
	scale := sp.runtimeState.Scale
	centerX := (cfg.Pivot.X + (minX+maxX)/2) * scale * flipX
	centerY := (cfg.Pivot.Y + (minY+maxY)/2) * scale * flipY
	x, y := sp.getXY()
	x += centerX*cos - centerY*sin
	y += centerX*sin + centerY*cos
	halfX := math.Abs((maxX-minX)*scale) / 2
	halfY := math.Abs((maxY-minY)*scale) / 2
	extentX := math.Abs(cos)*halfX + math.Abs(sin)*halfY
	extentY := math.Abs(sin)*halfX + math.Abs(cos)*halfY
	if cfg.Type == physicsColliderCircle {
		extentX = math.Max(cfg.Params[0]*scale, minCircleRadius)
		extentY = extentX
	}
	return collision.AABB{MinX: x - extentX, MinY: y - extentY, MaxX: x + extentX, MaxY: y + extentY}, true
}
