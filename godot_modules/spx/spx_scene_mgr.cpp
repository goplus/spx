/**************************************************************************/
/*  spx_scene_mgr.cpp                                                     */
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

#include "spx_scene_mgr.h"

#include "core/config/project_settings.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/sprite_2d.h"
#include "scene/resources/2d/capsule_shape_2d.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/convex_polygon_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"

#include "spx_coordinate.h"
#include "spx_draw_tiles.h"
#include "spx_engine.h"
#include "spx_layer_sorter.h"
#include "spx_physics_mgr.h"
#include "spx_res_mgr.h"
#include "spx_sprite.h"
#include "spx_sprite_mgr.h"

void SpxSceneMgr::_request_export(SubViewport *viewport) {
	viewport_to_export = viewport;
	export_pending = true;
	elapsed = 0.0;
}

void SpxSceneMgr::_export_vp_png(SubViewport *viewport) {
	Ref<Image> image = viewport->get_texture()->get_image();
	Error err = image->save_png(DEFAULT_SAVE_PATH);
	String full_path = ProjectSettings::get_singleton()->globalize_path(DEFAULT_SAVE_PATH);
	if (err == OK) {
		print_line_rich("TileMap scene saved to: " + full_path);
	} else {
		print_error("Failed to save TileMap scene!");
	}

	viewport->queue_free();
}

void SpxSceneMgr::on_awake() {
	SpxBaseMgr::on_awake();
	pure_sprite_root = memnew(Node2D);
	pure_sprite_root->set_name("pure_sprite_root");
	get_spx_root()->add_child(pure_sprite_root);
}

void SpxSceneMgr::on_update(float delta) {
	if (export_pending) {
		elapsed += delta;
		if (elapsed >= 1.0) {
			export_pending = false;
			_export_vp_png(viewport_to_export);
		}
	}
}

void SpxSceneMgr::on_destroy() {
	// Clear pure sprites without recreating the root node (we're destroying)
	id_pure_sprites.clear();
	SpxLayerSorter::instance().reset();
	if (pure_sprite_root) {
		pure_sprite_root->queue_free();
	}
	pure_sprite_root = nullptr;
	SpxBaseMgr::on_destroy();
}

void SpxSceneMgr::on_reset(int reset_code) {
	clear_pure_sprites();
}

void SpxSceneMgr::clear_pure_sprites() {
	id_pure_sprites.clear();
	// Clear SpxLayerSorter to avoid dangling pointers
	SpxLayerSorter::instance().reset();
	if (pure_sprite_root) {
		pure_sprite_root->queue_free();
		pure_sprite_root = memnew(Node2D);
		pure_sprite_root->set_name("pure_sprite_root");
		get_spx_root()->add_child(pure_sprite_root);
	}
}

void SpxSceneMgr::create_pure_sprite(GdString texture_path, GdVec2 pos, GdInt zindex) {
	if (pure_sprite_root == nullptr) {
		return;
	}
	create_render_sprite(texture_path, pos, 0, GdVec2(1, 1), zindex, GdVec2(0, 0));
}

void SpxSceneMgr::destroy_pure_sprite(GdObj id) {
	if (id_pure_sprites.has(id)) {
		auto sprite = id_pure_sprites[id];
		id_pure_sprites.erase(id);

		// Clear SpxLayerSorter to avoid dangling pointers
		// Note: This is conservative but safe. Could be optimized later with per-id removal.
		SpxLayerSorter::instance().reset();

		// Cast to Node2D to remove from scene tree
		Node2D *node = dynamic_cast<Node2D *>(sprite);
		if (node && node->is_inside_tree()) {
			node->queue_free();
		}
	}
}

void SpxSceneMgr::collect_sortable_sprites(Vector<ISortableSprite *> &out) {
	for (auto &pair : id_pure_sprites) {
		if (pair.value && pair.value->is_node_valid()) {
			out.push_back(pair.value);
		}
	}
}

