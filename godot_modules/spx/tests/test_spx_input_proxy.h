/**************************************************************************/
/*  test_spx_input_proxy.h                                                */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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

#ifndef TEST_SPX_INPUT_PROXY_H
#define TEST_SPX_INPUT_PROXY_H

#include "../spx_engine.h"
#include "../spx_input_mgr.h"
#include "core/templates/vector.h"
#include "scene/gui/slider.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "tests/test_macros.h"

namespace TestSpxInputProxy {

static int mouse_presses[10];
static int mouse_releases[10];
static Vector<GdInt> key_presses;
static Vector<GdInt> key_releases;

struct InputFixture {
	Window *window = nullptr;
	Node *root = nullptr;
	SpxInputMgr *input = nullptr;

	InputFixture() {
		for (int i = 0; i < 10; i++) {
			mouse_presses[i] = 0;
			mouse_releases[i] = 0;
		}
		key_presses.clear();
		key_releases.clear();
		SpxCallbackInfo callbacks = {};
		callbacks.func_on_mouse_pressed = [](GdInt p_button) { mouse_presses[p_button]++; };
		callbacks.func_on_mouse_released = [](GdInt p_button) { mouse_releases[p_button]++; };
		callbacks.func_on_key_pressed = [](GdInt p_key) { key_presses.push_back(p_key); };
		callbacks.func_on_key_released = [](GdInt p_key) { key_releases.push_back(p_key); };
		SpxEngine::register_callbacks(&callbacks);
		SpxEngine *engine = SpxEngine::get_singleton();
		SceneTree *tree = SceneTree::get_singleton();
		window = tree->get_root();
		root = memnew(Node);
		window->add_child(root);
		engine->set_root_node(tree, root);
		input = engine->get_input();
		input->on_start();
	}

	~InputFixture() {
		input->on_destroy();
		SpxEngine::shutdown();
		memdelete(root);
		key_presses.clear();
		key_releases.clear();
	}
};

static void push_mouse_button(
		Window *p_window,
		MouseButton p_button,
		bool p_pressed,
		BitField<MouseButtonMask> p_button_mask,
		const Vector2 &p_position) {
	Ref<InputEventMouseButton> event;
	event.instantiate();
	event->set_button_index(p_button);
	event->set_pressed(p_pressed);
	event->set_button_mask(p_button_mask);
	event->set_position(p_position);
	event->set_global_position(p_position);
	p_window->push_input(event, true);
}

TEST_CASE("[SceneTree][SPX] Input proxy defers mouse presses and pairs forwarded releases") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	InputFixture fixture;
	Window *window = fixture.window;
	Node *root = fixture.root;
	Control *monitor = memnew(Control);
	monitor->set_mouse_filter(Control::MOUSE_FILTER_IGNORE);
	root->add_child(monitor);
	HSlider *slider = memnew(HSlider);
	slider->set_size(Size2(50, 50));
	monitor->add_child(slider);
	Control *decoration = memnew(Control);
	decoration->set_position(Vector2(75, 75));
	decoration->set_size(Size2(50, 50));
	decoration->set_mouse_filter(Control::MOUSE_FILTER_IGNORE);
	root->add_child(decoration);
	Control *display_monitor = memnew(Control);
	display_monitor->set_position(Vector2(150, 0));
	display_monitor->set_size(Size2(50, 50));
	display_monitor->set_mouse_filter(Control::MOUSE_FILTER_STOP);
	root->add_child(display_monitor);
	Control *display_content = memnew(Control);
	display_content->set_size(Size2(50, 50));
	display_content->set_mouse_filter(Control::MOUSE_FILTER_IGNORE);
	display_monitor->add_child(display_content);
	CHECK_EQ(window->gui_find_control(Vector2(10, 10)), slider);
	CHECK_FALSE(window->gui_find_control(Vector2(100, 100)));
	CHECK_EQ(window->gui_find_control(Vector2(175, 25)), display_monitor);

	// Readout contents ignore mouse input, but their panel still blocks stage clicks.
	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(175, 25));
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::NONE, Vector2(175, 25));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 0);
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 0);

	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(10, 10));
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::NONE, Vector2(10, 10));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 0);
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 0);

	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(100, 100));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 1);

	// A second button gives the slider mouse focus, so it consumes the first
	// button's release even though that press started on the stage.
	push_mouse_button(window, MouseButton::RIGHT, true, (int)MouseButtonMask::LEFT | (int)MouseButtonMask::RIGHT, Vector2(10, 10));
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::RIGHT, Vector2(10, 10));
	push_mouse_button(window, MouseButton::RIGHT, false, MouseButtonMask::NONE, Vector2(10, 10));
	CHECK_EQ(mouse_presses[(int)MouseButton::RIGHT], 0);
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 1);
	CHECK_EQ(mouse_releases[(int)MouseButton::RIGHT], 0);
}

