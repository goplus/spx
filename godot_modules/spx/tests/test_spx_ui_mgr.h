/**************************************************************************/
/*  test_spx_ui_mgr.h                                                   */
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

#ifndef TEST_SPX_UI_MGR_H
#define TEST_SPX_UI_MGR_H

#include "../spx_list_monitor.h"
#include "../spx_ui_mgr.h"
#include "spx_test_fixtures.h"

namespace TestSpxUiMgr {

TEST_CASE("[SceneTree][SPX] UI manager creates typed controls and preserves text") {
	TestSpx::EngineScene scene;
	SpxUiMgr *ui = scene.engine->get_ui();
	const GdObj ids[] = {
		ui->create_label("", "label"),
		ui->create_button("", "button"),
		ui->create_input("", "input"),
		ui->create_toggle("", true),
	};
	const ESpxUiType types[] = { ESpxUiType::Label, ESpxUiType::Button, ESpxUiType::Input, ESpxUiType::Toggle };
	const char *texts[] = { "label", "button", "input", "" };
	for (int i = 0; i < 4; i++) {
		REQUIRE(ids[i] != NULL_OBJECT_ID);
		CHECK(ui->get_type(ids[i]) == (int)types[i]);
		CHECK(TestSpx::take_string(ui->get_text(ids[i])) == texts[i]);
		ui->set_text(ids[i], "你好 SPX");
		CHECK(TestSpx::take_string(ui->get_text(ids[i])) == String::utf8("你好 SPX"));
		CHECK(ui->get_node_type(ui->get_node(ids[i])->get_control()) == types[i]);
	}
	CHECK(ui->get_node(ids[3])->get_toggle()->is_pressed());
	const GdObj unchecked = ui->create_toggle("", false);
	CHECK_FALSE(ui->get_node(unchecked)->get_toggle()->is_pressed());
	CHECK(ui->get_node_type(nullptr) == ESpxUiType::None);
	CHECK(ui->get_node_type(scene.root) == ESpxUiType::None);
	CHECK(ui->set_text_max_width(ids[0], "wide label", 10000) > 0);
	ui->on_reset(0);
	for (GdObj id : ids) {
		CHECK(ui->get_node(id) == nullptr);
	}
}

TEST_CASE("[SceneTree][SPX] UI transforms styling and image flips reach the control") {
	TestSpx::EngineScene scene;
	SpxUiMgr *ui = scene.engine->get_ui();
	const GdObj id = ui->create_image("");
	REQUIRE(id != NULL_OBJECT_ID);
	Control *control = ui->get_node(id)->get_control();
	REQUIRE(control != nullptr);
	Node *canvas = control->get_parent();
	Control *parent = memnew(Control);
	canvas->add_child(parent);
	canvas->remove_child(control);
	parent->add_child(control);
	CHECK(ui->get_node_type(control) == ESpxUiType::Image);
	ui->set_rect(id, Rect2(10, 20, 100, 80));
	CHECK(Rect2(ui->get_rect(id)) == Rect2(10, 20, 100, 80));
	CHECK(control->get_rect() == Rect2(10, 20, 100, 80));
	ui->set_size(id, Vector2(200, 100));
	CHECK(Vector2(ui->get_size(id)) == Vector2(200, 100));
	ui->set_position(id, Vector2(15, 25));
	CHECK(Vector2(ui->get_position(id)) == Vector2(15, 25));
	ui->set_global_position(id, Vector2(30, 40));
	CHECK(Vector2(ui->get_global_position(id)) == Vector2(30, 40));
	ui->set_scale(id, Vector2(2, 3));
	CHECK(Vector2(ui->get_scale(id)) == Vector2(2, 3));
	CHECK(Vector2(ui->get_canvas_scale(id)) == Vector2(2, 3));
	ui->set_rotation(id, 0.5);
	CHECK(ui->get_rotation(id) == doctest::Approx(0.5));
	CHECK(control->get_rotation() == doctest::Approx(0.5));
	const Color tint(0.25, 0.5, 0.75, 0.5);
	ui->set_color(id, tint);
	CHECK(Color(ui->get_color(id)) == tint);
	CHECK(control->get_self_modulate() == tint);
	ui->set_font_size(id, 24);
	CHECK(ui->get_font_size(id) == 24);
	for (bool visible : { false, true }) {
		ui->set_visible(id, visible);
		CHECK(ui->get_visible(id) == visible);
		CHECK(control->is_visible() == visible);
	}
	ui->set_layout_direction(id, Control::LAYOUT_DIRECTION_RTL);
	CHECK(ui->get_layout_direction(id) == Control::LAYOUT_DIRECTION_RTL);
	ui->set_layout_mode(id, Control::LAYOUT_MODE_ANCHORS);
	CHECK(ui->get_layout_mode(id) == Control::LAYOUT_MODE_ANCHORS);
	ui->set_anchors_preset(id, Control::PRESET_FULL_RECT);
	CHECK(ui->get_anchors_preset(id) == Control::PRESET_FULL_RECT);
	for (bool horizontal : { true, false }) {
		for (bool flip : { true, false }) {
			ui->set_flip(id, horizontal, flip);
			CHECK(ui->get_flip(id, horizontal) == flip);
		}
	}
	CHECK(ui->get_texture(id) == nullptr);
	CHECK(ui->destroy_node(id));
	CHECK(control->is_queued_for_deletion());
}

struct RangeListener : Object {
	int changes = 0;
	void changed(double) { changes++; }
};

TEST_CASE("[SceneTree][SPX] UI range updates clamp and snap without user input signals") {
	TestSpx::EngineScene scene;
	SpxUiMgr *ui = scene.engine->get_ui();
	const GdObj id = ui->create_slider("", 10);
	REQUIRE(id != NULL_OBJECT_ID);
	HSlider *slider = ui->get_node(id)->get_slider();
	REQUIRE(slider != nullptr);
	CHECK(ui->get_range_value(id) == 10);
	CHECK(ui->get_node_type(slider) == ESpxUiType::Slider);
	RangeListener listener;
	slider->connect("value_changed", callable_mp(&listener, &RangeListener::changed));
	ui->set_range(id, -5, 5, 0.5, 2.3);
	CHECK(slider->get_min() == -5);
	CHECK(slider->get_max() == 5);
	CHECK(slider->get_step() == 0.5);
	CHECK(ui->get_range_value(id) == 2.5);
	CHECK_FALSE(slider->is_blocking_signals());
	ui->set_range(id, -5, 5, 0.5, 100);
	CHECK(ui->get_range_value(id) == 5);
	slider->set_block_signals(true);
	ui->set_range(id, -5, 5, 0.5, -100);
	CHECK(ui->get_range_value(id) == -5);
	CHECK(slider->is_blocking_signals());
	CHECK(listener.changes == 0);
	slider->set_block_signals(false);
	slider->set_value(0);
	CHECK(listener.changes == 1);
}

TEST_CASE("[SceneTree][SPX] UI binding preserves an authored child and rejects invalid paths") {
	TestSpx::EngineScene scene;
	SpxUiMgr *ui = scene.engine->get_ui();
	Control *parent = memnew(Control);
	SpxUi *wrapper = ui->on_create_node(parent, (int)ESpxUiType::Control);
	REQUIRE(wrapper != nullptr);
	CheckButton *child = memnew(CheckButton);
	child->set_name("Choice");
	parent->add_child(child);
	const GdObj id = ui->bind_node(wrapper->get_gid(), "Choice");
	REQUIRE(id != NULL_OBJECT_ID);
	CHECK(ui->get_type(id) == (int)ESpxUiType::Toggle);
	CHECK(ui->get_node(id)->get_control() == child);
	CHECK(child->get_parent() == parent);
	CHECK(ui->bind_node(wrapper->get_gid(), "missing") == NULL_OBJECT_ID);
	CHECK(ui->bind_node(0, "Choice") == NULL_OBJECT_ID);
	CHECK(ui->on_create_node(nullptr, (int)ESpxUiType::Control) == nullptr);
	ERR_PRINT_OFF;
	CHECK_FALSE(ui->destroy_node(0));
	CHECK_FALSE(ui->get_visible(0));
	CHECK(ui->get_type(0) == 0);
	CHECK(ui->get_text(0) == nullptr);
	ERR_PRINT_ON;
}

class ListMonitor : public SpxListMonitor {
public:
	using SpxListMonitor::gui_input;
};

TEST_CASE("[SceneTree][SPX] List monitor scrolls long lists and clears stale content") {
	TestSpx::EngineScene scene;
	SpxUiMgr *ui = scene.engine->get_ui();
	ListMonitor *monitor = memnew(ListMonitor);
	SpxUi *wrapper = ui->on_create_node(monitor, (int)ESpxUiType::Control);
	REQUIRE(wrapper != nullptr);
	monitor->set_size(Size2(200, 144));
	auto *scroll = Object::cast_to<VScrollBar>(monitor->get_node(NodePath("ScrollBar")));
	REQUIRE(scroll != nullptr);
	GdString values[10] = { "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten" };
	GdArrayInfo array = {};
	array.type = GD_ARRAY_TYPE_STRING;
	array.size = 10;
	array.data = values;
	ui->set_list_items(wrapper->get_gid(), "items", &array, Color(1, 0, 0));
	CHECK(scroll->is_visible());
	CHECK(scroll->get_max() == 240);
	CHECK(scroll->get_page() == 100);
	MessageQueue::get_singleton()->flush();
	Ref<InputEventMouseButton> wheel;
	wheel.instantiate();
	wheel->set_button_index(MouseButton::WHEEL_DOWN);
	wheel->set_pressed(true);
	monitor->gui_input(wheel);
	CHECK(scroll->get_value() == 72);
	MessageQueue::get_singleton()->flush();
	wheel->set_button_index(MouseButton::WHEEL_UP);
	monitor->gui_input(wheel);
	CHECK(scroll->get_value() == 0);
	Ref<InputEventPanGesture> pan;
	pan.instantiate();
	pan->set_delta(Vector2(0, 1));
	monitor->gui_input(pan);
	CHECK(scroll->get_value() == 24);
	Ref<InputEventScreenDrag> drag;
	drag.instantiate();
	drag->set_relative(Vector2(0, 8));
	monitor->gui_input(drag);
	CHECK(scroll->get_value() == 16);
	ui->set_list_items(wrapper->get_gid(), "empty", nullptr, Color(0, 1, 0));
	CHECK_FALSE(scroll->is_visible());
	CHECK(scroll->get_value() == 0);
	CHECK(scroll->get_max() == scroll->get_page());
	MessageQueue::get_singleton()->flush();
}

TEST_CASE("[SceneTree][SPX] Authored UI scenes forward clicks and invalidate destroyed bindings") {
	TestSpx::TempDirectory files("spx_authored_ui");
	files.write("button.tscn", "[gd_scene format=3]\n[node name=\"Button\" type=\"Button\"]\ntext = \"Authored\"\n");
	files.write("control.tscn", "[gd_scene format=3]\n[node name=\"Root\" type=\"Control\"]\n");
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	auto *ui = scene.engine->get_ui();
	static GdObj clicked, destroyed;
	clicked = 0;
	destroyed = 0;
	scene.engine->get_callbacks()->func_on_ui_clicked = [](GdObj id) { clicked = id; };
	scene.engine->get_callbacks()->func_on_ui_destroyed = [](GdObj id) { destroyed = id; };
	const auto path = files.file("button.tscn").utf8();
	GdObj id = ui->create_node(path.get_data());
	REQUIRE(id != 0);
	auto *button = ui->get_node(id)->get_button();
	REQUIRE(button);
	CHECK(button->get_text() == "Authored");
	button->emit_signal("pressed");
	CHECK(clicked == id);
	memdelete(button);
	CHECK(destroyed == id);
	CHECK(ui->get_node(id) == nullptr);
	const auto control_path = files.file("control.tscn").utf8();
	GdObj control = ui->create_node(control_path.get_data());
	REQUIRE(control != 0);
	CHECK(ui->get_type(control) == (GdInt)ESpxUiType::Control);
	CHECK(ui->create_node("") == 0);
}
TEST_CASE("[SceneTree][SPX] UI texture bindings share resources across images buttons and toggles") {
	TestSpx::EngineScene scene;
	auto *ui = scene.engine->get_ui();
	auto image = Image::create_empty(4, 4, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0));
	Ref<ImageTexture> texture = ImageTexture::create_from_image(image);
	const String path = "res://spx_ui_texture_test.tres";
	texture->set_path(path);
	texture->set_name("icon");
	const auto text = path.utf8();
	GdObj ids[] = { ui->create_image(""), ui->create_button("", ""), ui->create_toggle("", false) };
	for (GdObj id : ids) {
		ui->set_texture(id, text.get_data());
		CHECK(TestSpx::take_string(ui->get_texture(id)) == "icon");
	}
	CHECK(ui->get_node(ids[0])->get_image()->get_texture() == texture);
	CHECK(ui->get_node(ids[1])->get_button()->get_button_icon() == texture);
	CHECK(ui->get_node(ids[2])->get_toggle()->get_button_icon() == texture);
}

} // namespace TestSpxUiMgr

#endif // TEST_SPX_UI_MGR_H
