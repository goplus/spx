/**************************************************************************/
/*  test_spx_draw_tiles.h                                               */
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

#ifndef TEST_SPX_DRAW_TILES_H
#define TEST_SPX_DRAW_TILES_H
#include "../spx_draw_tiles.h"
#include "../spx_tilemap_mgr.h"
#include "spx_test_fixtures.h"
namespace TestSpxDrawTiles {
TEST_CASE("[SceneTree][SPX] Tile editing preserves layers, collision geometry and undo history") {
	TestSpx::TempDirectory files("spx_draw_tiles");
	Ref<Image> image = Image::create_empty(32, 32, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	REQUIRE(image->save_png(files.file("tile.png")) == OK);
	CharString path = files.file("tile.png").utf8();
	TestSpx::EngineScene scene;
	auto *tiles = memnew(SpxDrawTiles);
	tiles->set_tile_size(16);
	scene.root->add_child(tiles);
	tiles->set_layer_index_spx(2);
	tiles->enter_editor_mode();
	tiles->undo();
	tiles->redo();
	tiles->place_tile_spx(Vector2(8, -8), path.get_data());
	auto *layer = Object::cast_to<TileMapLayer>(tiles->get_child(0));
	REQUIRE(layer);
	CHECK(layer->get_z_index() == 2);
	CHECK(layer->get_used_cells().size() == 1);
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(8, -8))) == files.file("tile.png"));
	const int source_id = layer->get_cell_source_id(Vector2i(0, 0));
	REQUIRE(source_id != TileSet::INVALID_SOURCE);
	Ref<TileSetAtlasSource> source = layer->get_tile_set()->get_source(source_id);
	REQUIRE(source.is_valid());
	auto *data = source->get_tile_data(Vector2i(), 0);
	REQUIRE(data);
	REQUIRE(data->get_collision_polygons_count(0) == 1);
	CHECK(data->get_collision_polygon_points(0, 0) == Vector<Vector2>({ Vector2(-8, -8), Vector2(8, -8), Vector2(8, 8), Vector2(-8, 8) }));
	tiles->undo();
	CHECK(layer->get_cell_source_id(Vector2i()) == TileSet::INVALID_SOURCE);
	tiles->redo();
	CHECK(layer->get_cell_source_id(Vector2i()) == source_id);
	tiles->erase_tile_spx(Vector2(8, -8));
	CHECK(layer->get_used_cells().is_empty());
	tiles->undo();
	CHECK(layer->get_used_cells().size() == 1);
	tiles->redo();
	CHECK(layer->get_used_cells().is_empty());
	tiles->place_tile_spx(Vector2(24, -8), path.get_data());
	tiles->undo();
	tiles->place_tile_spx(Vector2(40, -8), path.get_data());
	tiles->redo(); // A new edit invalidates the redo branch.
	CHECK(layer->get_used_cells().size() == 1);
	CHECK(layer->get_cell_source_id(Vector2i(2, 0)) == source_id);
	tiles->set_layer_offset_spx(2, Vector2(16, 32));
	CHECK(layer->get_position() == Vector2(16, -32));
	CHECK(tiles->get_layer_offset_spx(2) == Vector2(16, 32));
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(56, 24), 2)) == files.file("tile.png"));
	CHECK(tiles->get_layer_offset_spx(99) == Vector2());
	tiles->set_layer_offset_spx(99, Vector2(1, 2));
	tiles->place_tile_spx(Vector2(56, 24), path.get_data(), 3);
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(56, 24), -1)) == files.file("tile.png"));
	tiles->erase_tile_spx(Vector2(56, 24), -1);
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(56, 24), 2)).is_empty());
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(56, 24), 3)).is_empty());
	tiles->erase_tile_spx(Vector2(), 99);
	auto tile_set = layer->get_tile_set();
	tiles->clear_all_layers();
	CHECK(tile_set->get_source_count() == 0);
	CHECK(tiles->get_child_count() == 0);
	tiles->undo();
	tiles->redo();
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(), -1)).is_empty());
	tiles->set_layer_index(0);
	tiles->set_tile_size(8);
	CHECK(Object::cast_to<TileMapLayer>(tiles->get_child(0))->get_tile_set()->get_tile_size() == Vector2i(8, 8));
	tiles->place_tile_spx(Vector2(4, -4), path.get_data());
	CHECK(tile_set->get_source_count() == 1);
	CHECK(tile_set->get_source(1) != source);
	tiles->exit_editor_mode();
	tiles->undo(); // Runtime mode cannot alter editor history.
	CHECK_FALSE(TestSpx::take_string(tiles->get_tile_spx(Vector2(4, -4))).is_empty());
}

