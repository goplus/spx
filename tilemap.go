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
	"path"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v3/fs"
	"github.com/goplus/spx/v3/internal/base/collision"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	tm "github.com/goplus/spx/v3/internal/tilemap"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
type DecoratorJSON = tm.DecoratorJSON

type gameTilemapMgr struct {
	g              *Game
	fs             spxfs.Dir       // filesystem for loading tilemap files
	datas          *tm.TscnMapData // old format data
	decoratorDatas *DecoratorJSON  // new format decorator data
	useNewLoader   bool            // true if using C++ TileMapParser (new format)
	tilemapDir     string          // current tilemap directory (e.g., "tilemaps/map1")
	currentMap     string          // current loaded map name (e.g., "map1")
}

// -----------------------------------------------------------------------------
// Manager Setup
// -----------------------------------------------------------------------------
func (p *gameTilemapMgr) engine() *engineManagers {
	return p.g.engine()
}

func (p *gameTilemapMgr) init(g *Game, fs spxfs.Dir, tilemapPath string) {
	p.g = g
	p.fs = fs
	if tilemapPath == "" {
		return
	}
	// Load the default tilemap specified in config
	p.loadMap(tilemapPath)
}

func (p *gameTilemapMgr) hasData() bool {
	return p.datas != nil || p.useNewLoader
}

// -----------------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------------
// loadMap loads a tilemap from the specified path.
// mapDir can be either:
//   - A directory path (new format): "tilemaps/map1" -> uses C++ TileMapParser
//   - A file path (old format): "tilemaps/map1.json" -> uses Go loader
func (p *gameTilemapMgr) loadMap(mapDir string) {
	if mapDir == "" {
		return
	}
	loaded, err := tm.Load(p.fs, mapDir)
	if err != nil {
		spxlog.Panicf("Failed to load tilemap JSON file %s: %v", mapDir, err)
	}
	p.setMap(loaded)
}

func (p *gameTilemapMgr) replaceMap(loaded tm.LoadResult) {
	p.unloadMap()
	p.setMap(loaded)
}

func (p *gameTilemapMgr) setMap(loaded tm.LoadResult) {
	p.datas = loaded.Data
	p.decoratorDatas = loaded.DecoratorData
	p.useNewLoader = loaded.UseNewLoader
	p.tilemapDir = loaded.TilemapDir
	p.currentMap = loaded.CurrentMap

	if p.useNewLoader {
		p.engine().TilemapparserMgr.LoadTilemap(engine.ToAssetPath(loaded.TilemapPath))
		if loaded.DecoratorErr != nil {
			spxlog.Info("Tilemap: no decorator.json found at %s (this is OK if no decorators)", loaded.DecoratorPath)
		}
	}
}

// unloadMap unloads the current tilemap and cleans up resources.
func (p *gameTilemapMgr) unloadMap() {
	if p.currentMap == "" {
		return
	}
	if p.useNewLoader {
		p.engine().TilemapparserMgr.DestroyAllTilemaps()
	}
	p.cleanupDecorators()
	p.datas = nil
	p.decoratorDatas = nil
	p.currentMap = ""
	p.tilemapDir = ""
	p.useNewLoader = false
}

// getCurrentMap returns the name of the currently loaded tilemap.
func (p *gameTilemapMgr) getCurrentMap() string {
	return p.currentMap
}

// cleanupDecorators removes all static sprites (decorators) created by tilemaps.
func (p *gameTilemapMgr) cleanupDecorators() {
	p.engine().SceneMgr.ClearPureSprites()
}

func (p *gameTilemapMgr) loadTilemaps(datas *tm.TscnMapData) {
	tm.LoadTilemaps(datas, p.g.setTileInfo, p.g.setTileMapLayerIndex, p.g.PlaceTiles__1)
}

func (p *gameTilemapMgr) loadDecorators(datas *tm.TscnMapData) {
	p.loadDecoratorNodes(datas.Decorators, "tilemaps")
}

func (p *gameTilemapMgr) loadDecoratorNodes(decorators []tm.DecoratorNode, tilemapDir string) {
	const headingOffset = -90.0
	for _, item := range decorators {
		position := item.Position.ToVec2()
		pivot := item.Pivot.ToVec2()
		relativePath := path.Join(tilemapDir, item.Path)
		assetPath := engine.ToAssetPath(relativePath)
		texSize := p.engine().ResMgr.GetImageSize(assetPath)
		colliderPivot := item.ColliderPivot.ToVec2().Add(pivot)
		pivot = pivot.Sub(texSize.Divf(2))
		p.g.createStaticSprite(relativePath, position, item.Ratation+headingOffset,
			item.Scale.ToVec2(), int64(item.ZIndex), pivot, item.ColliderType, colliderPivot, item.ColliderParams)
	}
}

