/**************************************************************************/
/*  spx_ui_mgr.cpp                                                        */
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

#include "spx_ui_mgr.h"

#include "scene/main/canvas_layer.h"
#include "scene/resources/packed_scene.h"

#include "spx.h"
#include "spx_object_guard.h"

#define SPX_CALLBACK SpxEngine::get_singleton()->get_callbacks()

// Refactored UI node validation using unified SpxObjectGuard (RAII pattern)
// See spx_object_guard.h for details
#define SPX_REQUIRE_UI_VOID() \
	SPX_UI_GUARD_VOID(obj, __func__)

#define SPX_REQUIRE_UI_RETURN(VALUE) \
	SPX_UI_GUARD_RETURN(obj, __func__, VALUE)

Node *SpxUiMgr::create_owner_node() {
	return memnew(CanvasLayer);
}

SpxUi *SpxUiMgr::get_node(GdObj obj) {
	if (id_objects.has(obj)) {
		return id_objects[obj];
	}
	return nullptr;
}

ESpxUiType SpxUiMgr::get_node_type(Node *obj) {
	if (obj == nullptr) {
		return ESpxUiType::None;
	}

	if (dynamic_cast<SpxLabel *>(obj)) {
		return ESpxUiType::Label;
	}

	if (dynamic_cast<SpxButton *>(obj)) {
		return ESpxUiType::Button;
	}

	if (dynamic_cast<SpxImage *>(obj)) {
		return ESpxUiType::Image;
	}

	if (dynamic_cast<SpxToggle *>(obj)) {
		return ESpxUiType::Toggle;
	}

	if (dynamic_cast<SpxInput *>(obj)) {
		return ESpxUiType::Input;
	}

	if (dynamic_cast<SpxSlider *>(obj)) {
		return ESpxUiType::Slider;
	}

	if (dynamic_cast<SpxControl *>(obj)) {
		return ESpxUiType::Control;
	}
	return ESpxUiType::None;
}

void SpxUiMgr::on_spx_ui_clicked(GdObj p_gid, ObjectID p_binding_id) {
	SpxUi *node = get_node(p_gid);
	if (node == nullptr || !node->owns_binding(p_binding_id)) {
		return;
	}
	SPX_CALLBACK->func_on_ui_clicked(p_gid);
}

void SpxUiMgr::on_spx_ui_destroyed(GdObj p_gid, ObjectID p_binding_id) {
	if (!Spx::is_initialized()) {
		return;
	}

	SpxUi *node = get_node(p_gid);
	if (node == nullptr || !node->owns_binding(p_binding_id)) {
		return;
	}

	if (id_objects.erase(p_gid)) {
		SPX_CALLBACK->func_on_ui_destroyed(p_gid);
	}
	node->detach_control_binding(p_binding_id);
	memdelete(node);
}

void SpxUiMgr::on_awake() {
	SpxBaseMgr::on_awake();
}

void SpxUiMgr::_clear_nodes(bool emit_destroyed, bool queue_controls) {
	Vector<SpxUi *> nodes;
	for (const auto &entry : id_objects) {
		nodes.push_back(entry.value);
	}
	id_objects.clear();

	for (SpxUi *node : nodes) {
		if (node == nullptr) {
			continue;
		}

		if (emit_destroyed) {
			SPX_CALLBACK->func_on_ui_destroyed(node->get_gid());
		}

		Control *control = node->get_control_item();
		node->detach_control_binding();
		if (control != nullptr) {
			if (queue_controls) {
				control->queue_free();
			}
		}

		memdelete(node);
	}
}

void SpxUiMgr::on_destroy() {
	_clear_nodes(false, false);
	SpxBaseMgr::on_destroy();
}

void SpxUiMgr::on_reset(int reset_code) {
	_clear_nodes(true, true);
	if (owner != nullptr) {
		owner->queue_free();
	}
	owner = create_owner_node();
	if (owner == nullptr) {
		return;
	}
	owner->set_name(get_class_name());
	get_spx_root()->add_child(owner);
}

