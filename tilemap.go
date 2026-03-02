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
	"fmt"
	"path"
	"sort"
	"strings"

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
	tm "github.com/goplus/spx/v2/internal/tilemap"

	"github.com/goplus/spbase/mathf"
)

// DecoratorJSON represents the structure of decorator.json file (new format).
type DecoratorJSON struct {
	Version    int                `json:"version"`
	Decorators []tm.DecoratorNode `json:"decorators"`
}

type gameTilemapMgr struct {
	g              *Game
	fs             spxfs.Dir       // filesystem for loading tilemap files
	datas          *tm.TscnMapData // old format data
	decoratorDatas *DecoratorJSON  // new format decorator data
	useNewLoader   bool            // true if using C++ TileMapParser (new format)
	tilemapDir     string          // current tilemap directory (e.g., "tilemaps/map1")
	currentMap     string          // current loaded map name (e.g., "map1")
}

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

// loadDecoratorJSON loads decorator data from a separate decorator.json file.
func (p *gameTilemapMgr) loadDecoratorJSON(fs spxfs.Dir, decoratorPath string) {
	var data DecoratorJSON
	err := loadJson(&data, fs, decoratorPath)
	if err != nil {
		fmt.Printf("[TILEMAP] No decorator.json found at %s (this is OK if no decorators)\n", decoratorPath)
		return
	}
	p.decoratorDatas = &data
}

func (p *gameTilemapMgr) hasData() bool {
	return p.datas != nil || p.useNewLoader
}

// isNewFormat checks if the tilemap path is in the new format (directory path).
// New format: path does NOT contain ".json" (e.g., "tilemaps/map1")
// Old format: path contains ".json" (e.g., "tilemaps/map1.json")
func (p *gameTilemapMgr) isNewFormat(tilemapPath string) bool {
	return !strings.Contains(tilemapPath, ".json")
}

// loadMap loads a tilemap from the specified path.
// mapDir can be either:
//   - A directory path (new format): "tilemaps/map1" -> uses C++ TileMapParser
//   - A file path (old format): "tilemaps/map1.json" -> uses Go loader
func (p *gameTilemapMgr) loadMap(mapDir string) {
	if mapDir == "" {
		return
	}

	// Determine format based on path
	p.useNewLoader = p.isNewFormat(mapDir)

	if p.useNewLoader {
		// New format: directory path
		p.tilemapDir = mapDir
		p.currentMap = path.Base(mapDir)

		// Build paths to tilemap.json and decorator.json
		tilemapPath := path.Join(mapDir, "tilemap.json")
		decoratorPath := path.Join(mapDir, "decorator.json")

		// Load tilemap using C++ TileMapParser
		enginePath := engine.ToAssetPath(tilemapPath)
		p.engine().TilemapparserMgr.LoadTilemap(enginePath)
		p.useNewLoader = true

		// Load decorator.json
		p.loadDecoratorJSON(p.fs, decoratorPath)
	} else {
		// Old format: file path (contains .json)
		p.tilemapDir = path.Dir(mapDir)
		p.currentMap = strings.TrimSuffix(path.Base(mapDir), ".json")

		// Load using Go loader
		var data tm.TscnMapData
		err := loadJson(&data, p.fs, mapDir)
		if err != nil {
			panic(fmt.Sprintf("Failed to load tilemap JSON file %s: %v", mapDir, err))
		}
		p.datas = &data
		tm.ConvertData(&data)
	}
}

