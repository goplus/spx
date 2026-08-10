/**************************************************************************/
/*  spx_physics_mgr.cpp                                                   */
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

#include "spx_physics_mgr.h"

#include "core/templates/hash_set.h"
#include "core/variant/typed_array.h"
#include "scene/2d/camera_2d.h"
#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/main/window.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"
#include "scene/resources/world_2d.h"
#include "servers/physics_server_2d.h"
#include "servers/physics_server_3d.h"

#include "gdextension_spx_ext.h"
#include "spx_camera_mgr.h"
#include "spx_coordinate.h"
#include "spx_engine.h"
#include "spx_sprite.h"
#include "spx_sprite_mgr.h"

GdArray SpxRaycastInfo::ToArray() {
	GdArray result_array = SpxBaseMgr::create_array(GD_ARRAY_TYPE_INT64, 6);
	SpxBaseMgr::set_array(result_array, 0, (GdInt)collide);
	SpxBaseMgr::set_array(result_array, 1, (GdInt)sprite_gid);
	SpxBaseMgr::set_array(result_array, 2, spx_float_to_int(position.x));
	SpxBaseMgr::set_array(result_array, 3, spx_float_to_int(position.y));
	SpxBaseMgr::set_array(result_array, 4, spx_float_to_int(normal.x));
	SpxBaseMgr::set_array(result_array, 5, spx_float_to_int(normal.y));
	return result_array;
}

void SpxPhysicsDefine::set_global_gravity(GdFloat gravity) {
	SpxPhysicsDefine::global_gravity = gravity;
}

GdFloat SpxPhysicsDefine::get_global_gravity() {
	return SpxPhysicsDefine::global_gravity;
}

void SpxPhysicsDefine::set_global_friction(GdFloat friction) {
	SpxPhysicsDefine::global_friction = friction;
}

GdFloat SpxPhysicsDefine::get_global_friction() {
	return SpxPhysicsDefine::global_friction;
}

void SpxPhysicsDefine::set_global_air_drag(GdFloat air_drag) {
	SpxPhysicsDefine::global_air_drag = air_drag;
}

GdFloat SpxPhysicsDefine::get_global_air_drag() {
	return SpxPhysicsDefine::global_air_drag;
}

void SpxPhysicsMgr::on_awake() {
	SpxBaseMgr::on_awake();
	is_collision_by_pixel = true;
}

void SpxPhysicsMgr::on_reset(int reset_code) {
}

SpxRaycastInfo SpxPhysicsMgr::_raycast(GdVec2 from, GdVec2 to, GdArray ignore_sprites, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies) {
	SpxRaycastInfo info;
	info.collide = false;
	info.position = GdVec2{ 0, 0 };
	info.normal = GdVec2{ 0, 0 };
	info.sprite_gid = 0;

	GdVec2 current_from = spx_to_godot_vec2(from);
	GdVec2 target_to = spx_to_godot_vec2(to);

	HashSet<RID> ignore_set;
	if (ignore_sprites && ignore_sprites->size > 0) {
		GdObj *sprite_data = (SpxBaseMgr::get_array<GdObj>(ignore_sprites, 0));
		for (int i = 0; i < ignore_sprites->size; i++) {
			auto obj = sprite_data[i];
			auto sprite = spriteMgr->get_sprite(obj);
			if (sprite != nullptr) {
				ignore_set.insert(sprite->get_rid());
				auto trigger = sprite->get_area2d();
				if (trigger != nullptr) {
					ignore_set.insert(trigger->get_rid());
				}
			}
		}
	}
	PhysicsDirectSpaceState2D::RayResult result;
	PhysicsDirectSpaceState2D::RayParameters params;
	params.from = current_from;
	params.to = target_to;
	params.collision_mask = (uint32_t)collision_mask;
	params.collide_with_areas = collide_with_areas;
	params.collide_with_bodies = collide_with_bodies;
	params.exclude = ignore_set;

	auto node = (Node2D *)get_root();
	PhysicsDirectSpaceState2D *space_state = node->get_world_2d()->get_direct_space_state();
	if (!space_state) {
		return info;
	}

	bool hit = space_state->intersect_ray(params, result);
	if (!hit) {
		return info;
	}
	SpxSprite *collider = dynamic_cast<SpxSprite *>(result.collider);
	GdObj current_gid = collider ? collider->get_gid() : 0;
	info.collide = true;
	info.position = godot_to_spx_vec2(result.position);
	info.normal = godot_to_spx_vec2(result.normal);
	info.sprite_gid = current_gid;
	return info;
}