Control *SpxUiMgr::create_control(GdString path) {
	const String path_str = SpxStr(path);
	Control *node = nullptr;
	if (path_str != "") {
		Ref<PackedScene> scene = ResourceLoader::load(path_str);
		if (scene.is_null()) {
			print_error("Failed to load sprite scene " + path_str);
			return NULL_OBJECT_ID;
		} else {
			node = dynamic_cast<Control *>(scene->instantiate());
			if (node == nullptr) {
				print_error("Failed to load sprite scene , type invalid " + path_str);
			}
		}
	}
	return node;
}

SpxUi *SpxUiMgr::on_create_node(Control *control, GdInt type, bool is_attach) {
	if (control == nullptr) {
		return nullptr;
	}
	SpxUi *node = memnew(SpxUi);
	if (is_attach) {
		owner->add_child(control);
	}
	node->set_type(type);
	node->set_gid(get_unique_id());
	if (node->set_control_item(control, this) != OK) {
		if (is_attach) {
			owner->remove_child(control);
			memdelete(control);
		}
		memdelete(node);
		return nullptr;
	}
	node->on_start();
	id_objects[node->get_gid()] = node;
	SPX_CALLBACK->func_on_ui_ready(node->get_gid());
	return node;
}

#define CREATE_UI_NODE(TYPE)                                              \
	Spx##TYPE *control = dynamic_cast<Spx##TYPE *>(create_control(path)); \
	if (control == nullptr) {                                             \
		control = memnew(Spx##TYPE);                                      \
	}                                                                     \
	auto node = on_create_node(control, (GdInt)ESpxUiType::TYPE);         \
	if (node == nullptr) {                                                \
		return NULL_OBJECT_ID;                                            \
	}

GdObj SpxUiMgr::create_node(GdString path) {
	auto control = create_control(path);
	if (control == nullptr) {
		print_error("Failed to create node " + SpxStr(path));
		return NULL_OBJECT_ID;
	}
	auto type = get_node_type(control);
	auto node = on_create_node(control, (GdInt)type);
	if (node == nullptr) {
		return NULL_OBJECT_ID;
	}
	return node->get_gid();
}

GdObj SpxUiMgr::create_button(GdString path, GdString text) {
	CREATE_UI_NODE(Button)
	control->set_text(SpxStr(text));
	return node->get_gid();
}

GdObj SpxUiMgr::create_label(GdString path, GdString text) {
	CREATE_UI_NODE(Label)
	control->set_text(SpxStr(text));
	return node->get_gid();
}

GdObj SpxUiMgr::create_image(GdString path) {
	CREATE_UI_NODE(Image)
	return node->get_gid();
}

GdObj SpxUiMgr::create_toggle(GdString path, GdBool value) {
	CREATE_UI_NODE(Toggle)
	return node->get_gid();
}

GdObj SpxUiMgr::create_slider(GdString path, GdFloat value) {
	CREATE_UI_NODE(Slider)
	control->set_value(value);
	return node->get_gid();
}

GdObj SpxUiMgr::create_input(GdString path, GdString text) {
	CREATE_UI_NODE(Input)
	control->set_text(SpxStr(text));
	return node->get_gid();
}

GdBool SpxUiMgr::destroy_node(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(false)
	node->queue_free();
	return true;
}

GdObj SpxUiMgr::bind_node(GdObj obj, GdString rel_path) {
	auto parent_node = get_node(obj);
	if (parent_node == nullptr) {
		print_line("bind_node error :can not find a node ", obj);
		return NULL_OBJECT_ID;
	}

	Control *parent_control = parent_node->get_control_item();
	if (parent_control == nullptr) {
		print_line("bind_node error : parent node was destroyed ", obj);
		return NULL_OBJECT_ID;
	}

	auto path = SpxStr(rel_path);
	if (!parent_control->has_node(path)) {
		print_line("bind_node error :can not find a child node ", obj, path);
		return NULL_OBJECT_ID;
	}

	auto child = parent_control->get_node(path);
	auto type = get_node_type(child);
	if (type == ESpxUiType::None) {
		print_line("bind_node error : unknown type ", obj, path);
		return NULL_OBJECT_ID;
	}

	auto node = on_create_node((Control *)child, (GdInt)type, false);
	if (node == nullptr) {
		return NULL_OBJECT_ID;
	}
	return node->get_gid();
}

GdInt SpxUiMgr::get_type(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(0)
	return node->get_type();
}

void SpxUiMgr::set_interactable(GdObj obj, GdBool interactable) {
	SPX_REQUIRE_UI_VOID()
	node->set_interactable(interactable);
}

GdBool SpxUiMgr::get_interactable(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(false)
	return node->is_interactable();
}

void SpxUiMgr::set_text(GdObj obj, GdString text) {
	SPX_REQUIRE_UI_VOID()
	node->set_text(text);
}

GdString SpxUiMgr::get_text(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdString())
	return node->get_text();
}

