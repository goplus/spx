/**************************************************************************/
/*  spx_sprite_render_util.h                                              */
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

#ifndef SPX_SPRITE_RENDER_UTIL_H
#define SPX_SPRITE_RENDER_UTIL_H

#include "core/math/vector2.h"
#include "scene/resources/atlas_texture.h"
#include "scene/resources/sprite_frames.h"

// RenderRoot owns the costume center offset. In single-image mode the animation
// node inherits it directly; multi-frame animations cancel it before applying
// their own frame metadata so the final placement remains unchanged.
static inline Vector2 spx_compute_anim_offset(bool p_is_single_image_mode, const Vector2 &p_base_offset, const Vector2 &p_render_offset, const Vector2 &p_frame_offset, const Vector2 &p_render_scale) {
	Vector2 final_offset = p_base_offset + (p_frame_offset * p_render_scale);
	if (!p_is_single_image_mode) {
		final_offset -= p_render_offset;
	}
	return final_offset;
}

// Returns the normalized atlas region for an animation frame. This is SPX
// rendering policy, so keep it in the module instead of extending
// AnimatedSprite2D's core API.
static inline Rect2 spx_get_animation_frame_uv_rect(const Ref<SpriteFrames> &p_frames, const StringName &p_animation, int p_frame) {
	const Rect2 default_uv(0, 0, 1, 1);
	if (p_frames.is_null() || !p_frames->has_animation(p_animation) ||
			p_frame < 0 || p_frame >= p_frames->get_frame_count(p_animation)) {
		return default_uv;
	}

	const Ref<Texture2D> texture = p_frames->get_frame_texture(p_animation, p_frame);
	if (texture.is_null()) {
		return default_uv;
	}

	const Ref<AtlasTexture> atlas_texture = Object::cast_to<AtlasTexture>(texture.ptr());
	if (atlas_texture.is_null() || atlas_texture->get_atlas().is_null()) {
		return default_uv;
	}

	const Size2 atlas_size = atlas_texture->get_atlas()->get_size();
	if (atlas_size.x <= 0.0f || atlas_size.y <= 0.0f) {
		return default_uv;
	}

	const Rect2 region = atlas_texture->get_region();
	return Rect2(region.position / atlas_size, region.size / atlas_size);
}

#endif // SPX_SPRITE_RENDER_UTIL_H
