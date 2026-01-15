package tilemap

import (
	"path"
	"slices"
	"strings"

	"github.com/goplus/spbase/mathf"
)

type vec2i struct {
	X int32 `json:"x"`
	Y int32 `json:"y"`
}

// Vec2 represents a 2D coordinate in world space (pixels)
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v Vec2) ToVec2() mathf.Vec2 {
	return mathf.NewVec2(v.X, v.Y)
}
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{X: v.X + other.X, Y: v.Y + other.Y}
}
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{X: v.X - other.X, Y: v.Y - other.Y}
}

// tileSize represents the dimensions of a tile
type tileSize struct {
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

// physicsData represents physics properties of a tile
type physicsData struct {
	CollisionPoints []Vec2 `json:"collision_points,omitempty"`
	// other properties
}

// tileInfo represents information about a single tile in the tileset
type tileInfo struct {
	AtlasCoords vec2i       `json:"atlas_coords"`
	Physics     physicsData `json:"physics,omitempty"`
}

// tileSource represents a tileset source
type tileSource struct {
	ID          int32      `json:"id"`
	TexturePath string     `json:"texture_path"`
	Tiles       []tileInfo `json:"tiles"`
}

// tileSet represents the complete tileset information
type tileSet struct {
	Sources []tileSource `json:"sources"`
	// Other properties
}

// tileInstance represents a placed tile in the map
type tileInstance struct {
	TileCoords  vec2i `json:"tile_coords"`
	SourceID    int32 `json:"source_id"`
	AtlasCoords vec2i `json:"atlas_coords"`
}

// tilemapLayer represents a tilemap layer with compact tile data format
type tilemapLayer struct {
	ID       int32   `json:"id"`
	Name     string  `json:"name"`
	ZIndex   int     `json:"z_index"`
	TileData []int32 `json:"tile_data"`
}

// tileMapData represents the complete tilemap data
type tileMapData struct {
	Format   int32          `json:"format"`
	TileSize tileSize       `json:"tile_size"`
	TileSet  tileSet        `json:"tileset"`
	Layers   []tilemapLayer `json:"layers"`
}

// DecoratorNode represents a Sprite2D node in the scene
type DecoratorNode struct {
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Parent         string    `json:"parent"`
	Position       Vec2      `json:"position"`
	Scale          Vec2      `json:"scale,omitempty"`
	Ratation       float64   `json:"rotation,omitempty"`
	Pivot          Vec2      `json:"pivot,omitempty"`
	ZIndex         int32     `json:"z_index,omitempty"`
	ColliderType   string    `json:"collider_type,omitempty"` //"none","auto","circle","rect","capsule","polygon",
	ColliderPivot  Vec2      `json:"collider_pivot,omitempty"`
	ColliderParams []float64 `json:"collider_params,omitempty"`
}

// spriteNode represents an instantiated prefab node in the scene
type spriteNode struct {
	Name           string                 `json:"name"`
	Path           string                 `json:"path"`
	Parent         string                 `json:"parent"`
	Position       Vec2                   `json:"position"`
	Scale          Vec2                   `json:"scale,omitempty"`
	Ratation       float64                `json:"rotation,omitempty"`
	ZIndex         int32                  `json:"z_index,omitempty"`
	Pivot          Vec2                   `json:"pivot,omitempty"`
	ColliderType   string                 `json:"collider_type,omitempty"` //"none","auto","circle","rect","capsule","polygon",
	ColliderPivot  Vec2                   `json:"collider_pivot,omitempty"`
	ColliderParams []float64              `json:"collider_params,omitempty"`
	Properties     map[string]interface{} `json:"properties,omitempty"`
}

// TscnMapData represents the root structure for JSON output
type TscnMapData struct {
	TileMap    tileMapData     `json:"tilemap"`
	Decorators []DecoratorNode `json:"decorators"`
	Sprites    []spriteNode    `json:"sprites"`
}

const tilemapRelDir = "tilemaps"

func toTilemapPath(p string) string {
	if strings.HasPrefix(p, tilemapRelDir) {
		return p
	}
	return path.Join(tilemapRelDir, p)
}

// Runtime utilities for parsing tile data
func ConvertData(data *TscnMapData) {
	for _, item := range data.Decorators {
		item.Path = toTilemapPath(item.Path)
	}
}
func LoadTilemaps(datas *TscnMapData, funcSetTile func(texturePath string, points []float64), funcSetLayer func(layerIndex int64),
	funcPlaceTiles func(positions []float64, texturePath string, layerIndex int64)) {
	paths := make(map[int32]string)
	for _, item := range datas.TileMap.TileSet.Sources {
		paths[item.ID] = toTilemapPath(item.TexturePath)
		points := make([]float64, 0)
		for _, tile := range item.Tiles {
			pts := tile.Physics.CollisionPoints
			for _, p := range pts {
				points = append(points, p.X, p.Y)
			}
		}
		funcSetTile(paths[item.ID], points)
	}
	for _, layer := range datas.TileMap.Layers {
		layerId := int64(layer.ZIndex)
		funcSetLayer(layerId)
		tileData := layer.TileData
		tileSizeX, tileSizeY := datas.TileMap.TileSize.Width, datas.TileMap.TileSize.Height
		tiles := parseTileData(tileData)
		slices.SortFunc(tiles, func(a, b tileInstance) int {
			return int(a.SourceID - b.SourceID)
		})
		lastId := int32(-1)
		path := ""
		positions := make([]float64, 0, len(tiles)*2)
		for _, tile := range tiles {
			if lastId != tile.SourceID {
				if len(positions) > 0 {
					funcPlaceTiles(positions, path, layerId)
				}
				positions = positions[:0]
				lastId = tile.SourceID
				path = paths[tile.SourceID]
			}
			x, y := tile.TileCoords.X*tileSizeX, tile.TileCoords.Y*tileSizeY
			positions = append(positions, float64(x), float64(y))
		}
		if len(positions) > 0 {
			funcPlaceTiles(positions, path, layerId)
		}
	}
}

// TileMapParser provides utilities for parsing compact tile data
// ParseTileData converts compact tile data array to tile instances
// New format: [source_id, tile_x, tile_y, atlas_x, atlas_y] (5 elements per tile)
// Where source_id is the ID of the tileset source,
// tile_x and tile_y are the tile coordinates in the map,
// and atlas_x and atlas_y are the coordinates in the tileset texture.
func parseTileData(tileData []int32) []tileInstance {
	tileCount := len(tileData) / 5
	tiles := make([]tileInstance, 0, tileCount)

	for i := 0; i < len(tileData); i += 5 {
		if i+4 >= len(tileData) {
			break
		}

		sourceID := tileData[i]
		tileX := tileData[i+1]
		tileY := tileData[i+2]
		atlasX := tileData[i+3]
		atlasY := tileData[i+4]

		tile := tileInstance{
			TileCoords:  vec2i{X: tileX, Y: tileY},
			SourceID:    sourceID,
			AtlasCoords: vec2i{X: atlasX, Y: atlasY},
		}

		tiles = append(tiles, tile)
	}

	return tiles
}
