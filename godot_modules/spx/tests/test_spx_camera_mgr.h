/**************************************************************************/
/*  test_spx_camera_mgr.h                                               */
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

#ifndef TEST_SPX_CAMERA_MGR_H
#define TEST_SPX_CAMERA_MGR_H

#include "../spx_camera_mgr.h"
#include "scene/2d/camera_2d.h"
#include "spx_test_fixtures.h"

namespace TestSpxCameraMgr {

TEST_CASE("[SceneTree][SPX] Camera controls convert coordinates and preserve stage limits") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_camera();
	auto *camera = Object::cast_to<Camera2D>(scene.root->get_node(NodePath("SpxCamera2D")));
	REQUIRE(camera != nullptr);
	CHECK(manager->get_camera() == camera);
	manager->set_camera_position(Vector2(30, 40));
	CHECK(camera->get_position() == Vector2(30, -40));
	CHECK(Vector2(manager->get_camera_position()) == Vector2(30, 40));
	manager->set_camera_zoom(Vector2(2, 3));
	CHECK(camera->get_zoom() == Vector2(2, 3));
	CHECK(Vector2(manager->get_camera_zoom()) == Vector2(2, 3));
	manager->set_camera_limit(SIDE_LEFT, -100);
	manager->set_camera_limit(SIDE_TOP, -50);
	manager->set_camera_limit(SIDE_RIGHT, 300);
	manager->set_camera_limit(SIDE_BOTTOM, 200);
	CHECK(Rect2(manager->get_stage_limits_rect()) == Rect2(-100, -50, 400, 250));
	CHECK(camera->get_limit(SIDE_BOTTOM) == 200);
	manager->set_camera_smoothing(true);
	CHECK(camera->is_position_smoothing_enabled());
	manager->set_camera_smoothing(false);
	CHECK_FALSE(camera->is_position_smoothing_enabled());
	CHECK(Rect2(manager->get_viewport_rect()) == camera->get_viewport_rect());
	camera->force_update_scroll();
	const Rect2 world = manager->get_global_camera_rect();
	CHECK(world.size.is_equal_approx(camera->get_viewport_rect().size / Vector2(2, 3)));
	manager->on_reset(0);
	CHECK(Vector2(manager->get_camera_position()) == Vector2());
	CHECK(Vector2(manager->get_camera_zoom()) == Vector2(2, 3));
}

TEST_CASE("[SceneTree][SPX] Camera stretch uses a transparent viewport and releases its limits on teardown") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_camera();
	auto *viewport = memnew(SubViewport);
	viewport->set_size(Vector2i(64, 64));
	scene.root->add_child(viewport);
	manager->get_camera()->reparent(viewport);
	RenderingServer *rendering = RenderingServer::get_singleton();
	struct RestoreClearColor {
		RenderingServer *rendering;
		Color color = rendering->get_default_clear_color();
		~RestoreClearColor() { rendering->set_default_clear_color(color); }
	} restore{ rendering };
	viewport->set_transparent_background(false);
	rendering->set_default_clear_color(Color(1, 0, 0, 1));
	manager->set_stretch_clear_color();
	CHECK(viewport->has_transparent_background());
	CHECK_FALSE(viewport->is_using_hdr_2d());
	CHECK(rendering->get_default_clear_color() == Color(0, 0, 0, 0));
	manager->on_destroy();
	CHECK(manager->get_camera() == nullptr);
	CHECK(Rect2(manager->get_stage_limits_rect()) == Rect2());
	manager->on_reset(0);
}

} // namespace TestSpxCameraMgr

#endif // TEST_SPX_CAMERA_MGR_H
