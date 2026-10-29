/**************************************************************************/
/*  test_spx_sprite_mgr.h                                               */
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

#ifndef TEST_SPX_SPRITE_MGR_H
#define TEST_SPX_SPRITE_MGR_H

#include "../spx_layer_sorter.h"
#include "../spx_image_texture.h"
#include "../spx_physics_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "scene/2d/visible_on_screen_notifier_2d.h"
#include "scene/resources/2d/capsule_shape_2d.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/convex_polygon_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"
#include "scene/resources/material.h"
#include "spx_test_fixtures.h"

namespace TestSpxSpriteMgr {

TEST_CASE("[SceneTree][SPX] Sprite manager converts coordinates and owns sprite lifetime") {
	TestSpx::EngineScene scene;
	SpxSpriteMgr *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2(10, 20));
	SpxSprite *sprite = manager->get_sprite(id);
	REQUIRE(sprite != nullptr);
	CHECK(manager->is_sprite_alive(id));
	CHECK(sprite->get_position() == Vector2(10, -20));
	manager->set_transform(id, Vector2(-12, 30), 0.5, Vector2(2, 3), false, Vector2(4, 5));
	CHECK(Vector2(manager->get_position(id)) == Vector2(-12, 30));
	CHECK(sprite->get_position() == Vector2(-12, -30));
	CHECK(manager->get_rotation(id) == doctest::Approx(0.5));
	CHECK(Vector2(manager->get_scale(id)) == Vector2(2, 3));
	CHECK_FALSE(manager->get_visible(id));
	manager->set_visible(id, true);
	CHECK(sprite->is_visible());
	manager->set_position(id, Vector2(7, 8));
	CHECK(sprite->get_position() == Vector2(7, -8));
	manager->set_rotation(id, 0.25);
	CHECK(sprite->get_rotation() == doctest::Approx(0.25));
	manager->set_scale(id, Vector2(1.5, 2));
	CHECK(sprite->get_scale() == Vector2(1.5, 2));
	manager->set_render_scale(id, Vector2(0.5, 0.75));
	CHECK(Vector2(manager->get_render_scale(id)) == Vector2(0.5, 0.75));
	manager->set_pivot(id, Vector2(3, 9));
	CHECK(Vector2(manager->get_pivot(id)) == Vector2(3, 9));
	manager->set_z_index(id, 8);
	CHECK(manager->get_z_index(id) == 8);
	CHECK(sprite->get_z_index() == 8);
	manager->set_type_name(id, "Dragon");
	CHECK(sprite->get_spx_type_name() == "Dragon");
	manager->set_process(id, false);
	manager->set_physic_process(id, false);
	CHECK_FALSE(sprite->is_processing());
	CHECK_FALSE(sprite->is_physics_processing());
	Node2D *child = memnew(Node2D);
	child->set_name("attachment");
	sprite->add_child(child);
	manager->set_child_position(id, "attachment", Vector2(5, 6));
	CHECK(child->get_position() == Vector2(5, -6));
	CHECK(Vector2(manager->get_child_position(id, "attachment")) == Vector2(5, 6));
	manager->set_child_rotation(id, "attachment", 0.75);
	CHECK(manager->get_child_rotation(id, "attachment") == doctest::Approx(0.75));
	manager->set_child_scale(id, "attachment", Vector2(2, 4));
	CHECK(Vector2(manager->get_child_scale(id, "attachment")) == Vector2(2, 4));
	CHECK(child->get_scale() == Vector2(2, 4));
	Node *logic = memnew(Node);
	logic->set_name("logic");
	sprite->add_child(logic);
	for (const char *path : { "missing", "logic" }) {
		manager->set_child_position(id, path, Vector2(9, 8));
		manager->set_child_rotation(id, path, 1);
		manager->set_child_scale(id, path, Vector2(3, 4));
		CHECK(Vector2(manager->get_child_position(id, path)) == Vector2());
		CHECK(manager->get_child_rotation(id, path) == 0);
		CHECK(Vector2(manager->get_child_scale(id, path)) == Vector2());
	}
	CHECK(logic->get_parent() == sprite);
	CHECK(child->get_scale() == Vector2(2, 4));
	const GdObj clone = manager->clone_sprite(id);
	REQUIRE(clone != NULL_OBJECT_ID);
	CHECK(clone != id);
	CHECK(Vector2(manager->get_position(clone)) == Vector2(7, 8));
	manager->set_position(clone, Vector2(1, 2));
	CHECK(Vector2(manager->get_position(id)) == Vector2(7, 8));
	Vector<ISortableSprite *> sprites;
	manager->collect_sortable_sprites(sprites);
	CHECK(sprites.size() == 2);
	CHECK(manager->destroy_sprite(id));
	CHECK(sprite->is_queued_for_deletion());
	CHECK_FALSE(manager->is_sprite_alive(id));
	CHECK(manager->get_sprite(id) == nullptr);
	sprites.clear();
	manager->collect_sortable_sprites(sprites);
	REQUIRE(sprites.size() == 1);
	CHECK(sprites[0] == manager->get_sprite(clone));
	ERR_PRINT_OFF;
	CHECK_FALSE(manager->destroy_sprite(id));
	CHECK(manager->clone_sprite(id) == NULL_OBJECT_ID);
	ERR_PRINT_ON;
}

