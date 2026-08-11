/**************************************************************************/
/*  spx_sprite.cpp                                                        */
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

#include "spx_sprite.h"

#include "core/config/project_settings.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/visible_on_screen_notifier_2d.h"

#include "spx.h"
#include "spx_collision_debug_overlay.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "spx_sprite_mgr.h"

namespace {

void connect_signal_once(Object *source, const StringName &signal_name, Object *target, const StringName &method_name) {
	if (source == nullptr || target == nullptr) {
		return;
	}

	Callable callable(target, method_name);
	if (!source->is_connected(signal_name, callable)) {
		source->connect(signal_name, callable);
	}
}

template <typename T>
T *get_named_child(Node *owner, const char *child_name) {
	if (owner == nullptr) {
		return nullptr;
	}

	return Object::cast_to<T>(owner->get_node_or_null(NodePath(child_name)));
}

} // namespace

void SpxStaticSprite::set_collider(CollisionShape2D *p_collider) {
	collider2d = p_collider;
	if (collider2d != nullptr) {
		spx_ensure_collision_debug_overlay(collider2d, Color(0, 0, 1, 0.2), true);
	}
}

void SpxSprite::_set_use_default_frames(bool p_enabled) {
	use_default_frames = p_enabled;
}

bool SpxSprite::_get_use_default_frames() {
	return use_default_frames;
}

void SpxSprite::_bind_methods() {
	ClassDB::bind_method(D_METHOD("set_gid", "gid"), &SpxSprite::set_gid);
	ClassDB::bind_method(D_METHOD("get_gid"), &SpxSprite::get_gid);
	ADD_PROPERTY(PropertyInfo(Variant::INT, "gid"), "set_gid", "get_gid");

	ClassDB::bind_method(D_METHOD("set_use_default_frames", "use_default_frames"), &SpxSprite::_set_use_default_frames);
	ClassDB::bind_method(D_METHOD("get_use_default_frames"), &SpxSprite::_get_use_default_frames);
	ADD_PROPERTY(PropertyInfo(Variant::BOOL, "use_default_frames"), "set_use_default_frames", "get_use_default_frames");

	ClassDB::bind_method(D_METHOD("set_spx_type_name", "spx_type_name"), &SpxSprite::set_spx_type_name);
	ClassDB::bind_method(D_METHOD("get_spx_type_name"), &SpxSprite::get_spx_type_name);
	ADD_PROPERTY(PropertyInfo(Variant::STRING, "spx_type_name"), "set_spx_type_name", "get_spx_type_name");

	ClassDB::bind_method(D_METHOD("on_destroy_call"), &SpxSprite::on_destroy_call);
	ClassDB::bind_method(D_METHOD("on_area_entered", "area"), &SpxSprite::_on_area_entered);
	ClassDB::bind_method(D_METHOD("on_area_exited", "area"), &SpxSprite::_on_area_exited);

	ClassDB::bind_method(D_METHOD("on_sprite_frames_set_changed"), &SpxSprite::_on_sprite_frames_set_changed);
	ClassDB::bind_method(D_METHOD("on_sprite_animation_changed"), &SpxSprite::_on_sprite_animation_changed);
	ClassDB::bind_method(D_METHOD("on_sprite_frame_changed"), &SpxSprite::_on_sprite_frame_changed);
	ClassDB::bind_method(D_METHOD("on_sprite_animation_looped"), &SpxSprite::_on_sprite_animation_looped);
	ClassDB::bind_method(D_METHOD("on_sprite_animation_finished"), &SpxSprite::_on_sprite_animation_finished);
	ClassDB::bind_method(D_METHOD("on_sprite_vfx_finished"), &SpxSprite::_on_sprite_vfx_finished);
	ClassDB::bind_method(D_METHOD("on_sprite_screen_exited"), &SpxSprite::_on_sprite_screen_exited);
	ClassDB::bind_method(D_METHOD("on_sprite_screen_entered"), &SpxSprite::_on_sprite_screen_entered);

	ClassDB::bind_method(D_METHOD("set_physics_mode", "mode"), &SpxSprite::set_physics_mode);
	ClassDB::bind_method(D_METHOD("get_physics_mode"), &SpxSprite::get_physics_mode);
	ADD_PROPERTY(PropertyInfo(Variant::INT, "physics_mode", PROPERTY_HINT_ENUM, "NoPhysics,Kinematic,Dynamic,Static"), "set_physics_mode", "get_physics_mode");

	ClassDB::bind_method(D_METHOD("set_use_gravity", "enabled"), &SpxSprite::set_use_gravity);
	ClassDB::bind_method(D_METHOD("is_use_gravity"), &SpxSprite::is_use_gravity);
	ADD_PROPERTY(PropertyInfo(Variant::BOOL, "use_gravity"), "set_use_gravity", "is_use_gravity");

	ClassDB::bind_method(D_METHOD("set_gravity_scale", "scale"), &SpxSprite::set_gravity_scale);
	ClassDB::bind_method(D_METHOD("get_gravity_scale"), &SpxSprite::get_gravity_scale);
	ADD_PROPERTY(PropertyInfo(Variant::FLOAT, "gravity_scale"), "set_gravity_scale", "get_gravity_scale");

	ClassDB::bind_method(D_METHOD("set_drag", "drag"), &SpxSprite::set_drag);
	ClassDB::bind_method(D_METHOD("get_drag"), &SpxSprite::get_drag);
	ADD_PROPERTY(PropertyInfo(Variant::FLOAT, "drag"), "set_drag", "get_drag");

	ClassDB::bind_method(D_METHOD("set_friction", "friction"), &SpxSprite::set_friction);
	ClassDB::bind_method(D_METHOD("get_friction"), &SpxSprite::get_friction);
	ADD_PROPERTY(PropertyInfo(Variant::FLOAT, "friction"), "set_friction", "get_friction");

	ClassDB::bind_method(D_METHOD("set_debug_collision_visible", "enabled"), &SpxSprite::set_debug_collision_visible);
	ClassDB::bind_method(D_METHOD("is_debug_collision_visible"), &SpxSprite::is_debug_collision_visible);
	ADD_PROPERTY(PropertyInfo(Variant::BOOL, "debug_collision_visible"), "set_debug_collision_visible", "is_debug_collision_visible");
}

