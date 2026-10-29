/**************************************************************************/
/*  test_spx_platform_mgr.h                                             */
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

#ifndef TEST_SPX_PLATFORM_MGR_H
#define TEST_SPX_PLATFORM_MGR_H
#include "../spx.h"
#include "../spx_platform_mgr.h"
#include "core/config/engine.h"
#include "spx_test_fixtures.h"
namespace TestSpxPlatformMgr {
TEST_CASE("[SceneTree][SPX] Platform settings control engine timing, scaling and persistent storage") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_platform();
	auto *engine = Engine::get_singleton();
	auto *window = SceneTree::get_singleton()->get_root();
	struct Restore {
		Engine *engine;
		Window *window;
		double time_scale = engine->get_time_scale();
		int max_fps = engine->get_max_fps();
		bool debug = Spx::is_debug_mode();
		Size2i window_size = window->get_size();
		Size2i content_size = window->get_content_scale_size();
		Window::ContentScaleMode scale_mode = window->get_content_scale_mode();
		Window::ContentScaleAspect scale_aspect = window->get_content_scale_aspect();

		~Restore() {
			engine->set_time_scale(time_scale);
			engine->set_max_fps(max_fps);
			Spx::set_debug_mode(debug);
			window->set_size(window_size);
			window->set_content_scale_size(content_size);
			window->set_content_scale_mode(scale_mode);
			window->set_content_scale_aspect(scale_aspect);
		}
	} restore{ engine, window };
	manager->set_time_scale(0.5);
	CHECK(manager->get_time_scale() == 0.5);
	CHECK(engine->get_time_scale() == 0.5);
	manager->set_max_fps(30);
	CHECK(manager->get_max_fps() == 30);
	CHECK(engine->get_max_fps() == 30);
	CHECK(manager->is_main_thread());
	manager->set_debug_mode(true);
	CHECK(manager->is_debug_mode());
	CHECK(Spx::is_debug_mode());
	manager->set_debug_mode(false);
	CHECK_FALSE(manager->is_debug_mode());
	manager->set_persistent_data_dir("user://spx-test/");
	CHECK(TestSpx::take_string(manager->get_persistent_data_dir()) == "user://spx-test/");
	CHECK(manager->is_in_persistent_data_dir("user://spx-test/save.json"));
	CHECK_FALSE(manager->is_in_persistent_data_dir("user://spx-other/save.json"));
	manager->set_stretch(true, 480, 360);
	CHECK(window->get_content_scale_mode() == Window::CONTENT_SCALE_MODE_CANVAS_ITEMS);
	CHECK(window->get_content_scale_aspect() == Window::CONTENT_SCALE_ASPECT_KEEP);
	manager->set_stretch(true, 480, 360);
	manager->set_window_size(480, 360, true);
	CHECK(window->get_content_scale_size() == Vector2i(480, 360));
	manager->on_reset(0);
	CHECK(window->get_content_scale_mode() == Window::CONTENT_SCALE_MODE_DISABLED);
	CHECK(window->get_content_scale_aspect() == Window::CONTENT_SCALE_ASPECT_IGNORE);
}
} // namespace TestSpxPlatformMgr
#endif