TEST_CASE("[SceneTree][SPX] Destroyed sprite handles return defaults and cannot mutate queued nodes") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2(3, 4));
	auto *sprite = manager->get_sprite(id);
	manager->set_velocity(id, Vector2(5, 6));
	const Transform2D transform = sprite->get_transform();
	const auto frames = sprite->get_anim2d()->get_sprite_frames();
	const auto shape = sprite->get_collider(false)->get_shape();
	Node *parent = sprite->get_parent();
	REQUIRE(manager->destroy_sprite(id));
	const GdObj survivor = manager->create_bare_sprite(Vector2(7, 8));
	REQUIRE(survivor != id);

	// A queued node still exists until the frame ends. Every public operation
	// must respect pending deletion, including setters that load assets.
	ERR_PRINT_OFF;
	for (auto setter : { &SpxSpriteMgr::set_dont_destroy_on_load, &SpxSpriteMgr::pause_anim,
			 &SpxSpriteMgr::stop_anim, &SpxSpriteMgr::move_and_slide }) {
		(manager->*setter)(id);
	}
	for (auto setter : { &SpxSpriteMgr::set_process, &SpxSpriteMgr::set_physic_process,
			 &SpxSpriteMgr::set_debug_collision_visible, &SpxSpriteMgr::set_visible,
			 &SpxSpriteMgr::set_anim_centered, &SpxSpriteMgr::set_anim_flip_h,
			 &SpxSpriteMgr::set_anim_flip_v, &SpxSpriteMgr::set_use_gravity,
			 &SpxSpriteMgr::set_collision_enabled, &SpxSpriteMgr::set_trigger_enabled }) {
		(manager->*setter)(id, false);
	}
	for (auto setter : { &SpxSpriteMgr::set_position, &SpxSpriteMgr::set_scale,
			 &SpxSpriteMgr::set_render_scale, &SpxSpriteMgr::set_pivot,
			 &SpxSpriteMgr::set_anim_offset, &SpxSpriteMgr::set_velocity,
			 &SpxSpriteMgr::add_force, &SpxSpriteMgr::add_impulse }) {
		(manager->*setter)(id, Vector2(90, 80));
	}
	for (auto setter : { &SpxSpriteMgr::set_rotation, &SpxSpriteMgr::set_anim_speed_scale,
			 &SpxSpriteMgr::set_gravity, &SpxSpriteMgr::set_mass,
			 &SpxSpriteMgr::set_gravity_scale, &SpxSpriteMgr::set_drag,
			 &SpxSpriteMgr::set_friction }) {
		(manager->*setter)(id, 12);
	}
	for (auto setter : { &SpxSpriteMgr::set_z_index, &SpxSpriteMgr::set_anim_frame,
			 &SpxSpriteMgr::set_physics_mode, &SpxSpriteMgr::set_collision_layer,
			 &SpxSpriteMgr::set_collision_mask, &SpxSpriteMgr::set_trigger_layer,
			 &SpxSpriteMgr::set_trigger_mask }) {
		(manager->*setter)(id, 2);
	}
	for (auto setter : { &SpxSpriteMgr::set_type_name, &SpxSpriteMgr::set_material_shader,
			 &SpxSpriteMgr::set_texture, &SpxSpriteMgr::set_texture_direct,
			 &SpxSpriteMgr::set_anim, &SpxSpriteMgr::play_backwards_anim }) {
		(manager->*setter)(id, "res://missing_destroyed_sprite_asset");
	}
	for (auto setter : { &SpxSpriteMgr::set_collider_rect, &SpxSpriteMgr::set_collider_capsule,
			 &SpxSpriteMgr::set_trigger_rect, &SpxSpriteMgr::set_trigger_capsule }) {
		(manager->*setter)(id, Vector2(1, 2), Vector2(10, 20));
	}
	for (auto setter : { &SpxSpriteMgr::set_collider_circle, &SpxSpriteMgr::set_trigger_circle }) {
		(manager->*setter)(id, Vector2(1, 2), 10);
	}
	for (auto setter : { &SpxSpriteMgr::set_collider_polygon, &SpxSpriteMgr::set_trigger_polygon }) {
		(manager->*setter)(id, Vector2(), nullptr);
	}
	for (auto setter : { &SpxSpriteMgr::set_texture_atlas, &SpxSpriteMgr::set_texture_atlas_direct }) {
		(manager->*setter)(id, "res://missing_destroyed_sprite_asset", Rect2(0, 0, 1, 1));
	}
	manager->set_transform(id, Vector2(10, 20), 1, Vector2(2, 3), false, Vector2(4, 5));
	manager->set_color(id, Color(1, 0, 0));
	manager->set_child_position(id, "RenderRoot", Vector2(10, 20));
	manager->set_child_rotation(id, "RenderRoot", 1);
	manager->set_child_scale(id, "RenderRoot", Vector2(2, 3));
	manager->set_material_params(id, "amount", 5);
	manager->set_material_params_vec(id, "bounds", 1, 2, 3, 4);
	manager->set_material_params_vec4(id, "bounds", Vector4(1, 2, 3, 4));
	manager->set_material_params_color(id, "color", Color(1, 0, 0));
	manager->play_anim(id, "missing", 1, true, false);

	for (auto getter : { &SpxSpriteMgr::get_position, &SpxSpriteMgr::get_scale,
			 &SpxSpriteMgr::get_render_scale, &SpxSpriteMgr::get_pivot,
			 &SpxSpriteMgr::get_anim_offset, &SpxSpriteMgr::get_velocity,
			 &SpxSpriteMgr::get_last_motion, &SpxSpriteMgr::get_position_delta,
			 &SpxSpriteMgr::get_floor_normal, &SpxSpriteMgr::get_wall_normal,
			 &SpxSpriteMgr::get_real_velocity }) {
		CHECK(Vector2((manager->*getter)(id)) == Vector2());
	}
	for (auto getter : { &SpxSpriteMgr::is_sprite_alive, &SpxSpriteMgr::is_debug_collision_visible,
			 &SpxSpriteMgr::get_visible, &SpxSpriteMgr::is_playing_anim,
			 &SpxSpriteMgr::is_anim_centered, &SpxSpriteMgr::is_anim_flipped_h,
			 &SpxSpriteMgr::is_anim_flipped_v, &SpxSpriteMgr::is_on_floor,
			 &SpxSpriteMgr::is_on_floor_only, &SpxSpriteMgr::is_on_wall,
			 &SpxSpriteMgr::is_on_wall_only, &SpxSpriteMgr::is_on_ceiling,
			 &SpxSpriteMgr::is_on_ceiling_only, &SpxSpriteMgr::is_use_gravity,
			 &SpxSpriteMgr::is_collision_enabled, &SpxSpriteMgr::is_trigger_enabled }) {
		CHECK_FALSE((manager->*getter)(id));
	}
	for (auto getter : { &SpxSpriteMgr::get_rotation, &SpxSpriteMgr::get_gravity,
			 &SpxSpriteMgr::get_mass, &SpxSpriteMgr::get_drag, &SpxSpriteMgr::get_friction }) {
		CHECK((manager->*getter)(id) == 0);
	}
	for (auto getter : { &SpxSpriteMgr::get_anim_speed_scale, &SpxSpriteMgr::get_anim_playing_speed,
			 &SpxSpriteMgr::get_gravity_scale }) {
		CHECK((manager->*getter)(id) == 1);
	}
	for (auto getter : { &SpxSpriteMgr::get_z_index, &SpxSpriteMgr::get_anim_frame,
			 &SpxSpriteMgr::get_physics_mode, &SpxSpriteMgr::get_collision_layer,
			 &SpxSpriteMgr::get_collision_mask, &SpxSpriteMgr::get_trigger_layer,
			 &SpxSpriteMgr::get_trigger_mask }) {
		CHECK((manager->*getter)(id) == 0);
	}
	for (auto getter : { &SpxSpriteMgr::get_texture, &SpxSpriteMgr::get_material_shader,
			 &SpxSpriteMgr::get_anim, &SpxSpriteMgr::get_current_anim_name }) {
		CHECK((manager->*getter)(id) == nullptr);
	}
	CHECK(Color(manager->get_color(id)) == Color());
	CHECK(Vector2(manager->get_child_position(id, "RenderRoot")) == Vector2());
	CHECK(manager->get_child_rotation(id, "RenderRoot") == 0);
	CHECK(Vector2(manager->get_child_scale(id, "RenderRoot")) == Vector2());
	CHECK(manager->get_material_params(id, "amount") == 0);
	CHECK(Vector4(manager->get_material_params_vec4(id, "bounds")) == Vector4());
	CHECK(Color(manager->get_material_params_color(id, "color")) == Color());
	CHECK_FALSE(manager->check_collision(id, survivor, false, false));
	CHECK_FALSE(manager->check_collision(survivor, id, false, false));
	CHECK_FALSE(manager->check_collision_with_sprite(id, survivor, 0.5, true));
	CHECK_FALSE(manager->check_collision_with_sprite(survivor, id, 0.5, true));
	CHECK_FALSE(manager->check_collision_with_point(id, Vector2(3, 4), false));
	CHECK_FALSE(manager->check_collision_by_color(id, Color(1, 0, 0), 0.1, 0.5));
	CHECK_FALSE(manager->check_collision_by_colors(id, Color(1, 0, 0), Color(0, 1, 0), 0.1, 0.5));
	CHECK_FALSE(manager->check_collision_by_alpha(id, 0.5));
	CHECK_FALSE(manager->destroy_sprite(id));
	CHECK(manager->clone_sprite(id) == NULL_OBJECT_ID);
	ERR_PRINT_ON;

	CHECK(sprite->is_queued_for_deletion());
	CHECK(sprite->get_parent() == parent);
	CHECK(sprite->get_transform() == transform);
	CHECK(sprite->get_velocity() == Vector2(5, -6));
	CHECK(sprite->get_anim2d()->get_sprite_frames() == frames);
	CHECK(sprite->get_collider(false)->get_shape() == shape);
	CHECK(sprite->is_visible());
	CHECK(Vector2(manager->get_position(survivor)) == Vector2(7, 8));
	Vector<ISortableSprite *> sprites;
	manager->collect_sortable_sprites(sprites);
	REQUIRE(sprites.size() == 1);
	CHECK(sprites[0] == manager->get_sprite(survivor));
}

