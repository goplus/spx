/**************************************************************************/
/*  spx_tilemapparser_mgr.cpp                                            */
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

#include "spx_tilemapparser_mgr.h"

#include "core/core_bind.h"
#include "core/io/marshalls.h"
#include "scene/2d/tile_map_layer.h"
#include "scene/resources/2d/tile_set.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"

// ============================================================================
// Lifecycle Methods
// ============================================================================

void SpxTilemapparserMgr::on_awake() {
	SpxBaseMgr::on_awake();
}

void SpxTilemapparserMgr::on_destroy() {
	SpxBaseMgr::on_destroy();
	destroy_all_tilemaps();
	tileset_cache.clear();
}

void SpxTilemapparserMgr::on_reset(int reset_code) {
	destroy_all_tilemaps();
	tileset_cache.clear();
}

// ============================================================================
// Main API
// ============================================================================

void SpxTilemapparserMgr::load_tilemap(GdString json_path) {
	String path = SpxStr(json_path);
	String engine_path = resMgr->_to_engine_path(path);

	// Stage 1: Parse JSON -> Data Structure (handled by SpxTileMapData)
	SpxTileMapData data;
	if (!SpxTileMapData::parse_from_file(engine_path, data)) {
		print_error("SpxTilemapparserMgr: Failed to parse tilemap JSON: " + engine_path);
		return;
	}

	// Get tilemap name from JSON, or derive from path
	// Priority: 1. JSON name field  2. Parent directory name  3. Filename basename
	String tilemap_name = data.name;
	if (tilemap_name.is_empty()) {
		// Use parent directory name as tilemap name
		// e.g., "tilemaps/map2/tilemap.json" -> "map2"
		String dir_path = path.get_base_dir();
		tilemap_name = dir_path.get_file();
		if (tilemap_name.is_empty()) {
			// Fallback to filename basename if no parent directory
			tilemap_name = path.get_file().get_basename();
		}
	}

	// Check if already loaded
	if (tilemap_layers.has(tilemap_name)) {
		print_line("SpxTilemapparserMgr: Tilemap already loaded: " + tilemap_name);
		return;
	}

	// Get base path for relative texture paths
	String base_path = _get_base_path(engine_path);

	// Stage 2: Data Structure -> Godot Objects (handled by this manager)
	Ref<TileSet> tileset = _create_tileset(data.tileset, base_path);
	if (tileset.is_null()) {
		print_error("SpxTilemapparserMgr: Failed to create TileSet for: " + tilemap_name);
		return;
	}

	// Cache tileset
	tileset_cache[tilemap_name] = tileset;

	// Create TileMapLayers
	Vector<TileMapLayer *> layers;
	for (int i = 0; i < data.layers.size(); i++) {
		TileMapLayer *layer = _create_tilemap_layer(data.layers[i], tileset, data.node_offset);
		if (layer != nullptr) {
			// Add to scene tree
			Node *spx_root = get_spx_root();
			if (spx_root != nullptr) {
				spx_root->add_child(layer);
			}
			layers.push_back(layer);
		}
	}

	// Store layers in cache
	tilemap_layers[tilemap_name] = layers;
}

void SpxTilemapparserMgr::unload_tilemap(GdString name) {
	String tilemap_name = SpxStr(name);

	// Destroy layers
	if (tilemap_layers.has(tilemap_name)) {
		Vector<TileMapLayer *> &layers = tilemap_layers[tilemap_name];
		for (TileMapLayer *layer : layers) {
			if (layer != nullptr) {
				layer->queue_free();
			}
		}
		tilemap_layers.erase(tilemap_name);
	}

	// Remove from tileset cache
	tileset_cache.erase(tilemap_name);
}

void SpxTilemapparserMgr::destroy_all_tilemaps() {
	// Destroy all layers
	for (KeyValue<String, Vector<TileMapLayer *>> &kv : tilemap_layers) {
		for (TileMapLayer *layer : kv.value) {
			if (layer != nullptr) {
				layer->queue_free();
			}
		}
	}
	tilemap_layers.clear();
	tileset_cache.clear();
}

// ============================================================================
// Query API
// ============================================================================

GdBool SpxTilemapparserMgr::has_tilemap(GdString name) {
	String tilemap_name = SpxStr(name);
	return tilemap_layers.has(tilemap_name);
}

GdInt SpxTilemapparserMgr::get_tilemap_layer_count(GdString name) {
	String tilemap_name = SpxStr(name);
	if (tilemap_layers.has(tilemap_name)) {
		return tilemap_layers[tilemap_name].size();
	}
	return 0;
}

// ============================================================================
// Helper Methods
// ============================================================================

String SpxTilemapparserMgr::_get_base_path(const String &json_path) {
	int last_slash = json_path.rfind("/");
	if (last_slash == -1) {
		last_slash = json_path.rfind("\\");
	}

	if (last_slash != -1) {
		return json_path.substr(0, last_slash);
	}
	return "";
}

