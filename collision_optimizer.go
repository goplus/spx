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

// ======================== Collision Optimization System ========================
// This file implements spatial partitioning and AABB (Axis-Aligned Bounding Box)
// based broad-phase collision detection to reduce expensive pixel-perfect checks.

// ======================== Configuration ========================

// defaultSpatialHashCellSize defines the default cell size for spatial hash grid.
// This value can be tuned based on your game's average sprite size:
const defaultSpatialHashCellSize = 100.0

// ======================== Data Structures ========================

// SpriteAABB represents an axis-aligned bounding box for a sprite.
type SpriteAABB struct {
	sprite *SpriteImpl
	minX   float64
	minY   float64
	maxX   float64
	maxY   float64
}

// newSpriteAABB creates an AABB from a sprite's bounds.
func newSpriteAABB(sprite *SpriteImpl) *SpriteAABB {
	bounds := sprite.bounds()
	if bounds == nil {
		return nil
	}

	return &SpriteAABB{
		sprite: sprite,
		minX:   bounds.Position.X,
		minY:   bounds.Position.Y,
		maxX:   bounds.Position.X + bounds.Size.X,
		maxY:   bounds.Position.Y + bounds.Size.Y,
	}
}

// intersects checks if two AABBs overlap (broad-phase collision detection).
func (a *SpriteAABB) intersects(b *SpriteAABB) bool {
	if a == nil || b == nil {
		return false
	}

	// AABB intersection test
	return a.minX <= b.maxX &&
		a.maxX >= b.minX &&
		a.minY <= b.maxY &&
		a.maxY >= b.minY
}

// SpatialHash implements a simple spatial hash grid for broad-phase collision detection.
type SpatialHash struct {
	cellSize float64
	grid     map[int64]map[int64][]*SpriteAABB
}

// newSpatialHash creates a new spatial hash grid.
func newSpatialHash(cellSize float64) *SpatialHash {
	return &SpatialHash{
		cellSize: cellSize,
		grid:     make(map[int64]map[int64][]*SpriteAABB),
	}
}

// clear empties the spatial hash by clearing all existing entries.
// This reuses both the top-level and inner map memory instead of reallocating.
// Note: Maps may grow but never shrink, which is acceptable for typical game scenarios.
// where sprites move within a bounded area.
func (s *SpatialHash) clear() {
	for _, yGrid := range s.grid {
		for y := range yGrid {
			delete(yGrid, y)
		}
	}
}

// getCellCoords converts world coordinates to cell coordinates.
func (s *SpatialHash) getCellCoords(x, y float64) (int64, int64) {
	return int64(x / s.cellSize), int64(y / s.cellSize)
}

// insert adds a sprite AABB to the spatial hash
func (s *SpatialHash) insert(aabb *SpriteAABB) {
	if aabb == nil {
		return
	}

	// Get all cells that this AABB overlaps
	minCellX, minCellY := s.getCellCoords(aabb.minX, aabb.minY)
	maxCellX, maxCellY := s.getCellCoords(aabb.maxX, aabb.maxY)

	// Insert into all overlapping cells
	for x := minCellX; x <= maxCellX; x++ {
		if s.grid[x] == nil {
			s.grid[x] = make(map[int64][]*SpriteAABB)
		}
		for y := minCellY; y <= maxCellY; y++ {
			s.grid[x][y] = append(s.grid[x][y], aabb)
		}
	}
}

// query returns all AABBs that might collide with the given AABB.
func (s *SpatialHash) query(aabb *SpriteAABB) []*SpriteAABB {
	if aabb == nil {
		return nil
	}

	// Get all cells that this AABB overlaps
	minCellX, minCellY := s.getCellCoords(aabb.minX, aabb.minY)
	maxCellX, maxCellY := s.getCellCoords(aabb.maxX, aabb.maxY)

	// Use a map to avoid duplicates
	seen := make(map[*SpriteAABB]bool)
	var results []*SpriteAABB

	// Gather all sprites from overlapping cells
	for x := minCellX; x <= maxCellX; x++ {
		if s.grid[x] == nil {
			continue
		}
		for y := minCellY; y <= maxCellY; y++ {
			for _, candidate := range s.grid[x][y] {
				if !seen[candidate] && candidate != aabb {
					seen[candidate] = true
					results = append(results, candidate)
				}
			}
		}
	}

	return results
}

// buildSpatialHashForNames builds a spatial hash with sprites matching the given name filter.
// Uses a reusable spatial hash to avoid repeated allocations.
func (p *Game) buildSpatialHashForNames(dst *SpriteImpl, nameFilter func(string) bool) *SpatialHash {
	// Lazy initialization of the reusable spatial hash
	if p.spatialHash == nil {
		p.spatialHash = newSpatialHash(defaultSpatialHashCellSize)
	}

	// Clear and reuse the existing spatial hash
	p.spatialHash.clear()

	for _, item := range p.spriteMgr.items {
		if sp, ok := item.(*SpriteImpl); ok && sp != dst {
			if nameFilter(sp.name) && sp.isVisible && !sp.isDying && sp.syncSprite != nil {
				aabb := newSpriteAABB(sp)
				if aabb != nil {
					p.spatialHash.insert(aabb)
				}
			}
		}
	}

	return p.spatialHash
}

// findCollisionsInSpatialHash performs AABB and pixel-perfect collision detection.
func findCollisionsInSpatialHash(dstAABB *SpriteAABB, spatialHash *SpatialHash, findFirst bool) []*SpriteImpl {
	var results []*SpriteImpl

	// Query spatial hash for potential collisions
	potentialCollisions := spatialHash.query(dstAABB)

	// AABB intersection and pixel-perfect collision tests
	for _, candidateAABB := range potentialCollisions {
		if !dstAABB.intersects(candidateAABB) {
			continue
		}

		// Pixel-perfect collision detection (narrow-phase)
		if candidateAABB.sprite.touchingSprite(dstAABB.sprite) {
			results = append(results, candidateAABB.sprite)
			if findFirst {
				return results
			}
		}
	}

	return results
}

// findTouchingSpriteOptimized uses spatial partitioning for efficient collision detection.
func (p *Game) findTouchingSpriteOptimized(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil || dst.syncSprite == nil {
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
	if dst == nil || dst.syncSprite == nil {
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
