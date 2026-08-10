/**************************************************************************/
/*  spx_collision_debug_overlay.cpp                                       */
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

#include "spx_collision_debug_overlay.h"

#include "core/config/engine.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"

#include "spx.h"

namespace {

void update_debug_overlays(Node *p_node, bool p_enabled) {
	if (SpxCollisionDebugOverlay *overlay = Object::cast_to<SpxCollisionDebugOverlay>(p_node)) {
		overlay->sync_debug_mode(p_enabled);
	}

	for (int i = 0; i < p_node->get_child_count(true); ++i) {
		update_debug_overlays(p_node->get_child(i, true), p_enabled);
	}
}

} // namespace

Color spx_collision_debug_shape_color(const Color &p_color, bool p_disabled) {
	if (!p_disabled) {
		return p_color;
	}

	Color draw_color = p_color;
	const float value = draw_color.get_v();
	draw_color.r = value;
	draw_color.g = value;
	draw_color.b = value;
	draw_color.a *= 0.5;
	return draw_color;
}

Color spx_collision_debug_one_way_color(const Color &p_color, bool p_disabled) {
	Color draw_color = p_color.inverted();
	if (p_disabled) {
		draw_color = draw_color.darkened(0.25);
	}
	return draw_color;
}

CollisionShape2D *SpxCollisionDebugOverlay::_get_target() const {
	return Object::cast_to<CollisionShape2D>(ObjectDB::get_instance(target_id));
}

bool SpxCollisionDebugOverlay::_is_native_collision_debug_visible() const {
	if (Engine::get_singleton()->is_editor_hint()) {
		return true;
	}

	const SceneTree *tree = is_inside_tree() ? get_tree() : nullptr;
	return tree != nullptr && tree->is_debugging_collisions_hint();
}

void SpxCollisionDebugOverlay::_sync_visibility(bool p_debug_mode) {
	const bool monitor_native_debug = requested_visible && p_debug_mode && !Engine::get_singleton()->is_editor_hint();
	set_process(monitor_native_debug);

	const bool should_be_visible = requested_visible && p_debug_mode && !_is_native_collision_debug_visible();
	if (is_visible() == should_be_visible) {
		return;
	}

	set_visible(should_be_visible);
	if (should_be_visible) {
		queue_redraw();
	}
}

void SpxCollisionDebugOverlay::_on_target_redrawn() {
	if (is_visible()) {
		queue_redraw();
	}
}

void SpxCollisionDebugOverlay::_draw_debug_shape() {
	CollisionShape2D *target = _get_target();
	if (target == nullptr) {
		return;
	}

	const Ref<Shape2D> shape = target->get_shape();
	if (shape.is_null()) {
		return;
	}

	shape->draw(get_canvas_item(), spx_collision_debug_shape_color(debug_color, target->is_disabled()));

	if (!target->is_one_way_collision_enabled()) {
		return;
	}

	const Color one_way_color = spx_collision_debug_one_way_color(debug_color, target->is_disabled());
	const Vector2 line_to(0, 20);
	draw_line(Vector2(), line_to, one_way_color, 2);

	const real_t triangle_size = 8;
	const Vector<Vector2> points{
		line_to + Vector2(0, triangle_size),
		line_to + Vector2(Math_SQRT12 * triangle_size, 0),
		line_to + Vector2(-Math_SQRT12 * triangle_size, 0),
	};
	const Vector<Color> colors{ one_way_color, one_way_color, one_way_color };
	draw_primitive(points, colors, Vector<Vector2>());
}

void SpxCollisionDebugOverlay::_notification(int p_what) {
	switch (p_what) {
		case NOTIFICATION_ENTER_TREE:
		case NOTIFICATION_PROCESS:
			_sync_visibility(Spx::debug_mode);
			break;
		case NOTIFICATION_DRAW:
			_draw_debug_shape();
			break;
		default:
			break;
	}
}

void SpxCollisionDebugOverlay::configure(CollisionShape2D *p_target, const Color &p_color, bool p_visible) {
	CollisionShape2D *old_target = _get_target();
	const Callable redraw_callable = callable_mp(this, &SpxCollisionDebugOverlay::_on_target_redrawn);
	if (old_target != nullptr && old_target != p_target && old_target->is_connected(SNAME("draw"), redraw_callable)) {
		old_target->disconnect(SNAME("draw"), redraw_callable);
	}

	target_id = p_target != nullptr ? p_target->get_instance_id() : ObjectID();
	if (p_target != nullptr && !p_target->is_connected(SNAME("draw"), redraw_callable)) {
		p_target->connect(SNAME("draw"), redraw_callable);
	}

	set_debug_color(p_color);
	set_requested_visible(p_visible);
}

void SpxCollisionDebugOverlay::set_debug_color(const Color &p_color) {
	const bool color_changed = debug_color != p_color;
	debug_color = p_color;
	if (CollisionShape2D *target = _get_target()) {
		// Native collision debugging draws on CollisionShape2D itself. Keeping its
		// public debug color in sync prevents a duplicate overlay in that mode.
		if (target->get_debug_color() != p_color) {
			target->set_debug_color(p_color);
		}
	}
	if (color_changed) {
		queue_redraw();
	}
}

void SpxCollisionDebugOverlay::set_requested_visible(bool p_visible) {
	requested_visible = p_visible;
	_sync_visibility(Spx::debug_mode);
}

SpxCollisionDebugOverlay::SpxCollisionDebugOverlay() {
	set_visible(false);
	set_process(false);
}

SpxCollisionDebugOverlay *spx_find_collision_debug_overlay(CollisionShape2D *p_target) {
	ERR_FAIL_NULL_V(p_target, nullptr);

	for (int i = 0; i < p_target->get_child_count(true); ++i) {
		SpxCollisionDebugOverlay *overlay = Object::cast_to<SpxCollisionDebugOverlay>(p_target->get_child(i, true));
		if (overlay != nullptr && overlay->get_target() == p_target) {
			return overlay;
		}
	}
	return nullptr;
}

SpxCollisionDebugOverlay *spx_ensure_collision_debug_overlay(CollisionShape2D *p_target, const Color &p_color, bool p_visible) {
	ERR_FAIL_NULL_V(p_target, nullptr);

	SpxCollisionDebugOverlay *overlay = spx_find_collision_debug_overlay(p_target);
	if (overlay == nullptr) {
		overlay = memnew(SpxCollisionDebugOverlay);
		overlay->set_name("_SpxCollisionDebugOverlay");
		p_target->add_child(overlay, false, Node::INTERNAL_MODE_FRONT);
	}
	overlay->configure(p_target, p_color, p_visible);
	return overlay;
}

void spx_collision_debug_mode_changed(bool p_enabled) {
	SceneTree *tree = SceneTree::get_singleton();
	if (tree == nullptr || tree->get_root() == nullptr) {
		return;
	}
	update_debug_overlays(tree->get_root(), p_enabled);
}
