/**************************************************************************/
/*  spx_sprite_spine.cpp                                                  */
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
/* MERCHANTABILITY AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR  */
/* COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, */
/* WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT */
/* OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN  */
/* THE SOFTWARE.                                                          */
/**************************************************************************/

#include "spx_sprite.h"

#ifdef SPX_SPINE_ENABLED
#include "scene/2d/animated_sprite_2d.h"
#include "scene/resources/material.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "thirdparty/spine_godot/SpineAnimationState.h"
#include "thirdparty/spine_godot/SpineSkeleton.h"
#include "thirdparty/spine_godot/SpineSprite.h"
#include "thirdparty/spine_godot/SpineTrackEntry.h"

GdBool SpxSprite::set_spine_skeleton(GdString p_atlas, GdString p_skeleton,
		GdFloat p_mix) {
	ERR_FAIL_NULL_V(render_root, false);
	ERR_FAIL_NULL_V(SpxEngine::get_singleton(), false);
	Ref<SpineSkeletonDataResource> data =
			resMgr->load_spine_data(SpxStr(p_atlas), SpxStr(p_skeleton), p_mix);
	if (data.is_null()) {
		return false;
	}

	// Prepare before replacing the live visual. A failed load leaves it intact.
	SpineSprite *child = memnew(SpineSprite);
	child->set_skeleton_data_res(data);
	Ref<SpineSkeleton> skeleton = child->get_skeleton();
	if (skeleton.is_null()) {
		memdelete(child);
		return false;
	}
	skeleton->set_to_setup_pose();
	skeleton->update_world_transform(SpineConstant::Physics_Update);
	const Rect2 bounds = skeleton->get_bounds();
	const Color color = get_color();
	clear_spine_skeleton();
	// Node::duplicate() may copy the visual subtree before Go initializes the
	// clone. Its Spine node is not owned by this sprite's native state yet.
	SpineSprite *inherited = Object::cast_to<SpineSprite>(render_root->get_node_or_null(NodePath("Spine")));
	if (inherited) {
		inherited->set_update_mode(SpineConstant::UpdateMode_Manual);
		render_root->remove_child(inherited);
		inherited->queue_free();
	}
	spine_child = child;
	spine_bounds = bounds;
	spine_child->set_name("Spine");
	_set_spine_color(color);
	render_root->add_child(spine_child);
	spine_child->connect("animation_started",
			callable_mp(this, &SpxSprite::_on_spine_started));
	spine_child->connect("animation_completed",
			callable_mp(this, &SpxSprite::_on_spine_completed));
	anim2d->stop();
	anim2d->hide();
	_update_spine_transform();
	_update_spine_speed();
	return true;
}

void SpxSprite::clear_spine_skeleton() {
	if (spine_child == nullptr) {
		return;
	}
	// Detach and disconnect before deferred destruction: old tracks must not
	// deliver completion events into a replacement skeleton.
	SpineSprite *child = spine_child;
	spine_child = nullptr;
	child->disconnect("animation_started",
			callable_mp(this, &SpxSprite::_on_spine_started));
	child->disconnect("animation_completed",
			callable_mp(this, &SpxSprite::_on_spine_completed));
	child->set_update_mode(SpineConstant::UpdateMode_Manual);
	child->get_parent()->remove_child(child);
	child->queue_free();
	spine_animation = String();
	spine_bounds = Rect2();
	spine_playing = false;
	if (anim2d) {
		anim2d->show();
	}
}

void SpxSprite::_update_spine_transform() {
	if (spine_child == nullptr) {
		return;
	}
	const Vector2 flip(anim2d->is_flipped_h() ? -1.0f : 1.0f,
			anim2d->is_flipped_v() ? -1.0f : 1.0f);
	spine_child->set_scale(_render_scale * flip);
	spine_child->set_position(base_offset * _render_scale);
}

void SpxSprite::_update_spine_speed() {
	if (spine_child == nullptr) {
		return;
	}
	const float speed_scale = anim2d->get_speed_scale();
	spine_child->set_time_scale(spine_playing ? Math::abs(speed_scale) : 0.0f);
	Ref<SpineTrackEntry> entry =
			spine_child->get_animation_state()->get_current(0);
	if (entry.is_valid()) {
		entry->set_reverse(playback_speed * speed_scale < 0.0f);
		entry->set_time_scale(Math::abs(playback_speed));
	}
}