Rect2 SpxSceneMgr::get_scene_bounds(Node *node) {
	Rect2 total_rect;

	for (int i = 0; i < node->get_child_count(); i++) {
		Node *child = node->get_child(i);
		Rect2 child_rect;

		if (TileMapLayer *tml = Object::cast_to<TileMapLayer>(child)) {
			Rect2 tml_rect = get_tilemap_bounds(tml);
			if (tml_rect.has_area()) {
				child_rect = tml->get_global_transform().xform(tml_rect);
			}

		} else if (SpxSprite *sp = Object::cast_to<SpxSprite>(child)) {
			child_rect = sp->get_rect();
		}

		if (child_rect.has_area()) {
			total_rect = total_rect.has_area() ? total_rect.merge(child_rect) : child_rect;
		}

		Rect2 descendants_rect = get_scene_bounds(child);
		if (descendants_rect.has_area()) {
			total_rect = total_rect.has_area() ? total_rect.merge(descendants_rect) : descendants_rect;
		}
	}

	return total_rect;
}

Rect2 SpxSceneMgr::get_tilemap_bounds(TileMapLayer *layer) {
	if (!layer) {
		return Rect2();
	}

	Rect2i used = layer->get_used_rect();

	if (used.size == Vector2i(0, 0)) {
		return Rect2();
	}

	Vector2 top_left = layer->map_to_local(used.position);
	Vector2 bottom_right = layer->map_to_local(used.position + used.size);

	Rect2 rect(top_left - cached_cell_size / 2, bottom_right - top_left);
	return rect;
}

void SpxSceneMgr::export_scene_as_png(Node *root) {
	if (!root) {
		print_error("Root is null");
		return;
	}

	Rect2 rect = get_scene_bounds(root);
	if (rect.size == Vector2(0, 0)) {
		print_error("No TileMapLayer found in scene!");
		return;
	}

	SubViewport *viewport = memnew(SubViewport);
	viewport->set_size(rect.size);
	viewport->set_update_mode(SubViewport::UPDATE_ALWAYS);
	viewport->set_clear_mode(SubViewport::CLEAR_MODE_ALWAYS);

	Node *copy = root->duplicate(Node::DUPLICATE_USE_INSTANTIATION);
	if (Node2D *n2d = Object::cast_to<Node2D>(copy)) {
		n2d->set_position(n2d->get_global_position() - rect.position);
	}
	viewport->add_child(copy);
	get_tree()->get_current_scene()->add_child(viewport);
	_request_export(viewport);
}

GdObj SpxSceneMgr::create_render_sprite(GdString texture_path, GdVec2 pos, GdFloat degree, GdVec2 scale, GdInt zindex, GdVec2 pivot) {
	if (pure_sprite_root == nullptr) {
		return NULL_OBJECT_ID;
	}

	SpxRenderSprite *sprite = memnew(SpxRenderSprite);
	sprite->set_pivot(spx_to_godot_vec2(pivot));
	auto path_str = SpxStr(texture_path);
	Ref<Texture2D> texture = resMgr->load_texture(path_str, true);
	sprite->set_texture(texture);
	sprite->set_position(spx_to_godot_vec2(pos));
	sprite->set_rotation_degrees(degree);
	sprite->set_scale(Vector2(scale.x, scale.y));
	sprite->set_name(path_str.get_file());
	sprite->set_z_index(zindex);

	GdObj id = get_unique_id();
	sprite->set_sort_id(id);
	id_pure_sprites[id] = sprite;

	pure_sprite_root->add_child(sprite);

	return id;
}

