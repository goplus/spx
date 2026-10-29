/**************************************************************************/
/*  test_spx_physics_mgr.h                                              */
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

#ifndef TEST_SPX_PHYSICS_MGR_H
#define TEST_SPX_PHYSICS_MGR_H
#include "../spx_camera_mgr.h"
#include "../spx_physics_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "scene/resources/world_2d.h"
#include "spx_test_fixtures.h"
namespace TestSpxPhysicsMgr {
TEST_CASE("[SceneTree][SPX] Stage and camera boundaries use transformed trigger shapes") {
	TestSpx::EngineScene scene;
	auto *physics = scene.engine->get_physics();
	auto *camera = scene.engine->get_camera();
	auto *sprites = scene.engine->get_sprite();
	camera->set_camera_limit(SIDE_LEFT, -100);
	camera->set_camera_limit(SIDE_RIGHT, 100);
	camera->set_camera_limit(SIDE_TOP, -50);
	camera->set_camera_limit(SIDE_BOTTOM, 50);
	GdObj id = sprites->create_bare_sprite(Vector2());
	sprites->set_trigger_rect(id, Vector2(), Vector2(10, 10));
	CHECK(physics->check_touched_stage_boundaries(id) == 0);
	CHECK(physics->check_nearest_touched_stage_boundary(id) == 0);
	const Vector2 positions[] = { Vector2(-98, 0), Vector2(0, 48), Vector2(98, 0), Vector2(0, -48) };
	const int edges[] = { BOUND_LEFT, BOUND_TOP, BOUND_RIGHT, BOUND_BOTTOM };
	for (int i = 0; i < 4; ++i) {
		sprites->set_position(id, positions[i]);
		CHECK(physics->check_touched_stage_boundaries(id) == edges[i]);
		CHECK(physics->check_touched_stage_boundary(id, edges[i]));
		CHECK(physics->check_nearest_touched_stage_boundary(id) == edges[i]);
	}
	sprites->set_position(id, Vector2(-98, 48));
	CHECK(physics->check_touched_stage_boundaries(id) == (BOUND_LEFT | BOUND_TOP));
	CHECK(physics->check_nearest_touched_stage_boundary(id) == BOUND_LEFT);
	auto *node = Object::cast_to<Camera2D>(scene.root->get_node(NodePath("SpxCamera2D")));
	REQUIRE(node);
	node->force_update_scroll();
	Rect2 rect = camera->get_global_camera_rect();
	sprites->set_position(id, Vector2(rect.position.x, -rect.get_center().y));
	CHECK(physics->check_touched_camera_boundary(id, BOUND_LEFT));
	CHECK((physics->check_touched_camera_boundaries(id) & BOUND_LEFT) != 0);
	CHECK(physics->check_nearest_touched_camera_boundary(id) == BOUND_LEFT);
	sprites->get_sprite(id)->get_trigger()->set_shape(Ref<Shape2D>());
	CHECK(physics->check_touched_stage_boundaries(id) == 0);
	CHECK(physics->check_nearest_touched_stage_boundary(id) == 0);
	ERR_PRINT_OFF;
	CHECK(physics->check_touched_stage_boundaries(0) == 0);
	CHECK(physics->check_nearest_touched_stage_boundary(0) == 0);
	ERR_PRINT_ON;
}

TEST_CASE("[SceneTree][SPX] Physics queries respect masks, exclusions and SPX coordinates") {
	TestSpx::EngineScene scene;
	auto *physics = scene.engine->get_physics();
	auto *sprites = scene.engine->get_sprite();
	GdObj id = sprites->create_bare_sprite(Vector2(20, 30));
	sprites->set_collider_rect(id, Vector2(), Vector2(10, 10));
	sprites->set_physics_mode(id, SpxSprite::STATIC);
	sprites->set_collision_layer(id, 4);
	sprites->set_trigger_layer(id, 0);
	SceneTree::get_singleton()->flush_transform_notifications();
	PhysicsServer2D *server = PhysicsServer2D::get_singleton();
	server->sync();
	server->end_sync();
	server->step(1.0 / 60);
	server->sync();
	// Keep the server synchronized even if an assertion aborts this scope.
	struct SyncGuard {
		~SyncGuard() { PhysicsServer2D::get_singleton()->end_sync(); }
	} guard;
	CHECK(physics->raycast(Vector2(0, 30), Vector2(40, 30), 4) == id);
	CHECK(physics->raycast(Vector2(0, 30), Vector2(40, 30), 2) == 0);
	CHECK(physics->check_collision(Vector2(0, 30), Vector2(40, 30), 4, false, true));
	auto hit = TestSpx::take_array(physics->raycast_with_details(Vector2(0, 30), Vector2(40, 30), nullptr, 4, false, true));
	REQUIRE(hit->size == 6);
	const auto *values = SpxAbi::get_array<GdInt>(hit.get(), 0);
	REQUIRE(values);
	CHECK(values[0] == 1);
	CHECK(values[1] == id);
	CHECK(values[2] == 150000);
	CHECK(values[3] == 300000);
	CHECK(values[4] == -10000);
	CHECK(values[5] == 0);
	auto ignore = TestSpx::take_array(SpxAbi::create_array(GD_ARRAY_TYPE_GDOBJ, 1));
	SpxAbi::set_array(ignore.get(), 0, id);
	auto miss = TestSpx::take_array(physics->raycast_with_details(Vector2(0, 30), Vector2(40, 30), ignore.get(), 4, true, true));
	CHECK(SpxAbi::get_array<GdInt>(miss.get(), 0)[0] == 0);
	// A malformed exclusion buffer must not silently become an unfiltered query.
	auto malformed = TestSpx::take_array(SpxAbi::create_array(GD_ARRAY_TYPE_FLOAT, 1));
	ERR_PRINT_OFF;
	auto rejected = TestSpx::take_array(physics->raycast_with_details(Vector2(0, 30), Vector2(40, 30), malformed.get(), 4, false, true));
	ERR_PRINT_ON;
	REQUIRE(rejected != nullptr);
	const auto *rejected_values = SpxAbi::get_array<GdInt>(rejected.get(), 0);
	REQUIRE(rejected_values);
	if (!rejected_values) {
		return;
	}
	CHECK(rejected_values[0] == 0);
	CHECK(physics->raycast(Vector2(0, 30), Vector2(40, 30), 4) == id);
	for (bool circle : { false, true }) {
		auto hits = TestSpx::take_array(circle ? physics->check_collision_circle(Vector2(20, 30), 2, 4) : physics->check_collision_rect(Vector2(20, 30), Vector2(4, 4), 4));
		REQUIRE(hits->size == 1);
		CHECK(SpxAbi::get_array<GdObj>(hits.get(), 0)[0] == id);
	}
	auto empty = TestSpx::take_array(physics->check_collision_circle(Vector2(200, 300), 2, 4));
	CHECK(empty->size == 0);
}

TEST_CASE("[SceneTree][SPX] Global physics settings are shared by sprite physics") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_physics();
	const auto gravity = manager->get_global_gravity(), friction = manager->get_global_friction(), drag = manager->get_global_air_drag();
	manager->set_global_gravity(2.5);
	manager->set_global_friction(0.25);
	manager->set_global_air_drag(0.75);
	CHECK(SpxPhysicsDefine::get_global_gravity() == 2.5);
	CHECK(SpxPhysicsDefine::get_global_friction() == 0.25);
	CHECK(SpxPhysicsDefine::get_global_air_drag() == 0.75);
	manager->set_global_gravity(gravity);
	manager->set_global_friction(friction);
	manager->set_global_air_drag(drag);
	manager->set_collision_system_type(false);
	CHECK_FALSE(manager->is_collision_by_pixel);
	manager->set_collision_system_type(true);
	CHECK(manager->is_collision_by_pixel);
}
} // namespace TestSpxPhysicsMgr
#endif