TEST_CASE("[SceneTree][SPX] Sprite collider and trigger shapes preserve SPX coordinates") {
	TestSpx::EngineScene scene;
	SpxSpriteMgr *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2());
	SpxSprite *sprite = manager->get_sprite(id);
	REQUIRE(sprite != nullptr);
	auto *body = Object::cast_to<CollisionShape2D>(sprite->get_node(NodePath("Collider2D")));
	auto *trigger = Object::cast_to<CollisionShape2D>(sprite->get_node(NodePath("Area2D/Trigger2D")));
	REQUIRE(body != nullptr);
	REQUIRE(trigger != nullptr);
	for (bool use_trigger : { false, true }) {
		CollisionShape2D *shape = use_trigger ? trigger : body;
		if (use_trigger) {
			manager->set_trigger_rect(id, Vector2(2, 3), Vector2(10, 20));
		} else {
			manager->set_collider_rect(id, Vector2(2, 3), Vector2(10, 20));
		}
		Ref<RectangleShape2D> rectangle = shape->get_shape();
		REQUIRE(rectangle.is_valid());
		CHECK(rectangle->get_size() == Vector2(10, 20));
		CHECK(shape->get_position() == Vector2(2, -3));
		if (use_trigger) {
			manager->set_trigger_circle(id, Vector2(-1, 4), 6);
		} else {
			manager->set_collider_circle(id, Vector2(-1, 4), 6);
		}
		Ref<CircleShape2D> circle = shape->get_shape();
		REQUIRE(circle.is_valid());
		CHECK(circle->get_radius() == 6);
		if (use_trigger) {
			manager->set_trigger_capsule(id, Vector2(), Vector2(8, 20));
		} else {
			manager->set_collider_capsule(id, Vector2(), Vector2(8, 20));
		}
		Ref<CapsuleShape2D> capsule = shape->get_shape();
		REQUIRE(capsule.is_valid());
		CHECK(capsule->get_radius() == 4);
		CHECK(capsule->get_height() == 20);
		float coordinates[] = { -4, -3, 4, -3, 0, 6 };
		GdArrayInfo points = {};
		points.type = GD_ARRAY_TYPE_FLOAT;
		points.size = 6;
		points.data = coordinates;
		if (use_trigger) {
			manager->set_trigger_polygon(id, Vector2(1, 2), &points);
		} else {
			manager->set_collider_polygon(id, Vector2(1, 2), &points);
		}
		Ref<ConvexPolygonShape2D> polygon = shape->get_shape();
		REQUIRE(polygon.is_valid());
		CHECK(polygon->get_points() == Vector<Vector2>({ Vector2(-4, 3), Vector2(4, 3), Vector2(0, -6) }));
		ERR_PRINT_OFF;
		if (use_trigger) {
			manager->set_trigger_polygon(id, Vector2(), nullptr);
		} else {
			manager->set_collider_polygon(id, Vector2(), nullptr);
		}
		ERR_PRINT_ON;
		CHECK(shape->get_shape() == polygon);
	}
	manager->set_collision_layer(id, 5);
	manager->set_collision_mask(id, 10);
	manager->set_trigger_layer(id, 8);
	manager->set_trigger_mask(id, 3);
	CHECK(manager->get_collision_layer(id) == 5);
	CHECK(sprite->get_collision_layer() == 5);
	CHECK(manager->get_collision_mask(id) == 10);
	CHECK(manager->get_trigger_layer(id) == 8);
	CHECK(manager->get_trigger_mask(id) == 3);
	ERR_PRINT_OFF;
	manager->set_trigger_layer(id, -1);
	manager->set_trigger_mask(id, GdInt(UINT32_MAX) + 1);
	ERR_PRINT_ON;
	CHECK(manager->get_trigger_layer(id) == 8);
	CHECK(manager->get_trigger_mask(id) == 3);
	for (bool enabled : { true, false }) {
		manager->set_collision_enabled(id, enabled);
		manager->set_trigger_enabled(id, enabled);
		CHECK(manager->is_collision_enabled(id) == enabled);
		CHECK(manager->is_trigger_enabled(id) == enabled);
	}
}

TEST_CASE("[SceneTree][SPX] Sprite material parameters affect the attached shader") {
	TestSpx::TempDirectory files("spx_material");
	files.write("sprite.gdshader", "shader_type canvas_item; uniform vec4 color : source_color; uniform float amount; uniform vec4 bounds;");
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2());
	SpxSprite *sprite = manager->get_sprite(id);
	REQUIRE(sprite != nullptr);
	auto *animated = Object::cast_to<AnimatedSprite2D>(sprite->get_node(NodePath("RenderRoot/Anim2D")));
	REQUIRE(animated != nullptr);
	CHECK(manager->get_material_shader(id) == nullptr);
	const Color tint(0.25, 0.5, 0.75, 0.5);
	manager->set_color(id, tint);
	CHECK(animated->get_self_modulate() == tint);
	CHECK(Color(manager->get_color(id)) == tint);
	const CharString path = files.file("sprite.gdshader").utf8();
	manager->set_material_shader(id, path.get_data());
	Ref<ShaderMaterial> material = animated->get_material();
	REQUIRE(material.is_valid());
	REQUIRE(material->get_shader().is_valid());
	CHECK(TestSpx::take_string(manager->get_material_shader(id)) == files.file("sprite.gdshader"));
	manager->set_material_params(id, "amount", 0.75);
	CHECK(manager->get_material_params(id, "amount") == doctest::Approx(0.75));
	CHECK(double(material->get_shader_parameter("amount")) == doctest::Approx(0.75));
	manager->set_material_params_vec4(id, "bounds", Vector4(1, 2, 3, 4));
	CHECK(Vector4(manager->get_material_params_vec4(id, "bounds")) == Vector4(1, 2, 3, 4));
	CHECK(Vector4(material->get_shader_parameter("bounds")) == Vector4(1, 2, 3, 4));
	manager->set_material_params_vec(id, "bounds", 4, 3, 2, 1);
	CHECK(Vector4(material->get_shader_parameter("bounds")) == Vector4(4, 3, 2, 1));
	manager->set_material_params_color(id, "color", tint);
	CHECK(Color(manager->get_material_params_color(id, "color")) == tint);
	manager->set_color(id, Color(1, 0, 0, 1));
	CHECK(Color(manager->get_color(id)) == Color(1, 0, 0, 1));
	CHECK(Color(material->get_shader_parameter("color")) == Color(1, 0, 0, 1));
}