GdObj SpxSceneMgr::create_static_sprite(GdString texture_path, GdVec2 pos, GdFloat degree, GdVec2 scale, GdInt zindex, GdVec2 pivot, GdInt collider_type, GdVec2 collider_pivot, GdArray collider_params) {
	if (pure_sprite_root == nullptr) {
		return NULL_OBJECT_ID;
	}
	auto type = (ColliderType)collider_type;
	if (type == ColliderType::NONE) {
		return create_render_sprite(texture_path, pos, degree, scale, zindex, pivot);
	}

	auto path_str = SpxStr(texture_path);
	// Create StaticBody2D
	SpxStaticSprite *static_body = memnew(SpxStaticSprite);
	static_body->set_position(spx_to_godot_vec2(pos));
	static_body->set_rotation_degrees(degree);
	static_body->set_name(path_str.get_file());

	// Load and create sprite child
	Sprite2D *sprite = memnew(Sprite2D);
	Ref<Texture2D> texture = resMgr->load_texture(path_str, true);
	sprite->set_texture(texture);
	sprite->set_z_index(zindex);
	static_body->add_child(sprite);
	sprite->set_position(spx_to_godot_vec2(pivot));

	// Create collision shape (default: rectangle matching texture size)
	CollisionShape2D *collision_shape = memnew(CollisionShape2D);

	static_body->set_collider(collision_shape);
	static_body->add_child(collision_shape);
	collision_shape->set_position(spx_to_godot_vec2(collider_pivot));
	auto data_len = collider_params == nullptr ? 0 : collider_params->size;
	switch (type) {
		case ColliderType::NONE:
			if (texture.is_valid()) {
				Ref<RectangleShape2D> rect = memnew(RectangleShape2D);
				Vector2 texture_size = texture->get_size();
				rect->set_size(texture_size);
				collision_shape->set_shape(rect);
			}
			break;
		case ColliderType::CIRCLE: {
			Ref<CircleShape2D> circle = memnew(CircleShape2D);
			if (data_len > 0) {
				auto radius = *(SpxBaseMgr::get_array<real_t>(collider_params, 0));
				circle->set_radius(radius);
			}
			collision_shape->set_shape(circle);
			break;
		}
		case ColliderType::RECT: {
			Ref<RectangleShape2D> rect = memnew(RectangleShape2D);
			if (data_len >= 2) {
				auto width = *(SpxBaseMgr::get_array<real_t>(collider_params, 0));
				auto height = *(SpxBaseMgr::get_array<real_t>(collider_params, 1));
				rect->set_size(Vector2(width, height));
			}
			collision_shape->set_shape(rect);
			break;
		}
		case ColliderType::CAPSULE: {
			Ref<CapsuleShape2D> capsule = memnew(CapsuleShape2D);
			if (data_len >= 2) {
				auto radius = *(SpxBaseMgr::get_array<real_t>(collider_params, 0));
				auto height = *(SpxBaseMgr::get_array<real_t>(collider_params, 1));
				capsule->set_radius(radius / 2);
				capsule->set_height(height);
			}
			collision_shape->set_shape(capsule);
			break;
		}
		case ColliderType::POLYGON: {
			Ref<ConvexPolygonShape2D> polygon = memnew(ConvexPolygonShape2D);
			Vector<Vector2> points = {};
			auto len = data_len;
			for (int i = 0; i + 1 < len; i += 2) {
				auto x = *(SpxBaseMgr::get_array<real_t>(collider_params, i));
				auto y = *(SpxBaseMgr::get_array<real_t>(collider_params, i + 1));
				points.append(spx_to_godot_vec2(Vector2(x, y)));
			}
			polygon->set_points(points);
			collision_shape->set_shape(polygon);
			break;
		}
		default:
			print_error("Invalid collider type: " + itos((int)type));
			break;
	}

	static_body->set_scale(Vector2(scale.x, scale.y));
	// Assign unique ID and register
	GdObj id = get_unique_id();
	static_body->set_sort_id(id);
	id_pure_sprites[id] = static_body;

	// Add to scene tree
	pure_sprite_root->add_child(static_body);

	return id;
}

void SpxSceneMgr::destroy_all_sprites() {
	spriteMgr->destroy_all_sprites();
}

void SpxSceneMgr::change_scene_to_file(GdString path) {
	get_tree()->change_scene_to_file(SpxStr(path));
}

GdInt SpxSceneMgr::reload_current_scene() {
	return get_tree()->reload_current_scene();
}

void SpxSceneMgr::unload_current_scene() {
	get_tree()->unload_current_scene();
}
