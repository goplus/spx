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
	"reflect"
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
				"decorators":[],
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