GdArray SpxPhysicsMgr::raycast_with_details(GdVec2 from, GdVec2 to, GdArray ignore_sprites, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies) {
	SpxRaycastInfo info = _raycast(from, to, ignore_sprites, collision_mask, collide_with_areas, collide_with_bodies);
	return info.ToArray();
}

GdObj SpxPhysicsMgr::raycast(GdVec2 from, GdVec2 to, GdInt collision_mask) {
	auto node = (Node2D *)get_root();
	PhysicsDirectSpaceState2D *space_state = node->get_world_2d()->get_direct_space_state();

	PhysicsDirectSpaceState2D::RayResult result;
	PhysicsDirectSpaceState2D::RayParameters params;
	from = spx_to_godot_vec2(from);
	to = spx_to_godot_vec2(to);
	params.from = from;
	params.to = to;
	params.collision_mask = (uint32_t)collision_mask;
	bool hit = space_state->intersect_ray(params, result);
	if (hit) {
		SpxSprite *collider = dynamic_cast<SpxSprite *>(result.collider);
		if (collider != nullptr) {
			return collider->get_gid();
		}
	}
	return 0;
}

GdBool SpxPhysicsMgr::check_collision(GdVec2 from, GdVec2 to, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies) {
	auto node = (Node2D *)get_root();
	PhysicsDirectSpaceState2D *space_state = node->get_world_2d()->get_direct_space_state();
	PhysicsDirectSpaceState2D::RayResult result;
	PhysicsDirectSpaceState2D::RayParameters params;

	from = spx_to_godot_vec2(from);
	to = spx_to_godot_vec2(to);
	params.from = from;
	params.to = to;
	params.collision_mask = (uint32_t)collision_mask;
	params.collide_with_areas = collide_with_areas;
	params.collide_with_bodies = collide_with_bodies;
	bool hit = space_state->intersect_ray(params, result);
	return hit;
}

// Internal helper function for boundary checking
GdInt SpxPhysicsMgr::_check_touched_boundaries(GdObj obj, GdBool use_stage_limits) {
	auto sprite = spriteMgr->get_sprite(obj);
	if (sprite == nullptr) {
		print_error("try to get property of a null sprite gid=" + itos(obj));
		return false;
	}

	CollisionShape2D *collision_shape = sprite->get_trigger();
	if (!collision_shape) {
		return false;
	}
	Ref<Shape2D> sprite_shape = collision_shape->get_shape();
	if (sprite_shape.is_null()) {
		return false;
	}
	Transform2D shape_transform = collision_shape->get_global_transform();

	// Get boundary rect from camera manager
	Rect2 boundary_rect = use_stage_limits ? cameraMgr->get_stage_limits_rect() : cameraMgr->get_global_camera_rect();

	real_t bound_left = boundary_rect.position.x;
	real_t bound_top = boundary_rect.position.y;
	real_t bound_right = boundary_rect.position.x + boundary_rect.size.x;
	real_t bound_bottom = boundary_rect.position.y + boundary_rect.size.y;
	real_t width = boundary_rect.size.x;
	real_t height = boundary_rect.size.y;
	Vector2 center_pos = boundary_rect.get_center();

	Ref<RectangleShape2D> vertical_edge_shape;
	vertical_edge_shape.instantiate();
	// Use full boundary size * 2 to handle rotation and scaling
	vertical_edge_shape->set_size(Vector2(2, height * 2));

	Ref<RectangleShape2D> horizontal_edge_shape;
	horizontal_edge_shape.instantiate();
	horizontal_edge_shape->set_size(Vector2(width * 2, 2));

	Transform2D left_edge_transform(0, Vector2(bound_left, center_pos.y));
	Transform2D right_edge_transform(0, Vector2(bound_right, center_pos.y));
	Transform2D top_edge_transform(0, Vector2(center_pos.x, bound_top));
	Transform2D bottom_edge_transform(0, Vector2(center_pos.x, bound_bottom));

	bool is_colliding_left = sprite_shape->collide(shape_transform, vertical_edge_shape, left_edge_transform);
	bool is_colliding_right = sprite_shape->collide(shape_transform, vertical_edge_shape, right_edge_transform);
	bool is_colliding_top = sprite_shape->collide(shape_transform, horizontal_edge_shape, top_edge_transform);
	bool is_colliding_bottom = sprite_shape->collide(shape_transform, horizontal_edge_shape, bottom_edge_transform);

	GdInt result = 0;
	result += is_colliding_top ? BOUND_TOP : 0;
	result += is_colliding_right ? BOUND_RIGHT : 0;
	result += is_colliding_bottom ? BOUND_BOTTOM : 0;
	result += is_colliding_left ? BOUND_LEFT : 0;
	return result;
}

