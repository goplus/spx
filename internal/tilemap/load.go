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

package tilemap

import (
	"encoding/json"
	"math"
	"path"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

type DecoratorJSON struct {
	Version    int             `json:"version"`
	Decorators []DecoratorNode `json:"decorators"`
}

type LoadResult struct {
	Data          *TscnMapData
	DecoratorData *DecoratorJSON
	DecoratorErr  error
	UseNewLoader  bool
	TilemapDir    string
	CurrentMap    string
	TilemapPath   string
	DecoratorPath string
}

type WorldBounds struct {
	MinWorldX   int
	MinWorldY   int
	WorldWidth  int
	WorldHeight int
}

func Load(fs spxfs.Dir, mapDir string) (LoadResult, error) {
	if mapDir == "" {
		return LoadResult{}, nil
	}

	if isNewFormat(mapDir) {
		tilemapPath := path.Join(mapDir, "tilemap.json")
		decoratorPath := path.Join(mapDir, "decorator.json")
		var tilemapJSON json.RawMessage
		if err := coreproject.LoadJSON(&tilemapJSON, fs, tilemapPath); err != nil {
			return LoadResult{}, err
		}
		decoratorData, decoratorErr := loadDecoratorJSON(fs, decoratorPath)
		return LoadResult{
			DecoratorData: decoratorData,
			DecoratorErr:  decoratorErr,
			UseNewLoader:  true,
			TilemapDir:    mapDir,
			CurrentMap:    path.Base(mapDir),
			TilemapPath:   tilemapPath,
			DecoratorPath: decoratorPath,
		}, nil
	}

	var data TscnMapData
	if err := coreproject.LoadJSON(&data, fs, mapDir); err != nil {
		return LoadResult{}, err
	}
	return LoadResult{
		Data:       &data,
		TilemapDir: path.Dir(mapDir),
		CurrentMap: strings.TrimSuffix(path.Base(mapDir), ".json"),
	}, nil
}

// CalcWorldBounds reports tile bounds representable by the runtime's int coordinates.
// Empty maps and bounds whose size or edges exceed that range return false.
func CalcWorldBounds(data *TscnMapData) (WorldBounds, bool) {
	if data == nil || len(data.TileMap.Layers) == 0 {
		return WorldBounds{}, false
	}

	tileSizeX := int64(data.TileMap.TileSize.Width)
	tileSizeY := int64(data.TileMap.TileSize.Height)

	var minX, maxX, minY, maxY int64
	hasAnyTiles := false

	for _, layer := range data.TileMap.Layers {
		for i := 0; i+4 < len(layer.TileData); i += 5 {
			tileX := int64(layer.TileData[i+1])
			tileY := int64(layer.TileData[i+2])
			if !hasAnyTiles {
				minX, maxX = tileX, tileX
				minY, maxY = tileY, tileY
				hasAnyTiles = true
				continue
			}
			minX, maxX = min(minX, tileX), max(maxX, tileX)
			minY, maxY = min(minY, tileY), max(maxY, tileY)
		}
	}

	if !hasAnyTiles {
		return WorldBounds{}, false
	}

	minWorldX := minX * tileSizeX
	maxWorldX := (maxX + 1) * tileSizeX
	minWorldY := (minY - 1) * tileSizeY
	maxWorldY := maxY * tileSizeY
	width, height := maxWorldX-minWorldX, maxWorldY-minWorldY
	// The runtime also adds the width and negates the vertical edges.
	for _, value := range [...]int64{minWorldX, minWorldY, maxWorldX, -minWorldY, -maxWorldY, width, height} {
		if value < math.MinInt || value > math.MaxInt {
			return WorldBounds{}, false
		}
	}
	return WorldBounds{
		MinWorldX:   int(minWorldX),
		MinWorldY:   int(minWorldY),
		WorldWidth:  int(width),
		WorldHeight: int(height),
	}, true
}

func isNewFormat(tilemapPath string) bool {
	return !strings.Contains(tilemapPath, ".json")
}

func loadDecoratorJSON(fs spxfs.Dir, decoratorPath string) (*DecoratorJSON, error) {
	var data DecoratorJSON
	if err := coreproject.LoadJSON(&data, fs, decoratorPath); err != nil {
		return nil, err
	}
	return &data, nil
}
