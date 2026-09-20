/**************************************************************************/
/*  spx_sprite_animation.cpp                                              */
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

#include "scene/2d/animated_sprite_2d.h"

#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "spx_sprite_render_util.h"
#include "svg_mgr.h"

GdString SpxSprite::get_current_anim_name() {
	return SpxReturnStr(visual_source.animation_name);
}

bool SpxSprite::_prepare_animation(const String &p_name,
		PreparedVisual &r_visual,
		int p_raster_scale) {
	r_visual = PreparedVisual();
	r_visual.source.animation_name = p_name;
	r_visual.source.key = p_name;
	Ref<SpriteFrames> shared_frames = default_sprite_frames;
	if (resMgr->is_dynamic_anim_mode()) {
		String key = resMgr->get_anim_key_name(get_spx_type_name(), p_name);
		// set_anim/get_anim also accept the complete engine animation key.
		if (resMgr->get_anim_frames(key).is_null() &&
				resMgr->get_anim_frames(p_name).is_valid()) {
			key = p_name;
		}
		r_visual.source.key = key;
		if (resMgr->is_svg_animation(key)) {
			r_visual.source.kind = VisualKind::SVG_ANIMATION;
			r_visual.source.raster_scale = p_raster_scale > 0
					? p_raster_scale
					: _get_actual_match_render_scale();
			shared_frames =
					svgMgr->get_svg_animation(key, r_visual.source.raster_scale);
		} else {
			shared_frames = resMgr->get_anim_frames(key);
		}
	}
	r_visual.animation = r_visual.source.key;
	ERR_FAIL_COND_V_MSG(shared_frames.is_null() ||
					!shared_frames->has_animation(r_visual.animation) ||
					shared_frames->get_frame_count(r_visual.animation) ==
							0,
			false, "SpxSprite: animation frames are missing.");
	r_visual.shared_frames = shared_frames;
	// Replaying the same clip must retain frame/progress, just as Godot play()
	// does. A new source gets private metadata, while textures stay shared.
	if (source_sprite_frames == shared_frames && anim2d->get_sprite_frames().is_valid() && anim2d->get_sprite_frames()->has_animation(r_visual.animation)) {
		r_visual.frames = anim2d->get_sprite_frames();
	} else if (resMgr->is_dynamic_anim_mode()) {
		r_visual.frames = spx_copy_animation_frames(shared_frames, r_visual.animation);
	} else {
		// Scene-authored sprites may contain several clips. Retain that complete
		// library when Node::duplicate() copies the current visual into a clone.
		r_visual.frames = shared_frames->duplicate(false);
	}
	return r_visual.frames.is_valid();
}

void SpxSprite::play_anim(GdString p_name, GdFloat p_speed, GdBool p_is_loop, GdBool p_from_end) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	PreparedVisual visual;
	if (!_prepare_animation(SpxStr(p_name), visual)) {
		return;
	}
	const bool changed_animation = anim2d->get_animation() != visual.animation;
	visual.frames->set_animation_loop(visual.animation, p_is_loop);
	_commit_visual(visual);
	playback_speed = p_speed;
	anim2d->play(visual.animation, p_speed, p_from_end);
	if (changed_animation && p_from_end) {
		// commit selected the new name already; preserve Godot play() semantics
		// for a new clip even when its effective playback speed is positive.
		anim2d->set_frame_and_progress(visual.frames->get_frame_count(visual.animation) - 1, 1.0);
	}
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}

void SpxSprite::play_backwards_anim(GdString p_name) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	PreparedVisual visual;
	if (!_prepare_animation(SpxStr(p_name), visual)) {
		return;
	}
	const bool changed_animation = anim2d->get_animation() != visual.animation;
	_commit_visual(visual);
	playback_speed = -1.0f;
	anim2d->play_backwards(visual.animation);
	if (changed_animation) {
		anim2d->set_frame_and_progress(visual.frames->get_frame_count(visual.animation) - 1, 1.0);
	}
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}

void SpxSprite::pause_anim() {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->pause();
}

void SpxSprite::stop_anim() {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->stop();
	playback_speed = 1.0f;
}

GdBool SpxSprite::is_playing_anim() const {
	ERR_FAIL_NULL_V_MSG(anim2d, false, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->is_playing();
}

void SpxSprite::set_anim(GdString p_name) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	PreparedVisual visual;
	if (!_prepare_animation(SpxStr(p_name), visual)) {
		return;
	}
	const bool was_playing = anim2d->is_playing();
	const bool changed_animation = anim2d->get_animation() != visual.animation;
	const bool backwards = playback_speed * anim2d->get_speed_scale() < 0;
	_commit_visual(visual);
	if (was_playing) {
		anim2d->play(visual.animation, playback_speed, changed_animation && backwards);
	}
}

GdString SpxSprite::get_anim() const {
	ERR_FAIL_NULL_V_MSG(anim2d, GdString(), "SpxSprite: AnimatedSprite2D component is missing.");
	return SpxReturnStr(String(anim2d->get_animation()));
}

void SpxSprite::set_anim_frame(GdInt p_frame) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_frame(p_frame);
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}

GdInt SpxSprite::get_anim_frame() const {
	ERR_FAIL_NULL_V_MSG(anim2d, 0, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->get_frame();
}

void SpxSprite::set_anim_speed_scale(GdFloat p_speed_scale) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_speed_scale(p_speed_scale);
}

GdFloat SpxSprite::get_anim_speed_scale() const {
	ERR_FAIL_NULL_V_MSG(anim2d, 1.0f, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->get_speed_scale();
}

GdFloat SpxSprite::get_anim_playing_speed() const {
	ERR_FAIL_NULL_V_MSG(anim2d, 0.0f, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->get_playing_speed();
}

void SpxSprite::set_anim_centered(GdBool p_center) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_centered(p_center);
}

GdBool SpxSprite::is_anim_centered() const {
	ERR_FAIL_NULL_V_MSG(anim2d, false, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->is_centered();
}

void SpxSprite::set_anim_offset(GdVec2 p_offset) {
	base_offset = p_offset;
	if (enable_dynamic_frame_offset) {
		_on_frame_changed();
		return;
	}

	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_offset(p_offset);
}

GdVec2 SpxSprite::get_anim_offset() const {
	ERR_FAIL_NULL_V_MSG(anim2d, GdVec2(), "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->get_offset();
}

void SpxSprite::set_anim_flip_h(GdBool p_flip) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_flip_h(p_flip);
}

GdBool SpxSprite::is_anim_flipped_h() const {
	ERR_FAIL_NULL_V_MSG(anim2d, false, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->is_flipped_h();
}

void SpxSprite::set_anim_flip_v(GdBool p_flip) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	anim2d->set_flip_v(p_flip);
}

GdBool SpxSprite::is_anim_flipped_v() const {
	ERR_FAIL_NULL_V_MSG(anim2d, false, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->is_flipped_v();
}