// loadDecoratorsFromJSON loads decorators from the separate decorator.json file (new format).
func (p *gameTilemapMgr) loadDecoratorsFromJSON() {
	if p.decoratorDatas == nil || len(p.decoratorDatas.Decorators) == 0 {
		return
	}
	p.loadDecoratorNodes(p.decoratorDatas.Decorators, p.tilemapDir)
}

func (p *gameTilemapMgr) parseTilemap() {
	if p.useNewLoader {
		p.loadDecoratorsFromJSON()
		return
	}
	if p.datas == nil {
		return
	}
	p.loadTilemaps(p.datas)
	p.loadDecorators(p.datas)
	p.calcWorldSize()
}

// calcWorldSize calculates and updates world size based on actual tile distribution in tilemap.
func (p *gameTilemapMgr) calcWorldSize() {
	if p.datas == nil || len(p.datas.TileMap.Layers) == 0 {
		spxlog.Debug("Tilemap: no tilemap data or layers, skipping world size update")
		return
	}
	bounds, ok := tm.CalcWorldBounds(p.datas)
	if !ok {
		spxlog.Debug("Tilemap: no tiles found in any layer")
		return
	}
	p.g.displayState.MinWorldX = bounds.MinWorldX
	p.g.displayState.MinWorldY = bounds.MinWorldY
	p.g.displayState.WorldWidth = bounds.WorldWidth
	p.g.displayState.WorldHeight = bounds.WorldHeight
}

// -----------------------------------------------------------------------------
// Tile Placement
// -----------------------------------------------------------------------------
func (p *Game) PlaceTiles__0(positions []float64, texturePath string) {
	path := engine.ToAssetPath(texturePath)
	p.engine().TilemapMgr.PlaceTiles(engine.F64Tof32(positions), path)
}

func (p *Game) PlaceTiles__1(positions []float64, texturePath string, layerIndex int64) {
	path := engine.ToAssetPath(texturePath)
	p.engine().TilemapMgr.PlaceTilesWithLayer(engine.F64Tof32(positions), path, layerIndex)
}

func (p *Game) PlaceTile(x, y float64, texturePath string) {
	path := engine.ToAssetPath(texturePath)
	p.engine().TilemapMgr.PlaceTile(mathf.NewVec2(x, y), path)
}

// -----------------------------------------------------------------------------
// Tile Removal
// -----------------------------------------------------------------------------
func (p *Game) EraseTile__0(x, y float64) {
	p.engine().TilemapMgr.EraseTile(mathf.NewVec2(x, y))
}

func (p *Game) EraseTile__1(x, y float64, layerIndex int64) {
	p.engine().TilemapMgr.EraseTileWithLayer(mathf.NewVec2(x, y), layerIndex)
}

// -----------------------------------------------------------------------------
// Tile Query
// -----------------------------------------------------------------------------
func (p *Game) GetTile__0(x, y float64) string {
	return p.engine().TilemapMgr.GetTile(mathf.NewVec2(x, y))
}

func (p *Game) GetTile__1(x, y float64, layerIndex int64) string {
	return p.engine().TilemapMgr.GetTileWithLayer(mathf.NewVec2(x, y), layerIndex)
}

// -----------------------------------------------------------------------------
// Dynamic Loading
// -----------------------------------------------------------------------------
// LoadTilemap dynamically loads a tilemap from the specified path.
// mapDir can be either:
//   - A directory path (new format): "tilemaps/map1" -> uses C++ TileMapParser
//   - A file path (old format): "tilemaps/map1.json" -> uses Go loader
//
// This will unload any currently loaded tilemap before loading the new one.
func (p *Game) LoadTilemap(mapDir string) {
	p.tilemapMgr.unloadMap()
	p.tilemapMgr.loadMap(mapDir)
	p.applyTilemap()
}

// UnloadTilemap unloads the currently loaded tilemap and cleans up resources.
func (p *Game) UnloadTilemap() {
	p.tilemapMgr.unloadMap()
}

// TilemapName returns the name of the currently loaded tilemap.
func (p *Game) TilemapName() string {
	return p.tilemapMgr.getCurrentMap()
}

// -----------------------------------------------------------------------------
// Engine Bridge
// -----------------------------------------------------------------------------
func (p *Game) setTileMapLayerIndex(index int64) {
	p.engine().TilemapMgr.SetLayerIndex(index)
}

func (p *Game) setTileInfo(texturePath string, collisionPoints []float64) {
	path := engine.ToAssetPath(texturePath)
	p.engine().TilemapMgr.SetTileWithCollisionInfo(path, engine.F64Tof32(collisionPoints))
}

func (p *Game) createStaticSprite(texturePath string, pos mathf.Vec2, rot float64, scale mathf.Vec2, zindex int64, pivot mathf.Vec2, colliderType string, colliderPivot mathf.Vec2, colliderParams []float64) {
	colliderTypeInt := collision.ParseColliderShapeType(colliderType, 0)
	p.engine().SceneMgr.CreateStaticSprite(engine.ToAssetPath(texturePath), pos, rot, scale, zindex, pivot, colliderTypeInt, colliderPivot, colliderParams)
}
