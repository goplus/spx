/**************************************************************************/
/*  spx_sprite_texture.cpp                                                */
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
#include "scene/main/viewport.h"
#include "scene/resources/atlas_texture.h"

#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "spx_sprite_mgr.h"
#include "svg_mgr.h"

void SpxSprite::set_texture_atlas_direct(GdString p_path, GdRect2 p_region, GdBool p_direct) {
	String path_str = SpxStr(p_path);
	current_anim_name = "";
	current_svg_anim_key = "";
	current_svg_path = "";
	is_svg_mode = false;

	Ref<Texture2D> texture = resMgr->load_texture(path_str, p_direct);
	ERR_FAIL_COND_MSG(texture.is_null(), "SpxSprite: texture atlas source is null.");

	Ref<AtlasTexture> atlas_texture_frame = memnew(AtlasTexture);
	atlas_texture_frame->set_atlas(texture);
	atlas_texture_frame->set_region(p_region);

	_play_single_image_animation(atlas_texture_frame);
}

void SpxSprite::set_texture_direct(GdString p_path, GdBool p_direct) {
	String path_str = SpxStr(p_path);
	current_anim_name = "";
	current_svg_anim_key = "";
	current_svg_path = "";
	is_single_image_mode = false;

	Ref<Texture2D> texture;
	is_svg_mode = svgMgr->is_svg_file(path_str);
	if (is_svg_mode) {
		int target_scale = _get_actual_match_render_scale();
		current_svg_scale = target_scale;
		current_svg_path = path_str;
		texture = svgMgr->get_svg_image(path_str, target_scale);
	} else {
		current_svg_scale = 1;
		texture = resMgr->load_texture(path_str, p_direct);
	}

	_play_single_image_animation(texture);
}

void SpxSprite::set_texture_atlas(GdString p_path, GdRect2 p_region) {
	set_texture_atlas_direct(p_path, p_region, false);
}

void SpxSprite::set_texture(GdString p_path) {
	set_texture_direct(p_path, false);
}

GdString SpxSprite::get_texture() {
	Ref<SpriteFrames> frames = anim2d != nullptr ? anim2d->get_sprite_frames() : Ref<SpriteFrames>();
	if (frames.is_null() || !frames->has_animation(SpxSpriteMgr::default_texture_anim) || frames->get_frame_count(SpxSpriteMgr::default_texture_anim) == 0) {
		return nullptr;
	}

	Ref<Texture2D> texture = frames->get_frame_texture(SpxSpriteMgr::default_texture_anim, 0);
	if (texture.is_null()) {
		return nullptr;
	}

	return SpxReturnStr(texture->get_name());
}

Rect2 SpxSprite::get_rect() const {
	Ref<SpriteFrames> frames = anim2d != nullptr ? anim2d->get_sprite_frames() : Ref<SpriteFrames>();
	if (frames.is_null() || !frames->has_animation(SpxSpriteMgr::default_texture_anim) || frames->get_frame_count(SpxSpriteMgr::default_texture_anim) == 0) {
		return Rect2(0, 0, 1, 1);
	}

	Ref<Texture2D> texture = frames->get_frame_texture(SpxSpriteMgr::default_texture_anim, 0);
	if (texture.is_null()) {
		return Rect2(0, 0, 1, 1);
	}

	Size2i size = texture->get_size();
	Point2 offset = render_root->get_global_position()  - Size2(size) / 2;

	if (get_viewport() != nullptr && get_viewport()->is_snap_2d_transforms_to_pixel_enabled()) {
		offset = (offset + Point2(0.5, 0.5)).floor();
	}

	if (size == Size2(0, 0)) {
		size = Size2(1, 1);
	}

	return Rect2(offset, size);
}

void SpxSprite::_play_single_image_animation(Ref<Texture2D> p_texture) {
	ERR_FAIL_COND_MSG(anim2d == nullptr, "SpxSprite: AnimatedSprite2D component is missing.");
	ERR_FAIL_COND_MSG(p_texture.is_null(), "SpxSprite: texture is null.");

	is_single_image_mode = true;
	anim2d->set_sprite_frames(default_sprite_frames);

	Ref<SpriteFrames> frames = anim2d->get_sprite_frames();
	ERR_FAIL_COND_MSG(frames.is_null(), "SpxSprite: SpriteFrames resource is missing.");

	if (!frames->has_animation(SpxSpriteMgr::default_texture_anim)) {
		frames->add_animation(SpxSpriteMgr::default_texture_anim);
	}

	if (frames->get_frame_count(SpxSpriteMgr::default_texture_anim) == 0) {
		frames->add_frame(SpxSpriteMgr::default_texture_anim, p_texture);
	} else {
		frames->set_frame(SpxSpriteMgr::default_texture_anim, 0, p_texture);
	}

	anim2d->set_animation(SpxSpriteMgr::default_texture_anim);
	_update_anim_scale();
	_on_frame_changed();
	_update_current_frame_shader_uv_rect();
}
