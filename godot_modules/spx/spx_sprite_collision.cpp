/**************************************************************************/
/*  spx_sprite_collision.cpp                                              */
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

#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/resources/2d/capsule_shape_2d.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/convex_polygon_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"

#include <cstdint>

#include "spx_base_mgr.h"
#include "spx_coordinate.h"

namespace {

template <typename TShape>
void apply_shape(CollisionShape2D *p_collision_shape, const Ref<TShape> &p_shape, GdVec2 p_center) {
	if (p_collision_shape == nullptr) {
		return;
	}

	p_collision_shape->set_shape(p_shape);
	p_collision_shape->set_position(spx_to_godot_vec2(p_center));
}

bool build_polygon_points(GdArray p_points, Vector<Vector2> &r_points) {
	if (!p_points || p_points->size < 6) {
		return false;
	}

	const float *data = SpxBaseMgr::get_array<float>(p_points, 0);
	int point_count = p_points->size / 2;

	for (int i = 0; i < point_count; ++i) {
		r_points.push_back(spx_to_godot_vec2(Vector2(data[i * 2], data[i * 2 + 1])));
	}

	return true;
}

bool is_valid_collision_bits(GdInt p_value) {
	return p_value >= 0 && static_cast<uint64_t>(p_value) <= UINT32_MAX;
}

} // namespace

void SpxSprite::set_trigger_layer(GdInt p_layer) {
	ERR_FAIL_NULL_MSG(area2d, "SpxSprite: Area2D component is missing.");
	ERR_FAIL_COND_MSG(!is_valid_collision_bits(p_layer), "SpxSprite: trigger layer must fit in 32 bits.");
	area2d->set_collision_layer(static_cast<uint32_t>(p_layer));
}

GdInt SpxSprite::get_trigger_layer() {
	ERR_FAIL_NULL_V_MSG(area2d, 0, "SpxSprite: Area2D component is missing.");
	return static_cast<GdInt>(area2d->get_collision_layer());
}

void SpxSprite::set_trigger_mask(GdInt p_mask) {
	ERR_FAIL_NULL_MSG(area2d, "SpxSprite: Area2D component is missing.");
	ERR_FAIL_COND_MSG(!is_valid_collision_bits(p_mask), "SpxSprite: trigger mask must fit in 32 bits.");
	area2d->set_collision_mask(static_cast<uint32_t>(p_mask));
}

GdInt SpxSprite::get_trigger_mask() {
	ERR_FAIL_NULL_V_MSG(area2d, 0, "SpxSprite: Area2D component is missing.");
	return static_cast<GdInt>(area2d->get_collision_mask());
}

void SpxSprite::set_collider_rect(GdVec2 p_center, GdVec2 p_size) {
	Ref<RectangleShape2D> rect = memnew(RectangleShape2D);
	rect->set_size(p_size);
	apply_shape(collider2d, rect, p_center);
}

void SpxSprite::on_set_visible(GdBool p_visible) {
	(void)p_visible;
	_update_collider_disabled_state();
	_update_trigger_disabled_state();
}

void SpxSprite::set_collider_circle(GdVec2 p_center, GdFloat p_radius) {
	Ref<CircleShape2D> circle = memnew(CircleShape2D);
	circle->set_radius(p_radius);
	apply_shape(collider2d, circle, p_center);
}

void SpxSprite::set_collider_capsule(GdVec2 p_center, GdVec2 p_size) {
	Ref<CapsuleShape2D> capsule = memnew(CapsuleShape2D);
	capsule->set_radius(p_size.x / 2);
	capsule->set_height(p_size.y);
	apply_shape(collider2d, capsule, p_center);
}

void SpxSprite::set_collider_polygon(GdVec2 p_center, GdArray p_points) {
	Vector<Vector2> polygon_points;
	if (!build_polygon_points(p_points, polygon_points)) {
		print_error("set_collider_polygon: need at least 3 points");
		return;
	}

	Ref<ConvexPolygonShape2D> polygon = memnew(ConvexPolygonShape2D);
	polygon->set_points(polygon_points);
	apply_shape(collider2d, polygon, p_center);
}

