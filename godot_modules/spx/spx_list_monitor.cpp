/**************************************************************************/
/*  spx_list_monitor.cpp                                                          */
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

#include "spx_list_monitor.h"

#include "scene/resources/text_line.h"

void SpxListMonitor::_bind_methods() {
	ClassDB::bind_method(D_METHOD("set_items", "label", "items", "color"), &SpxListMonitor::set_items);
}

SpxListMonitor::SpxListMonitor() {
	set_custom_minimum_size(Size2(100, 60));
	set_clip_contents(true);
	set_mouse_filter(MOUSE_FILTER_STOP);
	panel_style.instantiate();
	panel_style->set_bg_color(Color(0.898039, 0.941176, 1));
	panel_style->set_border_width_all(1);
	panel_style->set_border_color(Color(0, 0, 0, 0.15));
	panel_style->set_corner_radius_all(4);
	row_style.instantiate();
	row_style->set_bg_color(Color(1, 0.4, 0.1));
	row_style->set_border_width_all(1);
	row_style->set_border_color(Color(0, 0, 0, 0.15));
	row_style->set_corner_radius_all(4);
	scroll = memnew(VScrollBar);
	scroll->set_name("ScrollBar");
	scroll->set_step(0);
	add_child(scroll);
	scroll->connect("value_changed", callable_mp(this, &SpxListMonitor::scroll_changed));
}

void SpxListMonitor::set_items(const String &p_label, const PackedStringArray &p_items, const Color &p_color) {
	label = p_label;
	items = p_items;
	row_style->set_bg_color(p_color);
	update_scroll();
	queue_redraw();
}

void SpxListMonitor::update_scroll() {
	const double page = MAX(0.0, get_size().y - BAR_HEIGHT * 2);
	const double total = double(items.size()) * ROW_HEIGHT;
	scroll->set_position(Vector2(get_size().x - scroll->get_combined_minimum_size().x - 1, BAR_HEIGHT));
	scroll->set_size(Size2(scroll->get_combined_minimum_size().x, page));
	scroll->set_max(MAX(total, page));
	scroll->set_page(page);
	scroll->set_visible(total > page);
}

void SpxListMonitor::scroll_changed(double p_value) {
	queue_redraw();
}

void SpxListMonitor::draw_text(const String &p_text, const Rect2 &p_rect, const Color &p_color, HorizontalAlignment p_alignment) {
	Ref<TextLine> line;
	line.instantiate();
	line->add_string(p_text, get_theme_font("font", "Label"), FONT_SIZE);
	line->set_width(MAX(0.0, p_rect.size.x));
	line->set_horizontal_alignment(p_alignment);
	line->set_text_overrun_behavior(TextServer::OVERRUN_TRIM_ELLIPSIS);
	line->draw(get_canvas_item(), p_rect.position + Vector2(0, (p_rect.size.y - line->get_size().y) / 2), p_color);
}

void SpxListMonitor::_notification(int p_what) {
	switch (p_what) {
		case NOTIFICATION_RESIZED:
		case NOTIFICATION_THEME_CHANGED:
		case NOTIFICATION_TRANSLATION_CHANGED:
			update_scroll();
			queue_redraw();
			break;
		case NOTIFICATION_DRAW: {
			const Size2 size = get_size();
			const Color text_color(0.341176, 0.368627, 0.458824);
			draw_style_box(panel_style, Rect2(Vector2(), size));
			const float right = size.x - (scroll->is_visible() ? scroll->get_size().x + 1 : 0);
			const float index_width = get_theme_font("font", "Label")->get_string_size(itos(items.size()), HORIZONTAL_ALIGNMENT_LEFT, -1, FONT_SIZE).x + 10;
			const double offset = scroll->get_value();
			const int first = int(offset / ROW_HEIGHT);
			const int end = MIN(items.size(), int(Math::ceil((offset + size.y - BAR_HEIGHT * 2) / ROW_HEIGHT)));
			for (int i = first; i < end; i++) {
				const float y = BAR_HEIGHT + i * double(ROW_HEIGHT) - offset;
				draw_text(itos(i + 1), Rect2(2, y, index_width - 4, ROW_HEIGHT), text_color, HORIZONTAL_ALIGNMENT_RIGHT);
				const Rect2 row(index_width + 2, y + 1, MAX(0.0f, right - index_width - 5), ROW_HEIGHT - 2);
				draw_style_box(row_style, row);
				draw_text(items[i].get_slice("\n", 0).get_slice("\r", 0), row.grow_individual(-5, 0, -5, 0), Color(1, 1, 1));
			}
			if (items.is_empty()) {
				draw_text(atr("(empty)"), Rect2(2, BAR_HEIGHT + 3, right - 4, ROW_HEIGHT), text_color, HORIZONTAL_ALIGNMENT_CENTER);
			}
			// Opaque bars also clip partially visible rows at the viewport edges.
			draw_rect(Rect2(1, 1, size.x - 2, BAR_HEIGHT - 1), Color(1, 1, 1));
			draw_rect(Rect2(1, size.y - BAR_HEIGHT, size.x - 2, BAR_HEIGHT - 1), Color(1, 1, 1));
			draw_line(Vector2(1, BAR_HEIGHT), Vector2(size.x - 1, BAR_HEIGHT), Color(0, 0, 0, 0.15));
			draw_text(label, Rect2(4, 0, size.x - 8, BAR_HEIGHT), text_color, HORIZONTAL_ALIGNMENT_CENTER);
			draw_text(atr("length %d").replace("%d", itos(items.size())), Rect2(4, size.y - BAR_HEIGHT, size.x - 8, BAR_HEIGHT), text_color, HORIZONTAL_ALIGNMENT_CENTER);
			break;
		}
	}
}

void SpxListMonitor::gui_input(const Ref<InputEvent> &p_event) {
	Ref<InputEventMouseButton> mouse = p_event;
	if (mouse.is_valid() && mouse->is_pressed()) {
		if (mouse->get_button_index() == MouseButton::WHEEL_UP || mouse->get_button_index() == MouseButton::WHEEL_DOWN) {
			const int direction = mouse->get_button_index() == MouseButton::WHEEL_UP ? -1 : 1;
			scroll->set_value(scroll->get_value() + direction * ROW_HEIGHT * 3 * mouse->get_factor());
			accept_event();
		}
	}
	Ref<InputEventPanGesture> pan = p_event;
	if (pan.is_valid()) {
		scroll->set_value(scroll->get_value() + pan->get_delta().y * ROW_HEIGHT);
		accept_event();
	}
	Ref<InputEventScreenDrag> drag = p_event;
	if (drag.is_valid()) {
		scroll->set_value(scroll->get_value() - drag->get_relative().y);
		accept_event();
	}
}
