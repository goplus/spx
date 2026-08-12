/**************************************************************************/
/*  spx_tilemapparser_mgr.h                                              */
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

#ifndef SPX_TILEMAP_PARSER_MGR_H
#define SPX_TILEMAP_PARSER_MGR_H

#include "gdextension_spx_ext.h"
#include "scene/resources/2d/tile_set.h"
#include "spx_base_mgr.h"
#include "spx_tilemap_types.h"

class TileMapLayer;
class TileSetAtlasSource;
class TileData;

// Manager for loading TileMap resources from JSON files
// Provides runtime API for loading TileMaps without Godot's import process
// Uses SpxResMgr for texture loading (SPX direct loading)
//
// Architecture:
//   JSON -> SpxTileMapData (via from_json) -> Godot Objects (via this manager)
//   Godot Objects -> SpxTileMapData (via to_json) -> JSON (for export plugin)
//
class SpxTilemapparserMgr : public SpxBaseMgr {
	SPXCLASS(SpxTilemapparserMgr, SpxBaseMgr)

public:
	virtual ~SpxTilemapparserMgr() = default;

private:
	// Cache of loaded TileSets by tilemap name
	HashMap<String, Ref<TileSet>> tileset_cache;

	// Cache of loaded TileMapLayers by tilemap name
	HashMap<String, Vector<TileMapLayer *>> tilemap_layers;

private:
	// Godot object creation methods (using parsed data structures)
	Ref<TileSet> _create_tileset(const SpxTileSetData &data, const String &base_path);
	void _create_atlas_source(Ref<TileSet> tileset, const SpxTileSetSourceData &data, const String &base_path);
	void _setup_tile_physics(TileData *tile_data, const SpxTileData &data);
	TileMapLayer *_create_tilemap_layer(const SpxTileMapLayerData &data, Ref<TileSet> tileset, const Vector2 &node_offset);

	// Helper methods
	String _get_base_path(const String &json_path);
	Vector<uint8_t> _base64_decode(const String &base64_str);

public:
	// Lifecycle methods
	void on_awake() override;
	void on_destroy() override;
	void on_reset(int reset_code) override;

public:
	// Main API
	SPX_API void load_tilemap(GdString json_path);
	SPX_API void unload_tilemap(GdString name);
	SPX_API void destroy_all_tilemaps();

	// Query API
	SPX_API GdBool has_tilemap(GdString name);
	SPX_API GdInt get_tilemap_layer_count(GdString name);
};

#endif // SPX_TILEMAP_PARSER_MGR_H
