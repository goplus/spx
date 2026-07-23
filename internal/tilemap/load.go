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
	ConvertData(&data)
	return LoadResult{
		Data:       &data,
		TilemapDir: path.Dir(mapDir),
		CurrentMap: strings.TrimSuffix(path.Base(mapDir), ".json"),
	}, nil
}

func CalcWorldBounds(data *TscnMapData) (WorldBounds, bool) {
	if data == nil || len(data.TileMap.Layers) == 0 {
		return WorldBounds{}, false
	}

	tileSizeX := int(data.TileMap.TileSize.Width)
	tileSizeY := int(data.TileMap.TileSize.Height)

	var minX, maxX, minY, maxY int32
	hasAnyTiles := false

	for _, layer := range data.TileMap.Layers {
		for i := 0; i+4 < len(layer.TileData); i += 5 {
			tileX := layer.TileData[i+1]
			tileY := layer.TileData[i+2]
			if !hasAnyTiles {
				minX, maxX = tileX, tileX
				minY, maxY = tileY, tileY
				hasAnyTiles = true
				continue
			}
			if tileX < minX {
				minX = tileX
			}
			if tileX > maxX {
				maxX = tileX
			}
			if tileY < minY {
				minY = tileY
			}
			if tileY > maxY {
				maxY = tileY
			}
		}
	}

	if !hasAnyTiles {
		return WorldBounds{}, false
	}

	minWorldX := int(minX) * tileSizeX
	maxWorldX := int(maxX+1) * tileSizeX
	minWorldY := int(minY-1) * tileSizeY
	maxWorldY := int(maxY) * tileSizeY
	return WorldBounds{
		MinWorldX:   minWorldX,
		MinWorldY:   minWorldY,
		WorldWidth:  maxWorldX - minWorldX,
		WorldHeight: maxWorldY - minWorldY,
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
