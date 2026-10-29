/**************************************************************************/
/*  test_spx_debug_mgr.h                                                */
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

#ifndef TEST_SPX_DEBUG_MGR_H
#define TEST_SPX_DEBUG_MGR_H
#include "../spx_debug_mgr.h"
#include "scene/2d/line_2d.h"
#include "spx_test_fixtures.h"
namespace TestSpxDebugMgr {
TEST_CASE("[SceneTree][SPX] Debug shapes convert SPX coordinates and expire after one frame") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_debug();
	auto *root = scene.root->get_node(NodePath("debug_root"));
	REQUIRE(root);
	manager->debug_draw_circle(Vector2(10, 20), 5, Color(1, 0, 0));
	manager->debug_draw_rect(Vector2(30, 40), Vector2(10, 20), Color(0, 1, 0));
	manager->debug_draw_line(Vector2(1, 2), Vector2(4, 6), Color(0, 0, 1));
	REQUIRE(root->get_child_count() == 3);
	auto *circle = Object::cast_to<Line2D>(root->get_child(0));
	auto *rect = Object::cast_to<Line2D>(root->get_child(1));
	auto *line = Object::cast_to<Line2D>(root->get_child(2));
	REQUIRE(circle);
	REQUIRE(rect);
	REQUIRE(line);
	CHECK(circle->get_position() == Vector2(10, -20));
	CHECK(circle->get_default_color() == Color(1, 0, 0));
	CHECK(circle->get_points().size() == 17);
	for (const auto &point : circle->get_points()) {
		CHECK(point.length() == doctest::Approx(5));
	}
	CHECK(rect->get_position() == Vector2(30, -40));
	CHECK(rect->get_points() == Vector<Vector2>({ Vector2(-5, -10), Vector2(5, -10), Vector2(5, 10), Vector2(-5, 10), Vector2(-5, -10) }));
	CHECK(line->get_position() == Vector2(1, -2));
	CHECK(line->get_points() == Vector<Vector2>({ Vector2(), Vector2(3, -4) }));
	manager->on_update(0.016);
	CHECK(circle->is_queued_for_deletion());
	CHECK(rect->is_queued_for_deletion());
	CHECK(line->is_queued_for_deletion());
	manager->debug_draw_circle(Vector2(), 30, Color(1, 1, 1));
	auto *large = Object::cast_to<Line2D>(root->get_child(3));
	REQUIRE(large);
	CHECK(large->get_points().size() == 25);
	manager->on_reset(0);
	CHECK(large->is_queued_for_deletion());
}
} // namespace TestSpxDebugMgr
#endif