TEST_CASE("[SceneTree][SPX] Sprite animation controls update playback and frame transforms") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2());
	auto *sprite = manager->get_sprite(id);
	auto *animated = sprite->get_anim2d();
	Ref<SpriteFrames> frames;
	frames.instantiate();
	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(16, 8));
	for (const char *name : { "walk", "jump" }) {
		frames->add_animation(name);
		frames->set_animation_speed(name, 12);
		frames->add_frame(name, texture);
		frames->add_frame(name, texture);
	}
	animated->set_sprite_frames(frames);
	sprite->on_start();
	manager->play_anim(id, "walk", 2, false, false);
	CHECK(manager->is_playing_anim(id));
	CHECK(TestSpx::take_string(manager->get_anim(id)) == "walk");
	CHECK(TestSpx::take_string(manager->get_current_anim_name(id)) == "walk");
	CHECK_FALSE(animated->get_sprite_frames()->get_animation_loop("walk"));
	manager->set_anim_frame(id, 1);
	CHECK(manager->get_anim_frame(id) == 1);
	CHECK(animated->get_frame() == 1);
	manager->set_anim_speed_scale(id, 0.5);
	CHECK(manager->get_anim_speed_scale(id) == 0.5);
	CHECK(manager->get_anim_playing_speed(id) == 1);
	manager->pause_anim(id);
	CHECK_FALSE(manager->is_playing_anim(id));
	CHECK(manager->get_anim_frame(id) == 1);
	manager->play_backwards_anim(id, "jump");
	CHECK(manager->is_playing_anim(id));
	CHECK(manager->get_anim_playing_speed(id) == -0.5);
	manager->set_anim(id, "walk");
	CHECK(TestSpx::take_string(manager->get_anim(id)) == "walk");
	CHECK(manager->get_anim_playing_speed(id) == -0.5);
	manager->stop_anim(id);
	CHECK_FALSE(manager->is_playing_anim(id));
	CHECK(manager->get_anim_frame(id) == 0);
	for (bool flag : { false, true }) {
		manager->set_anim_centered(id, flag);
		manager->set_anim_flip_h(id, flag);
		manager->set_anim_flip_v(id, flag);
		CHECK(manager->is_anim_centered(id) == flag);
		CHECK(manager->is_anim_flipped_h(id) == flag);
		CHECK(manager->is_anim_flipped_v(id) == flag);
		CHECK(animated->is_centered() == flag);
		CHECK(animated->is_flipped_h() == flag);
		CHECK(animated->is_flipped_v() == flag);
	}
	manager->set_anim_offset(id, Vector2(3, 5));
	CHECK(Vector2(manager->get_anim_offset(id)) == Vector2(3, 5));
	CHECK(animated->get_offset() == Vector2(3, -5));
	const auto old_frames = animated->get_sprite_frames();
	const StringName old_animation = animated->get_animation();
	ERR_PRINT_OFF;
	manager->set_anim(id, "missing");
	ERR_PRINT_ON;
	CHECK(animated->get_sprite_frames() == old_frames);
	CHECK(animated->get_animation() == old_animation);
}

TEST_CASE("[SceneTree][SPX] Incomplete sprite components keep visual and collision operations inert") {
	TestSpx::TempDirectory files("spx_incomplete_sprite");
	files.write("sprite.gdshader", "shader_type canvas_item;");
	TestSpx::EngineScene scene;
	SpxSprite *sprite = memnew(SpxSprite);
	scene.root->add_child(sprite);
	ERR_PRINT_OFF;
	sprite->on_start();
	ERR_PRINT_ON;
	CHECK(sprite->get_anim2d() == nullptr);
	CHECK(sprite->get_texture() == nullptr);
	CHECK(sprite->get_rect() == Rect2(0, 0, 1, 1));
	CHECK(sprite->get_material_shader() == nullptr);
	const auto shader = files.file("sprite.gdshader").utf8();
	sprite->set_material_shader(shader.get_data());
	sprite->set_color(Color(1, 0, 0));
	CHECK(Color(sprite->get_color()) == Color(GdColor()));
	sprite->set_material_params("amount", 1);
	sprite->set_material_params_vec4("bounds", Vector4(1, 2, 3, 4));
	sprite->set_material_params_color("color", Color(1, 0, 0));
	sprite->set_uv_remap(Vector4(1, 1, 0, 0));
	CHECK(sprite->get_material_params("amount") == 0);
	CHECK(Vector4(sprite->get_material_params_vec4("bounds")) == Vector4());
	CHECK(Color(sprite->get_material_params_color("color")) == Color(GdColor()));
	sprite->set_render_scale(Vector2(2, 3));
	sprite->set_render_offset(Vector2(4, 5));
	sprite->set_render_offset(Vector2(4, 5));
	sprite->set_dynamic_frame_offset_enabled(true);
	CHECK(sprite->is_dynamic_frame_offset_enabled());
	CHECK(Vector2(sprite->get_render_scale()) == Vector2(2, 3));
	sprite->set_collider_rect(Vector2(), Vector2(8, 8));
	sprite->set_trigger_circle(Vector2(), 4);
	CHECK_FALSE(sprite->check_collision(nullptr, false, false));
	CHECK_FALSE(sprite->check_collision(sprite, false, false));
	CHECK_FALSE(sprite->check_collision_with_point(Vector2(), true));
	CHECK(sprite->get_collider(false) == nullptr);
	CHECK(sprite->get_collider(true) == nullptr);
	ERR_PRINT_OFF;
	sprite->play_anim("missing", 1, true, false);
	sprite->play_backwards_anim("missing");
	sprite->pause_anim();
	sprite->stop_anim();
	sprite->set_anim("missing");
	sprite->set_anim_frame(2);
	sprite->set_anim_speed_scale(2);
	sprite->set_anim_centered(true);
	sprite->set_anim_flip_h(true);
	sprite->set_anim_flip_v(true);
	CHECK_FALSE(sprite->is_playing_anim());
	CHECK(sprite->get_anim() == nullptr);
	CHECK(sprite->get_anim_frame() == 0);
	CHECK(sprite->get_anim_speed_scale() == 1);
	CHECK(sprite->get_anim_playing_speed() == 0);
	CHECK_FALSE(sprite->is_anim_centered());
	CHECK_FALSE(sprite->is_anim_flipped_h());
	CHECK_FALSE(sprite->is_anim_flipped_v());
	CHECK(Vector2(sprite->get_anim_offset()) == Vector2());
	sprite->set_texture("missing.png");
	sprite->set_texture_atlas("missing.png", Rect2(0, 0, 1, 1));
	sprite->set_trigger_layer(3);
	sprite->set_trigger_mask(3);
	CHECK(sprite->get_trigger_layer() == 0);
	CHECK(sprite->get_trigger_mask() == 0);
	ERR_PRINT_ON;
	CHECK(sprite->get_anim2d() == nullptr);
	CHECK(sprite->get_texture() == nullptr);
	CHECK(sprite->get_trigger() == nullptr);

	// Invalid authored scenes can acquire their missing components later.
	// Repeating startup must neither discard existing children nor bind twice.
	auto *collider = memnew(CollisionShape2D);
	collider->set_name("Collider2D");
	sprite->add_child(collider);
	ERR_PRINT_OFF;
	sprite->on_start();
	ERR_PRINT_ON;
	CHECK(sprite->get_collider(false) == collider);
	CHECK(sprite->get_anim2d() == nullptr);
	auto *animation = memnew(AnimatedSprite2D);
	sprite->add_child(animation);
	ERR_PRINT_OFF;
	sprite->on_start();
	ERR_PRINT_ON;
	CHECK(sprite->get_anim2d() == animation);
	CHECK(sprite->get_trigger() == nullptr);
	auto *area = memnew(Area2D);
	area->set_name("Area2D");
	sprite->add_child(area);
	ERR_PRINT_OFF;
	sprite->on_start();
	ERR_PRINT_ON;
	CHECK(sprite->get_trigger() == nullptr);
	auto *trigger = memnew(CollisionShape2D);
	trigger->set_name("Trigger2D");
	area->add_child(trigger);
	sprite->on_start();
	sprite->on_start();
	CHECK(sprite->get_trigger() == trigger);
	CHECK(animation->get_parent() == sprite->get_node(NodePath("RenderRoot")));
	List<Object::Connection> connections;
	area->get_signal_connection_list("area_entered", &connections);
	CHECK(connections.size() == 1);
}

