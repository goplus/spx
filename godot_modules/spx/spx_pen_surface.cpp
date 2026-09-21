/**************************************************************************/
/*  spx_pen_surface.cpp                                                   */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including  */
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

#include "spx_pen_surface.h"

#include "scene/resources/canvas_item_material.h"
#include "servers/rendering_server.h"
#include "spx_pixel_query.h"

RID SpxPenCanvas::_create_draw_item(const Ref<Texture2D> &p_texture, const Ref<Material> &p_material) {
	RenderingServer *server = RenderingServer::get_singleton();
	RID item = server->canvas_item_create();
	server->canvas_item_set_parent(item, get_canvas_item());
	server->canvas_item_set_draw_index(item, draw_items.size());
	draw_items.push_back({ item, p_texture, p_material });
	return item;
}

void SpxPenCanvas::_clear_draw_items() {
	for (const DrawItem &item : draw_items) {
		RenderingServer::get_singleton()->free(item.rid);
	}
	draw_items.clear();
}

void SpxPenCanvas::_draw_line_batch(int p_begin, int p_end) {
	constexpr int CAP_SEGMENTS = 12;
	constexpr int DISC_VERTEX_COUNT = CAP_SEGMENTS + 1;
	constexpr int DISC_INDEX_COUNT = CAP_SEGMENTS * 3;
	Vector<int> indices;
	Vector<Point2> vertices;
	Vector<Color> colors;
	int64_t required_vertices = 0;
	int64_t required_indices = 0;
	for (int i = p_begin; i < p_end; i++) {
		const DrawCommand &command = pending_commands[i];
		if ((command.to - command.from).is_zero_approx()) {
			required_vertices += DISC_VERTEX_COUNT;
			required_indices += DISC_INDEX_COUNT;
		} else {
			const int disc_count = command.draw_start_cap ? 2 : 1;
			required_vertices += 4 + disc_count * DISC_VERTEX_COUNT;
			required_indices += 6 + disc_count * DISC_INDEX_COUNT;
		}
	}

	ERR_FAIL_COND_MSG(required_vertices > INT_MAX || required_indices > INT_MAX, "Pen line batch is too large.");
	ERR_FAIL_COND_MSG(vertices.resize(required_vertices) != OK, "Failed to allocate pen line vertices.");
	ERR_FAIL_COND_MSG(colors.resize(required_vertices) != OK, "Failed to allocate pen line colors.");
	ERR_FAIL_COND_MSG(indices.resize(required_indices) != OK, "Failed to allocate pen line indices.");

	Point2 *vertex_data = vertices.ptrw();
	Color *color_data = colors.ptrw();
	int *index_data = indices.ptrw();
	int vertex_count = 0;
	int index_count = 0;

	auto append_vertex = [&](const Vector2 &p_position, const Color &p_color) {
		vertex_data[vertex_count] = p_position;
		color_data[vertex_count] = p_color;
		vertex_count++;
	};
	auto append_index = [&](int p_index) {
		index_data[index_count++] = p_index;
	};
	auto append_disc = [&](const Vector2 &p_center, float p_radius, const Color &p_color) {
		const int center_index = vertex_count;
		append_vertex(p_center, p_color);
		for (int i = 0; i < CAP_SEGMENTS; i++) {
			const float angle = Math_TAU * (float)i / (float)CAP_SEGMENTS;
			append_vertex(p_center + Vector2(Math::cos(angle), Math::sin(angle)) * p_radius, p_color);
		}
		for (int i = 0; i < CAP_SEGMENTS; i++) {
			append_index(center_index);
			append_index(center_index + 1 + i);
			append_index(center_index + 1 + ((i + 1) % CAP_SEGMENTS));
		}
	};

	for (int i = p_begin; i < p_end; i++) {
		const DrawCommand &command = pending_commands[i];
		const float radius = MAX(command.width * 0.5f, 0.5f);
		const Vector2 delta = command.to - command.from;
		if (delta.is_zero_approx()) {
			append_disc(command.from, radius, command.color);
			continue;
		}

		const Vector2 normal = Vector2(-delta.y, delta.x).normalized() * radius;
		const int quad_index = vertex_count;
		append_vertex(command.from + normal, command.color);
		append_vertex(command.from - normal, command.color);
		append_vertex(command.to + normal, command.color);
		append_vertex(command.to - normal, command.color);
		append_index(quad_index);
		append_index(quad_index + 1);
		append_index(quad_index + 2);
		append_index(quad_index + 2);
		append_index(quad_index + 1);
		append_index(quad_index + 3);

		// Scratch uses round pen caps. A continuing stroke reuses the preceding
		// endpoint cap, including when the segments are flushed in different frames.
		if (command.draw_start_cap) {
			append_disc(command.from, radius, command.color);
		}
		append_disc(command.to, radius, command.color);
	}

	DEV_ASSERT(vertex_count == required_vertices);
	DEV_ASSERT(index_count == required_indices);
	if (!indices.is_empty()) {
		RenderingServer::get_singleton()->canvas_item_add_triangle_array(_create_draw_item(), indices, vertices, colors);
	}
}