TEST_CASE("[SceneTree][SPX] Tilemap manager loads textures lazily and places batches across layers") {
	TestSpx::TempDirectory files("spx_tile_batches");
	Ref<Image> image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	image->fill(Color(0, 1, 0));
	REQUIRE(image->save_png(files.file("tile.png")) == OK);
	CharString path = files.file("tile.png").utf8();
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_tilemap();
	CHECK(TestSpx::take_string(manager->get_tile(Vector2())).is_empty());
	CHECK(TestSpx::take_string(manager->get_tile_with_layer(Vector2(), 1)).is_empty());
	manager->set_layer_index(1);
	manager->open_draw_tiles_with_size(32); // Already open: preserve the original grid.
	manager->set_tile(path.get_data(), false);
	float coords[] = { 8, -8, 24, -8, 8, -24 };
	GdArrayInfo positions = {};
	positions.type = GD_ARRAY_TYPE_FLOAT;
	positions.size = 6;
	positions.data = coords;
	manager->place_tiles(&positions, path.get_data());
	for (int i = 0; i < 6; i += 2) {
		CHECK(TestSpx::take_string(manager->get_tile(Vector2(coords[i], coords[i + 1]))) == files.file("tile.png"));
	}
	manager->place_tiles_with_layer(&positions, path.get_data(), 2);
	manager->set_layer_offset(2, Vector2(32, 16));
	CHECK(Vector2(manager->get_layer_offset(2)) == Vector2(32, 16));
	CHECK(TestSpx::take_string(manager->get_tile_with_layer(Vector2(40, 8), 2)) == files.file("tile.png"));
	manager->erase_tile_with_layer(Vector2(40, 8), 2);
	CHECK(TestSpx::take_string(manager->get_tile_with_layer(Vector2(40, 8), 2)).is_empty());
	manager->erase_tile(Vector2(56, 8));
	CHECK(TestSpx::take_string(manager->get_tile(Vector2(56, 8))).is_empty());
	manager->place_tile(Vector2(56, 8), path.get_data());
	manager->place_tile_with_layer(Vector2(8, -8), path.get_data(), 3);
	CHECK(TestSpx::take_string(manager->get_tile_with_layer(Vector2(8, -8), 3)) == files.file("tile.png"));
	manager->place_tiles(nullptr, path.get_data());
	manager->on_reset(0);
	CHECK(TestSpx::take_string(manager->get_tile(Vector2(8, -8))).is_empty());
	manager->open_draw_tiles_with_size(8);
	manager->set_tile_with_collision_info(path.get_data(), nullptr);
	manager->set_layer_index(0);
	manager->place_tile(Vector2(4, -4), path.get_data());
	CHECK(TestSpx::take_string(manager->get_tile(Vector2(4, -4))) == files.file("tile.png"));
	manager->exit_tilemap_editor_mode();
	CHECK(TestSpx::take_string(manager->get_tile(Vector2(4, -4))).is_empty());
	manager->close_draw_tiles();
}
class TileEditor : public SpxDrawTiles {
public:
	using SpxDrawTiles::input;
};

