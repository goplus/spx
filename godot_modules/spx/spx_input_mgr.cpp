/**************************************************************************/
/*  spx_input_mgr.cpp                                                     */
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

#include "spx_input_mgr.h"

#include "scene/main/window.h"

#include "gdextension_spx_ext.h"
#include "spx_camera_mgr.h"
#include "spx_coordinate.h"
#include "spx_engine.h"

void SpxInputMgr::on_start() {
	if (!input_proxy) {
		input_proxy = memnew(SpxInputProxy);
		input_proxy->set_name("input_proxy");
	}
	if (input_proxy->get_parent()) {
		return;
	}
	get_spx_root()->add_child(input_proxy);
}

void SpxInputMgr::on_destroy() {
	if (input_proxy) {
		if (input_proxy->get_parent()) {
			input_proxy->get_parent()->remove_child(input_proxy);
		}
		input_proxy->queue_free();
		input_proxy = nullptr;
	}
}

void SpxInputMgr::on_reset(int reset_code) {
	// Stop input immediately while keeping the proxy for the next run.
	if (input_proxy && input_proxy->get_parent()) {
		input_proxy->get_parent()->remove_child(input_proxy);
	}
	action_names.clear();
	action_ids.clear();
}

GdVec2 SpxInputMgr::get_global_mouse_pos() {
	auto mouse_pos = cameraMgr->get_global_mouse_position();
	return godot_to_spx_vec2(mouse_pos);
}

GdBool SpxInputMgr::get_mouse_state(GdInt mouse_id) {
	return Input::get_singleton()->is_mouse_button_pressed((MouseButton)mouse_id);
}

GdBool SpxInputMgr::get_key(GdInt key) {
	Input *input = Input::get_singleton();
	if (key == KEY_ANY) {
		return input->is_anything_pressed_except_mouse();
	}
	return input->is_key_pressed((Key)key) ||
			input->is_key_label_pressed((Key)key);
}

GdInt SpxInputMgr::get_key_state(GdInt key) {
	return get_key(key) ? 1 : 0;
}

GdFloat SpxInputMgr::get_axis(GdString neg_action, GdString pos_action) {
	return Input::get_singleton()->get_axis(SpxStr(neg_action), SpxStr(pos_action));
}

GdBool SpxInputMgr::is_action_pressed(GdString action) {
	return Input::get_singleton()->is_action_pressed(SpxStr(action));
}

GdBool SpxInputMgr::is_action_just_pressed(GdString action) {
	return Input::get_singleton()->is_action_just_pressed(SpxStr(action));
}

GdBool SpxInputMgr::is_action_just_released(GdString action) {
	return Input::get_singleton()->is_action_just_released(SpxStr(action));
}

GdInt SpxInputMgr::register_action(GdString action) {
	StringName name(SpxStr(action));
	if (const GdInt *id = action_ids.getptr(name)) {
		return *id;
	}

	GdInt id = action_names.size();
	action_names.push_back(name);
	action_ids.insert(name, id);
	return id;
}

GdFloat SpxInputMgr::get_axis_id(GdInt neg_action_id, GdInt pos_action_id) {
	const StringName *neg = get_registered_action(neg_action_id);
	const StringName *pos = get_registered_action(pos_action_id);
	if (neg == nullptr || pos == nullptr) {
		return 0;
	}
	return Input::get_singleton()->get_axis(*neg, *pos);
}

GdBool SpxInputMgr::is_action_pressed_id(GdInt action_id) {
	const StringName *action = get_registered_action(action_id);
	return action != nullptr && Input::get_singleton()->is_action_pressed(*action);
}

GdBool SpxInputMgr::is_action_just_pressed_id(GdInt action_id) {
	const StringName *action = get_registered_action(action_id);
	return action != nullptr && Input::get_singleton()->is_action_just_pressed(*action);
}

GdBool SpxInputMgr::is_action_just_released_id(GdInt action_id) {
	const StringName *action = get_registered_action(action_id);
	return action != nullptr && Input::get_singleton()->is_action_just_released(*action);
}

void SpxInputMgr::write_snapshot(float out[3]) {
	if (!out) {
		return;
	}

	GdVec2 pos = get_global_mouse_pos();
	BitField<MouseButtonMask> mouse_bits;
	Input *input = Input::get_singleton();
	for (MouseButton button : { MouseButton::LEFT, MouseButton::RIGHT, MouseButton::MIDDLE }) {
		if (input->is_mouse_button_pressed(button)) {
			mouse_bits.set_flag(mouse_button_to_mask(button));
		}
	}

	out[0] = pos.x;
	out[1] = pos.y;
	out[2] = (float)mouse_bits;
}

const StringName *SpxInputMgr::get_registered_action(GdInt action_id) const {
	if (action_id < 0 || action_id >= action_names.size()) {
		return nullptr;
	}
	return &action_names[action_id];
}