TEST_CASE("[SceneTree][SPX] Input proxy normalizes key codes in priority order") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	InputFixture fixture;
	Window *window = fixture.window;
	const struct KeyCase {
		const char *name;
		char32_t unicode;
		Key label;
		Key logical;
		Key physical;
		Key expected;
	} key_cases[] = {
		{ "unicode", U'!', Key::B, Key::A, Key::C, Key::EXCLAM },
		{ "label", 0, Key::B, Key::A, Key::C, Key::B },
		{ "logical", 0, Key::NONE, Key::A, Key::C, Key::A },
		{ "physical", 0, Key::NONE, Key::NONE, Key::C, Key::C },
	};
	for (const KeyCase &test_case : key_cases) {
		INFO(test_case.name);
		key_presses.clear();
		key_releases.clear();
		Ref<InputEventKey> key;
		key.instantiate();
		key->set_unicode(test_case.unicode);
		key->set_key_label(test_case.label);
		key->set_keycode(test_case.logical);
		key->set_physical_keycode(test_case.physical);
		key->set_pressed(true);
		window->push_input(key, true);
		REQUIRE_EQ(key_presses.size(), 1);
		CHECK_EQ(key_presses[0], (GdInt)test_case.expected);
		CHECK(key_releases.is_empty());

		key->set_pressed(false);
		window->push_input(key, true);
		CHECK_EQ(key_presses.size(), 1);
		REQUIRE_EQ(key_releases.size(), 1);
		CHECK_EQ(key_releases[0], (GdInt)test_case.expected);
	}
}

TEST_CASE("[SceneTree][SPX] Input proxy reuses its node across reset and releases it on destroy") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	InputFixture fixture;
	Window *window = fixture.window;
	Node *root = fixture.root;
	SpxInputMgr *input = fixture.input;
	Node *old_proxy = root->get_node_or_null(NodePath("input_proxy"));
	REQUIRE(old_proxy != nullptr);
	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(100, 100));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 1);

	// Do not advance a frame: reset must stop input immediately without freeing the node.
	input->on_reset(0);
	CHECK_FALSE(old_proxy->is_queued_for_deletion());
	CHECK_FALSE(old_proxy->is_inside_tree());
	push_mouse_button(window, MouseButton::RIGHT, true, (int)MouseButtonMask::LEFT | (int)MouseButtonMask::RIGHT, Vector2(100, 100));
	push_mouse_button(window, MouseButton::RIGHT, false, MouseButtonMask::LEFT, Vector2(100, 100));
	Ref<InputEventKey> key;
	key.instantiate();
	key->set_keycode(Key::A);
	key->set_pressed(true);
	window->push_input(key, true);
	key->set_pressed(false);
	window->push_input(key, true);
	CHECK_EQ(mouse_presses[(int)MouseButton::RIGHT], 0);
	CHECK_EQ(mouse_releases[(int)MouseButton::RIGHT], 0);
	CHECK(key_presses.is_empty());
	CHECK(key_releases.is_empty());

	input->on_start();
	CHECK(root->get_node_or_null(NodePath("input_proxy")) == old_proxy);
	CHECK(old_proxy->is_inside_tree());
	// Reusing the node must not preserve the previous session's pending mouse release.
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::NONE, Vector2(100, 100));
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 0);
	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(100, 100));
	// Starting an active proxy again must neither duplicate it nor clear a pending press.
	input->on_start();
	CHECK_EQ(root->get_child_count(), 1);
	CHECK(root->get_node_or_null(NodePath("input_proxy")) == old_proxy);
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::NONE, Vector2(100, 100));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 2);
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 1);
	key->set_pressed(true);
	window->push_input(key, true);
	key->set_pressed(false);
	window->push_input(key, true);
	REQUIRE_EQ(key_presses.size(), 1);
	REQUIRE_EQ(key_releases.size(), 1);
	CHECK_EQ(key_presses[0], (GdInt)Key::A);
	CHECK_EQ(key_releases[0], (GdInt)Key::A);

	input->on_destroy();
	CHECK(old_proxy->is_queued_for_deletion());
	CHECK_FALSE(old_proxy->is_inside_tree());
	input->on_start();
	Node *new_proxy = root->get_node_or_null(NodePath("input_proxy"));
	REQUIRE(new_proxy != nullptr);
	CHECK(new_proxy != old_proxy);
	CHECK_EQ(root->get_child_count(), 1);
	push_mouse_button(window, MouseButton::LEFT, true, MouseButtonMask::LEFT, Vector2(100, 100));
	push_mouse_button(window, MouseButton::LEFT, false, MouseButtonMask::NONE, Vector2(100, 100));
	CHECK_EQ(mouse_presses[(int)MouseButton::LEFT], 3);
	CHECK_EQ(mouse_releases[(int)MouseButton::LEFT], 2);
	key->set_pressed(true);
	window->push_input(key, true);
	key->set_pressed(false);
	window->push_input(key, true);
	REQUIRE_EQ(key_presses.size(), 2);
	REQUIRE_EQ(key_releases.size(), 2);
	CHECK_EQ(key_presses[1], (GdInt)Key::A);
	CHECK_EQ(key_releases[1], (GdInt)Key::A);
}

} // namespace TestSpxInputProxy

#endif // TEST_SPX_INPUT_PROXY_H
