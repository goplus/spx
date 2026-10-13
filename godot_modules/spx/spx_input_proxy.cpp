/**************************************************************************/
/*  spx_input_proxy.cpp                                                   */
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

#include "spx_input_proxy.h"

#include "core/os/keyboard.h"
#include "spx_engine.h"

// Prefer Unicode so shifted symbols such as '!' keep their character code.
static GdInt normalize_key_event_code(const Ref<InputEventKey> &p_event) {
	const char32_t unicode = p_event->get_unicode();
	if (unicode >= 0x20 && unicode != 0x7F) {
		return (GdInt)fix_keycode(unicode, p_event->get_keycode());
	}
	const Key label = p_event->get_key_label();
	if (label != Key::NONE) {
		return (GdInt)label;
	}
	const Key keycode = p_event->get_keycode();
	return (GdInt)(keycode != Key::NONE ? keycode : p_event->get_physical_keycode());
}

void SpxInputProxy::_notification(int p_what) {
	if (p_what == NOTIFICATION_ENTER_TREE) {
		mouse_buttons_awaiting_release.clear();
		set_process_input(true);
		set_process_unhandled_input(true);
	}
}

void SpxInputProxy::input(const Ref<InputEvent> &p_event) {
	Ref<InputEventKey> key_event = p_event;
	if (key_event.is_valid()) {
		const GdInt keycode = normalize_key_event_code(key_event);
		if (key_event->is_pressed()) {
			SPX_CALLBACK->func_on_key_pressed(keycode);
		} else if (key_event->is_released()) {
			SPX_CALLBACK->func_on_key_released(keycode);
		}
		return;
	}

	Ref<InputEventMouseButton> mouse_event = p_event;
	if (mouse_event.is_null() || mouse_event->is_pressed()) {
		return;
	}

	const MouseButton button = mouse_event->get_button_index();
	const MouseButtonMask mask = mouse_button_to_mask(button);
	if (!mouse_buttons_awaiting_release.has_flag(mask)) {
		return;
	}
	// Release forwarded presses before GUI handling to avoid stuck buttons.
	mouse_buttons_awaiting_release.clear_flag(mask);
	SPX_CALLBACK->func_on_mouse_released((GdInt)button);
}

void SpxInputProxy::unhandled_input(const Ref<InputEvent> &p_event) {
	Ref<InputEventMouseButton> mouse_event = p_event;
	if (mouse_event.is_null() || !mouse_event->is_pressed()) {
		return;
	}

	const MouseButton button = mouse_event->get_button_index();
	const MouseButtonMask mask = mouse_button_to_mask(button);
	if (mask == MouseButtonMask::NONE) {
		return;
	}
	mouse_buttons_awaiting_release.set_flag(mask);
	SPX_CALLBACK->func_on_mouse_pressed((GdInt)button);
}