void SpxSprite::_play_spine_animation(const String &p_name, float p_speed,
		bool p_loop, bool p_from_end) {
	ERR_FAIL_COND(!Math::is_finite(p_speed));
	ERR_FAIL_COND_MSG(
			spine_child->get_skeleton_data_res()->find_animation(p_name).is_null(),
			"SPX: Spine animation is missing.");
	Ref<SpineAnimationState> state = spine_child->get_animation_state();
	Ref<SpineTrackEntry> entry = state->get_current(0);
	// Resuming/replaying the current clip preserves time, as AnimatedSprite2D
	// does.
	if (entry.is_null() || spine_animation != p_name ||
			(!entry->get_loop() && entry->is_complete())) {
		spine_animation = p_name;
		entry = state->set_animation(p_name, p_loop, 0);
	}
	ERR_FAIL_COND(entry.is_null());
	entry->set_loop(p_loop);
	playback_speed = p_speed;
	spine_playing = true;
	_update_spine_speed();
	// Spine's reverse flag samples from the end while advancing track time.
	if (p_from_end && p_speed >= 0) {
		entry->set_reverse(true);
	}
}

void SpxSprite::_on_spine_started(SpineSprite *p_sprite,
		Ref<SpineAnimationState> p_state,
		Ref<SpineTrackEntry> p_entry) {
	if (p_sprite == spine_child && p_entry.is_valid() &&
			p_entry->get_track_index() == 0 && !is_blocking_signals()) {
		_on_sprite_animation_changed();
	}
}

void SpxSprite::_on_spine_completed(SpineSprite *p_sprite,
		Ref<SpineAnimationState> p_state,
		Ref<SpineTrackEntry> p_entry) {
	if (p_sprite != spine_child || p_entry.is_null() ||
			p_entry->get_track_index() != 0 ||
			(p_state->get_current(0).is_null() ||
					p_state->get_current(0)->get_spine_object() !=
							p_entry->get_spine_object())) {
		return;
	}
	if (p_entry->get_loop()) {
		if (!is_blocking_signals()) {
			_on_sprite_animation_looped();
		}
	} else {
		spine_playing = false;
		_update_spine_speed();
		if (!is_blocking_signals()) {
			_on_sprite_animation_finished();
		}
	}
}

GdBool SpxSprite::has_spine_animation(GdString p_name) const {
	return spine_child != nullptr && spine_child->get_skeleton_data_res()->find_animation(SpxStr(p_name)).is_valid();
}

void SpxSprite::_set_spine_color(const Color &p_color) {
	Color color = p_color;
	Ref<ShaderMaterial> material = anim2d->get_material();
	if (material.is_valid()) {
		const Variant alpha = material->get_shader_parameter("alpha_amount");
		if (alpha.get_type() != Variant::NIL) {
			color.a *= CLAMP(1.0f - float(alpha), 0.0f, 1.0f);
		}
	}
	spine_child->set_modulate(color);
}

void SpxSprite::_stop_spine_animation() {
	spine_child->get_animation_state()->clear_tracks();
}

Rect2 SpxSprite::_get_spine_rect() const {
	return spine_child->get_global_transform().xform(spine_bounds);
}

#else

GdBool SpxSprite::set_spine_skeleton(GdString, GdString, GdFloat) {
	ERR_PRINT("Spine support is disabled. Rebuild with SPX_SPINE=1 (SCons spx_spine=yes).");
	return false;
}
void SpxSprite::clear_spine_skeleton() {}
GdBool SpxSprite::has_spine_animation(GdString) const { return false; }
void SpxSprite::_play_spine_animation(const String &, float, bool, bool) {}
void SpxSprite::_update_spine_transform() {}
void SpxSprite::_update_spine_speed() {}
void SpxSprite::_stop_spine_animation() {}
void SpxSprite::_set_spine_color(const Color &) {}
Rect2 SpxSprite::_get_spine_rect() const { return Rect2(); }

#endif
