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

import "github.com/goplus/spx/v2/internal/base/collisionutil"

// ======================== Collision Optimization System ========================
// This file implements spatial partitioning and AABB (Axis-Aligned Bounding Box)
// based broad-phase collision detection to reduce expensive pixel-perfect checks.

// ======================== Configuration ========================

// defaultSpatialHashCellSize defines the default cell size for spatial hash grid.
// This value can be tuned based on your game's average sprite size:
const defaultSpatialHashCellSize = 100.0

func newSpriteAABB(sprite *SpriteImpl) *collisionutil.Entry[*SpriteImpl] {
	bounds := sprite.bounds()
	if bounds == nil {
		return nil
	}

	return &collisionutil.Entry[*SpriteImpl]{
		Value: sprite,
		Box: collisionutil.AABB{
			MinX: bounds.Position.X,
			MinY: bounds.Position.Y,
			MaxX: bounds.Position.X + bounds.Size.X,
			MaxY: bounds.Position.Y + bounds.Size.Y,
		},
	}
}

// buildSpatialHashForNames builds a spatial hash with sprites matching the given name filter.
// Uses a reusable spatial hash to avoid repeated allocations.
func (p *Game) buildSpatialHashForNames(dst *SpriteImpl, nameFilter func(string) bool) *collisionutil.SpatialHash[*SpriteImpl] {
	// Lazy initialization of the reusable spatial hash
	if p.spatialHash == nil {
		p.spatialHash = collisionutil.NewSpatialHash[*SpriteImpl](defaultSpatialHashCellSize)
	}

	// Clear and reuse the existing spatial hash
	p.spatialHash.Clear()

	for _, item := range p.spriteMgr.items {
		if sp, ok := item.(*SpriteImpl); ok && sp != dst {
			if nameFilter(sp.name) && sp.spriteState.IsVisible && !sp.spriteState.IsDying && sp.runtimeState.SyncSprite != nil {
				aabb := newSpriteAABB(sp)
				if aabb != nil {
					p.spatialHash.Insert(aabb)
				}
			}
		}
	}

	return p.spatialHash
}

// findCollisionsInSpatialHash performs AABB and pixel-perfect collision detection.
func findCollisionsInSpatialHash(
	dstAABB *collisionutil.Entry[*SpriteImpl],
	spatialHash *collisionutil.SpatialHash[*SpriteImpl],
	findFirst bool,
) []*SpriteImpl {
	var results []*SpriteImpl

	// Query spatial hash for potential collisions
	potentialCollisions := spatialHash.Query(dstAABB.Box)

	// AABB intersection and pixel-perfect collision tests
	for _, candidateAABB := range potentialCollisions {
		if !dstAABB.Box.Intersects(candidateAABB.Box) {
			continue
		}

		// Pixel-perfect collision detection (narrow-phase)
		if candidateAABB.Value.touchingSprite(dstAABB.Value) {
			results = append(results, candidateAABB.Value)
			if findFirst {
				return results
			}
		}
	}

	return results
}

// findTouchingSpriteOptimized uses spatial partitioning for efficient collision detection.
func (p *Game) findTouchingSpriteOptimized(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil || dst.runtimeState.SyncSprite == nil {
		return nil
	}

	// Create AABB for the target sprite
	dstAABB := newSpriteAABB(dst)
	if dstAABB == nil {
		return nil
	}

	// Build spatial hash with name filter
	spatialHash := p.buildSpatialHashForNames(dst, func(spriteName string) bool {
		return spriteName == name
	})

	// Find first collision
	results := findCollisionsInSpatialHash(dstAABB, spatialHash, true)
	if len(results) > 0 {
		return results[0]
	}

	return nil
}

// touchingSpritesByOptimized returns all sprites touching the target sprite (optimized version).
func (p *Game) touchingSpritesByOptimized(dst *SpriteImpl, names []string) []*SpriteImpl {
	if dst == nil || dst.runtimeState.SyncSprite == nil {
		return nil
	}

	// Create AABB for the target sprite
	dstAABB := newSpriteAABB(dst)
	if dstAABB == nil {
		return nil
	}

	// Build a name set for quick lookup
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	// Build spatial hash with name filter
	spatialHash := p.buildSpatialHashForNames(dst, func(spriteName string) bool {
		return nameSet[spriteName]
	})

	// Find all collisions
	return findCollisionsInSpatialHash(dstAABB, spatialHash, false)
}
