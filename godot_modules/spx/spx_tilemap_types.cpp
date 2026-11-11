/**************************************************************************/
/*  spx_tilemap_types.cpp                                                 */
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

#include "spx_tilemap_types.h"

#include "core/io/file_access.h"
#include "core/io/json.h"

// ============================================================================
// Helper functions for parsing arrays
// ============================================================================

static Vector2i _parse_vector2i(const Array &arr) {
	if (arr.size() >= 2) {
		return Vector2i(arr[0], arr[1]);
	}
	return Vector2i();
}

static Vector2 _parse_vector2(const Array &arr) {
	if (arr.size() >= 2) {
		return Vector2(arr[0], arr[1]);
	}
	return Vector2();
}

void SpxPhysicsLayerData::from_json(const Dictionary &dict) {
	collision_layer = dict.get("collision_layer", 1);
	collision_mask = dict.get("collision_mask", 1);
}

void SpxTilePhysicsData::from_json(const Dictionary &dict) {
	layer = dict.get("layer", 0);

	polygons.clear();
	if (dict.has("polygons")) {
		Array polygons_arr = dict["polygons"];
		for (int i = 0; i < polygons_arr.size(); i++) {
			Array poly_arr = polygons_arr[i];
			Vector<float> poly;
			for (int j = 0; j < poly_arr.size(); j++) {
				poly.push_back(poly_arr[j]);
			}
			polygons.push_back(poly);
		}
	}
}

void SpxTileData::from_json(const Dictionary &dict) {
	if (dict.has("atlas_coords")) {
		atlas_coords = _parse_vector2i(dict["atlas_coords"]);
	}

	if (dict.has("size_in_atlas")) {
		size_in_atlas = _parse_vector2i(dict["size_in_atlas"]);
	} else {
		size_in_atlas = Vector2i(1, 1);
	}

	physics.clear();
	if (dict.has("physics")) {
		Array physics_arr = dict["physics"];
		for (int i = 0; i < physics_arr.size(); i++) {
			SpxTilePhysicsData phys_data;
			phys_data.from_json(physics_arr[i]);
			physics.push_back(phys_data);
		}
	}
}

void SpxTileSetSourceData::from_json(const Dictionary &dict) {
	id = dict.get("id", 0);
	type = dict.get("type", "atlas");
	texture = dict.get("texture", "");

	if (dict.has("texture_region_size")) {
		texture_region_size = _parse_vector2i(dict["texture_region_size"]);
	}

	if (dict.has("margins")) {
		margins = _parse_vector2i(dict["margins"]);
	}

	if (dict.has("separation")) {
		separation = _parse_vector2i(dict["separation"]);
	}

	tiles.clear();
	if (dict.has("tiles")) {
		Array tiles_arr = dict["tiles"];
		for (int i = 0; i < tiles_arr.size(); i++) {
			SpxTileData tile_data;
			tile_data.from_json(tiles_arr[i]);
			tiles.push_back(tile_data);
		}
	}
}

void SpxTileSetData::from_json(const Dictionary &dict) {
	if (dict.has("tile_size")) {
		tile_size = _parse_vector2i(dict["tile_size"]);
	}

	tile_shape = dict.get("tile_shape", "square");

	physics_layers.clear();
	if (dict.has("physics_layers")) {
		Array layers_arr = dict["physics_layers"];
		for (int i = 0; i < layers_arr.size(); i++) {
			SpxPhysicsLayerData layer_data;
			layer_data.from_json(layers_arr[i]);
			physics_layers.push_back(layer_data);
		}
	}

	sources.clear();
	if (dict.has("sources")) {
		Array sources_arr = dict["sources"];
		for (int i = 0; i < sources_arr.size(); i++) {
			SpxTileSetSourceData source_data;
			source_data.from_json(sources_arr[i]);
			sources.push_back(source_data);
		}
	}
}

void SpxTileMapLayerData::from_json(const Dictionary &dict) {
	name = dict.get("name", "");
	z_index = dict.get("z_index", 0);
	enabled = dict.get("enabled", true);

	if (dict.has("offset")) {
		offset = _parse_vector2(dict["offset"]);
	}

	tile_map_data_base64 = dict.get("tile_map_data", "");
}

void SpxTileMapData::from_json(const Dictionary &dict) {
	version = dict.get("version", 1);
	name = dict.get("name", "");

	if (dict.has("node_offset")) {
		node_offset = _parse_vector2(dict["node_offset"]);
	}

	if (dict.has("tileset")) {
		tileset.from_json(dict["tileset"]);
	}

	layers.clear();
	if (dict.has("layers")) {
		Array layers_arr = dict["layers"];
		for (int i = 0; i < layers_arr.size(); i++) {
			SpxTileMapLayerData layer_data;
			layer_data.from_json(layers_arr[i]);
			layers.push_back(layer_data);
		}
	}
}

bool SpxTileMapData::parse_from_file(const String &json_path, SpxTileMapData &out_data) {
	Ref<FileAccess> file = FileAccess::open(json_path, FileAccess::READ);
	if (file.is_null()) {
		ERR_PRINT("SpxTileMapData: Failed to open JSON file: " + json_path);
		return false;
	}

	String json_content = file->get_as_text();

	if (json_content.is_empty()) {
		ERR_PRINT("SpxTileMapData: JSON file is empty: " + json_path);
		return false;
	}

	JSON json;
	Error err = json.parse(json_content);
	if (err != OK) {
		ERR_PRINT("SpxTileMapData: Failed to parse JSON: " + json.get_error_message() + " at line " + itos(json.get_error_line()));
		return false;
	}

	Dictionary data = json.get_data();
	if (data.is_empty()) {
		ERR_PRINT("SpxTileMapData: JSON data is empty");
		return false;
	}

	out_data.from_json(data);
	return true;
}
