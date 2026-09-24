/**************************************************************************/
/*  spx_pixel_query.cpp                                                    */
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

#include "spx_pixel_query.h"

#include "scene/resources/material.h"
#include "spx_image_texture.h"

namespace SpxPixelQuery {

Ref<Texture2D> frame_texture(AnimatedSprite2D *p_anim2d) {
	if (!p_anim2d) {
		return Ref<Texture2D>();
	}

	Ref<SpriteFrames> frames = p_anim2d->get_sprite_frames();
	if (frames.is_null()) {
		return Ref<Texture2D>();
	}

	const StringName current_animation = p_anim2d->get_animation();
	const int current_frame = p_anim2d->get_frame();
	if (!frames->has_animation(current_animation)) {
		return Ref<Texture2D>();
	}

	return frames->get_frame_texture(current_animation, current_frame);
}

Rect2 local_rect(AnimatedSprite2D *p_anim2d, const Vector2 &p_texture_size) {
	Vector2 ofs = p_anim2d->get_offset();
	if (p_anim2d->is_centered()) {
		ofs -= p_texture_size / 2.0;
	}
	return Rect2(ofs, p_texture_size);
}

Rect2 world_bounds(const Transform2D &p_transform, const Rect2 &p_local_rect) {
	return p_transform.xform(p_local_rect);
}

static _FORCE_INLINE_ real_t get_collision_alpha_scale(AnimatedSprite2D *p_anim2d) {
	if (p_anim2d == nullptr) {
		return 0.0f;
	}

	static const StringName alpha_amount_param("alpha_amount");
	Ref<Material> base_material = p_anim2d->get_material();
	Ref<ShaderMaterial> shader_material = base_material;
	if (shader_material.is_valid() && shader_material->get_shader().is_valid()) {
		const Variant alpha_amount = shader_material->get_shader_parameter(alpha_amount_param);
		if (alpha_amount.get_type() != Variant::NIL) {
			return CLAMP((real_t)(1.0 - (double)alpha_amount), (real_t)0.0f, (real_t)1.0f);
		}
	}

	return 1.0f;
}

Rect2i pixel_centers(const Rect2 &p_rect) {
	const Vector2i begin(
			(int)Math::ceil(p_rect.position.x - 0.5f),
			(int)Math::ceil(p_rect.position.y - 0.5f));
	const Vector2i end(
			(int)Math::floor(p_rect.position.x + p_rect.size.x - 0.5f) + 1,
			(int)Math::floor(p_rect.position.y + p_rect.size.y - 0.5f) + 1);

	return Rect2i(begin, Vector2i(MAX(end.x - begin.x, 0), MAX(end.y - begin.y, 0)));
}

Rect2i overlap(const Rect2 &p_a, const Rect2 &p_b) {
	return pixel_centers(p_a).intersection(pixel_centers(p_b));
}

static _FORCE_INLINE_ bool read_image_pixel(const Ref<Image> &p_image, const Vector2i &p_image_size, const Vector2 &p_local_pos, Color &r_color) {
	const int px = (int)Math::floor(p_local_pos.x);
	const int py = (int)Math::floor(p_local_pos.y);
	if (px < 0 || px >= p_image_size.x || py < 0 || py >= p_image_size.y) {
		return false;
	}

	r_color = p_image->get_pixel(px, py);
	return true;
}

bool capture(AnimatedSprite2D *p_anim2d, bool p_apply_collision_alpha, Snapshot &r_query) {
	r_query = Snapshot();
	if (!p_anim2d) {
		return false;
	}
	r_query.collision_alpha_scale = p_apply_collision_alpha ? get_collision_alpha_scale(p_anim2d) : 1.0f;
	if (p_apply_collision_alpha && r_query.collision_alpha_scale <= 0.0f) {
		return false;
	}

	r_query.texture = frame_texture(p_anim2d);
	if (r_query.texture.is_null()) {
		return false;
	}

	const Vector2 texture_size = r_query.texture->get_size();
	r_query.local_rect = local_rect(p_anim2d, texture_size);
	r_query.bounds = world_bounds(p_anim2d->get_global_transform(), r_query.local_rect);
	r_query.inverse_transform = p_anim2d->get_global_transform().affine_inverse();
	r_query.image_size = Vector2i((int)texture_size.x, (int)texture_size.y);
	r_query.flip_h = p_anim2d->is_flipped_h();
	r_query.flip_v = p_anim2d->is_flipped_v();
	return true;
}

bool load_image(Snapshot &r_query, ImageCache *p_cache) {
	if (r_query.image.is_valid()) {
		return true;
	}
	if (r_query.texture.is_null()) {
		return false;
	}

	const SpxImageTexture *texture = p_cache != nullptr ? dynamic_cast<const SpxImageTexture *>(r_query.texture.ptr()) : nullptr;
	const uint64_t version = texture != nullptr ? texture->image_version() : 0;
	if (texture != nullptr) {
		const auto it = p_cache->find(texture);
		if (it != p_cache->end() && it->second.version == version) {
			r_query.image = it->second.image;
		}
	}
	if (r_query.image.is_null()) {
		r_query.image = r_query.texture->get_image();
		if (texture != nullptr) {
			if (r_query.image.is_valid()) {
				(*p_cache)[texture] = { r_query.texture, r_query.image, version };
			} else {
				p_cache->erase(texture);
			}
		}
	}
	if (r_query.image.is_null()) {
		return false;
	}

	r_query.image_size = r_query.image->get_size();
	return true;
}

static _FORCE_INLINE_ Vector2 to_image_coord(const Snapshot &p_query, const Vector2 &p_world_pos) {
	Vector2 image_pos = p_query.inverse_transform.xform(p_world_pos) - p_query.local_rect.position;
	if (p_query.flip_h) {
		image_pos.x = (real_t)p_query.image_size.x - image_pos.x;
	}
	if (p_query.flip_v) {
		image_pos.y = (real_t)p_query.image_size.y - image_pos.y;
	}
	return image_pos;
}

bool sample(
		const Snapshot &p_query,
		const Vector2 &p_world_pos,
		Color &r_color) {
	const Vector2 local_pos = to_image_coord(p_query, p_world_pos);
	if (!read_image_pixel(p_query.image, p_query.image_size, local_pos, r_color)) {
		return false;
	}
	r_color.a *= p_query.collision_alpha_scale;
	return true;
}

bool sample_premultiplied(
		const Snapshot &p_query,
		const Vector2 &p_world_pos,
		Color &r_color) {
	if (!sample(p_query, p_world_pos, r_color)) {
		return false;
	}
	const real_t rgb_scale = p_query.image_premultiplied ? p_query.collision_alpha_scale : r_color.a;
	r_color.r *= rgb_scale;
	r_color.g *= rgb_scale;
	r_color.b *= rgb_scale;
	return true;
}

bool Layer::in_front_of(const Layer &p_other) const {
	if (z_index != p_other.z_index) {
		return z_index > p_other.z_index;
	}
	if (order != p_other.order) {
		return order > p_other.order;
	}
	return tree_index > p_other.tree_index;
}

Color composite(
		const std::vector<Layer> &p_queries,
		const Vector2 &p_world_pos) {
	Color composed_color(0.0f, 0.0f, 0.0f, 0.0f);
	real_t remaining_alpha = 1.0f;

	for (const Layer &query : p_queries) {
		if (remaining_alpha <= 0.0f) {
			break;
		}
		if (!query.pixel_query.bounds.has_point(p_world_pos)) {
			continue;
		}

		Color sample_color;
		if (!sample_premultiplied(query.pixel_query, p_world_pos, sample_color) || sample_color.a <= 0.0f) {
			continue;
		}

		composed_color.r += sample_color.r * remaining_alpha;
		composed_color.g += sample_color.g * remaining_alpha;
		composed_color.b += sample_color.b * remaining_alpha;
		remaining_alpha *= 1.0f - sample_color.a;
	}

	// Scratch blends the remaining transparent area over a white clear color.
	composed_color.r += remaining_alpha;
	composed_color.g += remaining_alpha;
	composed_color.b += remaining_alpha;
	composed_color.a = 1.0f - remaining_alpha;
	return composed_color;
}

} // namespace SpxPixelQuery
