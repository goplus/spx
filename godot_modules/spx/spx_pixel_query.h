/**************************************************************************/
/*  spx_pixel_query.h                                                    */
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

#ifndef SPX_PIXEL_QUERY_H
#define SPX_PIXEL_QUERY_H

#include "scene/2d/animated_sprite_2d.h"
#include <vector>

namespace SpxPixelQuery {

// Per-query values and strong resource references, with no manager or sprite
// ownership. Images are fetched lazily, only after the world bounds overlap.
struct Snapshot {
	Ref<Texture2D> texture;
	Ref<Image> image;
	Rect2 bounds;
	Rect2 local_rect;
	Transform2D inverse_transform;
	Vector2i image_size;
	real_t collision_alpha_scale = 1.0f;
	bool flip_h = false;
	bool flip_v = false;
};

struct Layer {
	Snapshot pixel_query;
	int z_index = 0;
	int tree_index = 0;
};

Ref<Texture2D> frame_texture(AnimatedSprite2D *p_animation);
Rect2 local_rect(AnimatedSprite2D *p_animation, const Vector2 &p_texture_size);
Rect2 world_bounds(const Transform2D &p_transform, const Rect2 &p_local_rect);
bool capture(AnimatedSprite2D *p_animation, bool p_apply_collision_alpha, Snapshot &r_snapshot);
bool load_image(Snapshot &r_snapshot);

// Select world pixel centers, not pixel edges. Sampling is x-major and keeps
// its origin at this rectangle's first center even when the step exceeds one.
Rect2i pixel_centers(const Rect2 &p_bounds);
Rect2i overlap(const Rect2 &p_a, const Rect2 &p_b);
template <typename Predicate>
bool any_pixel_center(const Rect2i &p_rect, int p_step, const Predicate &p_matches) {
	// Callers clamp p_step to at least one when setting the sampling policy.
	const Vector2i end = p_rect.position + p_rect.size;
	for (int x = p_rect.position.x; x < end.x; x += p_step) {
		for (int y = p_rect.position.y; y < end.y; y += p_step) {
			if (p_matches(Vector2((real_t)x + 0.5f, (real_t)y + 0.5f))) {
				return true;
			}
		}
	}
	return false;
}

// load_image must succeed first. Apply inverse transform, pivot/centering and
// texture flips before flooring; this also preserves negative-scale behavior.
bool sample(const Snapshot &p_snapshot, const Vector2 &p_world_pos, Color &r_color);
bool sample_premultiplied(const Snapshot &p_snapshot, const Vector2 &p_world_pos, Color &r_color);

// Layers arrive front-to-back (descending z, then descending tree index).
// Scratch composites their premultiplied colors over a white clear color.
Color composite(const std::vector<Layer> &p_layers, const Vector2 &p_world_pos);

} // namespace SpxPixelQuery

#endif // SPX_PIXEL_QUERY_H
