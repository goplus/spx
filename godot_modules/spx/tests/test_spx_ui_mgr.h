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
#include "scene/resources/packed_scene.h"
#include "spx_test_fixtures.h"

namespace TestSpxUiMgr {

static int orphan_node_count() {
	static int count;
	count = 0;
	ObjectDB::debug_objects([](Object *object) {
		Node *node = Object::cast_to<Node>(object);
		if (node != nullptr && !node->is_inside_tree()) {
			count++;
		}
	});
	return count;
}

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
	Node *non_control = memnew(Node);
	non_control->set_name("NotAControl");
	parent->add_child(non_control);
	CHECK(ui->bind_node(wrapper->get_gid(), "NotAControl") == NULL_OBJECT_ID);
	CHECK(ui->on_create_node(nullptr, (int)ESpxUiType::Control) == nullptr);
	ERR_PRINT_OFF;
	CHECK_FALSE(ui->destroy_node(0));
	CHECK_FALSE(ui->get_visible(0));
	CHECK(ui->get_type(0) == 0);
	CHECK(ui->get_text(0) == nullptr);
	ERR_PRINT_ON;
	const GdObj parent_id = wrapper->get_gid();
	memdelete(parent);
	CHECK(ui->bind_node(parent_id, "Choice") == NULL_OBJECT_ID);
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
	// Malformed bridge arrays must leave the current list intact.
	for (GdArrayInfo invalid : { GdArrayInfo{ 10, GD_ARRAY_TYPE_INT64, values },
				 GdArrayInfo{ -1, GD_ARRAY_TYPE_STRING, values },
				 GdArrayInfo{ 10, GD_ARRAY_TYPE_STRING, nullptr } }) {
		ERR_PRINT_OFF;
		ui->set_list_items(wrapper->get_gid(), "invalid", &invalid, Color());
		ERR_PRINT_ON;
		CHECK(scroll->is_visible());
		CHECK(scroll->get_max() == 240);
	}
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
	GdArrayInfo empty = {};
	empty.type = GD_ARRAY_TYPE_STRING;
	ui->set_list_items(wrapper->get_gid(), "empty array", &empty, Color(0, 1, 0));
	CHECK_FALSE(scroll->is_visible());
	CHECK(scroll->get_value() == 0);
	CHECK(scroll->get_max() == scroll->get_page());
	ui->set_list_items(wrapper->get_gid(), "items", &array, Color(1, 0, 0));
	CHECK(scroll->is_visible());
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

TEST_CASE("[SceneTree][SPX] UI scene creation releases incompatible roots and preserves typed defaults") {
	TestSpx::TempDirectory files("spx_ui_scene_types");
	files.write("node.tscn", "[gd_scene format=3]\n[node name=\"Root\" type=\"Node\"]\n[node name=\"Child\" type=\"Node\" parent=\".\"]\n");
	files.write("label.tscn", "[gd_scene format=3]\n[node name=\"Root\" type=\"Label\"]\ntext = \"Authored\"\n");
	files.write("toggle.tscn", "[gd_scene format=3]\n[node name=\"Root\" type=\"CheckButton\"]\nbutton_pressed = true\ntext = \"Choice\"\n");
	TestSpx::EngineScene scene;
	auto *ui = scene.engine->get_ui();
	const int orphans = orphan_node_count();
	const auto node_path = files.file("node.tscn").utf8();
	CHECK(ui->create_node(node_path.get_data()) == NULL_OBJECT_ID);
	CHECK(orphan_node_count() == orphans);

	const auto label_path = files.file("label.tscn").utf8();
	const GdObj button = ui->create_button(label_path.get_data(), "Fallback");
	REQUIRE(button != NULL_OBJECT_ID);
	CHECK(ui->get_node(button)->get_button() != nullptr);
	CHECK(TestSpx::take_string(ui->get_text(button)) == "Fallback");
	CHECK(orphan_node_count() == orphans);

	const GdObj label = ui->create_label(label_path.get_data(), "Updated");
	REQUIRE(label != NULL_OBJECT_ID);
	CHECK(ui->get_node(label)->get_label()->get_name() == "Root");
	CHECK(TestSpx::take_string(ui->get_text(label)) == "Updated");
	const auto toggle_path = files.file("toggle.tscn").utf8();
	const GdObj toggle = ui->create_toggle(toggle_path.get_data(), false);
	REQUIRE(toggle != NULL_OBJECT_ID);
	CHECK_FALSE(ui->get_node(toggle)->get_toggle()->is_pressed());
	CHECK(TestSpx::take_string(ui->get_text(toggle)) == "Choice");

	const auto missing_path = files.file("missing.tscn").utf8();
	ERR_PRINT_OFF;
	CHECK(ui->create_node(missing_path.get_data()) == NULL_OBJECT_ID);
	ERR_PRINT_ON;
	CHECK(orphan_node_count() == orphans);

	// A valid resource can still contain no scene root to instantiate.
	Ref<PackedScene> empty_scene;
	empty_scene.instantiate();
	const String empty_path = "res://spx_empty_ui_scene.tscn";
	empty_scene->set_path(empty_path);
	const auto encoded_empty_path = empty_path.utf8();
	const int children = scene.root->get_child_count(true);
	ERR_PRINT_OFF;
	CHECK(ui->create_node(encoded_empty_path.get_data()) == NULL_OBJECT_ID);
	ERR_PRINT_ON;
	CHECK(orphan_node_count() == orphans);
	CHECK(scene.root->get_child_count(true) == children);
	CHECK(TestSpx::take_string(ui->get_text(button)) == "Fallback");
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
	ERR_PRINT_OFF;
	ui->set_texture(ids[0], "res://spx_missing_icon.png");
	ui->set_text(ids[0], "unsupported");
	CHECK(TestSpx::take_string(ui->get_text(ids[0])).is_empty());
	const GdObj label = ui->create_label("", "keep text");
	ui->set_texture(label, text.get_data());
	CHECK(ui->get_texture(label) == nullptr);
	ERR_PRINT_ON;
	CHECK(ui->get_node(ids[0])->get_image()->get_texture() == texture);
	CHECK(TestSpx::take_string(ui->get_text(label)) == "keep text");
}

TEST_CASE("[SceneTree][SPX] Stale UI handles cannot mutate surviving controls") {
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	auto *ui = scene.engine->get_ui();
	const GdObj stale = ui->create_label("", "removed");
	Control *removed = ui->get_node(stale)->get_control();
	REQUIRE(ui->destroy_node(stale));
	memdelete(removed);
	REQUIRE(ui->get_node(stale) == nullptr);
	const GdObj live = ui->create_label("", "unchanged");
	ui->set_position(live, Vector2(12, 34));
	ui->set_color(live, Color(0.2, 0.4, 0.6));
	ERR_PRINT_OFF;
	ui->set_text(stale, "changed");
	ui->set_texture(stale, "missing.png");
	ui->set_rect(stale, Rect2(1, 2, 3, 4));
	ui->set_color(stale, Color(1, 0, 0));
	ui->set_font_size(stale, 72);
	ui->set_visible(stale, false);
	ui->set_interactable(stale, false);
	ui->set_layout_direction(stale, Control::LAYOUT_DIRECTION_RTL);
	ui->set_layout_mode(stale, Control::LAYOUT_MODE_ANCHORS);
	ui->set_anchors_preset(stale, Control::PRESET_FULL_RECT);
	ui->set_scale(stale, Vector2(3, 4));
	ui->set_position(stale, Vector2(50, 60));
	ui->set_size(stale, Vector2(70, 80));
	ui->set_global_position(stale, Vector2(90, 100));
	ui->set_rotation(stale, 2);
	ui->set_flip(stale, true, true);
	ui->set_range(stale, 0, 100, 1, 50);
	ui->set_list_items(stale, "items", nullptr, Color());
	CHECK(ui->get_type(stale) == 0);
	CHECK(ui->get_text(stale) == nullptr);
	CHECK(ui->get_texture(stale) == nullptr);
	CHECK(ui->set_text_max_width(stale, "wrapped", 100) == 0);
	CHECK(Rect2(ui->get_rect(stale)) == Rect2());
	CHECK(Color(ui->get_color(stale)) == Color(GdColor()));
	CHECK(ui->get_font_size(stale) == 0);
	CHECK_FALSE(ui->get_visible(stale));
	CHECK_FALSE(ui->get_interactable(stale));
	CHECK(ui->get_layout_direction(stale) == 0);
	CHECK(ui->get_layout_mode(stale) == 0);
	CHECK(ui->get_anchors_preset(stale) == 0);
	CHECK(Vector2(ui->get_scale(stale)) == Vector2());
	CHECK(Vector2(ui->get_canvas_scale(stale)) == Vector2());
	CHECK(Vector2(ui->get_position(stale)) == Vector2());
	CHECK(Vector2(ui->get_size(stale)) == Vector2());
	CHECK(Vector2(ui->get_global_position(stale)) == Vector2());
	CHECK(ui->get_rotation(stale) == 0);
	CHECK_FALSE(ui->get_flip(stale, false));
	CHECK(ui->get_range_value(stale) == 0);
	ui->set_list_items(live, "wrong type", nullptr, Color());
	ui->set_range(live, 0, 100, 1, 50);
	CHECK(ui->get_range_value(live) == 0);
	ui->set_flip(live, true, true);
	CHECK_FALSE(ui->get_flip(live, true));
	CHECK(ui->set_text_max_width(ui->create_image(""), "wrong type", 100) == 0);
	ERR_PRINT_ON;
	CHECK(TestSpx::take_string(ui->get_text(live)) == "unchanged");
	CHECK(Vector2(ui->get_position(live)) == Vector2(12, 34));
	CHECK(Color(ui->get_color(live)) == Color(0.2, 0.4, 0.6));
	CHECK(ui->get_visible(live));
}

TEST_CASE("[SceneTree][SPX] Detached UI wrappers reject property access and mismatched types") {
	TestSpx::EngineScene scene;
	auto *ui = scene.engine->get_ui();
	const GdObj id = ui->create_button("", "keep");
	SpxUi *wrapper = ui->get_node(id);
	wrapper->detach_control_binding();
	ERR_PRINT_OFF;
	wrapper->queue_free();
	wrapper->set_rect(Rect2(1, 2, 3, 4));
	wrapper->set_color(Color(1, 0, 0));
	wrapper->set_font_size(24);
	wrapper->set_visible(true);
	wrapper->set_layout_mode(Control::LAYOUT_MODE_ANCHORS);
	CHECK(Rect2(wrapper->get_rect()) == Rect2());
	CHECK(Color(wrapper->get_color()) == Color(GdColor()));
	CHECK(wrapper->get_font_size() == 0);
	CHECK_FALSE(wrapper->get_visible());
	CHECK(wrapper->get_layout_mode() == 0);
	CHECK(wrapper->get_anchors_preset() == 0);
	CHECK(wrapper->set_control_item(nullptr, nullptr) == ERR_INVALID_PARAMETER);
	CHECK(wrapper->get_label() == nullptr);
	CHECK(wrapper->get_image() == nullptr);
	CHECK(wrapper->get_toggle() == nullptr);
	CHECK(wrapper->get_input() == nullptr);
	CHECK(wrapper->get_slider() == nullptr);
	wrapper->set_type((int)ESpxUiType::Label);
	CHECK(wrapper->get_button() == nullptr);
	CHECK(wrapper->set_text_max_width("detached", 100) == 0);
	ERR_PRINT_ON;
	CHECK_FALSE(wrapper->owns_binding(ObjectID()));
}

TEST_CASE("[SceneTree][SPX] UI teardown tolerates a detached canvas and repeated shutdown") {
	TestSpx::EngineScene scene;
	auto *ui = scene.engine->get_ui();
	const GdObj id = ui->create_button("", "kept until canvas deletion");
	Control *button = ui->get_node(id)->get_control();
	Node *canvas = button->get_parent();
	REQUIRE(canvas != nullptr);
	const ObjectID button_id = button->get_instance_id();
	canvas->get_parent()->remove_child(canvas);
	ui->on_destroy();
	CHECK(ui->get_node(id) == nullptr);
	CHECK(canvas->is_queued_for_deletion());
	CHECK(button->get_parent() == canvas);
	CHECK_FALSE(button->is_queued_for_deletion());
	ui->on_destroy();
	CHECK(ui->get_node(id) == nullptr);
	memdelete(canvas);
	CHECK(ObjectDB::get_instance(button_id) == nullptr);
}

} // namespace TestSpxUiMgr

#endif // TEST_SPX_UI_MGR_H