void SpxSprite::set_collision_enabled(GdBool p_enabled) {
	_is_collision_enabled = p_enabled;
	_update_collider_disabled_state();
}

GdBool SpxSprite::is_collision_enabled() {
	return _is_collision_enabled;
}

void SpxSprite::set_trigger_capsule(GdVec2 p_center, GdVec2 p_size) {
	Ref<CapsuleShape2D> capsule = memnew(CapsuleShape2D);
	capsule->set_radius(p_size.x / 2);
	capsule->set_height(p_size.y);
	apply_shape(trigger2d, capsule, p_center);
}

void SpxSprite::set_trigger_rect(GdVec2 p_center, GdVec2 p_size) {
	Ref<RectangleShape2D> rect = memnew(RectangleShape2D);
	rect->set_size(p_size);
	apply_shape(trigger2d, rect, p_center);
}

void SpxSprite::set_trigger_circle(GdVec2 p_center, GdFloat p_radius) {
	Ref<CircleShape2D> circle = memnew(CircleShape2D);
	circle->set_radius(p_radius);
	apply_shape(trigger2d, circle, p_center);
}

void SpxSprite::set_trigger_polygon(GdVec2 p_center, GdArray p_points) {
	Vector<Vector2> polygon_points;
	if (!build_polygon_points(p_points, polygon_points)) {
		print_error("set_trigger_polygon: need at least 3 points");
		return;
	}

	Ref<ConvexPolygonShape2D> polygon = memnew(ConvexPolygonShape2D);
	polygon->set_points(polygon_points);
	apply_shape(trigger2d, polygon, p_center);
}

void SpxSprite::set_trigger_enabled(GdBool p_enabled) {
	_is_trigger_enabled = p_enabled;
	_update_trigger_disabled_state();
}

GdBool SpxSprite::is_trigger_enabled() {
	return _is_trigger_enabled;
}

CollisionShape2D *SpxSprite::get_collider(bool p_is_trigger) {
	return p_is_trigger ? trigger2d : collider2d;
}

GdBool SpxSprite::check_collision(SpxSprite *p_other, GdBool p_is_src_trigger, GdBool p_is_dst_trigger) {
	if (p_other == nullptr) {
		return false;
	}

	CollisionShape2D *this_shape = p_is_src_trigger ? trigger2d : collider2d;
	CollisionShape2D *other_shape = p_other->get_collider(p_is_dst_trigger);
	if (this_shape == nullptr || other_shape == nullptr) {
		return false;
	}

	if (!this_shape->get_shape().is_valid() || !other_shape->get_shape().is_valid()) {
		return false;
	}

	return this_shape->get_shape()->collide(this_shape->get_global_transform(), other_shape->get_shape(), other_shape->get_global_transform());
}

GdBool SpxSprite::check_collision_with_point(GdVec2 p_point, GdBool p_is_trigger) {
	CollisionShape2D *shape = p_is_trigger ? trigger2d : collider2d;
	if (shape == nullptr || !shape->get_shape().is_valid()) {
		return false;
	}

	Ref<CircleShape2D> point_shape;
	point_shape.instantiate();
	point_shape->set_radius(3);

	Transform2D point_transform(0, p_point);
	return shape->get_shape()->collide(shape->get_global_transform(), point_shape, point_transform);
}

void SpxSprite::set_debug_collision_visible(GdBool p_enabled) {
	debug_collision_visible = p_enabled;
	_update_collision_debug_overlays();
}

GdBool SpxSprite::is_debug_collision_visible() const {
	return debug_collision_visible;
}

bool SpxSprite::_can_enable_collider() const {
	return physics_mode != NO_PHYSICS && _is_collision_enabled && is_visible();
}

void SpxSprite::_update_collider_disabled_state() {
	if (collider2d != nullptr) {
		collider2d->set_disabled(!_can_enable_collider());
	}
}

void SpxSprite::_update_trigger_disabled_state() {
	if (trigger2d != nullptr) {
		trigger2d->set_disabled(!(_is_trigger_enabled && is_visible()));
	}
}
