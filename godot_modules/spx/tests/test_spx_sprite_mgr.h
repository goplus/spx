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
	GdString shader_path = manager->get_material_shader(id);
	CHECK(SpxStr(shader_path) == files.file("sprite.gdshader"));
	SpxAbi::free_return_cstr(shader_path);
	manager->set_material_params(id, "amount", 0.75);
	CHECK(manager->get_material_params(id, "amount") == doctest::Approx(0.75));
	CHECK(double(material->get_shader_parameter("amount")) == doctest::Approx(0.75));
	manager->set_material_params_vec4(id, "bounds", Vector4(1, 2, 3, 4));
	CHECK(Vector4(manager->get_material_params_vec4(id, "bounds")) == Vector4(1, 2, 3, 4));
	CHECK(Vector4(material->get_shader_parameter("bounds")) == Vector4(1, 2, 3, 4));
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