GdBool SpxPhysicsMgr::_check_touched_boundary(GdObj obj, GdInt board_type, GdBool use_stage_limits) {
	auto result = _check_touched_boundaries(obj, use_stage_limits);
	return (result & board_type) != 0;
}

GdInt SpxPhysicsMgr::check_touched_camera_boundaries(GdObj obj) {
	return _check_touched_boundaries(obj, false);
}

GdBool SpxPhysicsMgr::check_touched_camera_boundary(GdObj obj, GdInt board_type) {
	return _check_touched_boundary(obj, board_type, false);
}

GdInt SpxPhysicsMgr::_check_nearest_touched_boundary(GdObj obj, GdBool use_stage_limits) {
	auto sprite = spriteMgr->get_sprite(obj);
	if (sprite == nullptr) {
		print_error("try to get property of a null sprite gid=" + itos(obj));
		return 0;
	}

	// Get sprite's bounding box
	CollisionShape2D *collision_shape = sprite->get_trigger();
	if (!collision_shape) {
		return 0;
	}

	Ref<Shape2D> sprite_shape = collision_shape->get_shape();
	if (sprite_shape.is_null()) {
		return 0;
	}

	Transform2D shape_transform = collision_shape->get_global_transform();
	Rect2 world_rect = shape_transform.xform(sprite_shape->get_rect());
	real_t left = world_rect.position.x;
	real_t right = world_rect.position.x + world_rect.size.x;
	real_t top = world_rect.position.y;
	real_t bottom = world_rect.position.y + world_rect.size.y;

	// Get boundary rect from camera manager
	Rect2 boundary_rect = use_stage_limits ? cameraMgr->get_stage_limits_rect() : cameraMgr->get_global_camera_rect();
	real_t bound_left = boundary_rect.position.x;
	real_t bound_top = boundary_rect.position.y;
	real_t bound_right = boundary_rect.position.x + boundary_rect.size.x;
	real_t bound_bottom = boundary_rect.position.y + boundary_rect.size.y;

	// Calculate distances to edges (positive when far away, clamped to 0 when beyond)
	real_t dist_left = MAX(0.0, left - bound_left);
	real_t dist_top = MAX(0.0, top - bound_top);
	real_t dist_right = MAX(0.0, bound_right - right);
	real_t dist_bottom = MAX(0.0, bound_bottom - bottom);

	// Find nearest edge
	real_t min_dist = INFINITY;
	GdInt nearest_edge = 0;

	if (dist_left < min_dist) {
		min_dist = dist_left;
		nearest_edge = BOUND_LEFT;
	}

	if (dist_top < min_dist) {
		min_dist = dist_top;
		nearest_edge = BOUND_TOP;
	}

	if (dist_right < min_dist) {
		min_dist = dist_right;
		nearest_edge = BOUND_RIGHT;
	}

	if (dist_bottom < min_dist) {
		min_dist = dist_bottom;
		nearest_edge = BOUND_BOTTOM;
	}

	if (min_dist > 0) {
		return 0;
	}

	return nearest_edge;
}

