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

import "github.com/goplus/spx/v3/internal/base/collision"

const defaultSpatialHashCellSize = 100.0

func (p *Game) buildSpatialHashForName(dst *SpriteImpl, name string) *collision.SpatialHash[*SpriteImpl] {
	if p.spatialHash == nil {
		p.spatialHash = collision.NewSpatialHash[*SpriteImpl](defaultSpatialHashCellSize)
	}
	p.spatialHash.Clear()

	for _, item := range p.shapeMgr.items {
		sp, ok := item.(*SpriteImpl)
		if !ok || sp == dst || sp.name != name || !sp.spriteState.IsVisible || sp.spriteState.IsDying || sp.runtimeState.SyncSprite == nil {
			continue
		}
		if aabb := newSpriteAABB(sp); aabb != nil {
			p.spatialHash.Insert(aabb)
		}
	}

	return p.spatialHash
}

// findTouchingSpriteOptimized uses spatial partitioning and AABB intersection
// to reduce expensive pixel-perfect collision checks.
func (p *Game) findTouchingSpriteOptimized(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil || dst.runtimeState.SyncSprite == nil {
		return nil
	}

	dstAABB := newSpriteAABB(dst)
	if dstAABB == nil {
		return nil
	}

	spatialHash := p.buildSpatialHashForName(dst, name)
	for _, candidate := range spatialHash.Query(dstAABB.Box) {
		if dstAABB.Box.Intersects(candidate.Box) && candidate.Value.touchingSprite(dstAABB.Value) {
			return candidate.Value
		}
	}

	return nil
}

func newSpriteAABB(sprite *SpriteImpl) *collision.Entry[*SpriteImpl] {
	bounds := sprite.bounds()
	if bounds == nil {
		return nil
	}

	return &collision.Entry[*SpriteImpl]{
		Value: sprite,
		Box: collision.AABB{
			MinX: bounds.Position.X,
			MinY: bounds.Position.Y,
			MaxX: bounds.Position.X + bounds.Size.X,
			MaxY: bounds.Position.Y + bounds.Size.Y,
		},
	}
}
