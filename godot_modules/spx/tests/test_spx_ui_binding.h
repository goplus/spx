/**************************************************************************/
/*  test_spx_ui_binding.h                                                  */
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

#ifndef TEST_SPX_UI_BINDING_H
#define TEST_SPX_UI_BINDING_H

#include "../spx_ui.h"
#include "../spx_ui_binding.h"
#include "scene/gui/button.h"
#include "tests/test_macros.h"

namespace TestSpxUiBinding {

struct BindingListener : SpxUiBindingListener {
	Vector<int> events;
	GdObj clicked_gid = 0;
	GdObj destroyed_gid = 0;
	ObjectID clicked_binding_id;
	ObjectID destroyed_binding_id;

	void on_spx_ui_clicked(GdObj p_gid, ObjectID p_binding_id) override {
		events.push_back(1);
		clicked_gid = p_gid;
		clicked_binding_id = p_binding_id;
	}

	void on_spx_ui_destroyed(GdObj p_gid, ObjectID p_binding_id) override {
		events.push_back(3);
		destroyed_gid = p_gid;
		destroyed_binding_id = p_binding_id;
	}
};

struct PressedListener : Object {
	Vector<int> *events = nullptr;

	void on_pressed() {
		events->push_back(2);
	}
};

TEST_CASE("[SceneTree][SPX] UI binding preserves click order and observes destruction") {
	BindingListener binding_listener;
	PressedListener pressed_listener;
	pressed_listener.events = &binding_listener.events;
	Button *button = memnew(Button);
	button->connect(SNAME("pressed"), callable_mp(&pressed_listener, &PressedListener::on_pressed));

	SpxUiBinding *binding = memnew(SpxUiBinding);
	binding->set_name("_spx_ui_binding");
	button->add_child(binding, false, Node::INTERNAL_MODE_BACK);
	REQUIRE(binding->attach(button, 42, &binding_listener) == OK);
	const ObjectID binding_id = binding->get_instance_id();

	button->emit_signal(SNAME("pressed"));
	REQUIRE(binding_listener.events.size() == 2);
	CHECK(binding_listener.events[0] == 1);
	CHECK(binding_listener.events[1] == 2);
	CHECK(binding_listener.clicked_gid == 42);
	CHECK(binding_listener.clicked_binding_id == binding_id);

	memdelete(button);
	REQUIRE(binding_listener.events.size() == 3);
	CHECK(binding_listener.events[2] == 3);
	CHECK(binding_listener.destroyed_gid == 42);
	CHECK(binding_listener.destroyed_binding_id == binding_id);
}

TEST_CASE("[SceneTree][SPX] Detached UI binding does not call a stale listener") {
	BindingListener binding_listener;
	Button *button = memnew(Button);
	SpxUiBinding *binding = memnew(SpxUiBinding);
	button->add_child(binding, false, Node::INTERNAL_MODE_BACK);
	REQUIRE(binding->attach(button, 43, &binding_listener) == OK);

	binding->detach();
	button->emit_signal(SNAME("pressed"));
	memdelete(button);
	CHECK(binding_listener.events.is_empty());
}

TEST_CASE("[SceneTree][SPX] UI wrapper resolves controls by ObjectID and uses standard layout APIs") {
	SpxUi ui;
	ui.set_gid(7);
	ui.set_type((GdInt)ESpxUiType::Control);
	Control *parent = memnew(Control);
	Control *control = memnew(Control);
	parent->add_child(control);
	REQUIRE(ui.set_control_item(control, nullptr) == OK);

	ui.set_layout_mode((GdInt)Control::LAYOUT_MODE_ANCHORS);
	CHECK(ui.get_layout_mode() == (GdInt)Control::LAYOUT_MODE_ANCHORS);
	ui.set_anchors_preset((GdInt)Control::PRESET_FULL_RECT);
	CHECK(ui.get_anchors_preset() == (GdInt)Control::PRESET_FULL_RECT);

	memdelete(parent);
	CHECK(ui.get_control_item() == nullptr);
}

} // namespace TestSpxUiBinding

#endif // TEST_SPX_UI_BINDING_H
