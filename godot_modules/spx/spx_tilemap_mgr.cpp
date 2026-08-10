/**************************************************************************/
/*  spx_tilemap_mgr.cpp                                                   */
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

#include "spx_tilemap_mgr.h"

#include "spx_draw_tiles.h"

void SpxTilemapMgr::on_destroy() {
	SpxBaseMgr::on_destroy();
}

void SpxTilemapMgr::on_reset(int reset_code) {
	close_draw_tiles();
}

void SpxTilemapMgr::open_draw_tiles() {
	open_draw_tiles_with_size(16); // default tile_size = 16
}

void SpxTilemapMgr::set_layer_offset(GdInt index, GdVec2 offset) {
	if (draw_tiles == nullptr) {
		print_error("The draw tiles not exist");
		return;
	}
	draw_tiles->set_layer_offset_spx(index, offset);
}
GdVec2 SpxTilemapMgr::get_layer_offset(GdInt index) {
	if (draw_tiles == nullptr) {
		print_error("The draw tiles not exist");
		return GdVec2();
	}
	return draw_tiles->get_layer_offset_spx(index);
}
void SpxTilemapMgr::open_draw_tiles_with_size(GdInt tile_size) {
	if (draw_tiles != nullptr) {
		return;
	}
	draw_tiles = memnew(SpxDrawTiles);
	draw_tiles->set_tile_size(tile_size);
	get_spx_root()->add_child(draw_tiles);
}

void SpxTilemapMgr::set_layer_index(GdInt index) {
	without_draw_tiles([&]() {
		draw_tiles->set_layer_index_spx(index);
	});
}
void SpxTilemapMgr::set_tile(GdString texture_path, GdBool with_collision) {
	without_draw_tiles([&]() {
		draw_tiles->set_tile_texture_spx(texture_path, with_collision ? &SpxDrawTiles::default_collision_rect : &SpxDrawTiles::no_collision_array);
	});
}
void SpxTilemapMgr::set_tile_with_collision_info(GdString texture_path, GdArray collision_points) {
	Vector<Vector2> points = {};
	auto len = collision_points == nullptr ? 0 : collision_points->size;
	for (int i = 0; i + 1 < len; i += 2) {
		auto x = *(SpxBaseMgr::get_array<real_t>(collision_points, i));
		auto y = *(SpxBaseMgr::get_array<real_t>(collision_points, i + 1));
		points.append(Vector2(x, y));
	}
	without_draw_tiles([&]() {
		draw_tiles->set_tile_texture_spx(texture_path, &points);
	});
}

void SpxTilemapMgr::place_tiles(GdArray positions, GdString texture_path) {
	without_draw_tiles([&]() {
		draw_tiles->place_tiles_spx(positions, texture_path);
	});
}
void SpxTilemapMgr::place_tiles_with_layer(GdArray positions, GdString texture_path, GdInt layer_index) {
	without_draw_tiles([&]() {
		draw_tiles->place_tiles_spx(positions, texture_path, layer_index);
	});
}

void SpxTilemapMgr::place_tile(GdVec2 pos, GdString texture_path) {
	without_draw_tiles([&]() {
		draw_tiles->place_tile_spx(pos, texture_path);
	});
}

void SpxTilemapMgr::place_tile_with_layer(GdVec2 pos, GdString texture_path, GdInt layer_index) {
	without_draw_tiles([&]() {
		draw_tiles->place_tile_spx(pos, texture_path, layer_index);
	});
}

void SpxTilemapMgr::erase_tile(GdVec2 pos) {
	with_draw_tiles([&]() {
		draw_tiles->erase_tile_spx(pos);
	});
}

void SpxTilemapMgr::erase_tile_with_layer(GdVec2 pos, GdInt layer_index) {
	with_draw_tiles([&]() {
		draw_tiles->erase_tile_spx(pos, layer_index);
	});
}

GdString SpxTilemapMgr::get_tile(GdVec2 pos) {
	if (draw_tiles != nullptr) {
		return draw_tiles->get_tile_spx(pos);
	}

	return SpxReturnStr("");
}

GdString SpxTilemapMgr::get_tile_with_layer(GdVec2 pos, GdInt layer_index) {
	if (draw_tiles != nullptr) {
		return draw_tiles->get_tile_spx(pos, layer_index);
	}

	return SpxReturnStr("");
}

void SpxTilemapMgr::close_draw_tiles() {
	if (draw_tiles != nullptr) {
		draw_tiles->queue_free();
		draw_tiles = nullptr;
	}
}

void SpxTilemapMgr::exit_tilemap_editor_mode() {
	if (draw_tiles != nullptr) {
		draw_tiles->exit_editor_mode();
		draw_tiles = nullptr;
	}
}
