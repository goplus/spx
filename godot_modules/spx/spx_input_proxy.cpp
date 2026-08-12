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

// Printable Unicode is normalized through fix_keycode() so shifted symbols
// such as '!' map correctly before falling back to label, logical, then physical keycodes.
static GdInt normalize_key_event_code(const Ref<InputEventKey> &p_event) {
	const char32_t unicode = p_event->get_unicode();
	if (unicode >= 0x20 && unicode != 0x7F) {
		return (GdInt)fix_keycode(unicode, p_event->get_keycode());
	}
	Key key = p_event->get_key_label();
	if (key == Key::NONE) {
		key = p_event->get_keycode();
		if (key == Key::NONE) {
			key = p_event->get_physical_keycode();
		}
	}
	return (GdInt)key;
}

void SpxInputProxy::ready() {
	set_process_input(true);
}

void SpxInputProxy::input(const Ref<InputEvent> &p_event) {
	Ref<InputEventKey> k = p_event;
	if (k.is_valid()) {
		const GdInt keyid = normalize_key_event_code(k);
		if (k->is_pressed()) {
			SPX_CALLBACK->func_on_key_pressed(keyid);
		} else if (k->is_released()) {
			SPX_CALLBACK->func_on_key_released(keyid);
		}
		return;
	}

	Ref<InputEventMouseButton> mb = p_event;
	if (!mb.is_valid()) {
		return;
	}

	if (mb->is_pressed()) {
		SPX_CALLBACK->func_on_mouse_pressed((GdInt)mb->get_button_index());
	} else if (mb->is_released()) {
		SPX_CALLBACK->func_on_mouse_released((GdInt)mb->get_button_index());
	}
}
