/**************************************************************************/
/*  test_spx_input_mgr.h                                                */
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

#ifndef TEST_SPX_INPUT_MGR_H
#define TEST_SPX_INPUT_MGR_H
#include "../spx_camera_mgr.h"
#include "../spx_input_mgr.h"
#include "core/input/input_map.h"
#include "spx_test_fixtures.h"
namespace TestSpxInputMgr {
TEST_CASE("[SceneTree][SPX] Input actions expose equivalent name and registered-ID state") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_input();
	auto *input = Input::get_singleton();
	auto *map = InputMap::get_singleton();
	const char *negative = "spx_test_left", *positive = "spx_test_right";
	map->add_action(negative);
	map->add_action(positive);
	struct Cleanup {
		~Cleanup() {
			for (const char *name : { "spx_test_left", "spx_test_right" }) {
				Input::get_singleton()->action_release(name);
				InputMap::get_singleton()->erase_action(name);
			}
		}
	} cleanup;
	manager->on_start();
	manager->on_start();
	CHECK(scene.root->has_node(NodePath("input_proxy")));
	auto left = manager->register_action(negative), right = manager->register_action(positive);
	CHECK(left != right);
	CHECK(manager->register_action(positive) == right);
	CHECK_FALSE(manager->is_action_pressed(positive));
	CHECK_FALSE(manager->is_action_pressed_id(right));
	CHECK_FALSE(manager->is_action_just_pressed_id(right));
	CHECK_FALSE(manager->is_action_just_released_id(right));
	input->action_press(positive, 0.75);
	CHECK(manager->is_action_pressed(positive));
	CHECK(manager->is_action_pressed_id(right));
	CHECK(manager->is_action_just_pressed(positive));
	CHECK(manager->is_action_just_pressed_id(right));
	CHECK(manager->get_axis(negative, positive) == doctest::Approx(0.75));
	CHECK(manager->get_axis_id(left, right) == doctest::Approx(0.75));
	input->action_release(positive);
	CHECK(manager->is_action_just_released(positive));
	CHECK(manager->is_action_just_released_id(right));
	CHECK(manager->get_axis_id(left, right) == 0);
	CHECK_FALSE(manager->is_action_pressed_id(-1));
	CHECK_FALSE(manager->is_action_just_pressed_id(100));
	CHECK_FALSE(manager->is_action_just_released_id(-1));
	CHECK(manager->get_axis_id(-1, right) == 0);
	CHECK(manager->get_axis_id(left, -1) == 0);
	manager->on_reset(0);
	CHECK_FALSE(scene.root->has_node(NodePath("input_proxy")));
	manager->on_reset(0); // Reset is safe when the reusable proxy is already detached.
	CHECK_FALSE(scene.root->has_node(NodePath("input_proxy")));
	CHECK_FALSE(manager->is_action_pressed_id(right));
	CHECK(manager->register_action(positive) == 0);
	manager->on_start();
	CHECK(scene.root->has_node(NodePath("input_proxy")));
}
TEST_CASE("[SceneTree][SPX] Input snapshots keep mouse and key states distinct") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_input();
	auto *input = Input::get_singleton();
	input->release_pressed_events();
	struct Cleanup {
		~Cleanup() { Input::get_singleton()->release_pressed_events(); }
	} cleanup;
	Ref<InputEventKey> key;
	key.instantiate();
	key->set_keycode(Key::A);
	// The layout label may differ from the logical keycode. Both are accepted.
	key->set_key_label(Key::B);
	key->set_pressed(true);
	input->parse_input_event(key);
	input->flush_buffered_events();
	CHECK(manager->get_key(int(Key::A)));
	CHECK(manager->get_key(int(Key::B)));
	CHECK(manager->get_key_state(int(Key::B)) == 1);
	CHECK(manager->get_key_state(int(Key::A)) == 1);
	CHECK(manager->get_key(-1));
	key = key->duplicate();
	key->set_pressed(false);
	input->parse_input_event(key);
	input->flush_buffered_events();
	CHECK_FALSE(manager->get_key(int(Key::A)));
	CHECK_FALSE(manager->get_key(int(Key::B)));
	CHECK(manager->get_key_state(int(Key::A)) == 0);
	Ref<InputEventMouseButton> button;
	button.instantiate();
	button->set_button_index(MouseButton::LEFT);
	button->set_pressed(true);
	input->parse_input_event(button);
	input->flush_buffered_events();
	CHECK(manager->get_mouse_state(int(MouseButton::LEFT)));
	CHECK_FALSE(manager->get_key(-1));
	float snapshot[3];
	manager->write_snapshot(snapshot);
	CHECK(snapshot[2] == int(MouseButtonMask::LEFT));
	Vector2 mouse = scene.engine->get_camera()->get_global_mouse_position();
	CHECK(snapshot[0] == mouse.x);
	CHECK(snapshot[1] == -mouse.y);
	manager->write_snapshot(nullptr);
	button = button->duplicate();
	button->set_pressed(false);
	input->parse_input_event(button);
	input->flush_buffered_events();
	CHECK_FALSE(manager->get_mouse_state(int(MouseButton::LEFT)));
}
TEST_CASE("[SceneTree][SPX] Input proxy destruction tolerates an already stopped manager") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_input();
	manager->on_start();
	manager->on_reset(0);
	manager->on_destroy();
	manager->on_reset(0);
	manager->on_destroy();
	CHECK_FALSE(scene.root->has_node(NodePath("input_proxy")));
	// A later start must create a usable replacement, rather than reuse the queued proxy.
	manager->on_start();
	CHECK(scene.root->has_node(NodePath("input_proxy")));
}
} // namespace TestSpxInputMgr
#endif
