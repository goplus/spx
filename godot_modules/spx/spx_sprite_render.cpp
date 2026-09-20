/**************************************************************************/
/*  spx_sprite_render.cpp                                                 */
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

#include "core/math/math_funcs.h"
#include "scene/2d/animated_sprite_2d.h"

#include "spx_camera_mgr.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "spx_sprite_render_util.h"
#include "svg_mgr.h"

void SpxSprite::set_render_offset(GdVec2 p_render_offset) {
	if (render_offset == p_render_offset) {
		return;
	}

	render_offset = p_render_offset;
	if (render_root != nullptr) {
		render_root->set_position(render_offset);
	}
	if (enable_dynamic_frame_offset) {
		_on_frame_changed();
	}
}

void SpxSprite::set_render_scale(GdVec2 p_scale) {
	_render_scale = p_scale;
	_update_anim_scale();
	_on_frame_changed();
}

GdVec2 SpxSprite::get_render_scale() {
	return _render_scale;
}

void SpxSprite::set_dynamic_frame_offset_enabled(GdBool p_enabled) {
	enable_dynamic_frame_offset = p_enabled;

	if (enable_dynamic_frame_offset) {
		_on_frame_changed();
		return;
	}

	if (anim2d != nullptr) {
		anim2d->set_offset(base_offset);
	}
}

GdBool SpxSprite::is_dynamic_frame_offset_enabled() const {
	return enable_dynamic_frame_offset;
}

void SpxSprite::_commit_visual(const PreparedVisual &p_visual) {
	visual_source = p_visual.source;
	source_sprite_frames = p_visual.shared_frames;
	anim2d->set_sprite_frames(p_visual.frames);
	anim2d->set_animation(p_visual.animation);
	anim2d->set_scale(_render_scale / visual_source.raster_scale);
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}

void SpxSprite::_update_anim_scale() {
	if (anim2d == nullptr) {
		return;
	}
	if (visual_source.is_svg()) {
		const int target_scale = _get_actual_match_render_scale();
		if (target_scale != visual_source.raster_scale) {
			_update_svg_scale_content(target_scale);
		}
	}
	// Failed rasterization retains the old source and its actual raster scale.
	anim2d->set_scale(_render_scale / visual_source.raster_scale);
}

bool SpxSprite::_update_svg_scale_content(int p_target_scale) {
	PreparedVisual visual;
	if (visual_source.is_single_image()) {
		Ref<Texture2D> texture = svgMgr->get_svg_image(visual_source.key, p_target_scale);
		if (texture.is_null()) {
			return false;
		}
		VisualSource source = visual_source;
		source.raster_scale = p_target_scale;
		_prepare_texture(texture, source, visual);
	} else {
		if (!_prepare_animation(visual_source.animation_name, visual,
					p_target_scale)) {
			return false;
		}
		const Ref<SpriteFrames> old_frames = anim2d->get_sprite_frames();
		visual.frames->set_animation_loop(
				visual.animation,
				old_frames->get_animation_loop(anim2d->get_animation()));
	}
	const bool was_playing = anim2d->is_playing();
	const int frame = anim2d->get_frame();
	const float progress = anim2d->get_frame_progress();
	_commit_visual(visual);
	if (!visual_source.is_single_image()) {
		// Preserve custom speed even while paused or speed_scale is zero.
		anim2d->play(visual.animation, playback_speed);
		if (!was_playing) {
			anim2d->pause();
		}
		anim2d->set_frame_and_progress(frame, progress);
		_on_frame_changed();
		_update_current_frame_shader_uv_rect();
	}
	return true;
}

int SpxSprite::_get_actual_match_render_scale() {
	return svgMgr->calculate_svg_scale(_get_actual_render_scale());
}

Vector2 SpxSprite::_get_actual_render_scale() {
	if (anim2d == nullptr) {
		return Vector2(1.0f, 1.0f);
	}

	Vector2 global_scale = get_global_transform().get_scale() * _render_scale;
	SpxEngine *engine = SpxEngine::get_singleton();
	if (engine != nullptr) {
		SpxCameraMgr *camera_mgr = engine->get_camera();
		if (camera_mgr != nullptr) {
			global_scale *= camera_mgr->get_camera_zoom();
		}
	}

	return global_scale;
}

void SpxSprite::_on_frame_changed() {
	if (anim2d == nullptr || !enable_dynamic_frame_offset) {
		return;
	}

	String current_anim = String(anim2d->get_animation());
	int current_frame = anim2d->get_frame();
	Vector2 frame_offset = resMgr->get_animation_frame_offset(current_anim, current_frame);
	Vector2 final_offset = spx_compute_anim_offset(visual_source.is_single_image(), base_offset, render_offset, frame_offset, _render_scale);
	anim2d->set_offset(final_offset);
}

void SpxSprite::_update_current_frame_shader_uv_rect() {
	if (anim2d == nullptr || default_material.is_null()) {
		return;
	}

	default_material->set_shader_parameter(SNAME("atlas_uv_rect2"), spx_get_animation_frame_uv_rect(anim2d->get_sprite_frames(), anim2d->get_animation(), anim2d->get_frame()));
}