void SpxSprite::on_destroy_call() {
	if (!Spx::is_initialized()) {
		return;
	}

	spriteMgr->on_sprite_destroy(this);
}

SpxSprite::SpxSprite() {
	_gravity = ProjectSettings::get_singleton()->get_setting("physics/2d/default_gravity", 980.0);
	physics_mode = NO_PHYSICS;
}

SpxSprite::~SpxSprite() {
}

void SpxSprite::_notification(int p_what) {
	switch (p_what) {
		case NOTIFICATION_PHYSICS_PROCESS:
			_physics_process(get_physics_process_delta_time());
			break;
		case NOTIFICATION_PREDELETE:
			on_destroy_call();
			break;
		default:
			break;
	}
}

void SpxSprite::_resolve_runtime_components() {
	render_root = get_named_child<Node2D>(this, "RenderRoot");
	if (render_root == nullptr) {
		render_root = memnew(Node2D);
		render_root->set_name("RenderRoot");
		add_child(render_root);
	}

	anim2d = get_named_child<AnimatedSprite2D>(render_root, "Anim2D");
	if (anim2d == nullptr) {
		anim2d = _get_component<AnimatedSprite2D>(true);
	}
	if (anim2d != nullptr && anim2d->get_parent() != render_root) {
		anim2d->reparent(render_root, true);
	}

	area2d = get_named_child<Area2D>(this, "Area2D");
	if (area2d == nullptr) {
		area2d = _get_component<Area2D>();
	}

	collider2d = get_named_child<CollisionShape2D>(this, "Collider2D");
	if (collider2d == nullptr) {
		collider2d = _get_component<CollisionShape2D>();
	}

	trigger2d = get_named_child<CollisionShape2D>(area2d, "Trigger2D");
	if (trigger2d == nullptr && area2d != nullptr) {
		trigger2d = _get_component<CollisionShape2D>(area2d);
	}

	render_root->set_position(render_offset);
}

void SpxSprite::_update_collision_debug_overlays() {
	if (trigger2d != nullptr) {
		spx_ensure_collision_debug_overlay(trigger2d, Color(1, 0, 0, 0.2), debug_collision_visible);
	}
	if (collider2d != nullptr) {
		spx_ensure_collision_debug_overlay(collider2d, Color(0, 0, 1, 0.2), debug_collision_visible);
	}
}

void SpxSprite::_initialize_default_frames() {
	default_sprite_frames = anim2d->get_sprite_frames();
	if (default_sprite_frames.is_valid() && !resMgr->is_dynamic_anim_mode()) {
		return;
	}

	default_sprite_frames.instantiate();
	anim2d->set_sprite_frames(default_sprite_frames);
}

