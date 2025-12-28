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
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

// ============================================================================
// Tilemap Layer Management
// ============================================================================

func (p *Game) setTileMapLayerIndex(index int64) {
	tilemapMgr.SetLayerIndex(index)
}

func (p *Game) setTileMapOffset(index int64, x, y float64) {
	tilemapMgr.SetLayerOffset(index, mathf.NewVec2(x, y))
}

// ============================================================================
// Tile Configuration
// ============================================================================

func (p *Game) setTileInfo__0(texturePath string, isCollision bool) {
	path := engine.ToAssetPath(texturePath)
	tilemapMgr.SetTile(path, isCollision)
}

func (p *Game) setTileInfo__1(texturePath string, collisionPoints []float64) {
	path := engine.ToAssetPath(texturePath)
	tilemapMgr.SetTileWithCollisionInfo(path, f64Tof32(collisionPoints))
}

// ============================================================================
// Tile Placement
// ============================================================================

func (p *Game) PlaceTiles__0(positions []float64, texturePath string) {
	path := engine.ToAssetPath(texturePath)
	tilemapMgr.PlaceTiles(f64Tof32(positions), path)
}

func (p *Game) PlaceTiles__1(positions []float64, texturePath string, layerIndex int64) {
	path := engine.ToAssetPath(texturePath)
	tilemapMgr.PlaceTilesWithLayer(f64Tof32(positions), path, layerIndex)
}

func (p *Game) PlaceTile(x, y float64, texturePath string) {
	path := engine.ToAssetPath(texturePath)
	tilemapMgr.PlaceTile(mathf.NewVec2(x, y), path)
}

// ============================================================================
// Tile Removal
// ============================================================================

func (p *Game) EraseTile__0(x, y float64) {
	tilemapMgr.EraseTile(mathf.NewVec2(x, y))
}

func (p *Game) EraseTile__1(x, y float64, layerIndex int64) {
	tilemapMgr.EraseTileWithLayer(mathf.NewVec2(x, y), layerIndex)
}

// ============================================================================
// Tile Query
// ============================================================================

func (p *Game) GetTile__0(x, y float64) string {
	return tilemapMgr.GetTile(mathf.NewVec2(x, y))
}

func (p *Game) GetTile__1(x, y float64, layerIndex int64) string {
	return tilemapMgr.GetTileWithLayer(mathf.NewVec2(x, y), layerIndex)
}

// ============================================================================
// Static Sprite Creation
// ============================================================================

func (p *Game) createDecorators(texturePath string, pos mathf.Vec2, rot float64, scale mathf.Vec2, zindex int64, pivot mathf.Vec2) {
	sceneMgr.CreateStaticSprite(engine.ToAssetPath(texturePath), pos, rot, scale, zindex, pivot, 0, mathf.NewVec2(0, 0), nil)
}

func (p *Game) createStaticSprite(texturePath string, pos mathf.Vec2, rot float64, scale mathf.Vec2, zindex int64, pivot mathf.Vec2, colliderType string, colliderPivot mathf.Vec2, colliderParams []float64) {
	colliderTypeInt := parseColliderShapeType(colliderType, 0)
	sceneMgr.CreateStaticSprite(engine.ToAssetPath(texturePath), pos, rot, scale, zindex, pivot, colliderTypeInt, colliderPivot, colliderParams)
}

// ============================================================================
// Dynamic Tilemap Loading API
// ============================================================================

// LoadTilemap dynamically loads a tilemap from the specified path
// mapDir can be either:
//   - A directory path (new format): "tilemaps/map1" -> uses C++ TileMapParser
//   - A file path (old format): "tilemaps/map1.json" -> uses Go loader
//
// This will unload any currently loaded tilemap before loading the new one.
func (p *Game) LoadTilemap(mapDir string) {
	p.tilemapMgr.unloadMap()
	p.tilemapMgr.loadMap(mapDir)
	p.tilemapMgr.parseTilemap()
}

// UnloadTilemap unloads the currently loaded tilemap and cleans up resources
func (p *Game) UnloadTilemap() {
	p.tilemapMgr.unloadMap()
}

// TilemapName returns the name of the currently loaded tilemap
// Returns empty string if no tilemap is loaded
func (p *Game) TilemapName() string {
	return p.tilemapMgr.getCurrentMap()
}
