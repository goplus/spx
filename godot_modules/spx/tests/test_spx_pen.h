/**************************************************************************/
/*  test_spx_pen.h                                                      */
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

#ifndef TEST_SPX_PEN_H
#define TEST_SPX_PEN_H
#include "../spx_pen_mgr.h"
#include "../spx_pen_surface.h"
#include "../spx_pixel_query.h"
#include "../spx_sprite_mgr.h"
#include "spx_test_fixtures.h"
namespace TestSpxPen {
TEST_CASE("[SceneTree][SPX] Pen strokes queue only while down and share a resizable canvas") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_pen();
	auto *surface = Object::cast_to<SpxPenSurface>(scene.root->get_node(NodePath("pen_root")));
	REQUIRE(surface);
	auto *viewport = Object::cast_to<SubViewport>(surface->get_node(NodePath("pen_render_target")));
	auto *canvas = Object::cast_to<SpxPenCanvas>(surface->get_node(NodePath("pen_render_target/pen_canvas_drawer")));
	REQUIRE(viewport);
	REQUIRE(canvas);
	manager->set_canvas_size(100, 80);
	CHECK(viewport->get_size() == Vector2i(100, 80));
	CHECK(canvas->get_position() == Vector2(50, 40));
	manager->set_canvas_size(100, 80);
	GdObj pen = manager->create_pen();
	REQUIRE(pen != 0);
	manager->move_pen_to(pen, Vector2(10, 20));
	CHECK_FALSE(canvas->has_pending_commands());
	manager->set_pen_color_to(pen, Color(1, 0, 0));
	manager->set_pen_size_to(pen, 3);
	manager->pen_down(pen, false);
	CHECK(canvas->has_pending_commands());
	manager->flush_all();
	CHECK_FALSE(canvas->has_pending_commands());
	CHECK(viewport->get_update_mode() == SubViewport::UPDATE_ONCE);
	manager->move_pen_to(pen, Vector2(10.2, 20));
	CHECK_FALSE(canvas->has_pending_commands());
	manager->move_pen_to(pen, Vector2(20, 20));
	CHECK(canvas->has_pending_commands());
	manager->move_pen_to(pen, Vector2(20, 40));
	manager->flush_all();
	CHECK_FALSE(canvas->has_pending_commands());
	for (int property = 0; property < 3; ++property) {
		manager->set_pen_to(pen, property, 0.5);
		manager->change_pen_by(pen, property, 0.25);
		manager->move_pen_to(pen, Vector2(30 + property * 10, 40));
		CHECK(canvas->has_pending_commands());
		manager->flush_all();
	}
	manager->change_pen_size_by(pen, -100);
	manager->move_pen_to(pen, Vector2(60, 40));
	CHECK(canvas->has_pending_commands());
	manager->pen_up(pen);
	manager->flush_all();
	manager->move_pen_to(pen, Vector2(80, 80));
	CHECK_FALSE(canvas->has_pending_commands());
	manager->pen_down(pen, true);
	manager->on_update(0.016);
	CHECK(canvas->has_pending_commands());
	manager->destroy_all_pens();
	CHECK_FALSE(canvas->has_pending_commands());
	// Erasing pixels keeps the pen alive and starts a fresh stroke.
	manager->move_pen_to(pen, Vector2(90, 90));
	CHECK(canvas->has_pending_commands());
	manager->on_reset(0);
	CHECK_FALSE(canvas->has_pending_commands());
	manager->set_canvas_size(0, -1);
	CHECK(viewport->get_size() == Vector2i(2, 2)); // Godot's SubViewport minimum.
	SpxPixelQuery::Snapshot snapshot;
	CHECK_FALSE(manager->capture(Rect2(1000, 1000, 10, 10), snapshot));
}

TEST_CASE("[SceneTree][SPX] Pen stamps hold resources until submission and accept sprite visuals") {
	TestSpx::TempDirectory files("spx_pen_stamp");
	auto image = Image::create_empty(4, 4, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	REQUIRE(image->save_png(files.file("stamp.png")) == OK);
	const CharString path = files.file("stamp.png").utf8();
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_pen();
	auto *canvas = Object::cast_to<SpxPenCanvas>(scene.root->get_node(NodePath("pen_root/pen_render_target/pen_canvas_drawer")));
	REQUIRE(canvas);
	GdObj pen = manager->create_pen();
	manager->pen_stamp(pen);
	CHECK_FALSE(canvas->has_pending_commands());
	manager->set_pen_stamp_texture(pen, path.get_data());
	manager->pen_stamp(pen);
	CHECK(canvas->has_pending_commands());
	manager->flush_all();
	CHECK_FALSE(canvas->has_pending_commands());
	manager->pen_stamp_with_transform(pen, path.get_data(), Vector2(1, 2), 0.5, Vector2(2, 3));
	CHECK(canvas->has_pending_commands());
	manager->flush_all();
	auto *sprites = scene.engine->get_sprite();
	GdObj id = sprites->create_bare_sprite(Vector2());
	manager->pen_stamp_sprite(id);
	CHECK_FALSE(canvas->has_pending_commands());
	sprites->set_texture(id, path.get_data());
	sprites->set_anim_flip_h(id, true);
	sprites->set_anim_flip_v(id, true);
	manager->pen_stamp_sprite(id);
	CHECK(canvas->has_pending_commands());
	manager->flush_all();
	CHECK_FALSE(canvas->has_pending_commands());
	manager->flush_all();
	manager->destroy_pen(pen);
}

TEST_CASE("[SceneTree][SPX] Pen command batches validate all operations before drawing") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_pen();
	auto *canvas = Object::cast_to<SpxPenCanvas>(scene.root->get_node(NodePath("pen_root/pen_render_target/pen_canvas_drawer")));
	REQUIRE(canvas);
	const GdObj pen = manager->create_pen();
	uint64_t id = pen;
	auto lane = [](uint32_t bits) {
		float value;
		memcpy(&value, &bits, sizeof(value));
		return value;
	};
	const float low = lane(uint32_t(id)), high = lane(uint32_t(id >> 32));
	float commands[] = { 5,
		4, low, high, 1, 0, 0, 1, 0,
		5, low, high, 3, 0, 0, 0, 0,
		2, low, high, 0, 0, 0, 0, 0,
		1, low, high, 20, 30, 0, 0, 0,
		3, low, high, 0, 0, 0, 0, 0 };
	manager->batch_update_commands(commands, 41);
	CHECK(canvas->has_pending_commands());
	manager->flush_all();
	CHECK_FALSE(canvas->has_pending_commands());
	manager->move_pen_to(pen, Vector2(40, 50));
	CHECK_FALSE(canvas->has_pending_commands());
	commands[1 + 4 * 8] = 99;
	ERR_PRINT_OFF;
	manager->batch_update_commands(commands, 41);
	manager->batch_update_commands(commands, 40);
	manager->batch_update_commands(nullptr, 0);
	float fractional[] = { 0.5 };
	manager->batch_update_commands(fractional, 1);
	ERR_PRINT_ON;
	CHECK_FALSE(canvas->has_pending_commands());
	manager->destroy_pen(pen);
}
} // namespace TestSpxPen
#endif
