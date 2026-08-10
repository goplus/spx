/**************************************************************************/
/*  spx_ui_binding.cpp                                                     */
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

#include "spx_ui_binding.h"

#include "core/object/callable_method_pointer.h"
#include "scene/gui/base_button.h"

namespace {
struct PreservedConnection {
	Callable callable;
	uint32_t flags = 0;
	int reference_count = 0;
};
} // namespace

Control *SpxUiBinding::get_control() const {
	return Object::cast_to<Control>(ObjectDB::get_instance(control_id));
}

Error SpxUiBinding::_connect_pressed_first(Control *p_control) {
	BaseButton *button = Object::cast_to<BaseButton>(p_control);
	if (button == nullptr) {
		return OK;
	}

	const StringName pressed_signal = SNAME("pressed");
	const Callable binding_callable = callable_mp(this, &SpxUiBinding::_on_pressed);
	List<Object::Connection> connections;
	button->get_signal_connection_list(pressed_signal, &connections);

	// BaseButton used to invoke the SPX callback immediately before emitting
	// "pressed". Preserve that observable ordering when moving the callback to
	// a standard signal, including for connections restored from PackedScene.
	Vector<PreservedConnection> preserved;
	for (const Object::Connection &connection : connections) {
		PreservedConnection item;
		item.callable = connection.callable;
		item.flags = connection.flags;
		while (button->is_connected(pressed_signal, item.callable)) {
			button->disconnect(pressed_signal, item.callable);
			item.reference_count++;
		}
		if (item.reference_count > 0) {
			preserved.push_back(item);
		}
	}

	Error error = button->connect(pressed_signal, binding_callable);
	if (error != OK) {
		for (const PreservedConnection &connection : preserved) {
			for (int i = 0; i < connection.reference_count; i++) {
				button->connect(pressed_signal, connection.callable, connection.flags);
			}
		}
		return error;
	}

	for (const PreservedConnection &connection : preserved) {
		for (int i = 0; i < connection.reference_count; i++) {
			error = button->connect(pressed_signal, connection.callable, connection.flags);
			if (error != OK) {
				return error;
			}
		}
	}

	return OK;
}

Error SpxUiBinding::attach(Control *p_control, GdObj p_gid, SpxUiBindingListener *p_listener) {
	ERR_FAIL_NULL_V(p_control, ERR_INVALID_PARAMETER);
	ERR_FAIL_COND_V(control_id.is_valid(), ERR_ALREADY_IN_USE);

	control_id = p_control->get_instance_id();
	gid = p_gid;
	listener = p_listener;

	const Error error = _connect_pressed_first(p_control);
	if (error != OK) {
		listener = nullptr;
		gid = 0;
		control_id = ObjectID();
	}
	return error;
}

void SpxUiBinding::detach() {
	Control *control = get_control();
	BaseButton *button = Object::cast_to<BaseButton>(control);
	if (button != nullptr) {
		const Callable binding_callable = callable_mp(this, &SpxUiBinding::_on_pressed);
		if (button->is_connected(SNAME("pressed"), binding_callable)) {
			button->disconnect(SNAME("pressed"), binding_callable);
		}
	}

	listener = nullptr;
	gid = 0;
	control_id = ObjectID();
}

void SpxUiBinding::_on_pressed() {
	SpxUiBindingListener *callback = listener;
	const GdObj callback_gid = gid;
	const ObjectID binding_id = get_instance_id();
	if (callback != nullptr) {
		callback->on_spx_ui_clicked(callback_gid, binding_id);
	}
}

void SpxUiBinding::_notification(int p_what) {
	if (p_what != NOTIFICATION_PREDELETE || listener == nullptr) {
		return;
	}

	SpxUiBindingListener *callback = listener;
	const GdObj callback_gid = gid;
	const ObjectID binding_id = get_instance_id();
	listener = nullptr;
	gid = 0;
	control_id = ObjectID();
	callback->on_spx_ui_destroyed(callback_gid, binding_id);
}