void SpxSprite::_ensure_visible_notifier() {
	visible_notifier = get_named_child<VisibleOnScreenNotifier2D>(render_root, "VisibleNotifier2D");
	if (visible_notifier == nullptr) {
		visible_notifier = _get_component<VisibleOnScreenNotifier2D>(true);
	}

	if (visible_notifier == nullptr) {
		visible_notifier = memnew(VisibleOnScreenNotifier2D);
		visible_notifier->set_name("VisibleNotifier2D");
		render_root->add_child(visible_notifier);
	} else if (visible_notifier->get_parent() != render_root) {
		visible_notifier->reparent(render_root, true);
	}
}

void SpxSprite::_connect_runtime_signals() {
	connect_signal_once(area2d, "area_entered", this, "on_area_entered");
	connect_signal_once(area2d, "area_exited", this, "on_area_exited");

	connect_signal_once(anim2d, "sprite_frames_changed", this, "on_sprite_frames_set_changed");
	connect_signal_once(anim2d, "animation_changed", this, "on_sprite_animation_changed");
	connect_signal_once(anim2d, "frame_changed", this, "on_sprite_frame_changed");
	connect_signal_once(anim2d, "animation_looped", this, "on_sprite_animation_looped");
	connect_signal_once(anim2d, "animation_finished", this, "on_sprite_animation_finished");

	connect_signal_once(visible_notifier, "screen_exited", this, "on_sprite_screen_exited");
	connect_signal_once(visible_notifier, "screen_entered", this, "on_sprite_screen_entered");
}

void SpxSprite::on_start() {
	_resolve_runtime_components();

	ERR_FAIL_NULL_MSG(collider2d, "SpxSprite: CollisionShape2D component is missing.");
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	ERR_FAIL_NULL_MSG(area2d, "SpxSprite: Area2D component is missing.");
	ERR_FAIL_NULL_MSG(trigger2d, "SpxSprite: Trigger2D component is missing.");

	_update_collision_debug_overlays();
	_initialize_default_frames();
	_ensure_visible_notifier();

	_is_collision_enabled = !collider2d->is_disabled();
	_is_trigger_enabled = !trigger2d->is_disabled();

	_connect_runtime_signals();
	_update_anim_scale();
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
	_update_physics_mode();
	_update_trigger_disabled_state();
}

void SpxSprite::set_gid(GdObj id) {
	gid = id;
}

GdObj SpxSprite::get_gid() {
	return gid;
}

void SpxSprite::set_type_name(GdString type_name) {
	String name = SpxStr(type_name);
	spx_type_name = name;
	set_name(name);
}

void SpxSprite::set_spx_type_name(String type_name) {
	spx_type_name = type_name;
}

String SpxSprite::get_spx_type_name() {
	return spx_type_name;
}

void SpxSprite::_on_area_entered(Node *p_node) {
	if (!Spx::is_initialized() || is_backdrop) {
		return;
	}

	Node *parent_node = p_node != nullptr ? p_node->get_parent() : nullptr;
	const SpxSprite *other = Object::cast_to<SpxSprite>(parent_node);
	if (other != nullptr) {
		spriteMgr->on_trigger_enter(gid, other->gid);
	}
}

void SpxSprite::_on_area_exited(Node *p_node) {
	if (!Spx::is_initialized() || is_backdrop) {
		return;
	}

	Node *parent_node = p_node != nullptr ? p_node->get_parent() : nullptr;
	const SpxSprite *other = Object::cast_to<SpxSprite>(parent_node);
	if (other != nullptr) {
		spriteMgr->on_trigger_exit(gid, other->gid);
	}
}

void SpxSprite::set_block_signals(bool p_block) {
	Object::set_block_signals(p_block);
}

void SpxSprite::_on_sprite_frames_set_changed() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_frames_set_changed(gid);
}

void SpxSprite::_on_sprite_animation_changed() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_animation_changed(gid);
}

void SpxSprite::_on_sprite_frame_changed() {
	if (!Spx::is_initialized()) {
		return;
	}

	_on_frame_changed();
	SPX_CALLBACK->func_on_sprite_frame_changed(gid);
	_update_current_frame_shader_uv_rect();
}

void SpxSprite::_on_sprite_animation_looped() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_animation_looped(gid);
}

void SpxSprite::_on_sprite_animation_finished() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_animation_finished(gid);
}

void SpxSprite::_on_sprite_vfx_finished() {
	if (!Spx::is_initialized()) {
		return;
	}
}

void SpxSprite::_on_sprite_screen_exited() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_screen_exited(gid);
}

void SpxSprite::_on_sprite_screen_entered() {
	if (!Spx::is_initialized()) {
		return;
	}

	SPX_CALLBACK->func_on_sprite_screen_entered(gid);
}