GdInt SpxPhysicsMgr::check_nearest_touched_camera_boundary(GdObj obj) {
	return _check_nearest_touched_boundary(obj, false);
}

GdInt SpxPhysicsMgr::check_touched_stage_boundaries(GdObj obj) {
	return _check_touched_boundaries(obj, true);
}

GdBool SpxPhysicsMgr::check_touched_stage_boundary(GdObj obj, GdInt board_type) {
	return _check_touched_boundary(obj, board_type, true);
}

GdInt SpxPhysicsMgr::check_nearest_touched_stage_boundary(GdObj obj) {
	return _check_nearest_touched_boundary(obj, true);
}

//
void SpxPhysicsMgr::set_collision_system_type(GdBool is_collision_by_alpha) {
	this->is_collision_by_pixel = is_collision_by_alpha;
}

void SpxPhysicsMgr::set_global_gravity(GdFloat gravity) {
	SpxPhysicsDefine::set_global_gravity(gravity);
}

GdFloat SpxPhysicsMgr::get_global_gravity() {
	return SpxPhysicsDefine::get_global_gravity();
}

void SpxPhysicsMgr::set_global_friction(GdFloat friction) {
	SpxPhysicsDefine::set_global_friction(friction);
}

GdFloat SpxPhysicsMgr::get_global_friction() {
	return SpxPhysicsDefine::get_global_friction();
}

void SpxPhysicsMgr::set_global_air_drag(GdFloat air_drag) {
	SpxPhysicsDefine::set_global_air_drag(air_drag);
}

GdFloat SpxPhysicsMgr::get_global_air_drag() {
	return SpxPhysicsDefine::get_global_air_drag();
}

GdArray SpxPhysicsMgr::_check_collision(RID shape, GdVec2 pos, GdInt collision_mask) {
	auto node = (Node2D *)get_root();
	PhysicsDirectSpaceState2D *space_state = node->get_world_2d()->get_direct_space_state();
	if (!space_state) {
		return create_array(GD_ARRAY_TYPE_GDOBJ, 0);
	}

	Transform2D query_transform(0, spx_to_godot_vec2(pos));

	PhysicsDirectSpaceState2D::ShapeParameters params;
	params.shape_rid = shape;
	params.transform = query_transform;
	params.collision_mask = (uint32_t)collision_mask;
	params.collide_with_areas = true;
	params.collide_with_bodies = true;
	params.motion = Vector2(0, 0);
	params.margin = 0.0;

	PhysicsDirectSpaceState2D::ShapeResult results[32];
	int result_count = space_state->intersect_shape(params, results, 32);

	Array resultIds;
	for (int i = 0; i < result_count; i++) {
		Object *collider_obj = results[i].collider;
		if (collider_obj) {
			SpxSprite *sprite = dynamic_cast<SpxSprite *>(collider_obj);
			if (sprite) {
				resultIds.push_back(sprite->get_gid());
			}
		}
	}
	int valid_count = resultIds.size();
	GdArray result_array = create_array(GD_ARRAY_TYPE_GDOBJ, valid_count);
	for (int i = 0; i < valid_count; i++) {
		auto val = (GdObj)resultIds.get(i);
		set_array(result_array, i, val);
	}
	return result_array;
}
GdArray SpxPhysicsMgr::check_collision_rect(GdVec2 pos, GdVec2 size, GdInt collision_mask) {
	Ref<RectangleShape2D> rect_shape;
	rect_shape.instantiate();
	rect_shape->set_size(size);
	return _check_collision(rect_shape->get_rid(), pos, collision_mask);
}

GdArray SpxPhysicsMgr::check_collision_circle(GdVec2 pos, GdFloat radius, GdInt collision_mask) {
	Ref<CircleShape2D> circle_shape;
	circle_shape.instantiate();
	circle_shape->set_radius(radius);
	return _check_collision(circle_shape->get_rid(), pos, collision_mask);
}
