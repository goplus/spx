/**************************************************************************/
/*  test_spx_tilemap_types.h                                              */
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

#ifndef TEST_SPX_TILEMAP_TYPES_H
#define TEST_SPX_TILEMAP_TYPES_H

#include "../spx_tilemap_types.h"

#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"

namespace TestSpxTileMapTypes {

static Array vec2(real_t p_x, real_t p_y) {
	Array result;
	result.push_back(p_x);
	result.push_back(p_y);
	return result;
}

static Array one(const Variant &p_value) {
	Array result;
	result.push_back(p_value);
	return result;
}

static Dictionary make_tilemap_data() {
	Dictionary physics_layer;
	physics_layer["collision_layer"] = 4;
	physics_layer["collision_mask"] = 7;

	Array polygon;
	polygon.push_back(0.0);
	polygon.push_back(1.5);
	polygon.push_back(2.0);
	polygon.push_back(3.5);
	Dictionary tile_physics;
	tile_physics["layer"] = 2;
	tile_physics["polygons"] = one(polygon);

	Dictionary tile;
	tile["atlas_coords"] = vec2(3, 5);
	tile["size_in_atlas"] = vec2(2, 4);
	tile["physics"] = one(tile_physics);

	Dictionary source;
	source["id"] = 9;
	source["type"] = "atlas";
	source["texture"] = "tiles.png";
	source["texture_region_size"] = vec2(16, 24);
	source["margins"] = vec2(1, 2);
	source["separation"] = vec2(3, 4);
	source["tiles"] = one(tile);

	Dictionary tileset;
	tileset["tile_size"] = vec2(16, 24);
	tileset["tile_shape"] = "isometric";
	tileset["physics_layers"] = one(physics_layer);
	tileset["sources"] = one(source);

	Dictionary layer;
	layer["name"] = "foreground";
	layer["z_index"] = 6;
	layer["offset"] = vec2(2.5, -3.5);
	layer["enabled"] = false;
	layer["tile_map_data"] = "AQID";

	Dictionary result;
	result["version"] = 3;
	result["name"] = "level";
	result["node_offset"] = vec2(12.5, -8.25);
	result["tileset"] = tileset;
	result["layers"] = one(layer);
	return result;
}

TEST_CASE("[SPX] Tile map data round trips every nested field") {
	SpxTileMapData data;
	REQUIRE(data.from_json(make_tilemap_data()));
	CHECK(data.version == 3);
	CHECK(data.name == "level");
	CHECK(data.node_offset == Vector2(12.5, -8.25));
	CHECK(data.tileset.tile_size == Vector2i(16, 24));
	CHECK(data.tileset.tile_shape == "isometric");
	REQUIRE(data.tileset.physics_layers.size() == 1);
	CHECK(data.tileset.physics_layers[0].collision_layer == 4);
	CHECK(data.tileset.physics_layers[0].collision_mask == 7);
	REQUIRE(data.tileset.sources.size() == 1);
	const SpxTileSetSourceData &source = data.tileset.sources[0];
	CHECK(source.id == 9);
	CHECK(source.type == "atlas");
	CHECK(source.texture == "tiles.png");
	CHECK(source.texture_region_size == Vector2i(16, 24));
	CHECK(source.margins == Vector2i(1, 2));
	CHECK(source.separation == Vector2i(3, 4));
	REQUIRE(source.tiles.size() == 1);
	CHECK(source.tiles[0].atlas_coords == Vector2i(3, 5));
	CHECK(source.tiles[0].size_in_atlas == Vector2i(2, 4));
	REQUIRE(source.tiles[0].physics.size() == 1);
	CHECK(source.tiles[0].physics[0].layer == 2);
	REQUIRE(source.tiles[0].physics[0].polygons.size() == 1);
	CHECK(source.tiles[0].physics[0].polygons[0].size() == 4);
	REQUIRE(data.layers.size() == 1);
	CHECK(data.layers[0].name == "foreground");
	CHECK(data.layers[0].z_index == 6);
	CHECK(data.layers[0].offset == Vector2(2.5, -3.5));
	CHECK_FALSE(data.layers[0].enabled);
	CHECK(data.layers[0].tile_map_data_base64 == "AQID");

	SpxTileMapData copy;
	REQUIRE(copy.from_json(data.to_json()));
	CHECK(copy.to_json() == data.to_json());
}

TEST_CASE("[SPX] Tile map data applies defaults and ignores short vectors") {
	Dictionary tile;
	tile["atlas_coords"] = one(9);
	Dictionary source;
	source["tiles"] = one(tile);
	Dictionary tileset;
	tileset["sources"] = one(source);
	Dictionary input;
	input["node_offset"] = one(4.0);
	input["tileset"] = tileset;
	input["layers"] = one(Dictionary());

	SpxTileMapData data;
	REQUIRE(data.from_json(input));
	CHECK(data.version == 1);
	CHECK(data.name.is_empty());
	CHECK(data.node_offset == Vector2());
	CHECK(data.tileset.tile_size == Vector2i(16, 16));
	CHECK(data.tileset.tile_shape == "square");
	REQUIRE(data.tileset.sources.size() == 1);
	REQUIRE(data.tileset.sources[0].tiles.size() == 1);
	CHECK(data.tileset.sources[0].tiles[0].atlas_coords == Vector2i());
	CHECK(data.tileset.sources[0].tiles[0].size_in_atlas == Vector2i(1, 1));
	REQUIRE(data.layers.size() == 1);
	CHECK(data.layers[0].enabled);
	CHECK(data.to_json().has("layers"));
}

TEST_CASE("[SPX] Tile map files report invalid input and preserve valid data") {
	const String valid_path = TestUtils::get_temp_path("spx_tilemap.json");
	const String empty_path = TestUtils::get_temp_path("spx_tilemap_empty.json");
	const String invalid_path = TestUtils::get_temp_path("spx_tilemap_invalid.json");
	const String object_path = TestUtils::get_temp_path("spx_tilemap_object.json");
	const String missing_path = TestUtils::get_temp_path("spx_tilemap_missing.json");
	DirAccess::remove_absolute(missing_path);

	SpxTileMapData original;
	REQUIRE(original.from_json(make_tilemap_data()));
	REQUIRE(original.save_to_file(valid_path));
	SpxTileMapData loaded;
	REQUIRE(SpxTileMapData::parse_from_file(valid_path, loaded));
	CHECK(loaded.to_json() == original.to_json());

	auto write = [](const String &p_path, const String &p_text) {
		Ref<FileAccess> file = FileAccess::open(p_path, FileAccess::WRITE);
		REQUIRE(file.is_valid());
		file->store_string(p_text);
	};
	write(empty_path, "");
	write(invalid_path, "{");
	write(object_path, "{}");
	CHECK_FALSE(SpxTileMapData::parse_from_file(missing_path, loaded));
	CHECK_FALSE(SpxTileMapData::parse_from_file(empty_path, loaded));
	CHECK_FALSE(SpxTileMapData::parse_from_file(invalid_path, loaded));
	CHECK_FALSE(SpxTileMapData::parse_from_file(object_path, loaded));
	CHECK_FALSE(original.save_to_file(missing_path.path_join("child.json")));

	for (const String &path : { valid_path, empty_path, invalid_path, object_path }) {
		DirAccess::remove_absolute(path);
	}
}

} // namespace TestSpxTileMapTypes

#endif // TEST_SPX_TILEMAP_TYPES_H