Vector<uint8_t> SpxTilemapparserMgr::_base64_decode(const String &base64_str) {
	PackedByteArray decoded = core_bind::Marshalls::get_singleton()->base64_to_raw(base64_str);

	Vector<uint8_t> result;
	result.resize(decoded.size());
	for (int i = 0; i < decoded.size(); i++) {
		result.write[i] = decoded[i];
	}

	return result;
}

// ============================================================================
// Godot Object Creation Methods
// ============================================================================

Ref<TileSet> SpxTilemapparserMgr::_create_tileset(const SpxTileSetData &data, const String &base_path) {
	Ref<TileSet> tileset;
	tileset.instantiate();

	// Set tile size
	tileset->set_tile_size(data.tile_size);

	// Create physics layers
	for (int i = 0; i < data.physics_layers.size(); i++) {
		const SpxPhysicsLayerData &layer_data = data.physics_layers[i];
		tileset->add_physics_layer();
		tileset->set_physics_layer_collision_layer(i, layer_data.collision_layer);
		tileset->set_physics_layer_collision_mask(i, layer_data.collision_mask);
	}

	// Create sources
	for (int i = 0; i < data.sources.size(); i++) {
		_create_atlas_source(tileset, data.sources[i], base_path);
	}

	return tileset;
}

void SpxTilemapparserMgr::_create_atlas_source(Ref<TileSet> tileset, const SpxTileSetSourceData &data, const String &base_path) {
	// Only support "atlas" type
	if (data.type != "atlas") {
		print_line("SpxTilemapparserMgr: Skipping non-atlas source type: " + data.type);
		return;
	}

	// Load texture using SPX resource manager (direct loading)
	if (data.texture.is_empty()) {
		print_error("SpxTilemapparserMgr: Source " + itos(data.id) + " has no texture");
		return;
	}

	// Resolve relative path
	String full_texture_path = base_path.is_empty() ? data.texture : base_path.path_join(data.texture);
	Ref<Texture2D> texture = resMgr->load_texture(full_texture_path, true);
	if (texture.is_null()) {
		print_error("SpxTilemapparserMgr: Failed to load texture: " + full_texture_path);
		return;
	}

	// Create atlas source
	Ref<TileSetAtlasSource> atlas;
	atlas.instantiate();
	atlas->set_texture(texture);
	atlas->set_texture_region_size(data.texture_region_size);
	atlas->set_margins(data.margins);
	atlas->set_separation(data.separation);

	// Add source to tileset FIRST - this is required so that TileData can access
	// the TileSet's physics layers configuration. When add_source is called,
	// it triggers set_tile_set() which initializes the physics array in TileData.
	tileset->add_source(atlas, data.id);

	// Create tiles (now TileData will have correct physics layer count)
	for (int i = 0; i < data.tiles.size(); i++) {
		const SpxTileData &tile_data = data.tiles[i];

		// Create the tile
		atlas->create_tile(tile_data.atlas_coords, tile_data.size_in_atlas);

		// Get tile data for physics setup
		TileData *td = atlas->get_tile_data(tile_data.atlas_coords, 0);
		if (td != nullptr) {
			_setup_tile_physics(td, tile_data);
		}
	}
}

void SpxTilemapparserMgr::_setup_tile_physics(TileData *tile_data, const SpxTileData &data) {
	for (int i = 0; i < data.physics.size(); i++) {
		const SpxTilePhysicsData &phys_data = data.physics[i];
		int layer_id = phys_data.layer;

		// Add collision polygons
		for (int p = 0; p < phys_data.polygons.size(); p++) {
			const Vector<float> &polygon_flat = phys_data.polygons[p];

			// Convert flat array to Vector<Vector2>
			Vector<Vector2> polygon;
			int point_count = polygon_flat.size() / 2;
			polygon.resize(point_count);
			for (int k = 0; k < point_count; k++) {
				polygon.write[k] = Vector2(polygon_flat[k * 2], polygon_flat[k * 2 + 1]);
			}

			if (polygon.size() >= 3) {
				// Ensure enough polygons exist
				while (tile_data->get_collision_polygons_count(layer_id) <= p) {
					tile_data->add_collision_polygon(layer_id);
				}
				tile_data->set_collision_polygon_points(layer_id, p, polygon);
			}
		}
	}
}

TileMapLayer *SpxTilemapparserMgr::_create_tilemap_layer(const SpxTileMapLayerData &data, Ref<TileSet> tileset, const Vector2 &node_offset) {
	TileMapLayer *layer = memnew(TileMapLayer);

	// Set layer name
	layer->set_name(data.name.is_empty() ? "layer" : data.name);

	// Set z_index
	layer->set_z_index(data.z_index);

	// Set offset (position) - combine layer offset with tilemap node offset for centering
	layer->set_position(data.offset + node_offset);

	// Set enabled
	layer->set_enabled(data.enabled);

	// Set tile set
	layer->set_tile_set(tileset);

	// Parse tile_map_data (Base64 encoded binary data)
	if (!data.tile_map_data_base64.is_empty()) {
		Vector<uint8_t> bytes = _base64_decode(data.tile_map_data_base64);
		if (bytes.size() > 0) {
			layer->set_tile_map_data_from_array(bytes);
		}
	}

	return layer;
}