TEST_CASE("[SceneTree][SPX] Sprite materials reject missing shaders without changing visible state") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2());
	auto *sprite = manager->get_sprite(id);
	const Color tint(0.2, 0.4, 0.6, 0.8);
	manager->set_color(id, tint);
	manager->set_material_params(id, "amount", 1);
	manager->set_material_params_color(id, "color", Color(1, 0, 0));
	manager->set_material_params_vec4(id, "bounds", Vector4(1, 2, 3, 4));
	sprite->set_uv_remap(Vector4(1, 1, 0, 0));
	CHECK(manager->get_material_params(id, "amount") == 0);
	CHECK(Vector4(manager->get_material_params_vec4(id, "bounds")) == Vector4());
	CHECK(Color(manager->get_material_params_color(id, "color")) == Color(GdColor()));
	ERR_PRINT_OFF;
	manager->set_material_shader(id, "res://missing_spx_shader.gdshader");
	ERR_PRINT_ON;
	CHECK(sprite->get_anim2d()->get_material().is_null());
	CHECK(Color(manager->get_color(id)).is_equal_approx(tint));
	CHECK(manager->get_material_shader(id) == nullptr);
}

TEST_CASE("[SceneTree][SPX] Point collision falls back to trigger geometry without readable pixels") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2(20, 30));
	const GdObj other = manager->create_bare_sprite(Vector2(20, 30));
	manager->set_trigger_rect(id, Vector2(5, 7), Vector2(10, 10));
	manager->set_collider_circle(id, Vector2(), 1);
	CHECK(manager->check_collision_with_point(id, Vector2(25, 37), true));
	CHECK_FALSE(manager->check_collision_with_point(id, Vector2(100, 100), true));
	manager->set_visible(id, false);
	CHECK_FALSE(manager->check_collision_with_point(id, Vector2(25, 37), true));
	CHECK(manager->check_collision_with_point(id, Vector2(25, 37), false));
	manager->get_sprite(id)->get_trigger()->set_shape(Ref<Shape2D>());
	CHECK_FALSE(manager->check_collision_with_point(id, Vector2(25, 37), false));
	CHECK_FALSE(manager->check_collision(id, other, true, true));
	CHECK_FALSE(manager->check_collision_by_alpha(id, 0.5));
	CHECK_FALSE(manager->check_collision_by_color(id, Color(1, 0, 0), 0.1, 0.5));
	ERR_PRINT_OFF;
	CHECK_FALSE(manager->is_anim_flipped_h(0));
	GdArrayInfo wrong_points{ 6, GD_ARRAY_TYPE_INT64, nullptr };
	manager->set_collider_polygon(id, Vector2(), &wrong_points);
	ERR_PRINT_ON;
	CHECK(Ref<CircleShape2D>(manager->get_sprite(id)->get_collider(false)->get_shape()).is_valid());
}

TEST_CASE("[SceneTree][SPX] Sprite force and impulse APIs convert SPX vectors before integration") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2());
	manager->set_physics_mode(id, SpxSprite::DYNAMIC);
	manager->set_use_gravity(id, false);
	manager->set_drag(id, 0);
	manager->set_friction(id, 0);
	manager->set_mass(id, 2);
	manager->add_force(id, Vector2(10, 20));
	manager->add_impulse(id, Vector2(6, 8));
	SceneTree::get_singleton()->physics_process(0.1);
	CHECK(Vector2(manager->get_velocity(id)).is_equal_approx(Vector2(4, 6)));
	CHECK(manager->get_sprite(id)->get_velocity().is_equal_approx(Vector2(4, -6)));
	SceneTree::get_singleton()->physics_process(0.1);
	CHECK(Vector2(manager->get_velocity(id)).is_equal_approx(Vector2(5, 8)));
	manager->set_velocity(id, Vector2());
	const Vector2 position = manager->get_position(id);
	manager->move_and_slide(id);
	CHECK(Vector2(manager->get_position(id)) == position);
	ERR_PRINT_OFF;
	CHECK(Vector2(manager->get_floor_normal(id)) == Vector2());
	CHECK(Vector2(manager->get_wall_normal(id)) == Vector2());
	ERR_PRINT_ON;
}

#ifdef THREADS_ENABLED
TEST_CASE("[SceneTree][SPX] Sprite getters and mutations reject worker access") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2(3, 4));
	manager->set_anim_flip_h(id, true);
	struct Request {
		SpxSpriteMgr *manager;
		GdObj id;
		Vector2 position;
		bool flipped = true;
		bool retrieved = true;
		float positions[2] = { 11, 12 };
	} request{ manager, id, Vector2(9, 9) };
	Thread worker;
	ERR_PRINT_OFF;
	const auto thread = worker.start([](void *p_data) {
		auto &data = *static_cast<Request *>(p_data);
		data.manager->set_position(data.id, Vector2(8, 9));
		data.position = data.manager->get_position(data.id);
		data.flipped = data.manager->is_anim_flipped_h(data.id);
		const float transforms[] = { 1, 0, float(data.id), 100, 200, 0, 1, 1, 0, 0, 1 };
		const float visuals[] = { 1, float(data.id), 2, 3, 0, 0, 0, 0, 0, 0 };
		float physics[] = { 1, 1, 0, 0, 100, 200, 0 };
		const uint32_t low = uint32_t(data.id), high = uint32_t(uint64_t(data.id) >> 32);
		memcpy(&physics[2], &low, sizeof(low));
		memcpy(&physics[3], &high, sizeof(high));
		data.manager->batch_update_transforms(transforms, 11);
		data.manager->batch_update_visuals(visuals, 10);
		data.manager->batch_update_physics(physics, 7);
		data.retrieved = data.manager->batch_retrieve_positions(&data.id, 1, data.positions, 2);
	}, &request);
	if (worker.is_started()) {
		worker.wait_to_finish();
	}
	ERR_PRINT_ON;
	REQUIRE(thread != Thread::UNASSIGNED_ID);
	CHECK(request.position == Vector2());
	CHECK_FALSE(request.flipped);
	CHECK_FALSE(request.retrieved);
	CHECK(request.positions[0] == 11);
	CHECK(request.positions[1] == 12);
	CHECK(Vector2(manager->get_position(id)) == Vector2(3, 4));
	CHECK(Vector2(manager->get_render_scale(id)) == Vector2(1, 1));
	CHECK(Vector2(manager->get_velocity(id)) == Vector2());
	CHECK(manager->is_anim_flipped_h(id));
}
#endif

