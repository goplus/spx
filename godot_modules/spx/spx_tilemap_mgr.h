/**************************************************************************/
/*  spx_tilemap_mgr.h                                                     */
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

#ifndef SPX_TILEMAP_MGR_H
#define SPX_TILEMAP_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

class SpxDrawTiles;

class SpxTilemapMgr : public SpxBaseMgr {
	SPXCLASS(SpxTilemapMgr, SpxBaseMgr)
public:
	virtual ~SpxTilemapMgr() = default;
	void on_destroy() override;
	void on_reset(int reset_code) override;

private:
	SpxDrawTiles *draw_tiles = nullptr;

public:
	SPX_API void open_draw_tiles_with_size(GdInt tile_size);
	SPX_API void open_draw_tiles();
	SPX_API void set_layer_index(GdInt index);
	SPX_API void set_tile(GdString texture_path, GdBool with_collision);
	SPX_API void set_tile_with_collision_info(GdString texture_path, GdArray collision_points);
	SPX_API void set_layer_offset(GdInt index, GdVec2 offset);
	SPX_API GdVec2 get_layer_offset(GdInt index);
	SPX_API void place_tiles(GdArray positions, GdString texture_path);
	SPX_API void place_tiles_with_layer(GdArray positions, GdString texture_path, GdInt layer_index);
	SPX_API void place_tile(GdVec2 pos, GdString texture_path);
	SPX_API void place_tile_with_layer(GdVec2 pos, GdString texture_path, GdInt layer_index);
	SPX_API void erase_tile(GdVec2 pos);
	SPX_API void erase_tile_with_layer(GdVec2 pos, GdInt layer_index);
	SPX_API GdString get_tile(GdVec2 pos);
	SPX_API GdString get_tile_with_layer(GdVec2 pos, GdInt layer_index);
	SPX_API void close_draw_tiles();
	SPX_API void exit_tilemap_editor_mode();

	template <typename Func>
	void with_draw_tiles(Func f, const String error_msg = "The draw tiles node is null, first open it!!!") {
		if (draw_tiles == nullptr) {
			print_error(error_msg);
			return;
		}
		f();
	}

	template <typename Func>
	void without_draw_tiles(Func f) {
		if (draw_tiles == nullptr) {
			open_draw_tiles();
		}
		f();
	}
};

#endif // SPX_TILEMAP_MGR_H
