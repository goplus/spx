/**************************************************************************/
/*  test_spx_navigation_mgr.h                                           */
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

#ifndef TEST_SPX_NAVIGATION_MGR_H
#define TEST_SPX_NAVIGATION_MGR_H

#include "../spx_navigation_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "spx_test_fixtures.h"

namespace TestSpxNavigationMgr {

static Vector<Vector2> find_path(SpxNavigationMgr *manager, bool jump = false) {
	auto result = TestSpx::take_array(manager->find_path(Vector2(-40, -8), Vector2(40, -8), jump));
	REQUIRE(result != nullptr);
	REQUIRE(result->type == GD_ARRAY_TYPE_FLOAT);
	REQUIRE(result->size >= 4);
	REQUIRE(result->size % 2 == 0);
	const float *values = SpxAbi::get_array<float>(result.get(), 0);
	REQUIRE(values != nullptr);
	Vector<Vector2> points;
	for (int i = 0; i < result->size; i += 2) {
		points.push_back(Vector2(values[i], values[i + 1]));
	}
	CHECK(points[0] == Vector2(-40, -8));
	CHECK(points[points.size() - 1] == Vector2(40, -8));
	return points;
}

TEST_CASE("[SceneTree][SPX] Navigation returns SPX coordinates and routes around static obstacles") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_navigation();
	manager->setup_path_finder_with_size(Vector2(16, 16), Vector2(16, 16), false, false);
	const auto direct = find_path(manager);
	REQUIRE(direct.size() == 6);
	for (int i = 0; i < direct.size(); i++) {
		CHECK(direct[i] == Vector2(-40 + i * 16, -8));
	}
	auto *sprites = scene.engine->get_sprite();
	const GdObj obstacle = sprites->create_bare_sprite(Vector2(8, -8));
	sprites->set_physics_mode(obstacle, SpxSprite::STATIC);
	sprites->set_collider_rect(obstacle, Vector2(), Vector2(16, 16));
	manager->set_obstacle(obstacle, true);
	const auto detour = find_path(manager);
	CHECK(detour.size() > direct.size());
	CHECK_FALSE(detour.has(Vector2(8, -8)));
	manager->set_obstacle(obstacle, false);
	CHECK(find_path(manager) == direct);
	manager->on_reset(0);
	manager->on_reset(0);
	// Reconfigure after reset, including the jumping implementation.
	manager->setup_path_finder_with_size(Vector2(16, 16), Vector2(16, 16), true, false);
	manager->set_obstacle(obstacle, false);
	find_path(manager, true);
}

TEST_CASE("[SceneTree][SPX] Navigation initializes lazily and can be reset before use") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_navigation();
	manager->on_reset(0);
	manager->set_obstacle(0, true);
	find_path(manager);
	manager->setup_path_finder(false);
	find_path(manager);
}

} // namespace TestSpxNavigationMgr

#endif // TEST_SPX_NAVIGATION_MGR_H