void SpxPenCanvas::_bind_methods() {
}

void SpxPenCanvas::submit(bool p_append) {
	if (!p_append) {
		_clear_draw_items();
	}

	RenderingServer *server = RenderingServer::get_singleton();
	for (int i = 0; i < pending_commands.size();) {
		const DrawCommand &command = pending_commands[i];
		int end = i + 1;
		if (command.type == DrawCommand::LINE) {
			while (end < pending_commands.size() && pending_commands[end].type == DrawCommand::LINE) {
				end++;
			}
			_draw_line_batch(i, end);
		} else {
			RID item = _create_draw_item(command.texture, command.material);
			server->canvas_item_set_transform(item, command.transform);
			server->canvas_item_set_material(item, command.material.is_valid() ? command.material->get_rid() : RID());
			server->canvas_item_set_default_texture_filter(item, RS::CanvasItemTextureFilter(command.texture_filter));
			server->canvas_item_set_default_texture_repeat(item, RS::CanvasItemTextureRepeat(command.texture_repeat));
			command.texture->draw_rect_region(item, command.rect, Rect2(Vector2(), command.texture->get_size()), command.color);
		}
		i = end;
	}
	pending_commands.clear();
}

void SpxPenCanvas::add_line(const Vector2 &p_from, const Vector2 &p_to, float p_width, const Color &p_color, bool p_draw_start_cap) {
	DrawCommand command;
	command.type = DrawCommand::LINE;
	command.from = p_from;
	command.to = p_to;
	command.width = MAX(p_width, 1.0f);
	command.color = p_color;
	command.draw_start_cap = p_draw_start_cap;
	pending_commands.push_back(command);
}

void SpxPenCanvas::add_stamp(const Ref<Texture2D> &p_texture, const Vector2 &p_position, float p_rotation, const Vector2 &p_scale) {
	ERR_FAIL_COND(p_texture.is_null());
	DrawCommand command;
	command.type = DrawCommand::STAMP;
	command.texture = p_texture;
	command.transform = Transform2D(p_rotation, p_scale, 0, p_position);
	command.rect = Rect2(-p_texture->get_size() * 0.5f, p_texture->get_size());
	command.color = Color(1, 1, 1, 1);
	pending_commands.push_back(command);
}

void SpxPenCanvas::add_stamp(AnimatedSprite2D *p_sprite, const Transform2D &p_transform) {
	DrawCommand command;
	command.type = DrawCommand::STAMP;
	command.texture = SpxPixelQuery::frame_texture(p_sprite);
	if (command.texture.is_null()) {
		return;
	}
	command.transform = p_transform;
	command.rect = SpxPixelQuery::local_rect(p_sprite, command.texture->get_size());
	if (p_sprite->is_flipped_h()) {
		command.rect.size.x = -command.rect.size.x;
	}
	if (p_sprite->is_flipped_v()) {
		command.rect.size.y = -command.rect.size.y;
	}
	command.color = p_sprite->get_modulate_in_tree() * p_sprite->get_self_modulate();
	command.texture_filter = p_sprite->get_texture_filter_in_tree();
	command.texture_repeat = p_sprite->get_texture_repeat_in_tree();
	const Ref<Material> material = p_sprite->get_material();
	if (material.is_valid()) {
		// Share textures and shader code, but freeze all material parameters at
		// the call site. Later effect changes must not alter queued stamps.
		command.material = material->duplicate(false);
	}
	pending_commands.push_back(command);
}

void SpxPenCanvas::clear_commands() {
	pending_commands.clear();
	_clear_draw_items();
}

SpxPenCanvas::~SpxPenCanvas() {
	_clear_draw_items();
}

void SpxPenSurface::_bind_methods() {
}

