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

static Array _vector2i_to_array(const Vector2i &v) {
	Array arr;
	arr.push_back(v.x);
	arr.push_back(v.y);
	return arr;
}

static Array _vector2_to_array(const Vector2 &v) {
	Array arr;
	arr.push_back(v.x);
	arr.push_back(v.y);
	return arr;
}

// ============================================================================
// SpxPhysicsLayerData
// ============================================================================

bool SpxPhysicsLayerData::from_json(const Dictionary &dict) {
	collision_layer = dict.get("collision_layer", 1);
	collision_mask = dict.get("collision_mask", 1);
	return true;
}

Dictionary SpxPhysicsLayerData::to_json() const {
	Dictionary dict;
	dict["collision_layer"] = collision_layer;
	dict["collision_mask"] = collision_mask;
	return dict;
}

// ============================================================================
// SpxTilePhysicsData
// ============================================================================

bool SpxTilePhysicsData::from_json(const Dictionary &dict) {
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
	return true;
}

Dictionary SpxTilePhysicsData::to_json() const {
	Dictionary dict;
	dict["layer"] = layer;

	Array polygons_arr;
	for (int i = 0; i < polygons.size(); i++) {
		Array poly_arr;
		const Vector<float> &poly = polygons[i];
		for (int j = 0; j < poly.size(); j++) {
			poly_arr.push_back(poly[j]);
		}
		polygons_arr.push_back(poly_arr);
	}
	dict["polygons"] = polygons_arr;

	return dict;
}

// ============================================================================
// SpxTileData
// ============================================================================

bool SpxTileData::from_json(const Dictionary &dict) {
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
			if (phys_data.from_json(physics_arr[i])) {
				physics.push_back(phys_data);
			}
		}
	}
	return true;
}

Dictionary SpxTileData::to_json() const {
	Dictionary dict;
	dict["atlas_coords"] = _vector2i_to_array(atlas_coords);
	dict["size_in_atlas"] = _vector2i_to_array(size_in_atlas);

	if (physics.size() > 0) {
		Array physics_arr;
		for (int i = 0; i < physics.size(); i++) {
			physics_arr.push_back(physics[i].to_json());
		}
		dict["physics"] = physics_arr;
	}

	return dict;
}

// ============================================================================
// SpxTileSetSourceData
// ============================================================================

bool SpxTileSetSourceData::from_json(const Dictionary &dict) {
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
			if (tile_data.from_json(tiles_arr[i])) {
				tiles.push_back(tile_data);
			}
		}
	}
	return true;
}

Dictionary SpxTileSetSourceData::to_json() const {
	Dictionary dict;
	dict["id"] = id;
	dict["type"] = type;
	dict["texture"] = texture;
	dict["texture_region_size"] = _vector2i_to_array(texture_region_size);
	dict["margins"] = _vector2i_to_array(margins);
	dict["separation"] = _vector2i_to_array(separation);

	Array tiles_arr;
	for (int i = 0; i < tiles.size(); i++) {
		tiles_arr.push_back(tiles[i].to_json());
	}
	dict["tiles"] = tiles_arr;

	return dict;
}

// ============================================================================
// SpxTileSetData
// ============================================================================

bool SpxTileSetData::from_json(const Dictionary &dict) {
	if (dict.has("tile_size")) {
		tile_size = _parse_vector2i(dict["tile_size"]);
	}

	tile_shape = dict.get("tile_shape", "square");

	physics_layers.clear();
	if (dict.has("physics_layers")) {
		Array layers_arr = dict["physics_layers"];
		for (int i = 0; i < layers_arr.size(); i++) {
			SpxPhysicsLayerData layer_data;
			if (layer_data.from_json(layers_arr[i])) {
				physics_layers.push_back(layer_data);
			}
		}
	}

	sources.clear();
	if (dict.has("sources")) {
		Array sources_arr = dict["sources"];
		for (int i = 0; i < sources_arr.size(); i++) {
			SpxTileSetSourceData source_data;
			if (source_data.from_json(sources_arr[i])) {
				sources.push_back(source_data);
			}
		}
	}
	return true;
}

Dictionary SpxTileSetData::to_json() const {
	Dictionary dict;
	dict["tile_size"] = _vector2i_to_array(tile_size);
	dict["tile_shape"] = tile_shape;

	Array layers_arr;
	for (int i = 0; i < physics_layers.size(); i++) {
		layers_arr.push_back(physics_layers[i].to_json());
	}
	dict["physics_layers"] = layers_arr;

	Array sources_arr;
	for (int i = 0; i < sources.size(); i++) {
		sources_arr.push_back(sources[i].to_json());
	}
	dict["sources"] = sources_arr;

	return dict;
}

// ============================================================================
// SpxTileMapLayerData
// ============================================================================

bool SpxTileMapLayerData::from_json(const Dictionary &dict) {
	name = dict.get("name", "");
	z_index = dict.get("z_index", 0);
	enabled = dict.get("enabled", true);

	if (dict.has("offset")) {
		offset = _parse_vector2(dict["offset"]);
	}

	tile_map_data_base64 = dict.get("tile_map_data", "");
	return true;
}

Dictionary SpxTileMapLayerData::to_json() const {
	Dictionary dict;
	dict["name"] = name;
	dict["z_index"] = z_index;
	dict["offset"] = _vector2_to_array(offset);
	dict["enabled"] = enabled;
	dict["tile_map_data"] = tile_map_data_base64;
	return dict;
}

// ============================================================================
// SpxTileMapData
// ============================================================================

bool SpxTileMapData::from_json(const Dictionary &dict) {
	version = dict.get("version", 1);
	name = dict.get("name", "");

	if (dict.has("node_offset")) {
		node_offset = _parse_vector2(dict["node_offset"]);
	}

	if (dict.has("tileset")) {
		if (!tileset.from_json(dict["tileset"])) {
			return false;
		}
	}

	layers.clear();
	if (dict.has("layers")) {
		Array layers_arr = dict["layers"];
		for (int i = 0; i < layers_arr.size(); i++) {
			SpxTileMapLayerData layer_data;
			if (layer_data.from_json(layers_arr[i])) {
				layers.push_back(layer_data);
			}
		}
	}
	return true;
}

Dictionary SpxTileMapData::to_json() const {
	Dictionary dict;
	dict["version"] = version;
	dict["name"] = name;
	dict["node_offset"] = _vector2_to_array(node_offset);
	dict["tileset"] = tileset.to_json();

	Array layers_arr;
	for (int i = 0; i < layers.size(); i++) {
		layers_arr.push_back(layers[i].to_json());
	}
	dict["layers"] = layers_arr;

	return dict;
}

bool SpxTileMapData::parse_from_file(const String &json_path, SpxTileMapData &out_data) {
	Ref<FileAccess> file = FileAccess::open(json_path, FileAccess::READ);
	if (file.is_null()) {
		ERR_PRINT("SpxTileMapData: Failed to open JSON file: " + json_path);
		return false;
	}

	String json_content;
	while (!file->eof_reached()) {
		String line = file->get_line();
		json_content += line + "\n";
	}
	file->close();

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

	return out_data.from_json(data);
}

bool SpxTileMapData::save_to_file(const String &json_path) const {
	Ref<FileAccess> file = FileAccess::open(json_path, FileAccess::WRITE);
	if (file.is_null()) {
		ERR_PRINT("SpxTileMapData: Failed to create JSON file: " + json_path);
		return false;
	}

	Dictionary dict = to_json();
	String json_str = JSON::stringify(dict, "\t");
	file->store_string(json_str);
	file->close();

	return true;
}