TEST_CASE("[SceneTree][SPX] Sprite signals stay silent while the runtime is inactive") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2());
	auto *sprite = manager->get_sprite(id);
	static int calls;
	calls = 0;
	auto count = [](GdObj) { ++calls; };
	auto *callbacks = scene.engine->get_callbacks();
	callbacks->func_on_sprite_animation_looped = count;
	callbacks->func_on_sprite_animation_finished = count;
	callbacks->func_on_sprite_screen_entered = count;
	callbacks->func_on_sprite_screen_exited = count;
	callbacks->func_on_trigger_enter = [](GdObj, GdObj) { ++calls; };
	callbacks->func_on_trigger_exit = [](GdObj, GdObj) { ++calls; };
	sprite->get_anim2d()->emit_signal("animation_looped");
	sprite->get_anim2d()->emit_signal("animation_finished");
	Node *notifier = sprite->get_node(NodePath("RenderRoot/VisibleNotifier2D"));
	notifier->emit_signal("screen_entered");
	notifier->emit_signal("screen_exited");
	sprite->get_area2d()->emit_signal("area_entered", sprite->get_area2d());
	sprite->get_area2d()->emit_signal("area_exited", sprite->get_area2d());
	CHECK(calls == 0);
}

TEST_CASE("[SceneTree][SPX] Sprite texture bounds and frame offsets follow authored visuals") {
	TestSpx::TempDirectory files("spx_frame_bounds");
	files.write("sprite.gdshader", "shader_type canvas_item; uniform vec4 atlas_uv_rect2;");
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const GdObj id = manager->create_bare_sprite(Vector2(10.25, -20.25));
	auto *sprite = manager->get_sprite(id);
	auto *animation = sprite->get_anim2d();
	Ref<SpriteFrames> frames;
	frames.instantiate();
	frames->add_frame("default", Ref<Texture2D>());
	animation->set_sprite_frames(frames);
	CHECK(sprite->get_texture() == nullptr);
	CHECK(sprite->get_rect() == Rect2(0, 0, 1, 1));
	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(8, 6));
	frames->set_frame("default", 0, texture);
	CHECK(sprite->get_rect() == Rect2(6.25, 17.25, 8, 6));
	auto *viewport = sprite->get_viewport();
	const bool snap = viewport->is_snap_2d_transforms_to_pixel_enabled();
	viewport->set_snap_2d_transforms_to_pixel(true);
	CHECK(sprite->get_rect() == Rect2(6, 17, 8, 6));
	viewport->set_snap_2d_transforms_to_pixel(snap);
	Ref<ImageTexture> empty;
	empty.instantiate();
	frames->set_frame("default", 0, empty);
	CHECK(sprite->get_rect() == Rect2(10.25, 20.25, 1, 1));
	frames->set_frame("default", 0, texture);
	sprite->set_render_offset(Vector2(2, 3));
	sprite->set_anim_offset(Vector2(5, 6));
	sprite->set_dynamic_frame_offset_enabled(true);
	CHECK(animation->get_offset() == Vector2(3, 3));
	sprite->set_dynamic_frame_offset_enabled(false);
	CHECK_FALSE(sprite->is_dynamic_frame_offset_enabled());
	CHECK(animation->get_offset() == Vector2(5, 6));
	const auto shader_path = files.file("sprite.gdshader").utf8();
	manager->set_material_shader(id, shader_path.get_data());
	sprite->on_start();
	Ref<ShaderMaterial> material = animation->get_material();
	REQUIRE(material.is_valid());
	CHECK(Rect2(material->get_shader_parameter("atlas_uv_rect2")) == Rect2(0, 0, 1, 1));
}

TEST_CASE("[SceneTree][SPX] Sortable render and static sprites retain pivots and tree lifetime") {
	TestSpx::EngineScene scene;
	auto *parent = memnew(Node2D);
	parent->set_position(Vector2(10, 20));
	scene.root->add_child(parent);
	auto *rendered = memnew(SpxRenderSprite);
	auto *body = memnew(SpxStaticSprite);
	rendered->set_sort_id(7);
	body->set_sort_id(8);
	rendered->set_pivot(Vector2(2, 3));
	body->set_pivot(Vector2(2, 3));
	CHECK(rendered->get_sort_id_internal() == 7);
	CHECK(body->get_sort_id_internal() == 8);
	CHECK(Vector2(rendered->get_pivot()) == Vector2(2, 3));
	CHECK(Vector2(body->get_pivot()) == Vector2(2, 3));
	for (Node2D *node : { static_cast<Node2D *>(rendered), static_cast<Node2D *>(body) }) {
		auto *sortable = dynamic_cast<ISortableSprite *>(node);
		REQUIRE(sortable != nullptr);
		CHECK_FALSE(sortable->is_node_valid());
		parent->add_child(node);
		node->set_position(Vector2(5, 6));
		CHECK(sortable->is_node_valid());
		CHECK(sortable->is_sort_static());
		CHECK(sortable->get_sort_position() == Vector2(13, 23));
		sortable->set_sort_z_index(-5);
		CHECK(sortable->get_sort_z_index() == -5);
		CHECK(node->get_z_index() == -5);
	}
	CHECK(rendered->get_sort_id() == 7);
	CHECK(body->get_sort_id() == 8);
}

TEST_CASE("[SceneTree][SPX] Pixel sensing skips hidden distant and unreadable visuals") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	auto assign = [&](GdObj id, const Ref<Texture2D> &texture) {
		Ref<SpriteFrames> frames;
		frames.instantiate();
		frames->add_frame("default", texture);
		manager->get_sprite(id)->get_anim2d()->set_sprite_frames(frames);
	};
	auto solid = [](Color color) {
		Ref<Image> image = Image::create_empty(8, 8, false, Image::FORMAT_RGBA8);
		image->fill(color);
		return SpxImageTexture::create_from_image(image);
	};
	const GdObj self = manager->create_bare_sprite(Vector2());
	const GdObj near = manager->create_bare_sprite(Vector2());
	const GdObj far = manager->create_bare_sprite(Vector2(100, 100));
	const GdObj unreadable = manager->create_bare_sprite(Vector2());
	assign(self, solid(Color(1, 0, 0)));
	assign(near, solid(Color(0, 1, 0)));
	assign(far, solid(Color(1, 0, 0)));
	Ref<PlaceholderTexture2D> placeholder;
	placeholder.instantiate();
	placeholder->set_size(Vector2(8, 8));
	assign(unreadable, placeholder);
	manager->set_visible(near, false);
	CHECK_FALSE(manager->check_collision_by_alpha(self, 0.5));
	// The dummy renderer cannot read back the pen viewport used by color sensing.
	ERR_PRINT_OFF;
	CHECK_FALSE(manager->check_collision_by_color(self, Color(1, 0, 0), 0.1, 0.5));
	ERR_PRINT_ON;
	CHECK_FALSE(manager->check_collision_with_sprite(self, far, 0.5, true));
	CHECK_FALSE(manager->check_collision_with_sprite(self, unreadable, 0.5, true));
	manager->set_visible(near, true);
	CHECK(manager->check_collision_by_alpha(self, 0.5));
	ERR_PRINT_OFF;
	CHECK(manager->check_collision_by_color(self, Color(0, 1, 0), 0.1, 0.5));
	ERR_PRINT_ON;
	assign(unreadable, solid(Color(0, 0, 1)));
	manager->set_z_index(unreadable, 10);
	ERR_PRINT_OFF;
	CHECK(manager->check_collision_by_color(self, Color(0, 0, 1), 0.1, 0.5));
	CHECK_FALSE(manager->check_collision_by_color(self, Color(0, 1, 0), 0.1, 0.5));
	ERR_PRINT_ON;
	assign(self, solid(Color(1, 0, 0, 0)));
	CHECK_FALSE(manager->check_collision_with_sprite(self, near, 0.5, true));
	assign(self, placeholder);
	CHECK_FALSE(manager->check_collision_by_alpha(self, 0.5));
	CHECK_FALSE(manager->check_collision_by_color(self, Color(0, 0, 1), 0.1, 0.5));
	manager->get_sprite(self)->get_anim2d()->set_sprite_frames(Ref<SpriteFrames>());
	CHECK_FALSE(manager->check_collision_with_sprite(self, near, 0.5, true));
}