void SpxPenSurface::initialize(const Size2i &p_size) {
	ERR_FAIL_COND(render_target != nullptr);

	// Scratch keeps pen pixels between the backdrop and all managed sprites.
	// Use an absolute layer so the result does not depend on the scene-tree
	// insertion order or a future parent z-index change.
	set_z_as_relative(false);
	set_z_index(0);

	render_target = memnew(SubViewport);
	render_target->set_name("pen_render_target");
	render_target->set_transparent_background(true);
#ifndef _3D_DISABLED
	render_target->set_disable_3d(true);
#endif
	render_target->set_handle_input_locally(false);
	render_target->set_clear_mode(SubViewport::CLEAR_MODE_ONCE);
	render_target->set_update_mode(SubViewport::UPDATE_ONCE);
	add_child(render_target);

	canvas = memnew(SpxPenCanvas);
	canvas->set_name("pen_canvas_drawer");
	render_target->add_child(canvas);

	Sprite2D *canvas_sprite = memnew(Sprite2D);
	canvas_sprite->set_name("pen_canvas");
	canvas_sprite->set_centered(true);
	canvas_sprite->set_texture_filter(CanvasItem::TEXTURE_FILTER_NEAREST);
	canvas_sprite->set_texture(render_target->get_texture());
	// Transparent render targets store premultiplied RGB. Do not multiply it
	// by alpha a second time when displaying the shared pen layer.
	Ref<CanvasItemMaterial> material;
	material.instantiate();
	material->set_blend_mode(CanvasItemMaterial::BLEND_MODE_PREMULT_ALPHA);
	canvas_sprite->set_material(material);
	add_child(canvas_sprite);
	set_canvas_size(p_size);
}

void SpxPenSurface::set_canvas_size(const Size2i &p_size) {
	ERR_FAIL_NULL(render_target);
	ERR_FAIL_NULL(canvas);

	const Size2i next_size(MAX(1, p_size.x), MAX(1, p_size.y));
	if (canvas_size == next_size) {
		return;
	}

	canvas_size = next_size;
	render_target->set_size(canvas_size);
	// All commands use stage coordinates; the canvas centers them in the target.
	canvas->set_position(Vector2(canvas_size) * 0.5f);
	clear();
}

void SpxPenSurface::draw_line(const Vector2 &p_from, const Vector2 &p_to, float p_width, const Color &p_color, bool p_draw_start_cap) {
	ERR_FAIL_NULL(canvas);
	canvas->add_line(p_from, p_to, p_width, p_color, p_draw_start_cap);
}

void SpxPenSurface::draw_stamp(const Ref<Texture2D> &p_texture, const Vector2 &p_position, float p_rotation, const Vector2 &p_scale) {
	ERR_FAIL_NULL(canvas);
	if (p_texture.is_null()) {
		return;
	}
	canvas->add_stamp(p_texture, p_position, p_rotation, p_scale);
}

void SpxPenSurface::clear() {
	if (canvas != nullptr) {
		canvas->clear_commands();
	}
	clear_requested = true;
}

void SpxPenSurface::draw_stamp(AnimatedSprite2D *p_sprite) {
	ERR_FAIL_NULL(canvas);
	ERR_FAIL_NULL(p_sprite);
	canvas->add_stamp(p_sprite, get_global_transform().affine_inverse() * p_sprite->get_global_transform());
}

void SpxPenSurface::flush() {
	if (canvas == nullptr || (!clear_requested && !canvas->has_pending_commands())) {
		return;
	}

	// A second flush before rendering must retain the first batch and its
	// pending clear. UPDATE_ONCE is reset by the renderer, not by submission.
	const bool pending = _is_render_pending();
	if (clear_requested || !pending) {
		render_target->set_clear_mode(clear_requested ? SubViewport::CLEAR_MODE_ONCE : SubViewport::CLEAR_MODE_NEVER);
	}
	canvas->submit(pending);
	render_target->set_update_mode(SubViewport::UPDATE_ONCE);
	collision_image.unref();
	clear_requested = false;
}

bool SpxPenSurface::_is_render_pending() const {
	return RenderingServer::get_singleton()->viewport_get_update_mode(render_target->get_viewport_rid()) == RS::VIEWPORT_UPDATE_ONCE;
}

bool SpxPenSurface::capture(const Rect2 &p_query_bounds, SpxPixelQuery::Snapshot &r_snapshot) {
	r_snapshot = SpxPixelQuery::Snapshot();
	if (render_target == nullptr || !is_inside_tree()) {
		return false;
	}
	r_snapshot.local_rect = Rect2(-Vector2(canvas_size) * 0.5f, Vector2(canvas_size));
	r_snapshot.bounds = SpxPixelQuery::world_bounds(get_global_transform(), r_snapshot.local_rect);
	if (!SpxPixelQuery::overlap(p_query_bounds, r_snapshot.bounds).has_area()) {
		return false;
	}
	r_snapshot.inverse_transform = get_global_transform().affine_inverse();
	flush();
	if (collision_image.is_null()) {
		RenderingServer *server = RenderingServer::get_singleton();
		if (_is_render_pending()) {
			// Submit directly instead of flushing the global deferred-call queue:
			// sensing must see this script's pen commands before the next frame.
			canvas->force_update_transform();
			CanvasItemMaterial::flush_changes();
			server->draw(false);
			server->sync();
		}
		collision_image = render_target->get_texture()->get_image();
	}
	if (collision_image.is_null() || collision_image->is_empty()) {
		return false;
	}
	r_snapshot.image = collision_image;
	r_snapshot.image_size = collision_image->get_size();
	r_snapshot.image_premultiplied = true;
	return true;
}
