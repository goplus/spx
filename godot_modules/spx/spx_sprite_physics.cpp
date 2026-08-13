/**************************************************************************/
/*  spx_sprite_physics.cpp                                                */
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
#include "scene/2d/physics/collision_shape_2d.h"

#include "spx_physics_mgr.h"

void SpxSprite::set_physics_mode(GdInt p_mode) {
	ERR_FAIL_COND_MSG(p_mode < NO_PHYSICS || p_mode > STATIC, "SpxSprite: invalid physics mode.");
	physics_mode = static_cast<PhysicsMode>(p_mode);
	_update_physics_mode();
}

GdInt SpxSprite::get_physics_mode() const {
	return static_cast<GdInt>(physics_mode);
}

void SpxSprite::set_use_gravity(GdBool p_enabled) {
	use_gravity = p_enabled;
}

GdBool SpxSprite::is_use_gravity() const {
	return use_gravity;
}

void SpxSprite::set_gravity_scale(GdFloat p_scale) {
	gravity_scale = p_scale;
}

GdFloat SpxSprite::get_gravity_scale() const {
	return gravity_scale;
}

void SpxSprite::set_drag(GdFloat p_drag) {
	drag_value = p_drag;
}

GdFloat SpxSprite::get_drag() const {
	return drag_value;
}

void SpxSprite::set_friction(GdFloat p_friction) {
	friction_value = p_friction;
}

GdFloat SpxSprite::get_friction() const {
	return friction_value;
}

void SpxSprite::set_gravity(GdFloat p_gravity) {
	_gravity = p_gravity;
}

GdFloat SpxSprite::get_gravity() {
	return _gravity;
}

void SpxSprite::set_mass(GdFloat p_mass) {
	mass_value = p_mass;
}

GdFloat SpxSprite::get_mass() {
	return mass_value;
}

void SpxSprite::add_force(GdVec2 p_force) {
	if (physics_mode != DYNAMIC) {
		return;
	}

	external_forces += Vector2(p_force.x, p_force.y);
}

void SpxSprite::add_impulse(GdVec2 p_impulse) {
	if (physics_mode != DYNAMIC) {
		return;
	}

	applied_forces += Vector2(p_impulse.x, p_impulse.y);
}

void SpxSprite::_physics_process(double p_delta) {
	switch (physics_mode) {
		case DYNAMIC:
			_handle_dynamic_physics(p_delta);
			move_and_slide();
			break;
		case KINEMATIC:
			_handle_kinematic_physics(p_delta);
			move_and_slide();
			break;
		case STATIC:
			_handle_static_physics(p_delta);
			break;
		case NO_PHYSICS:
			_handle_no_physics(p_delta);
			break;
		default:
			ERR_PRINT("SpxSprite: unknown physics mode.");
			break;
	}
}

void SpxSprite::_handle_dynamic_physics(double p_delta) {
	Vector2 current_velocity = get_velocity();

	if (use_gravity && !is_on_floor()) {
		current_velocity.y += _gravity * p_delta * gravity_scale * SpxPhysicsDefine::get_global_gravity();
	}

	float safe_mass = Math::is_zero_approx(mass_value) ? 1.0f : mass_value;
	current_velocity += applied_forces * p_delta / safe_mass;
	current_velocity += external_forces * p_delta;

	if (drag_value > 0.0f) {
		current_velocity = current_velocity.move_toward(Vector2(), drag_value * current_velocity.length() * p_delta * SpxPhysicsDefine::get_global_air_drag());
	}

	if (is_on_floor() && Math::abs(current_velocity.x) > 0.0f) {
		current_velocity.x = Math::move_toward(double(current_velocity.x), 0.0, p_delta * friction_value * SpxPhysicsDefine::get_global_friction());
	}

	set_velocity(current_velocity);
	applied_forces = Vector2();
}

void SpxSprite::_handle_kinematic_physics(double /* p_delta */) {
}

void SpxSprite::_handle_static_physics(double /* p_delta */) {
	set_velocity(Vector2());
	external_forces = Vector2();
	applied_forces = Vector2();
}

void SpxSprite::_handle_no_physics(double p_delta) {
	Vector2 current_velocity = get_velocity();
	if (current_velocity != Vector2()) {
		set_global_position(get_global_position() + current_velocity * p_delta);
	}

	external_forces = Vector2();
	applied_forces = Vector2();
}

void SpxSprite::_update_physics_mode() {
	switch (physics_mode) {
		case DYNAMIC:
		case KINEMATIC:
			_enable_collision();
			set_physics_process(true);
			break;
		case STATIC:
			_enable_collision();
			set_physics_process(true);
			break;
		case NO_PHYSICS:
			_disable_collision();
			set_physics_process(true);
			break;
		default:
			ERR_PRINT("SpxSprite: unknown physics mode.");
			_disable_collision();
			set_physics_process(false);
			break;
	}
}

void SpxSprite::_enable_collision() {
	_update_collider_disabled_state();
}

void SpxSprite::_disable_collision() {
	set_velocity(Vector2());
	_update_collider_disabled_state();
}