TEST_CASE("[SceneTree][SPX] Sprite physics controls preserve mode and velocity conventions") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2());
	auto *sprite = manager->get_sprite(id);
	manager->set_gravity(id, 50);
	manager->set_mass(id, 2);
	manager->set_use_gravity(id, false);
	manager->set_gravity_scale(id, 0.5);
	manager->set_drag(id, 0.25);
	manager->set_friction(id, 0.75);
	CHECK(manager->get_gravity(id) == 50);
	CHECK(manager->get_mass(id) == 2);
	CHECK_FALSE(manager->is_use_gravity(id));
	CHECK(manager->get_gravity_scale(id) == 0.5);
	CHECK(manager->get_drag(id) == 0.25);
	CHECK(manager->get_friction(id) == 0.75);
	for (int mode : { SpxSprite::DYNAMIC, SpxSprite::KINEMATIC, SpxSprite::STATIC, SpxSprite::NO_PHYSICS }) {
		manager->set_physics_mode(id, mode);
		CHECK(manager->get_physics_mode(id) == mode);
		CHECK(sprite->get_physics_mode() == mode);
	}
	manager->set_velocity(id, Vector2(20, 40));
	CHECK(Vector2(manager->get_velocity(id)) == Vector2(20, 40));
	CHECK(sprite->get_velocity() == Vector2(20, -40));
	CHECK_FALSE(manager->is_on_floor(id));
	CHECK_FALSE(manager->is_on_floor_only(id));
	CHECK_FALSE(manager->is_on_wall(id));
	CHECK_FALSE(manager->is_on_wall_only(id));
	CHECK_FALSE(manager->is_on_ceiling(id));
	CHECK_FALSE(manager->is_on_ceiling_only(id));
	CHECK(Vector2(manager->get_last_motion(id)) == Vector2());
	CHECK(Vector2(manager->get_position_delta(id)) == Vector2());
	CHECK(Vector2(manager->get_real_velocity(id)) == Vector2());
	manager->set_dont_destroy_on_load(id);
	CHECK(sprite->get_parent()->get_name() == StringName("dont_destroy_root"));
	GdObj disposable = manager->create_bare_sprite(Vector2(1, 2));
	manager->on_reset(0);
	CHECK_FALSE(manager->is_sprite_alive(id));
	CHECK_FALSE(manager->is_sprite_alive(disposable));
	manager->destroy_all_sprites();
	CHECK_FALSE(manager->is_sprite_alive(id));
}

TEST_CASE("[SceneTree][SPX] Pixel collision events track overlap, visibility and destroyed sprites") {
	TestSpx::TempDirectory files("spx_pixel_events");
	auto image = Image::create_empty(8, 8, false, Image::FORMAT_RGBA8);
	image->fill(Color(1, 0, 0, 1));
	REQUIRE(image->save_png(files.file("solid.png")) == OK);
	image->fill(Color(1, 0, 0, 0));
	REQUIRE(image->save_png(files.file("clear.png")) == OK);
	const CharString solid = files.file("solid.png").utf8(), clear = files.file("clear.png").utf8();
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	auto *manager = scene.engine->get_sprite();
	auto *physics = scene.engine->get_physics();
	const GdObj a = manager->create_bare_sprite(Vector2()), b = manager->create_bare_sprite(Vector2(2, 0));
	for (GdObj id : { a, b }) {
		manager->set_texture(id, solid.get_data());
		manager->set_collider_rect(id, Vector2(), Vector2(8, 8));
		manager->set_trigger_rect(id, Vector2(), Vector2(8, 8));
	}
	CHECK(manager->check_collision(a, b, false, false));
	CHECK(manager->check_collision_with_sprite(a, b, 0.5, false));
	CHECK(manager->check_collision_with_sprite(a, b, 0.5, true));
	CHECK(manager->check_collision_by_alpha(a, 0.5));
	CHECK_FALSE(manager->check_collision_by_alpha(a, 1));
	CHECK(manager->check_collision_with_point(a, Vector2(), true));
	CHECK_FALSE(manager->check_collision_with_point(a, Vector2(100, 100), true));
	manager->set_visible(a, false);
	CHECK_FALSE(manager->check_collision_with_point(a, Vector2(), true));
	CHECK_FALSE(manager->check_collision_with_sprite(a, b, 0.5, true));
	manager->set_visible(a, true);
	static Vector<TriggerPair> enters, exits;
	enters.clear();
	exits.clear();
	scene.engine->get_callbacks()->func_on_trigger_enter = [](GdObj first, GdObj second) { enters.push_back(TriggerPair(first, second)); };
	scene.engine->get_callbacks()->func_on_trigger_exit = [](GdObj first, GdObj second) { exits.push_back(TriggerPair(first, second)); };
	manager->on_trigger_enter(a, b);
	manager->on_trigger_enter(b, a);
	CHECK(enters.is_empty());
	manager->on_update(0.016);
	REQUIRE(enters.size() == 2);
	CHECK(enters[0] == TriggerPair(a, b));
	manager->on_update(0.016);
	CHECK(enters.size() == 2);
	manager->set_texture(b, clear.get_data());
	CHECK_FALSE(manager->check_collision_with_sprite(a, b, 0.5, true));
	manager->on_update(0.016);
	CHECK(exits.size() == 2);
	manager->set_texture(b, solid.get_data());
	manager->on_update(0.016);
	CHECK(enters.size() == 4);
	manager->on_trigger_exit(a, b);
	CHECK(exits.size() == 4);
	manager->on_trigger_exit(b, a);
	CHECK(exits.size() == 4);
	manager->on_trigger_enter(a, b);
	manager->on_update(0.016);
	REQUIRE(enters.size() == 6);
	auto *sprite = manager->get_sprite(b);
	memdelete(sprite);
	CHECK_FALSE(manager->is_sprite_alive(b));
	CHECK(exits.size() == 5); // Only the surviving sprite receives exit.
	manager->on_trigger_enter(a, b);
	manager->on_update(0.016);
	CHECK(enters.size() == 6);
	physics->set_collision_system_type(false);
	manager->on_trigger_enter(a, 0);
	manager->on_trigger_exit(a, 0);
	CHECK(enters.size() == 7);
	CHECK(exits.size() == 6);
	manager->set_debug_collision_visible(a, true);
	CHECK(manager->is_debug_collision_visible(a));
	manager->set_debug_collision_visible(a, false);
	CHECK_FALSE(manager->is_debug_collision_visible(a));
	manager->set_texture_atlas(a, solid.get_data(), Rect2(0, 0, 4, 4));
	CHECK(manager->get_sprite(a)->get_anim2d()->get_sprite_frames()->get_frame_texture("default", 0)->get_size() == Vector2(4, 4));
	manager->set_texture_atlas_direct(a, solid.get_data(), Rect2(0, 0, 2, 2));
	auto texture = manager->get_sprite(a)->get_anim2d()->get_sprite_frames()->get_frame_texture("default", 0);
	texture->set_name("cropped");
	CHECK(TestSpx::take_string(manager->get_texture(a)) == "cropped");
	manager->set_texture_direct(a, solid.get_data());
	CHECK(manager->check_collision_with_point(a, Vector2(), true));
}

