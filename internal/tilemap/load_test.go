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
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type testDir struct {
	files map[string]string
}

func (d testDir) Open(name string) (io.ReadCloser, error) {
	content, ok := d.files[name]
	if !ok {
		return nil, errors.New("file not found")
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (d testDir) Close() error {
	return nil
}

func TestLoadNewFormat(t *testing.T) {
	dir := testDir{
		files: map[string]string{
			"tilemaps/map1/tilemap.json":   `{}`,
			"tilemaps/map1/decorator.json": `{"version":1,"decorators":[{"name":"tree","path":"tree.png"}]}`,
		},
	}

	got, err := Load(dir, "tilemaps/map1")
	if err != nil {
		t.Fatalf("Load(new) error: %v", err)
	}
	if !got.UseNewLoader {
		t.Fatal("Load(new) UseNewLoader = false, want true")
	}
	if got.TilemapDir != "tilemaps/map1" || got.CurrentMap != "map1" {
		t.Fatalf("Load(new) dir/map = %q/%q", got.TilemapDir, got.CurrentMap)
	}
	if got.TilemapPath != "tilemaps/map1/tilemap.json" {
		t.Fatalf("Load(new) TilemapPath = %q", got.TilemapPath)
	}
	if got.DecoratorPath != "tilemaps/map1/decorator.json" {
		t.Fatalf("Load(new) DecoratorPath = %q", got.DecoratorPath)
	}
	if got.DecoratorErr != nil {
		t.Fatalf("Load(new) DecoratorErr = %v", got.DecoratorErr)
	}
	if got.DecoratorData == nil || len(got.DecoratorData.Decorators) != 1 {
		t.Fatalf("Load(new) DecoratorData = %#v", got.DecoratorData)
	}
}

func TestLoadNewFormatRequiresValidTilemapJSON(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{name: "missing", files: map[string]string{}},
		{name: "malformed", files: map[string]string{"tilemaps/map1/tilemap.json": `{`}},
		{name: "trailing content", files: map[string]string{"tilemaps/map1/tilemap.json": `{} trailing`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Load(testDir{files: test.files}, "tilemaps/map1"); err == nil {
				t.Fatal("Load(new) error = nil")
			}
		})
	}
}

func TestLoadOldFormat(t *testing.T) {
	dir := testDir{
		files: map[string]string{
			"tilemaps/map1.json": `{
				"tilemap":{"tile_size":{"width":32,"height":16},"tileset":{"sources":[]},"layers":[{"tile_data":[1,2,3,0,0]}]},
				"decorators":[{"path":"tree.png"},{"path":"tilemaps/bush.png"}],
				"sprites":[]
			}`,
		},
	}

	got, err := Load(dir, "tilemaps/map1.json")
	if err != nil {
		t.Fatalf("Load(old) error: %v", err)
	}
	if got.UseNewLoader {
		t.Fatal("Load(old) UseNewLoader = true, want false")
	}
	if got.TilemapDir != "tilemaps" || got.CurrentMap != "map1" {
		t.Fatalf("Load(old) dir/map = %q/%q", got.TilemapDir, got.CurrentMap)
	}
	if got.Data == nil {
		t.Fatal("Load(old) Data = nil")
	}
	if got.Data.Decorators[0].Path != "tree.png" || got.Data.Decorators[1].Path != "tilemaps/bush.png" {
		t.Fatalf("Load(old) changed decorator paths: %#v", got.Data.Decorators)
	}
}

func TestLoadTilemapsCompactData(t *testing.T) {
	type event struct {
		kind  string
		path  string
		layer int64
		data  []float64
	}
	// Atlas coordinates do not affect placement; incomplete records are ignored.
	for tail := 0; tail < 5; tail++ {
		tileData := []int32{
			2, 3, -2, 91, 92,
			1, -1, 4, 81, 82,
			2, 5, 6, 71, 72,
			-1, 1, 2, 61, 62,
			3, 7, 8, 51, 52,
		}
		tileData = append(tileData, []int32{1, 100, 200, 300}[:tail]...)
		data := &TscnMapData{TileMap: tileMapData{
			TileSize: tileSize{Width: 32, Height: 16},
			TileSet: tileSet{Sources: []tileSource{
				{ID: 2, TexturePath: "grass.png", Tiles: []tileInfo{{Physics: physicsData{CollisionPoints: []Vec2{{X: 1, Y: 2}, {X: 3, Y: 4}}}}}},
				{ID: 1, TexturePath: "tilemaps/stone.png"},
				{ID: -1, TexturePath: "sentinel.png"},
			}},
			Layers: []tilemapLayer{
				{ZIndex: 7, TileData: tileData},
				{ZIndex: -2, TileData: []int32{1, 2, 3, 11, 12}},
				{ZIndex: 9, TileData: []int32{2, 100}},
			},
		}}
		var got []event
		LoadTilemaps(data,
			func(path string, points []float64) {
				got = append(got, event{kind: "tile", path: path, data: append([]float64(nil), points...)})
			},
			func(layer int64) { got = append(got, event{kind: "layer", layer: layer}) },
			func(positions []float64, path string, layer int64) {
				got = append(got, event{kind: "place", path: path, layer: layer, data: append([]float64(nil), positions...)})
			},
		)
		want := []event{
			{kind: "tile", path: "tilemaps/grass.png", data: []float64{1, 2, 3, 4}},
			{kind: "tile", path: "tilemaps/stone.png"},
			{kind: "tile", path: "tilemaps/sentinel.png"},
			{kind: "layer", layer: 7},
			// Preserve the initial -1 source sentinel and missing-source behavior.
			{kind: "place", layer: 7, data: []float64{32, 32}},
			{kind: "place", path: "tilemaps/stone.png", layer: 7, data: []float64{-32, 64}},
			{kind: "place", path: "tilemaps/grass.png", layer: 7, data: []float64{96, -32, 160, 96}},
			{kind: "place", layer: 7, data: []float64{224, 128}},
			{kind: "layer", layer: -2},
			{kind: "place", path: "tilemaps/stone.png", layer: -2, data: []float64{64, 48}},
			{kind: "layer", layer: 9},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tail length %d: callback trace = %#v, want %#v", tail, got, want)
		}
	}
}

func TestCalcWorldBounds(t *testing.T) {
	data := &TscnMapData{
		TileMap: tileMapData{
			TileSize: tileSize{Width: 32, Height: 16},
			Layers: []tilemapLayer{
				{TileData: []int32{1, 2, 3, 0, 0, 1, 4, 5, 0, 0}},
			},
		},
	}

	got, ok := CalcWorldBounds(data)
	if !ok {
		t.Fatal("CalcWorldBounds ok = false, want true")
	}
	want := WorldBounds{
		MinWorldX:   64,
		MinWorldY:   32,
		WorldWidth:  96,
		WorldHeight: 48,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CalcWorldBounds = %#v, want %#v", got, want)
	}
}

func TestCalcWorldBoundsNoTiles(t *testing.T) {
	data := &TscnMapData{
		TileMap: tileMapData{
			TileSize: tileSize{Width: 32, Height: 16},
			Layers:   []tilemapLayer{{}},
		},
	}

	if _, ok := CalcWorldBounds(data); ok {
		t.Fatal("CalcWorldBounds ok = true, want false")
	}
}

func TestLoadTilemapsWidensCoordinateProducts(t *testing.T) {
	data := &TscnMapData{TileMap: tileMapData{
		TileSize: tileSize{Width: 64, Height: 64},
		TileSet:  tileSet{Sources: []tileSource{{ID: 1, TexturePath: "terrain.png"}}},
		Layers: []tilemapLayer{{TileData: []int32{
			1, 1 << 26, -(1 << 26), 0, 0,
			1, math.MinInt32, math.MaxInt32, 0, 0,
		}}},
	}}
	var positions []float64
	LoadTilemaps(data, func(string, []float64) {}, func(int64) {}, func(got []float64, _ string, _ int64) {
		positions = append(positions, got...)
	})
	want := []float64{4294967296, -4294967296, -137438953472, 137438953408}
	if !reflect.DeepEqual(positions, want) {
		t.Fatalf("tile positions = %v, want %v", positions, want)
	}
}

func TestCalcWorldBoundsIntegerEdges(t *testing.T) {
	tests := []struct {
		name                   string
		minX, maxX, minY, maxY int32
		width, height          int32
		want                   [4]int64
		bits                   int
	}{
		{"ordinary negative", -2, 1, -1, 2, 32, 16, [4]int64{-64, -32, 128, 64}, 32},
		{"upper x edge", math.MaxInt32, math.MaxInt32, 0, 0, 1, 1, [4]int64{math.MaxInt32, -1, 1, 1}, 64},
		{"lower y edge", 0, 0, math.MinInt32, math.MinInt32, 1, 1, [4]int64{0, -2147483649, 1, 1}, 64},
		{"top negation", 0, 0, math.MinInt32 + 1, math.MinInt32 + 1, 1, 1, [4]int64{0, math.MinInt32, 1, 1}, 64},
		{"wide span", math.MinInt32, math.MaxInt32, 0, 0, 1, 1, [4]int64{math.MinInt32, -1, 4294967296, 1}, 64},
		{"negative tile size", math.MinInt32, math.MinInt32, math.MaxInt32, math.MaxInt32, -1, -1, [4]int64{2147483648, -2147483646, -1, -1}, 64},
		{"maximum positive span", math.MinInt32, math.MaxInt32, math.MinInt32, math.MaxInt32, math.MaxInt32, math.MaxInt32, [4]int64{-4611686016279904256, -4611686018427387903, 9223372032559808512, 9223372032559808512}, 64},
		{"minimum negative span", math.MinInt32, math.MaxInt32, math.MinInt32, math.MaxInt32, math.MinInt32, math.MinInt32, [4]int64{4611686018427387904, 4611686020574871552, math.MinInt64, math.MinInt64}, 64},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := &TscnMapData{TileMap: tileMapData{
				TileSize: tileSize{Width: test.width, Height: test.height},
				Layers: []tilemapLayer{{TileData: []int32{
					1, test.minX, test.minY, 0, 0,
					1, test.maxX, test.maxY, 0, 0,
				}}},
			}}
			got, ok := CalcWorldBounds(data)
			if strconv.IntSize < test.bits {
				if ok || got != (WorldBounds{}) {
					t.Fatalf("unrepresentable bounds = (%+v, %v), want zero/false", got, ok)
				}
				return
			}
			want := WorldBounds{int(test.want[0]), int(test.want[1]), int(test.want[2]), int(test.want[3])}
			if !ok || got != want {
				t.Fatalf("bounds = (%+v, %v), want (%+v, true)", got, ok, want)
			}
		})
	}
}
