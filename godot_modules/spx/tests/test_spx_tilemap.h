/**************************************************************************/
/*  test_spx_tilemap.h                                                  */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#ifndef TEST_SPX_TILEMAP_H
#define TEST_SPX_TILEMAP_H

#include "../spx_res_mgr.h"
#include "../spx_tilemap_types.h"
#include "../spx_tilemapparser_mgr.h"
#include "core/core_bind.h"
#include "core/io/json.h"
#include "scene/2d/tile_map_layer.h"
#include "spx_test_fixtures.h"

namespace TestSpxTilemap {

static Dictionary map_document() {
	return JSON::parse_string(R"({
		"version": 1, "name": "meadow", "node_offset": [-16, 8.5],
		"tileset": {
			"tile_size": [8, 8], "tile_shape": "square",
			"physics_layers": [{"collision_layer": 2, "collision_mask": 5}],
			"sources": [{"id": 3, "type": "atlas", "texture": "atlas.png",
				"texture_region_size": [8, 8], "margins": [1, 1], "separation": [1, 1],
				"tiles": [{"atlas_coords": [0, 0], "size_in_atlas": [1, 1],
					"physics": [{"layer": 0, "polygons": [[-4, -4, 4, -4, 0, 4]]}]}]}]
		},
		"layers": [
			{"name": "ground", "z_index": -2, "offset": [2.5, -3], "enabled": true, "tile_map_data": ""},
			{"name": "", "z_index": 4, "offset": [0, 0], "enabled": false, "tile_map_data": ""}
		]
	})");
}

TEST_CASE("[SPX] Tilemap JSON preserves nested atlas physics and layer properties") {
	SpxTileMapData map;
	REQUIRE(map.from_json(map_document()));
	CHECK(map.version == 1);
	CHECK(map.name == "meadow");
	CHECK(map.node_offset == Vector2(-16, 8.5));
	CHECK(map.tileset.tile_size == Vector2i(8, 8));
	CHECK(map.tileset.tile_shape == "square");
	REQUIRE(map.tileset.physics_layers.size() == 1);
	CHECK(map.tileset.physics_layers[0].collision_layer == 2);
	CHECK(map.tileset.physics_layers[0].collision_mask == 5);
	REQUIRE(map.tileset.sources.size() == 1);
	const SpxTileSetSourceData &atlas = map.tileset.sources[0];
	CHECK(atlas.id == 3);
	CHECK(atlas.type == "atlas");
	CHECK(atlas.texture == "atlas.png");
	CHECK(atlas.texture_region_size == Vector2i(8, 8));
	CHECK(atlas.margins == Vector2i(1, 1));
	CHECK(atlas.separation == Vector2i(1, 1));
	REQUIRE(atlas.tiles.size() == 1);
	CHECK(atlas.tiles[0].atlas_coords == Vector2i(0, 0));
	CHECK(atlas.tiles[0].size_in_atlas == Vector2i(1, 1));
	REQUIRE(atlas.tiles[0].physics.size() == 1);
	CHECK(atlas.tiles[0].physics[0].layer == 0);
	REQUIRE(atlas.tiles[0].physics[0].polygons.size() == 1);
	CHECK(atlas.tiles[0].physics[0].polygons[0] == Vector<float>({ -4, -4, 4, -4, 0, 4 }));
	REQUIRE(map.layers.size() == 2);
	CHECK(map.layers[0].name == "ground");
	CHECK(map.layers[0].offset == Vector2(2.5, -3));
	CHECK(map.layers[0].z_index == -2);
	CHECK(map.layers[0].enabled);
	CHECK_FALSE(map.layers[1].enabled);

	TestSpx::TempDirectory files("spx_tilemap");
	REQUIRE(map.save_to_file(files.file("map.json")));
	SpxTileMapData restored;
	REQUIRE(SpxTileMapData::parse_from_file(files.file("map.json"), restored));
	CHECK(JSON::stringify(restored.to_json()) == JSON::stringify(map.to_json()));
	CHECK(restored.from_json(map_document()));
	CHECK(restored.tileset.sources.size() == 1);
	CHECK(restored.layers.size() == 2);
}

TEST_CASE("[SPX] Tilemap defaults and empty collections reset optional data") {
	SpxTileMapData map;
	REQUIRE(map.from_json(Dictionary()));
	CHECK(map.version == 1);
	CHECK(map.tileset.tile_size == Vector2i(16, 16));
	CHECK(map.layers.is_empty());
	SpxPhysicsLayerData physics;
	physics.from_json(Dictionary());
	CHECK(physics.collision_layer == 1);
	CHECK(physics.collision_mask == 1);
	SpxTileData tile;
	tile.from_json(Dictionary());
	CHECK(tile.size_in_atlas == Vector2i(1, 1));
	CHECK_FALSE(tile.to_json().has("physics"));
	Dictionary short_vectors = JSON::parse_string(R"({"atlas_coords":[3],"size_in_atlas":[]})");
	tile.from_json(short_vectors);
	CHECK(tile.atlas_coords == Vector2i());
	CHECK(tile.size_in_atlas == Vector2i());
	SpxTileMapLayerData layer;
	layer.from_json(JSON::parse_string(R"({"offset":[1]})"));
	CHECK(layer.offset == Vector2());
	CHECK(layer.enabled);
	CHECK(layer.tile_map_data_base64.is_empty());
	map.from_json(map_document());
	map.tileset.sources.write[0].tiles.write[0].physics.write[0].from_json(Dictionary());
	CHECK(map.tileset.sources[0].tiles[0].physics[0].polygons.is_empty());
	map.tileset.sources.write[0].tiles.write[0].from_json(Dictionary());
	CHECK(map.tileset.sources[0].tiles[0].physics.is_empty());
	map.tileset.sources.write[0].from_json(Dictionary());
	CHECK(map.tileset.sources[0].tiles.is_empty());
	map.tileset.from_json(Dictionary());
	CHECK(map.tileset.sources.is_empty());
	CHECK(map.tileset.physics_layers.is_empty());
	map.from_json(Dictionary());
	CHECK(map.layers.is_empty());
}