TEST_CASE("[SceneTree][SPX] Invalid tile batches preserve existing cells and missing layers retain undo history") {
	TestSpx::TempDirectory files("spx_invalid_tile_batches");
	auto image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	REQUIRE(image->save_png(files.file("tile.png")) == OK);
	image->resize(4, 4);
	REQUIRE(image->save_png(files.file("small.png")) == OK);
	auto valid_path = files.file("tile.png").utf8();
	auto small_path = files.file("small.png").utf8();
	TestSpx::EngineScene scene;
	auto *tiles = memnew(TileEditor);
	tiles->clear_all_layers();
	scene.root->add_child(tiles);
	tiles->enter_editor_mode();
	tiles->queue_redraw();
	MessageQueue::get_singleton()->flush();
	tiles->place_or_erase_tile(Vector2(), false);
	Ref<InputEventKey> export_key;
	export_key.instantiate();
	export_key->set_keycode(Key::E);
	export_key->set_ctrl_pressed(true);
	export_key->set_pressed(true);
	ERR_PRINT_OFF;
	tiles->input(export_key); // Empty editor exports do not allocate a snapshot.
	export_key->set_pressed(false);
	tiles->input(export_key);
	export_key->set_pressed(true);
	export_key->set_keycode(Key::SPACE);
	tiles->input(export_key);
	tiles->set_texture(Ref<Texture2D>());
	ERR_PRINT_ON;
	tiles->set_layer_index(0);
	auto *layer = Object::cast_to<TileMapLayer>(tiles->get_child(0));
	REQUIRE(layer);
	MessageQueue::get_singleton()->flush();
	tiles->place_tile(layer, Vector2i()); // No texture selected yet.
	CHECK(layer->get_used_cells().is_empty());
	Ref<InputEventMouseButton> ignored_click;
	ignored_click.instantiate();
	ignored_click->set_button_index(MouseButton::RIGHT);
	ignored_click->set_pressed(true);
	tiles->input(ignored_click);
	CHECK(layer->get_used_cells().is_empty());
	tiles->place_tile_spx(Vector2(8, -8), valid_path.get_data());
	const int source = layer->get_cell_source_id(Vector2i());
	REQUIRE(source != TileSet::INVALID_SOURCE);
	float coordinates[] = { 8, -8, 24 };
	GdArrayInfo positions = { 2, GD_ARRAY_TYPE_FLOAT, coordinates };
	ERR_PRINT_OFF;
	tiles->place_tiles_spx(&positions, small_path.get_data());
	positions.size = 0;
	tiles->place_tiles_spx(&positions, valid_path.get_data());
	positions.size = 2;
	positions.type = GD_ARRAY_TYPE_INT64;
	tiles->place_tiles_spx(&positions, valid_path.get_data());
	ERR_PRINT_ON;
	CHECK(layer->get_cell_source_id(Vector2i()) == source);
	positions.type = GD_ARRAY_TYPE_FLOAT;
	positions.size = 3;
	tiles->place_tiles_spx(&positions, valid_path.get_data());
	CHECK(layer->get_used_cells().size() == 1);
	tiles->undo();
	CHECK(layer->get_used_cells().is_empty());
	tiles->remove_child(layer);
	tiles->redo();
	tiles->add_child(layer);
	tiles->redo();
	CHECK(layer->get_cell_source_id(Vector2i()) == source);
	tiles->remove_child(layer);
	tiles->undo();
	tiles->add_child(layer);
	tiles->undo();
	CHECK(layer->get_used_cells().is_empty());
	tiles->exit_editor_mode();
	tiles->redo();
	CHECK(layer->get_used_cells().is_empty());
	tiles->erase_tile(layer, Vector2i());
	layer->set_cell(Vector2i(1, 0), 99, Vector2i());
	CHECK(TestSpx::take_string(tiles->get_tile_spx(Vector2(24, -8))).is_empty());
	auto no_collision = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	tiles->set_texture_with_collision_points(ImageTexture::create_from_image(no_collision), nullptr);
	tiles->place_tile(layer, Vector2i());
	REQUIRE(layer->get_cell_tile_data(Vector2i()));
	CHECK(layer->get_cell_tile_data(Vector2i())->get_collision_polygons_count(0) == 0);
}

