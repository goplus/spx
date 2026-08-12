/**************************************************************************/
/*  spx_tilemap_types.h                                                   */
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

#ifndef SPX_TILEMAP_TYPES_H
#define SPX_TILEMAP_TYPES_H

#include "core/math/vector2.h"
#include "core/math/vector2i.h"
#include "core/string/ustring.h"
#include "core/templates/vector.h"
#include "core/variant/array.h"
#include "core/variant/dictionary.h"

// ============================================================================
// Physics layer data
// ============================================================================
struct SpxPhysicsLayerData {
	uint32_t collision_layer = 1;
	uint32_t collision_mask = 1;

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// Tile physics data (collision polygons)
// ============================================================================
struct SpxTilePhysicsData {
	int layer = 0;
	Vector<Vector<float>> polygons; // Each polygon is a flat array [x1,y1,x2,y2,...]

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// Single tile data
// ============================================================================
struct SpxTileData {
	Vector2i atlas_coords;
	Vector2i size_in_atlas = Vector2i(1, 1);
	Vector<SpxTilePhysicsData> physics;

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// TileSet source data (Atlas type)
// ============================================================================
struct SpxTileSetSourceData {
	int id = 0;
	String type = "atlas";
	String texture; // Relative path
	Vector2i texture_region_size;
	Vector2i margins;
	Vector2i separation;
	Vector<SpxTileData> tiles;

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// TileSet data
// ============================================================================
struct SpxTileSetData {
	Vector2i tile_size = Vector2i(16, 16);
	String tile_shape = "square";
	Vector<SpxPhysicsLayerData> physics_layers;
	Vector<SpxTileSetSourceData> sources;

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// TileMapLayer data
// ============================================================================
struct SpxTileMapLayerData {
	String name;
	int z_index = 0;
	Vector2 offset;
	bool enabled = true;
	String tile_map_data_base64; // Base64 encoded binary data

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;
};

// ============================================================================
// Complete TileMap data (root structure)
// ============================================================================
struct SpxTileMapData {
	int version = 1;
	String name;
	Vector2 node_offset; // Center offset in pixels for positioning tilemap at origin
	SpxTileSetData tileset;
	Vector<SpxTileMapLayerData> layers;

	bool from_json(const Dictionary &dict);
	Dictionary to_json() const;

	// Convenience method: parse from file
	static bool parse_from_file(const String &json_path, SpxTileMapData &out_data);
	// Convenience method: save to file
	bool save_to_file(const String &json_path) const;
};

#endif // SPX_TILEMAP_TYPES_H