// unloadMap unloads the current tilemap and cleans up resources
func (p *gameTilemapMgr) unloadMap() {
	if p.currentMap == "" {
		return
	}

	if p.useNewLoader {
		// Unload from C++ TileMapParser
		p.engine().TilemapparserMgr.DestroyAllTilemaps()
	}

	// Clean up decorator sprites
	p.cleanupDecorators()

	// Reset state
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

// cleanupDecorators removes all static sprites (decorators) created by tilemaps
func (p *gameTilemapMgr) cleanupDecorators() {
	p.engine().SceneMgr.ClearPureSprites()
}

func (p *gameTilemapMgr) loadTilemaps(datas *tm.TscnMapData) {
	tm.LoadTilemaps(datas, p.g.setTileInfo__1, p.g.setTileMapLayerIndex, p.g.PlaceTiles__1)
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

func (p *gameTilemapMgr) loadSprites(datas *tm.TscnMapData) {

	sort.Slice(datas.Sprites, func(i, j int) bool {
		return datas.Sprites[i].Path < datas.Sprites[j].Path
	})

	for _, item := range datas.Sprites {
		sp, ok := p.g.sprs[item.Path]
		if ok {
			x, y := item.Position.X, item.Position.Y
			doClone(sp, nil, true, func(sprite *SpriteImpl) {
				sprite.SetXYpos(x, y)
				sprite.Show()
			})
		}
	}
}

func (p *gameTilemapMgr) parseTilemap() {
	// Handle new format: load decorators from separate JSON file
	if p.useNewLoader {
		p.loadDecoratorsFromJSON()
		return
	}

	// Old format: load from combined TscnMapData
	if p.datas == nil {
		return
	}
	p.loadTilemaps(p.datas)
	p.loadDecorators(p.datas)
	//p.loadSprites(p.datas)

	// Update world size based on actual tilemap content
	p.calcWorldSize()
}

// calcWorldSize calculates and updates world size based on actual tile distribution in tilemap.
func (p *gameTilemapMgr) calcWorldSize() {
	if p.datas == nil || len(p.datas.TileMap.Layers) == 0 {
		fmt.Println("[TILEMAP DEBUG] No tilemap data or layers, skipping world size update")
		return
	}

	tileSizeX := int(p.datas.TileMap.TileSize.Width)
	tileSizeY := int(p.datas.TileMap.TileSize.Height)

	var minX, maxX, minY, maxY int32 = 0, 0, 0, 0
	hasAnyTiles := false
	totalTiles := 0

	for _, layer := range p.datas.TileMap.Layers {
		tiles := p.parseTileDataForBounds(layer.TileData)
		totalTiles += len(tiles)
		for _, tile := range tiles {
			if !hasAnyTiles {
				minX, maxX = tile.X, tile.X
				minY, maxY = tile.Y, tile.Y
				hasAnyTiles = true
			} else {
				if tile.X < minX {
					minX = tile.X
				}
				if tile.X > maxX {
					maxX = tile.X
				}
				if tile.Y < minY {
					minY = tile.Y
				}
				if tile.Y > maxY {
					maxY = tile.Y
				}
			}
		}
	}

	if hasAnyTiles {
		minWorldX := int((minX) * int32(tileSizeX))
		maxWorldX := int((maxX + 1) * int32(tileSizeX)) // +1 to include the full size of the last tile
		minWorldY := int((minY - 1) * int32(tileSizeY)) // -1 to include the full size of the last tile
		maxWorldY := int((maxY) * int32(tileSizeY))

		worldWidth := maxWorldX - minWorldX
		worldHeight := maxWorldY - minWorldY

		p.g.minWorldX = minWorldX
		p.g.minWorldY = minWorldY
		p.g.worldWidth = worldWidth
		p.g.worldHeight = worldHeight

	} else {
		fmt.Println("[TILEMAP DEBUG] No tiles found in any layer")
	}
}

// parseTileDataForBounds parses tile data for boundary calculation (copied logic from internal/tilemap package).
func (p *gameTilemapMgr) parseTileDataForBounds(tileData []int32) []mathf.Vec2i {
	tileCount := len(tileData) / 5
	tiles := make([]mathf.Vec2i, 0, tileCount)

	for i := 0; i < len(tileData); i += 5 {
		if i+4 >= len(tileData) {
			break
		}

		tileX := tileData[i+1]
		tileY := tileData[i+2]

		tile := mathf.Vec2i{
			X: tileX,
			Y: tileY,
		}

		tiles = append(tiles, tile)
	}

	return tiles
}