TEST_CASE("[SceneTree][SPX] Destroying one collision pair preserves unrelated active pairs") {
	TestSpx::TempDirectory files("spx_independent_pixel_pairs");
	files.write_png("solid.png", Vector2i(8, 8), Color(1, 0, 0, 1));
	const CharString texture = files.file("solid.png").utf8();
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	auto *manager = scene.engine->get_sprite();
	const GdObj a = manager->create_bare_sprite(Vector2());
	const GdObj b = manager->create_bare_sprite(Vector2());
	const GdObj c = manager->create_bare_sprite(Vector2(100, 0));
	const GdObj d = manager->create_bare_sprite(Vector2(100, 0));
	for (GdObj id : { a, b, c, d }) {
		manager->set_texture(id, texture.get_data());
	}
	static Vector<TriggerPair> enters, exits;
	enters.clear();
	exits.clear();
	scene.engine->get_callbacks()->func_on_trigger_enter = [](GdObj first, GdObj second) { enters.push_back(TriggerPair(first, second)); };
	scene.engine->get_callbacks()->func_on_trigger_exit = [](GdObj first, GdObj second) { exits.push_back(TriggerPair(first, second)); };
	manager->on_trigger_enter(a, b);
	manager->on_trigger_enter(c, d);
	manager->on_update(0.016);
	REQUIRE(enters.size() == 4);
	// Pair cleanup happens on the actual PREDELETE notification, not when
	// destroy_sprite merely queues a node for a later SceneTree frame.
	memdelete(manager->get_sprite(b));
	CHECK_FALSE(manager->is_sprite_alive(b));
	CHECK(exits.size() == 1);
	if (exits.size() == 1) {
		CHECK(exits[0] == TriggerPair(a, b));
	}
	manager->on_update(0.016);
	CHECK(enters.size() == 4);
	CHECK(exits.size() == 1);
	// The surviving broad-phase pair must still be checked, and its active
	// pixel pair must not have been removed with the destroyed sprite.
	manager->set_visible(d, false);
	manager->on_update(0.016);
	CHECK(exits.size() == 3);
	if (exits.size() == 3) {
		CHECK(exits[1] == TriggerPair(c, d));
		CHECK(exits[2] == TriggerPair(c, d));
	}
	manager->set_visible(d, true);
	manager->on_update(0.016);
	CHECK(enters.size() == 6);
}

TEST_CASE("[SceneTree][SPX] Sprite scenes validate root type and apply requested positions") {
	TestSpx::TempDirectory files("spx_sprite_scene");
	files.write("sprite.tscn", "[gd_scene format=3]\n[node name=\"Dragon\" type=\"SpxSprite\"]\n[node name=\"RenderRoot\" type=\"Node2D\" parent=\".\"]\n[node name=\"Anim2D\" type=\"AnimatedSprite2D\" parent=\"RenderRoot\"]\n[node name=\"Collider2D\" type=\"CollisionShape2D\" parent=\".\"]\n[node name=\"Area2D\" type=\"Area2D\" parent=\".\"]\n[node name=\"Trigger2D\" type=\"CollisionShape2D\" parent=\"Area2D\"]\n");
	files.write("wrong.tscn", "[gd_scene format=3]\n[node name=\"Wrong\" type=\"Node\"]\n");
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	const CharString path = files.file("sprite.tscn").utf8(), wrong = files.file("wrong.tscn").utf8(), missing = files.file("missing.tscn").utf8();
	const GdObj id = manager->create_sprite(path.get_data(), Vector2(5, 10));
	REQUIRE(id != 0);
	CHECK(Vector2(manager->get_position(id)) == Vector2(5, 10));
	const GdObj backdrop = manager->create_backdrop(path.get_data());
	REQUIRE(backdrop != 0);
	CHECK(manager->get_sprite(backdrop)->is_backdrop_sprite());
	ERR_PRINT_OFF;
	CHECK(manager->create_sprite(wrong.get_data(), Vector2()) == 0);
	CHECK(manager->create_sprite(missing.get_data(), Vector2()) == 0);
	ERR_PRINT_ON;
}

TEST_CASE("[SceneTree][SPX] Authored sprite components migrate and forward engine signals once") {
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	auto *manager = scene.engine->get_sprite();
	auto *sprite = memnew(SpxSprite);
	auto *animation = memnew(AnimatedSprite2D);
	animation->set_name("LegacyAnimation");
	sprite->add_child(animation);
	auto *area = memnew(Area2D);
	area->set_name("LegacyArea");
	sprite->add_child(area);
	auto *trigger = memnew(CollisionShape2D);
	area->add_child(trigger);
	sprite->add_child(memnew(CollisionShape2D));
	auto *notifier = memnew(VisibleOnScreenNotifier2D);
	sprite->add_child(notifier);
	sprite->set_spx_type_name("Dragon");
	scene.root->add_child(sprite);
	static GdObj instantiated;
	static String type;
	instantiated = 0;
	type = "";
	scene.engine->get_callbacks()->func_on_scene_sprite_instantiated = [](GdObj id, GdString value) {
		instantiated = id;
		type = SpxStr(value);
	};
	manager->on_start();
	REQUIRE(instantiated != 0);
	CHECK(type == "Dragon");
	CHECK(manager->get_sprite(instantiated) == sprite);
	CHECK(animation->get_parent()->get_name() == StringName("RenderRoot"));
	CHECK(notifier->get_parent() == animation->get_parent());
	CHECK(sprite->get_area2d() == area);
	CHECK(sprite->get_trigger() == trigger);
	GdObj other_id = manager->create_bare_sprite(Vector2());
	auto *other = manager->get_sprite(other_id);
	static int signals, enters, exits;
	signals = 0;
	enters = 0;
	exits = 0;
	auto count = [](GdObj id) {
		CHECK(id == instantiated);
		++signals;
	};
	auto *callbacks = scene.engine->get_callbacks();
	callbacks->func_on_sprite_animation_changed = count;
	callbacks->func_on_sprite_frame_changed = count;
	callbacks->func_on_sprite_animation_looped = count;
	callbacks->func_on_sprite_animation_finished = count;
	callbacks->func_on_sprite_screen_entered = count;
	callbacks->func_on_sprite_screen_exited = count;
	callbacks->func_on_trigger_enter = [](GdObj, GdObj) { ++enters; };
	callbacks->func_on_trigger_exit = [](GdObj, GdObj) { ++exits; };
	for (const char *signal : { "animation_changed", "frame_changed", "animation_looped", "animation_finished" }) {
		animation->emit_signal(signal);
	}
	notifier->emit_signal("screen_entered");
	notifier->emit_signal("screen_exited");
	CHECK(signals == 6);
	scene.engine->get_physics()->set_collision_system_type(false);
	area->emit_signal("area_entered", other->get_area2d());
	area->emit_signal("area_exited", other->get_area2d());
	CHECK(enters == 1);
	CHECK(exits == 1);
	auto &sorter = SpxLayerSorter::instance();
	sorter.set_mode(LayerSortMode::VERTICAL);
	manager->on_update(0.016);
	CHECK(sprite->get_z_index() != other->get_z_index());
	sorter.set_mode(LayerSortMode::NONE);
}

} // namespace TestSpxSpriteMgr

#endif // TEST_SPX_SPRITE_MGR_H
