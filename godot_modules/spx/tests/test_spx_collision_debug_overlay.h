/**************************************************************************/
/*  test_spx_collision_debug_overlay.h                                    */
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

#ifndef TEST_SPX_COLLISION_DEBUG_OVERLAY_H
#define TEST_SPX_COLLISION_DEBUG_OVERLAY_H

#include "../spx.h"
#include "../spx_collision_debug_overlay.h"
#include "core/config/engine.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "tests/test_macros.h"

namespace TestSpxCollisionDebugOverlay {

TEST_CASE("[SPX] Collision debug overlay preserves disabled draw colors") {
	const Color debug_color(0.2, 0.4, 0.8, 0.6);

	CHECK(spx_collision_debug_shape_color(debug_color, false) == debug_color);
	const Color disabled_shape_color = spx_collision_debug_shape_color(debug_color, true);
	CHECK(disabled_shape_color == Color(0.8, 0.8, 0.8, 0.3));

	CHECK(spx_collision_debug_one_way_color(debug_color, false) == debug_color.inverted());
	CHECK(spx_collision_debug_one_way_color(debug_color, true) == debug_color.inverted().darkened(0.25));
}

TEST_CASE("[SceneTree][SPX] Collision debug overlay is internal, idempotent, and updates color") {
	CollisionShape2D *target = memnew(CollisionShape2D);
	const Color collider_color(0, 0, 1, 0.2);
	const Color trigger_color(1, 0, 0, 0.2);

	SpxCollisionDebugOverlay *overlay = spx_ensure_collision_debug_overlay(target, collider_color, true);
	REQUIRE(overlay != nullptr);
	CHECK(overlay->get_parent() == target);
	CHECK(overlay->get_internal_mode() == Node::INTERNAL_MODE_FRONT);
	CHECK(overlay->get_target() == target);
	CHECK(overlay->is_requested_visible());
	CHECK(overlay->get_debug_color() == collider_color);
	CHECK(target->get_debug_color() == collider_color);

	SpxCollisionDebugOverlay *same_overlay = spx_ensure_collision_debug_overlay(target, trigger_color, false);
	CHECK(same_overlay == overlay);
	CHECK(target->get_child_count(true) == 1);
	CHECK_FALSE(overlay->is_requested_visible());
	CHECK(overlay->get_debug_color() == trigger_color);
	CHECK(target->get_debug_color() == trigger_color);

	target->remove_child(overlay);
	memdelete(overlay);
	memdelete(target);
}

TEST_CASE("[SceneTree][SPX] Collision debug overlay honors requested and debug-mode visibility") {
	CollisionShape2D *target = memnew(CollisionShape2D);
	SpxCollisionDebugOverlay *overlay = spx_ensure_collision_debug_overlay(target, Color(0, 0, 1, 0.2), true);
	REQUIRE(overlay != nullptr);

	overlay->sync_debug_mode(true);
	CHECK(overlay->is_visible());
	overlay->set_requested_visible(false);
	CHECK_FALSE(overlay->is_visible());
	overlay->set_requested_visible(true);
	overlay->sync_debug_mode(true);
	CHECK(overlay->is_visible());
	overlay->sync_debug_mode(false);
	CHECK_FALSE(overlay->is_visible());

	target->remove_child(overlay);
	memdelete(overlay);
	memdelete(target);
}

TEST_CASE("[SceneTree][SPX] Collision debug mode updates overlays already in the tree") {
	const bool previous_debug_mode = Spx::is_debug_mode();
	CollisionShape2D *target = memnew(CollisionShape2D);
	SpxCollisionDebugOverlay *overlay = spx_ensure_collision_debug_overlay(target, Color(0, 0, 1, 0.2), true);
	REQUIRE(overlay != nullptr);
	SceneTree *tree = SceneTree::get_singleton();
	REQUIRE(tree != nullptr);
	tree->get_root()->add_child(target);

	Spx::set_debug_mode(true);
	const bool native_debug_visible = Engine::get_singleton()->is_editor_hint() || tree->is_debugging_collisions_hint();
	CHECK(overlay->is_visible() == !native_debug_visible);
	Spx::set_debug_mode(false);
	CHECK_FALSE(overlay->is_visible());

	Spx::set_debug_mode(previous_debug_mode);
	tree->get_root()->remove_child(target);
	target->remove_child(overlay);
	memdelete(overlay);
	memdelete(target);
}

} // namespace TestSpxCollisionDebugOverlay

#endif // TEST_SPX_COLLISION_DEBUG_OVERLAY_H