TEST_CASE("[SPX] Tilemap files reject missing malformed and empty documents") {
	TestSpx::TempDirectory files("spx_tilemap_invalid");
	SpxTileMapData map;
	map.name = "unchanged";
	ERR_PRINT_OFF;
	CHECK_FALSE(SpxTileMapData::parse_from_file(files.file("missing.json"), map));
	CHECK_FALSE(map.save_to_file(files.file("missing/map.json")));
	ERR_PRINT_ON;
	for (const String &contents : { String("{"), String(""), String("{}") }) {
		files.write("invalid.json", contents);
		ERR_PRINT_OFF;
		CHECK_FALSE(SpxTileMapData::parse_from_file(files.file("invalid.json"), map));
		ERR_PRINT_ON;
		CHECK(map.name == "unchanged");
	}
}

TEST_CASE("[SceneTree][SPX] Tilemap loader creates atlas collisions cells and owned layers") {
	TestSpx::TempDirectory files("spx_tilemap_loader");
	Ref<Image> image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	REQUIRE(image->save_png(files.file("atlas.png")) == OK);
	SpxTileMapData map;
	REQUIRE(map.from_json(map_document()));
	TileMapLayer *encoded = memnew(TileMapLayer);
	encoded->set_cell(Vector2i(2, -1), 3, Vector2i());
	map.layers.write[0].tile_map_data_base64 = core_bind::Marshalls::get_singleton()->raw_to_base64(encoded->get_tile_map_data_as_array());
	memdelete(encoded);
	REQUIRE(map.save_to_file(files.file("map.json")));
	TestSpx::EngineScene scene;
	auto *loader = scene.engine->get_tilemapparser();
	const CharString path = files.file("map.json").utf8();
	CHECK_FALSE(loader->has_tilemap("meadow"));
	CHECK(loader->get_tilemap_layer_count("meadow") == 0);
	loader->load_tilemap(path.get_data());
	REQUIRE(loader->has_tilemap("meadow"));
	CHECK(loader->get_tilemap_layer_count("meadow") == 2);
	auto *ground = Object::cast_to<TileMapLayer>(scene.root->get_node_or_null(NodePath("ground")));
	REQUIRE(ground != nullptr);
	CHECK(ground->get_position() == Vector2(-13.5, 5.5));
	CHECK(ground->get_z_index() == -2);
	CHECK(ground->get_cell_source_id(Vector2i(2, -1)) == 3);
	Ref<TileSet> tileset = ground->get_tile_set();
	REQUIRE(tileset.is_valid());
	CHECK(tileset->get_tile_size() == Vector2i(8, 8));
	CHECK(tileset->get_physics_layer_collision_layer(0) == 2);
	CHECK(tileset->get_physics_layer_collision_mask(0) == 5);
	Ref<TileSetAtlasSource> atlas = tileset->get_source(3);
	REQUIRE(atlas.is_valid());
	CHECK(atlas->get_margins() == Vector2i(1, 1));
	CHECK(atlas->get_separation() == Vector2i(1, 1));
	TileData *tile = atlas->get_tile_data(Vector2i(), 0);
	REQUIRE(tile != nullptr);
	CHECK(tile->get_collision_polygons_count(0) == 1);
	CHECK(tile->get_collision_polygon_points(0, 0) == Vector<Vector2>({ Vector2(-4, -4), Vector2(4, -4), Vector2(0, 4) }));
	auto *disabled = Object::cast_to<TileMapLayer>(scene.root->get_node_or_null(NodePath("layer")));
	REQUIRE(disabled != nullptr);
	CHECK_FALSE(disabled->is_enabled());
	const int child_count = scene.root->get_child_count();
	loader->load_tilemap(path.get_data());
	CHECK(scene.root->get_child_count() == child_count);
	SUBCASE("Unload is idempotent") {
		loader->unload_tilemap("meadow");
		loader->unload_tilemap("meadow");
	}
	SUBCASE("Reset releases every map") {
		loader->on_reset(0);
	}
	SUBCASE("Destroy releases every map") {
		loader->destroy_all_tilemaps();
	}
	CHECK_FALSE(loader->has_tilemap("meadow"));
	CHECK(loader->get_tilemap_layer_count("meadow") == 0);
	CHECK(ground->is_queued_for_deletion());
	CHECK(disabled->is_queued_for_deletion());
}

} // namespace TestSpxTilemap

#endif // TEST_SPX_TILEMAP_H