void SpxUiMgr::set_rect(GdObj obj, GdRect2 rect) {
	SPX_REQUIRE_UI_VOID()
	node->set_rect(rect);
}

GdRect2 SpxUiMgr::get_rect(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdRect2())
	return node->get_rect();
}

void SpxUiMgr::set_color(GdObj obj, GdColor color) {
	SPX_REQUIRE_UI_VOID()
	node->set_color(color);
}

GdColor SpxUiMgr::get_color(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdColor())
	return node->get_color();
}

void SpxUiMgr::set_font_size(GdObj obj, GdInt size) {
	SPX_REQUIRE_UI_VOID()
	node->set_font_size(size);
}

GdInt SpxUiMgr::get_font_size(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(0)
	return node->get_font_size();
}

void SpxUiMgr::set_visible(GdObj obj, GdBool visible) {
	SPX_REQUIRE_UI_VOID()
	node->set_visible(visible);
}

GdBool SpxUiMgr::get_visible(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(false)
	return node->get_visible();
}

void SpxUiMgr::set_texture(GdObj obj, GdString path) {
	SPX_REQUIRE_UI_VOID()
	node->set_texture(path);
}

GdString SpxUiMgr::get_texture(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(nullptr)
	return node->get_texture();
}

GdInt SpxUiMgr::get_layout_direction(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(0)
	return node->get_layout_direction();
}

void SpxUiMgr::set_layout_direction(GdObj obj, GdInt value) {
	SPX_REQUIRE_UI_VOID()
	node->set_layout_direction(value);
}

GdInt SpxUiMgr::get_layout_mode(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(0)
	return node->get_layout_mode();
}

void SpxUiMgr::set_layout_mode(GdObj obj, GdInt value) {
	SPX_REQUIRE_UI_VOID()
	node->set_layout_mode(value);
}

GdInt SpxUiMgr::get_anchors_preset(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(0)
	return node->get_anchors_preset();
}

void SpxUiMgr::set_anchors_preset(GdObj obj, GdInt value) {
	SPX_REQUIRE_UI_VOID()
	node->set_anchors_preset(value);
}

GdVec2 SpxUiMgr::get_scale(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdVec2())
	return node->get_scale();
}

void SpxUiMgr::set_scale(GdObj obj, GdVec2 value) {
	SPX_REQUIRE_UI_VOID()
	node->set_scale(value);
}

GdVec2 SpxUiMgr::get_position(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdVec2())
	return node->get_position();
}

void SpxUiMgr::set_position(GdObj obj, GdVec2 value) {
	SPX_REQUIRE_UI_VOID()
	node->set_position(value);
}

GdVec2 SpxUiMgr::get_size(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdVec2())
	return node->get_size();
}

void SpxUiMgr::set_size(GdObj obj, GdVec2 value) {
	SPX_REQUIRE_UI_VOID()
	node->set_size(value);
}

GdVec2 SpxUiMgr::get_global_position(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdVec2())
	return node->get_global_position();
}

void SpxUiMgr::set_global_position(GdObj obj, GdVec2 value) {
	SPX_REQUIRE_UI_VOID()
	node->set_global_position(value);
}

GdFloat SpxUiMgr::get_rotation(GdObj obj) {
	SPX_REQUIRE_UI_RETURN(GdFloat())
	return node->get_rotation();
}

void SpxUiMgr::set_rotation(GdObj obj, GdFloat value) {
	SPX_REQUIRE_UI_VOID()
	node->set_rotation(value);
}

GdBool SpxUiMgr::get_flip(GdObj obj, GdBool horizontal) {
	SPX_REQUIRE_UI_RETURN(GdBool())
	return node->get_flip(horizontal);
}

void SpxUiMgr::set_flip(GdObj obj, GdBool horizontal, GdBool is_flip) {
	SPX_REQUIRE_UI_VOID()
	node->set_flip(horizontal, is_flip);
}
