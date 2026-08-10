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
#include "svg_mgr.h"

GdString SpxSprite::get_current_anim_name() {
	return SpxReturnStr(current_anim_name);
}

void SpxSprite::play_anim(GdString p_name, GdFloat p_speed, GdBool p_is_loop, GdBool p_from_end) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");

	String anim_name = SpxStr(p_name);
	String final_anim_key = anim_name;

	is_svg_mode = false;
	is_single_image_mode = false;
	current_anim_name = anim_name;
	current_svg_path = "";

	if (resMgr->is_dynamic_anim_mode()) {
		String sprite_type = get_spx_type_name();
		String base_anim_key = resMgr->get_anim_key_name(sprite_type, anim_name);
		Ref<SpriteFrames> frames;

		current_svg_anim_key = base_anim_key;
		is_svg_mode = svgMgr->is_svg_animation(base_anim_key);
		if (is_svg_mode) {
			int target_scale = _get_actual_match_render_scale();
			current_svg_scale = target_scale;
			frames = svgMgr->get_svg_animation(base_anim_key, target_scale);
		} else {
			current_svg_scale = 1;
			frames = resMgr->get_anim_frames(base_anim_key);
		}

		ERR_FAIL_COND_MSG(frames.is_null(), "SpxSprite: animation frames are missing.");
		ERR_FAIL_COND_MSG(!frames->has_animation(base_anim_key), "SpxSprite: animation is missing from SpriteFrames.");

		final_anim_key = base_anim_key;
		anim2d->set_sprite_frames(frames);
		frames->set_animation_loop(final_anim_key, p_is_loop);
	}

	anim2d->play(final_anim_key, p_speed, p_from_end);
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}

void SpxSprite::play_backwards_anim(GdString p_name) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");

	String anim_name = SpxStr(p_name);

	if (resMgr->is_dynamic_anim_mode()) {
		String base_anim_key = resMgr->get_anim_key_name(get_spx_type_name(), anim_name);
		Ref<SpriteFrames> frames;

		current_anim_name = anim_name;
		current_svg_anim_key = base_anim_key;
		current_svg_path = "";
		is_single_image_mode = false;
		is_svg_mode = svgMgr->is_svg_animation(base_anim_key);
		if (is_svg_mode) {
			int target_scale = _get_actual_match_render_scale();
			current_svg_scale = target_scale;
			frames = svgMgr->get_svg_animation(base_anim_key, target_scale);
		} else {
			current_svg_scale = 1;
			frames = resMgr->get_anim_frames(base_anim_key);
		}

		ERR_FAIL_COND_MSG(frames.is_null(), "SpxSprite: animation frames are missing.");
		ERR_FAIL_COND_MSG(!frames->has_animation(base_anim_key), "SpxSprite: animation is missing from SpriteFrames.");

		anim_name = base_anim_key;
		anim2d->set_sprite_frames(frames);
	}

	anim2d->play_backwards(anim_name);
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
}

GdBool SpxSprite::is_playing_anim() const {
	ERR_FAIL_NULL_V_MSG(anim2d, false, "SpxSprite: AnimatedSprite2D component is missing.");
	return anim2d->is_playing();
}

void SpxSprite::set_anim(GdString p_name) {
	ERR_FAIL_NULL_MSG(anim2d, "SpxSprite: AnimatedSprite2D component is missing.");
	current_anim_name = SpxStr(p_name);
	anim2d->set_animation(StringName(current_anim_name));
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
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