TEST_CASE("[SceneTree][SPX] Tile collision arrays validate payloads before opening the editor") {
	TestSpx::TempDirectory files("spx_tile_collision_array");
	auto image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 1, 1));
	REQUIRE(image->save_png(files.file("tile.png")) == OK);
	auto path = files.file("tile.png").utf8();
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_tilemap();
	const int children = scene.root->get_child_count();
	GdArrayInfo invalid = { -1, GD_ARRAY_TYPE_FLOAT, nullptr };
	ERR_PRINT_OFF;
	manager->erase_tile(Vector2());
	manager->erase_tile_with_layer(Vector2(), 0);
	manager->set_layer_offset(0, Vector2(2, 3));
	CHECK(Vector2(manager->get_layer_offset(0)) == Vector2());
	manager->set_tile_with_collision_info(path.get_data(), &invalid);
	invalid.size = 2;
	manager->set_tile_with_collision_info(path.get_data(), &invalid);
	ERR_PRINT_ON;
	CHECK(scene.root->get_child_count() == children);
	float coordinates[] = { -8, -8, 8, -8, 0, 8 };
	GdArrayInfo points = { 6, GD_ARRAY_TYPE_FLOAT, coordinates };
	manager->set_tile_with_collision_info(path.get_data(), &points);
	manager->set_layer_index(0);
	manager->place_tile(Vector2(8, -8), path.get_data());
	auto *tiles = Object::cast_to<SpxDrawTiles>(scene.root->get_child(-1));
	REQUIRE(tiles);
	auto *layer = Object::cast_to<TileMapLayer>(tiles->get_child(0));
	REQUIRE(layer);
	auto *data = layer->get_cell_tile_data(Vector2i());
	REQUIRE(data);
	CHECK(data->get_collision_polygon_points(0, 0) == Vector<Vector2>({ Vector2(-8, -8), Vector2(8, -8), Vector2(0, 8) }));
}
TEST_CASE("[SceneTree][SPX] Tile editor mouse gestures paint, erase and drag the selected layer") {
	TestSpx::EngineScene scene;
	auto *viewport = memnew(SubViewport);
	viewport->set_size(Vector2i(256, 256));
	scene.root->add_child(viewport);
	auto *editor = memnew(TileEditor);
	viewport->add_child(editor);
	editor->set_process_input(false);
	editor->set_layer_index(0);
	editor->enter_editor_mode();
	auto image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	editor->set_texture(ImageTexture::create_from_image(image), false);
	auto *layer = Object::cast_to<TileMapLayer>(editor->get_child(0));
	REQUIRE(layer);
	auto motion = [&](Vector2 position) {
		Ref<InputEventMouseMotion> event;
		event.instantiate();
		event->set_position(position);
		viewport->push_input(event, true);
		editor->input(event);
		MessageQueue::get_singleton()->flush();
	};
	auto click = [&](bool pressed) {
		Ref<InputEventMouseButton> event;
		event.instantiate();
		event->set_position(viewport->get_mouse_position());
		event->set_button_index(MouseButton::LEFT);
		event->set_pressed(pressed);
		editor->input(event);
	};
	motion(Vector2(64, 64));
	click(true);
	motion(Vector2(72, 72));
	click(false);
	REQUIRE(layer->get_used_cells().size() == 1);
	CHECK(layer->get_cell_source_id(Vector2i(4, 4)) != TileSet::INVALID_SOURCE);
	Ref<InputEventKey> shift;
	shift.instantiate();
	shift->set_keycode(Key::SHIFT);
	shift->set_pressed(true);
	Input::get_singleton()->parse_input_event(shift);
	Input::get_singleton()->flush_buffered_events();
	click(true);
	motion(Vector2(72, 72));
	click(false);
	shift = shift->duplicate();
	shift->set_pressed(false);
	Input::get_singleton()->parse_input_event(shift);
	Input::get_singleton()->flush_buffered_events();
	CHECK(layer->get_used_cells().is_empty());
	editor->undo();
	CHECK(layer->get_used_cells().size() == 1);
	motion(Vector2());
	click(true);
	motion(Vector2(24, 32));
	click(false);
	CHECK(layer->get_position() == Vector2(24, 32));
	motion(Vector2(48, 48));
	CHECK(layer->get_position() == Vector2(24, 32));
	editor->exit_editor_mode();
	motion(Vector2(24, 32));
	click(true);
	motion(Vector2(80, 80));
	click(false);
	CHECK(layer->get_position() == Vector2(24, 32));
}

} // namespace TestSpxDrawTiles
#endif
