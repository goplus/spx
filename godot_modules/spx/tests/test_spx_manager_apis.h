/**************************************************************************/
/*  test_spx_manager_apis.h                                               */
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

#ifndef TEST_SPX_MANAGER_APIS_H
#define TEST_SPX_MANAGER_APIS_H

#include "../spx_camera_mgr.h"
#include "../spx_debug_mgr.h"
#include "../spx_draw_tiles.h"
#include "../spx_engine.h"
#include "../spx_ext_mgr.h"
#include "../spx_input_mgr.h"
#include "../spx_list_monitor.h"
#include "../spx_navigation_mgr.h"
#include "../spx_pen_mgr.h"
#include "../spx_physics_mgr.h"
#include "../spx_platform_mgr.h"
#include "../spx_scene_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "../spx_tilemap_mgr.h"
#include "../spx_tilemapparser_mgr.h"
#include "../spx_ui_mgr.h"

#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/io/resource_saver.h"
#include "core/input/input.h"
#include "core/input/input_map.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/visible_on_screen_notifier_2d.h"
#include "scene/main/canvas_layer.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/packed_scene.h"
#include "scene/resources/placeholder_textures.h"
#include "scene/resources/shader.h"
#include "scene/main/window.h"
#include "scene/resources/gradient_texture.h"
#include "servers/audio_server.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"

#include <cstdint>
#include <cstring>

namespace TestSpxManagerApis {

class ListMonitorProbe : public SpxListMonitor {
public:
	void send_gui_input(const Ref<InputEvent> &p_event) { gui_input(p_event); }
};

class DrawTilesProbe : public SpxDrawTiles {
public:
	void send_input(const Ref<InputEvent> &p_event) { input(p_event); }
};

struct EngineFixture {
	Node *root = nullptr;

	EngineFixture() {
		if (AudioServer::get_singleton() == nullptr) {
			AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
			memnew(AudioServer)->init();
		}
		root = memnew(Node);
		SceneTree *tree = SceneTree::get_singleton();
		tree->get_root()->add_child(root);
		SpxEngine::register_callbacks(nullptr);
		SpxEngine::get_singleton()->set_root_node(tree, root);
		SpxEngine::get_singleton()->on_awake();
	}

	~EngineFixture() {
		SpxEngine::shutdown();
		memdelete(root);
	}
};

struct FloatArray {
	Vector<float> values;
	GdArrayInfo info = {};

	FloatArray(std::initializer_list<float> p_values) {
		for (float value : p_values) {
			values.push_back(value);
		}
		info.type = GD_ARRAY_TYPE_FLOAT;
		info.size = values.size();
		info.data = values.ptrw();
	}

	GdArray ptr() { return &info; }
};

struct StringArray {
	Vector<CharString> storage;
	Vector<GdString> values;
	GdArrayInfo info = {};

	StringArray(std::initializer_list<String> p_values) {
		for (const String &value : p_values) {
			storage.push_back(value.utf8());
		}
		values.resize(storage.size());
		for (int i = 0; i < storage.size(); i++) {
			values.write[i] = storage[i].get_data();
		}
		info.type = GD_ARRAY_TYPE_STRING;
		info.size = values.size();
		info.data = values.ptrw();
	}

	GdArray ptr() { return &info; }
};

static float raw_u32(uint32_t p_value) {
	float result = 0.0f;
	std::memcpy(&result, &p_value, sizeof(result));
	return result;
}

static void append_pen_command(Vector<float> &r_packet, int p_command, GdObj p_id, float p_a = 0.0f, float p_b = 0.0f, float p_c = 0.0f, float p_d = 0.0f) {
	const uint64_t id = static_cast<uint64_t>(p_id);
	r_packet.push_back((float)p_command);
	r_packet.push_back(raw_u32((uint32_t)id));
	r_packet.push_back(raw_u32((uint32_t)(id >> 32)));
	r_packet.push_back(p_a);
	r_packet.push_back(p_b);
	r_packet.push_back(p_c);
	r_packet.push_back(p_d);
	r_packet.push_back(0.0f);
}

static String take_string(GdString p_value) {
	String result = SpxStr(p_value);
	SpxAbi::free_return_cstr(p_value);
	return result;
}

static void assign_scene_owner(Node *p_node, Node *p_owner) {
	for (int i = 0; i < p_node->get_child_count(); i++) {
		Node *child = p_node->get_child(i);
		child->set_owner(p_owner);
		assign_scene_owner(child, p_owner);
	}
}

static String save_packed_scene(Node *p_root, const String &p_name) {
	assign_scene_owner(p_root, p_root);
	Ref<PackedScene> scene;
	scene.instantiate();
	const String path = TestUtils::get_temp_path(p_name);
	const Error pack_error = scene->pack(p_root);
	const Error save_error = pack_error == OK ? ResourceSaver::save(scene, path) : pack_error;
	memdelete(p_root);
	return save_error == OK ? path : String();
}

TEST_CASE("[SceneTree][SPX] Sprite manager drives visual animation and physics state") {
	EngineFixture fixture;
	SpxSpriteMgr *sprites = SpxEngine::get_singleton()->get_sprite();
	const String texture_path = TestUtils::get_data_path("images/icon.webp");
	CharString texture_utf8 = texture_path.utf8();
	GdObj first = sprites->create_bare_sprite(Vector2(3, 4));
	GdObj second = sprites->create_bare_sprite(Vector2(3, 4));
	REQUIRE(first != NULL_OBJECT_ID);
	REQUIRE(second != NULL_OBJECT_ID);

	sprites->set_process(first, true);
	sprites->set_physic_process(first, true);
	sprites->set_type_name(first, "Player");
	sprites->set_position(first, Vector2(12, -8));
	CHECK(sprites->get_position(first) == Vector2(12, -8));
	sprites->set_rotation(first, 0.75f);
	CHECK(Math::is_equal_approx(sprites->get_rotation(first), 0.75f));
	sprites->set_scale(first, Vector2(2, 3));
	CHECK(sprites->get_scale(first) == Vector2(2, 3));
	sprites->set_render_scale(first, Vector2(0.5f, 0.75f));
	CHECK(sprites->get_render_scale(first) == Vector2(0.5f, 0.75f));
	sprites->set_pivot(first, Vector2(2, 5));
	CHECK(sprites->get_pivot(first) == Vector2(2, 5));
	sprites->set_transform(first, Vector2(1, 2), 0.25f, Vector2(1.5f, 2.5f), true, Vector2(4, 6));
	CHECK(sprites->get_position(first) == Vector2(1, 2));
	CHECK(sprites->get_visible(first));

	sprites->set_child_position(first, "RenderRoot", Vector2(4, 5));
	CHECK(sprites->get_child_position(first, "RenderRoot") == Vector2(4, 5));
	sprites->set_child_rotation(first, "RenderRoot", 0.5f);
	CHECK(Math::is_equal_approx(sprites->get_child_rotation(first, "RenderRoot"), 0.5f));
	sprites->set_child_scale(first, "RenderRoot", Vector2(2, 2));
	CHECK(sprites->get_child_scale(first, "RenderRoot") == Vector2(2, 2));

	sprites->set_texture_direct(first, texture_utf8.get_data());
	REQUIRE(sprites->get_sprite(first)->get_anim2d()->get_sprite_frames().is_valid());
	CHECK(sprites->get_sprite(first)->get_anim2d()->get_sprite_frames()->get_frame_count("default") == 1);
	const Rect2 sprite_rect = sprites->get_sprite(first)->get_rect();
	CHECK(sprite_rect.size.x > 0);
	CHECK(sprite_rect.size.y > 0);
	Viewport *sprite_viewport = sprites->get_sprite(first)->get_viewport();
	REQUIRE(sprite_viewport != nullptr);
	const bool snap_transforms = sprite_viewport->is_snap_2d_transforms_to_pixel_enabled();
	sprite_viewport->set_snap_2d_transforms_to_pixel(true);
	CHECK(sprites->get_sprite(first)->get_rect().size == sprite_rect.size);
	sprite_viewport->set_snap_2d_transforms_to_pixel(snap_transforms);
	take_string(sprites->get_texture(first));
	sprites->set_texture_atlas_direct(first, texture_utf8.get_data(), Rect2(0, 0, 16, 16));
	sprites->set_texture_direct(first, texture_utf8.get_data());
	sprites->set_texture(second, texture_utf8.get_data());
	sprites->set_texture_atlas(second, texture_utf8.get_data(), Rect2(0, 0, 16, 16));
	sprites->set_texture_direct(second, texture_utf8.get_data());

	const String shader_path = TestUtils::get_temp_path("spx_sprite_material.tres");
	Ref<Shader> shader;
	shader.instantiate();
	shader->set_code(R"(shader_type canvas_item;
uniform vec4 color : source_color = vec4(1.0);
uniform float amount = 0.0;
uniform vec4 vector_value = vec4(0.0);
uniform vec4 color_value : source_color = vec4(0.0);
uniform vec4 uv_remap = vec4(0.0, 0.0, 1.0, 1.0);
void fragment() { COLOR = texture(TEXTURE, UV) * color; })");
	REQUIRE(ResourceSaver::save(shader, shader_path) == OK);
	CharString shader_utf8 = shader_path.utf8();
	sprites->set_material_shader(first, shader_utf8.get_data());
	CHECK(take_string(sprites->get_material_shader(first)) == shader_path);
	sprites->set_material_params(first, "amount", 0.75f);
	CHECK(Math::is_equal_approx(sprites->get_material_params(first, "amount"), 0.75f));
	sprites->set_material_params_vec(first, "vector_value", 1, 2, 3, 4);
	CHECK(sprites->get_material_params_vec4(first, "vector_value") == Vector4(1, 2, 3, 4));
	sprites->set_material_params_vec4(first, "vector_value", Vector4(4, 3, 2, 1));
	CHECK(sprites->get_material_params_vec4(first, "vector_value") == Vector4(4, 3, 2, 1));
	sprites->set_material_params_color(first, "color_value", Color(0.1f, 0.2f, 0.3f, 0.4f));
	CHECK(sprites->get_material_params_color(first, "color_value").is_equal_approx(Color(0.1f, 0.2f, 0.3f, 0.4f)));
	sprites->set_color(first, Color(0.2f, 0.4f, 0.6f, 0.8f));
	CHECK(sprites->get_color(first).is_equal_approx(Color(0.2f, 0.4f, 0.6f, 0.8f)));
	sprites->set_z_index(first, 7);
	CHECK(sprites->get_z_index(first) == 7);
	sprites->set_visible(first, false);
	CHECK_FALSE(sprites->get_visible(first));
	sprites->set_visible(first, true);

	CHECK(take_string(sprites->get_anim(first)) == "default");
	sprites->set_anim_frame(first, 0);
	CHECK(sprites->get_anim_frame(first) == 0);
	sprites->set_anim_speed_scale(first, 1.5f);
	CHECK(Math::is_equal_approx(sprites->get_anim_speed_scale(first), 1.5f));
	sprites->set_anim_centered(first, false);
	CHECK_FALSE(sprites->is_anim_centered(first));
	sprites->set_anim_offset(first, Vector2(2, 3));
	CHECK(sprites->get_anim_offset(first) == Vector2(2, 3));
	sprites->set_anim_flip_h(first, true);
	CHECK(sprites->is_anim_flipped_h(first));
	sprites->set_anim_flip_v(first, true);
	CHECK(sprites->is_anim_flipped_v(first));
	sprites->set_anim(first, "default");
	sprites->play_anim(first, "default", 1.25f, true, false);
	CHECK_FALSE(sprites->is_playing_anim(first));
	CHECK(Math::is_finite(sprites->get_anim_playing_speed(first)));
	sprites->play_backwards_anim(first, "default");
	sprites->pause_anim(first);
	sprites->stop_anim(first);
	take_string(sprites->get_current_anim_name(first));

	sprites->set_velocity(first, Vector2(5, -6));
	CHECK(sprites->get_velocity(first) == Vector2(5, -6));
	sprites->set_gravity(first, 9.5f);
	CHECK(Math::is_equal_approx(sprites->get_gravity(first), 9.5f));
	sprites->set_mass(first, 2.5f);
	CHECK(Math::is_equal_approx(sprites->get_mass(first), 2.5f));
	sprites->set_physics_mode(first, 1);
	CHECK(sprites->get_physics_mode(first) == 1);
	sprites->set_use_gravity(first, false);
	CHECK_FALSE(sprites->is_use_gravity(first));
	sprites->set_gravity_scale(first, 0.5f);
	CHECK(Math::is_equal_approx(sprites->get_gravity_scale(first), 0.5f));
	sprites->set_drag(first, 0.2f);
	CHECK(Math::is_equal_approx(sprites->get_drag(first), 0.2f));
	sprites->set_friction(first, 0.3f);
	CHECK(Math::is_equal_approx(sprites->get_friction(first), 0.3f));
	sprites->set_physics_mode(first, SpxSprite::DYNAMIC);
	sprites->set_use_gravity(first, true);
	sprites->add_force(first, Vector2(2, 3));
	sprites->add_impulse(first, Vector2(4, 5));
	SpxSprite *physics_sprite = sprites->get_sprite(first);
	REQUIRE(physics_sprite != nullptr);
	physics_sprite->notification(Node::NOTIFICATION_PHYSICS_PROCESS);
	sprites->set_physics_mode(first, SpxSprite::KINEMATIC);
	physics_sprite->notification(Node::NOTIFICATION_PHYSICS_PROCESS);
	sprites->set_physics_mode(first, SpxSprite::STATIC);
	physics_sprite->notification(Node::NOTIFICATION_PHYSICS_PROCESS);
	sprites->set_physics_mode(first, SpxSprite::NO_PHYSICS);
	sprites->set_velocity(first, Vector2(1, 1));
	physics_sprite->notification(Node::NOTIFICATION_PHYSICS_PROCESS);
	sprites->set_physics_mode(first, SpxSprite::KINEMATIC);
	sprites->set_use_gravity(first, false);
	CHECK_FALSE(sprites->is_on_floor(first));
	CHECK_FALSE(sprites->is_on_floor_only(first));
	CHECK_FALSE(sprites->is_on_wall(first));
	CHECK_FALSE(sprites->is_on_wall_only(first));
	CHECK_FALSE(sprites->is_on_ceiling(first));
	CHECK_FALSE(sprites->is_on_ceiling_only(first));
	CHECK(sprites->get_last_motion(first).is_finite());
	CHECK(sprites->get_position_delta(first).is_finite());
	CHECK(sprites->get_floor_normal(first).is_finite());
	CHECK(sprites->get_wall_normal(first).is_finite());
	(void)sprites->get_real_velocity(first);
	sprites->move_and_slide(first);

	sprites->set_collision_layer(first, 3);
	CHECK(sprites->get_collision_layer(first) == 3);
	sprites->set_collision_mask(first, 5);
	CHECK(sprites->get_collision_mask(first) == 5);
	sprites->set_trigger_layer(first, 7);
	CHECK(sprites->get_trigger_layer(first) == 7);
	sprites->set_trigger_mask(first, 9);
	CHECK(sprites->get_trigger_mask(first) == 9);
	sprites->set_collider_rect(first, Vector2(), Vector2(20, 10));
	sprites->set_collider_circle(first, Vector2(1, 2), 8);
	sprites->set_collider_capsule(first, Vector2(), Vector2(8, 20));
	FloatArray polygon{ 0, 0, 12, 0, 0, 12 };
	sprites->set_collider_polygon(first, Vector2(), polygon.ptr());
	sprites->set_trigger_rect(first, Vector2(), Vector2(20, 10));
	sprites->set_trigger_circle(first, Vector2(), 8);
	sprites->set_trigger_capsule(first, Vector2(), Vector2(8, 20));
	sprites->set_trigger_polygon(first, Vector2(), polygon.ptr());
	sprites->set_collision_enabled(first, false);
	CHECK_FALSE(sprites->is_collision_enabled(first));
	sprites->set_collision_enabled(first, true);
	sprites->set_trigger_enabled(first, false);
	CHECK_FALSE(sprites->is_trigger_enabled(first));
	sprites->set_trigger_enabled(first, true);
	sprites->set_debug_collision_visible(first, true);
	CHECK(sprites->is_debug_collision_visible(first));

	sprites->set_position(second, sprites->get_position(first));
	CHECK(sprites->check_collision(first, second, true, true));
	sprites->check_collision_with_point(first, sprites->get_position(first), false);
	CHECK(sprites->check_collision_with_sprite(first, second, 0.05f, false));
	sprites->check_collision_with_sprite(first, second, 0.05f, true);
	sprites->check_collision_by_alpha(first, 0.05f);
	sprites->check_collision_by_color(first, Color(1, 1, 1), 0.1f, 0.05f);
	sprites->check_collision_by_colors(first, Color(1, 1, 1), Color(1, 1, 1), 0.1f, 0.05f);
	sprites->on_trigger_enter(first, second);
	sprites->on_trigger_exit(first, second);
	sprites->set_pixel_collision_sampling_step(0);
	CHECK(sprites->get_pixel_collision_sampling_step() == 1);
	sprites->set_pixel_collision_sampling_step(4);
	CHECK(sprites->get_pixel_collision_sampling_step() == 4);

	GdObj clone = sprites->clone_sprite(first);
	REQUIRE(clone != NULL_OBJECT_ID);
	CHECK(sprites->is_sprite_alive(clone));
	Vector<ISortableSprite *> sortables;
	sprites->collect_sortable_sprites(sortables);
	CHECK(sortables.size() == 3);
	CHECK(sprites->destroy_sprite(clone));
	Ref<SpriteFrames> edge_frames;
	edge_frames.instantiate();
	Ref<PlaceholderTexture2D> zero_size_texture;
	zero_size_texture.instantiate();
	zero_size_texture->set_size(Size2());
	edge_frames->add_frame("default", zero_size_texture);
	SpxSprite *second_sprite = sprites->get_sprite(second);
	REQUIRE(second_sprite != nullptr);
	second_sprite->get_anim2d()->set_sprite_frames(edge_frames);
	CHECK(second_sprite->get_rect().size == Size2(1, 1));
	edge_frames->set_frame("default", 0, Ref<Texture2D>(), 1.0f);
	CHECK(take_string(second_sprite->get_texture()).is_empty());
	CHECK(second_sprite->get_rect() == Rect2(0, 0, 1, 1));
	sprites->set_dont_destroy_on_load(second);
	CHECK(DirAccess::remove_absolute(shader_path) == OK);
}

TEST_CASE("[SceneTree][SPX] UI manager exercises each built-in control type") {
	EngineFixture fixture;
	SpxUiMgr *ui = SpxEngine::get_singleton()->get_ui();
	const GdObj button = ui->create_button("", "Run");
	const GdObj label = ui->create_label("", "Score");
	const GdObj image = ui->create_image("");
	const GdObj toggle = ui->create_toggle("", true);
	const GdObj slider = ui->create_slider("", 3.0f);
	const GdObj input = ui->create_input("", "name");
	REQUIRE(button != NULL_OBJECT_ID);
	REQUIRE(label != NULL_OBJECT_ID);
	REQUIRE(image != NULL_OBJECT_ID);
	REQUIRE(toggle != NULL_OBJECT_ID);
	REQUIRE(slider != NULL_OBJECT_ID);
	REQUIRE(input != NULL_OBJECT_ID);

	CHECK(ui->get_type(button) == (GdInt)ESpxUiType::Button);
	ui->set_text(button, "Start");
	CHECK(take_string(ui->get_text(button)) == "Start");
	ui->set_text(label, "Points");
	CHECK(take_string(ui->get_text(label)) == "Points");
	ui->set_text(toggle, "Music");
	CHECK(take_string(ui->get_text(toggle)) == "Music");
	ui->set_text(image, "unsupported");
	CHECK(take_string(ui->get_text(image)).is_empty());
	CHECK(ui->set_text_max_width(label, "A fairly long label", 40) <= 40);
	ui->set_color(label, Color(1, 0.5f, 0.25f));
	CHECK(ui->get_color(label).is_equal_approx(Color(1, 0.5f, 0.25f)));
	ui->set_font_size(label, 18);
	CHECK(ui->get_font_size(label) == 18);
	ui->set_visible(image, false);
	CHECK_FALSE(ui->get_visible(image));
	ui->set_interactable(button, false);
	CHECK(ui->get_interactable(button));

	ui->set_rect(image, Rect2(5, 6, 70, 80));
	CHECK(ui->get_rect(image) == Rect2(5, 6, 70, 80));
	ui->set_position(image, Vector2(9, 10));
	CHECK(ui->get_position(image) == Vector2(9, 10));
	ui->set_size(image, Vector2(50, 60));
	CHECK(ui->get_size(image) == Vector2(50, 60));
	ui->set_scale(image, Vector2(1.5f, 2.0f));
	CHECK(ui->get_scale(image) == Vector2(1.5f, 2.0f));
	CHECK(ui->get_canvas_scale(image) == Vector2(1.5f, 2.0f));
	ui->set_rotation(image, 0.25f);
	CHECK(Math::is_equal_approx(ui->get_rotation(image), 0.25f));
	ui->set_flip(image, true, true);
	CHECK(ui->get_flip(image, true));
	ui->set_flip(image, false, true);
	CHECK(ui->get_flip(image, false));
	ui->set_layout_direction(image, Control::LAYOUT_DIRECTION_RTL);
	CHECK(ui->get_layout_direction(image) == Control::LAYOUT_DIRECTION_RTL);
	ui->set_layout_mode(image, 0);
	CHECK(ui->get_layout_mode(image) >= 0);
	ui->set_anchors_preset(image, Control::PRESET_CENTER);
	CHECK(ui->get_anchors_preset(image) >= 0);
	ui->set_global_position(image, Vector2(11, 12));
	CHECK(ui->get_global_position(image) == Vector2(11, 12));

	CHECK(SpxUiMgr::get_node_type(nullptr) == ESpxUiType::None);
	for (GdObj id : { button, label, image, toggle, slider, input }) {
		SpxUi *node = ui->get_node(id);
		REQUIRE(node != nullptr);
		CHECK(SpxUiMgr::get_node_type(node->get_control_item()) != ESpxUiType::None);
	}
	SpxControl *parent_control = memnew(SpxControl);
	SpxLabel *child_control = memnew(SpxLabel);
	child_control->set_name("ChildLabel");
	parent_control->add_child(child_control);
	SpxUi *parent_node = ui->on_create_node(parent_control, (GdInt)ESpxUiType::Control);
	REQUIRE(parent_node != nullptr);
	const GdObj child_id = ui->bind_node(parent_node->get_gid(), "ChildLabel");
	REQUIRE(child_id != NULL_OBJECT_ID);
	CHECK(ui->get_type(child_id) == (GdInt)ESpxUiType::Label);
	CHECK(ui->bind_node(parent_node->get_gid(), "Missing") == NULL_OBJECT_ID);
	CHECK(ui->bind_node(999999, "ChildLabel") == NULL_OBJECT_ID);

	ui->set_range(slider, -2, 8, 0.5f, 4.5f);
	CHECK(Math::is_equal_approx(ui->get_range_value(slider), 4.5f));
	ui->set_text(input, "updated");
	CHECK(take_string(ui->get_text(input)) == "updated");

	const String texture_path = TestUtils::get_temp_path("spx_ui_texture.tres");
	Ref<GradientTexture2D> texture;
	texture.instantiate();
	texture->set_name("ui-texture");
	REQUIRE(ResourceSaver::save(texture, texture_path) == OK);
	CharString texture_utf8 = texture_path.utf8();
	ui->set_texture(image, texture_utf8.get_data());
	CHECK(take_string(ui->get_texture(image)) == "ui-texture");
	ui->set_texture(button, texture_utf8.get_data());
	CHECK(take_string(ui->get_texture(button)) == "ui-texture");
	ui->set_texture(toggle, texture_utf8.get_data());
	CHECK(take_string(ui->get_texture(toggle)) == "ui-texture");
	ui->set_texture(label, texture_utf8.get_data());
	CHECK(take_string(ui->get_texture(label)).is_empty());
	ui->set_texture(label, "missing-ui-texture.tres");
	SpxUi *label_node = ui->get_node(label);
	REQUIRE(label_node != nullptr);
	label_node->set_font("");
	CHECK(take_string(label_node->get_font()).is_empty());
	CHECK(DirAccess::remove_absolute(texture_path) == OK);

	ListMonitorProbe *monitor = memnew(ListMonitorProbe);
	SpxUi *monitor_node = ui->on_create_node(monitor, (GdInt)ESpxUiType::Control);
	REQUIRE(monitor_node != nullptr);
	monitor->set_size(Size2(160, 100));
	StringArray items{ "one", "two", "three", "four", "five", "six" };
	ui->set_list_items(monitor_node->get_gid(), "values", items.ptr(), Color(0.2f, 0.8f, 0.4f));
	Ref<InputEventMouseButton> wheel;
	wheel.instantiate();
	wheel->set_pressed(true);
	wheel->set_button_index(MouseButton::WHEEL_DOWN);
	wheel->set_factor(1.0f);
	monitor->send_gui_input(wheel);
	Ref<InputEventPanGesture> pan;
	pan.instantiate();
	pan->set_delta(Vector2(0, 2));
	monitor->send_gui_input(pan);
	Ref<InputEventScreenDrag> drag;
	drag.instantiate();
	drag->set_relative(Vector2(0, -4));
	monitor->send_gui_input(drag);
	ERR_PRINT_OFF
	monitor->notification(CanvasItem::NOTIFICATION_DRAW);
	ERR_PRINT_ON
	monitor->set_items("empty", PackedStringArray(), Color(1, 0, 0));
	ERR_PRINT_OFF
	monitor->notification(CanvasItem::NOTIFICATION_DRAW);
	ERR_PRINT_ON

	CHECK(ui->destroy_node(button));
	CHECK_FALSE(ui->destroy_node(999999));
}

TEST_CASE("[SceneTree][SPX] Serialized sprite and UI scenes preserve runtime bindings") {
	EngineFixture fixture;
	SpxSpriteMgr *sprites = SpxEngine::get_singleton()->get_sprite();
	SpxUiMgr *ui = SpxEngine::get_singleton()->get_ui();

	SpxSprite *authored = memnew(SpxSprite);
	authored->set_spx_type_name("AuthoredSprite");
	AnimatedSprite2D *animation = memnew(AnimatedSprite2D);
	Ref<SpriteFrames> frames;
	frames.instantiate();
	animation->set_sprite_frames(frames);
	authored->add_child(animation);
	Area2D *area = memnew(Area2D);
	authored->add_child(area);
	CollisionShape2D *trigger = memnew(CollisionShape2D);
	Ref<CircleShape2D> trigger_shape;
	trigger_shape.instantiate();
	trigger_shape->set_radius(5);
	trigger->set_shape(trigger_shape);
	area->add_child(trigger);
	CollisionShape2D *collider = memnew(CollisionShape2D);
	Ref<CircleShape2D> collider_shape;
	collider_shape.instantiate();
	collider_shape->set_radius(6);
	collider->set_shape(collider_shape);
	authored->add_child(collider);
	VisibleOnScreenNotifier2D *notifier = memnew(VisibleOnScreenNotifier2D);
	authored->add_child(notifier);
	const String sprite_scene_path = save_packed_scene(authored, "spx_authored_sprite.tscn");
	REQUIRE_FALSE(sprite_scene_path.is_empty());
	CharString sprite_scene_utf8 = sprite_scene_path.utf8();

	Ref<PackedScene> sprite_scene = ResourceLoader::load(sprite_scene_path);
	REQUIRE(sprite_scene.is_valid());
	SpxSprite *scene_sprite = Object::cast_to<SpxSprite>(sprite_scene->instantiate());
	REQUIRE(scene_sprite != nullptr);
	fixture.root->add_child(scene_sprite);
	sprites->on_start();
	CHECK(scene_sprite->get_gid() != NULL_OBJECT_ID);
	REQUIRE(scene_sprite->get_anim2d() != nullptr);
	CHECK(scene_sprite->get_anim2d()->get_parent()->get_name() == StringName("RenderRoot"));

	const GdObj loaded_id = sprites->create_sprite(sprite_scene_utf8.get_data(), Vector2(4, 5));
	const GdObj backdrop_id = sprites->create_backdrop(sprite_scene_utf8.get_data());
	REQUIRE(loaded_id != NULL_OBJECT_ID);
	REQUIRE(backdrop_id != NULL_OBJECT_ID);
	SpxSprite *loaded = sprites->get_sprite(loaded_id);
	REQUIRE(loaded != nullptr);
	CHECK(sprites->get_sprite(backdrop_id)->is_backdrop_sprite());
	loaded->call("on_sprite_frames_set_changed");
	loaded->call("on_sprite_animation_changed");
	loaded->call("on_sprite_frame_changed");
	loaded->call("on_sprite_animation_looped");
	loaded->call("on_sprite_animation_finished");
	loaded->call("on_sprite_vfx_finished");
	loaded->call("on_sprite_screen_exited");
	loaded->call("on_sprite_screen_entered");
	const GdObj other_id = sprites->create_bare_sprite(Vector2());
	SpxSprite *other = sprites->get_sprite(other_id);
	REQUIRE(other != nullptr);
	loaded->get_area2d()->emit_signal("area_entered", other->get_area2d());
	loaded->get_area2d()->emit_signal("area_exited", other->get_area2d());
	loaded->on_start();

	SpxButton *authored_button = memnew(SpxButton);
	authored_button->set_text("Authored");
	const String button_scene_path = save_packed_scene(authored_button, "spx_authored_button.tscn");
	REQUIRE_FALSE(button_scene_path.is_empty());
	CharString button_scene_utf8 = button_scene_path.utf8();
	const GdObj generic_button = ui->create_node(button_scene_utf8.get_data());
	const GdObj typed_button = ui->create_button(button_scene_utf8.get_data(), "Loaded");
	REQUIRE(generic_button != NULL_OBJECT_ID);
	REQUIRE(typed_button != NULL_OBJECT_ID);
	CHECK(ui->get_type(generic_button) == (GdInt)ESpxUiType::Button);
	SpxUi *generic_node = ui->get_node(generic_button);
	REQUIRE(generic_node != nullptr);
	generic_node->get_button()->emit_signal("pressed");
	CHECK_FALSE(generic_node->owns_binding(ObjectID()));

	const String invalid_scene_path = save_packed_scene(memnew(Node2D), "spx_invalid_control.tscn");
	REQUIRE_FALSE(invalid_scene_path.is_empty());
	CharString invalid_scene_utf8 = invalid_scene_path.utf8();
	Ref<PackedScene> empty_scene;
	empty_scene.instantiate();
	const String empty_scene_path = TestUtils::get_temp_path("spx_empty_control.res");
	REQUIRE(ResourceSaver::save(empty_scene, empty_scene_path) == OK);
	CharString empty_scene_utf8 = empty_scene_path.utf8();
	CHECK(sprites->create_sprite(invalid_scene_utf8.get_data(), Vector2()) == NULL_OBJECT_ID);
	CHECK(ui->create_node(invalid_scene_utf8.get_data()) == NULL_OBJECT_ID);
	ERR_PRINT_OFF
	CHECK(ui->create_node(empty_scene_utf8.get_data()) == NULL_OBJECT_ID);
	ERR_PRINT_ON
	CHECK(sprites->create_sprite("missing-spx-sprite.tscn", Vector2()) == NULL_OBJECT_ID);
	CHECK(ui->create_node("missing-spx-control.tscn") == NULL_OBJECT_ID);
	Node2D *plain_node = memnew(Node2D);
	CHECK(SpxUiMgr::get_node_type(plain_node) == ESpxUiType::None);
	memdelete(plain_node);
	SpxSprite *incomplete = memnew(SpxSprite);
	ERR_PRINT_OFF
	CHECK(take_string(incomplete->get_texture()).is_empty());
	incomplete->set_material_shader("missing-shader.tres");
	CHECK(incomplete->get_material_shader() == nullptr);
	incomplete->set_color(Color(1, 0, 0));
	CHECK(incomplete->get_color() == Color());
	incomplete->set_material_params("amount", 1);
	CHECK(incomplete->get_material_params("amount") == 0);
	incomplete->set_material_params_vec4("vector", Vector4(1, 2, 3, 4));
	incomplete->set_uv_remap(Vector4(0, 0, 1, 1));
	CHECK(incomplete->get_material_params_vec4("vector") == Vector4());
	incomplete->set_material_params_color("color", Color(1, 1, 1));
	CHECK(incomplete->get_material_params_color("color") == Color());
	ERR_PRINT_ON
	memdelete(incomplete);

	ui->on_reset(0);
	for (const String &path : { sprite_scene_path, button_scene_path, invalid_scene_path, empty_scene_path }) {
		CHECK(DirAccess::remove_absolute(path) == OK);
	}
}

TEST_CASE("[SceneTree][SPX] Input and extension managers expose runtime state") {
	EngineFixture fixture;
	SpxEngine *engine = SpxEngine::get_singleton();
	SpxInputMgr *input = engine->get_input();
	CHECK(input->get_global_mouse_pos().is_finite());
	CHECK_FALSE(input->get_mouse_state((GdInt)MouseButton::LEFT));
	CHECK_FALSE(input->get_key((GdInt)Key::F24));
	CHECK(input->get_key_state((GdInt)Key::F24) == 0);

	InputMap *input_map = InputMap::get_singleton();
	const StringName negative("spx_test_negative");
	const StringName positive("spx_test_positive");
	input_map->add_action(negative);
	input_map->add_action(positive);
	const GdInt negative_id = input->register_action("spx_test_negative");
	const GdInt positive_id = input->register_action("spx_test_positive");
	CHECK(input->register_action("spx_test_positive") == positive_id);
	Input::get_singleton()->action_press(positive, 0.75f);
	CHECK(input->is_action_pressed("spx_test_positive"));
	CHECK(input->is_action_pressed_id(positive_id));
	CHECK(Math::is_equal_approx(input->get_axis("spx_test_negative", "spx_test_positive"), 0.75f));
	CHECK(Math::is_equal_approx(input->get_axis_id(negative_id, positive_id), 0.75f));
	CHECK(input->get_axis_id(-1, positive_id) == 0.0f);
	CHECK_FALSE(input->is_action_pressed_id(999));
	CHECK_FALSE(input->is_action_just_pressed_id(999));
	CHECK_FALSE(input->is_action_just_released_id(999));
	input->is_action_just_pressed("spx_test_positive");
	input->is_action_just_released("spx_test_positive");
	float snapshot[3] = {};
	input->write_snapshot(nullptr);
	input->write_snapshot(snapshot);
	CHECK(Math::is_finite(snapshot[0]));
	CHECK(Math::is_finite(snapshot[1]));
	Input::get_singleton()->action_release(positive);
	input_map->erase_action(negative);
	input_map->erase_action(positive);

	SpxExtMgr::set_layer_sorter_mode((GdInt)LayerSortMode::VERTICAL);
	CHECK(SpxLayerSorter::instance().is_enabled());
	SpxExtMgr::pause();
	SpxExtMgr::is_paused();
	SpxExtMgr::next_frame();
	engine->on_update(0.016f);
	SpxExtMgr::resume();
	CHECK_FALSE(SpxExtMgr::is_paused());
	SpxExtMgr::on_runtime_panic("without callback");
	SpxExtMgr::set_layer_sorter_mode((GdInt)LayerSortMode::NONE);

	input->on_reset(0);
	input->on_start();
	input->on_start();
}

TEST_CASE("[SceneTree][SPX] Scene physics camera debug navigation and pen managers interoperate") {
	EngineFixture fixture;
	SpxEngine *engine = SpxEngine::get_singleton();
	SpxSpriteMgr *sprites = engine->get_sprite();
	GdObj sprite = sprites->create_bare_sprite(Vector2());
	REQUIRE(sprite != NULL_OBJECT_ID);

	SpxCameraMgr *camera = engine->get_camera();
	camera->set_camera_position(Vector2(10, 20));
	CHECK(camera->get_camera_position() == Vector2(10, 20));
	camera->set_camera_zoom(Vector2(1.5f, 2));
	CHECK(camera->get_camera_zoom() == Vector2(1.5f, 2));
	camera->set_camera_limit(SIDE_LEFT, -100);
	camera->set_camera_limit(SIDE_TOP, -80);
	camera->set_camera_limit(SIDE_RIGHT, 100);
	camera->set_camera_limit(SIDE_BOTTOM, 80);
	CHECK(camera->get_stage_limits_rect() == Rect2(-100, -80, 200, 160));
	camera->set_camera_smoothing(true);
	const Rect2 viewport_rect = camera->get_viewport_rect();
	const Rect2 global_camera_rect = camera->get_global_camera_rect();
	const Vector2 mouse_position = camera->get_global_mouse_position();
	CHECK(viewport_rect.size.x >= 0);
	CHECK(global_camera_rect.size.x >= 0);
	CHECK(Math::is_finite(mouse_position.x));
	camera->set_stretch_clear_color();

	SpxPhysicsMgr *physics = engine->get_physics();
	physics->set_collision_system_type(true);
	physics->set_global_gravity(9.8f);
	CHECK(Math::is_equal_approx(physics->get_global_gravity(), 9.8f));
	physics->set_global_friction(0.4f);
	CHECK(Math::is_equal_approx(physics->get_global_friction(), 0.4f));
	physics->set_global_air_drag(0.1f);
	CHECK(Math::is_equal_approx(physics->get_global_air_drag(), 0.1f));
	physics->check_touched_camera_boundaries(sprite);
	physics->check_touched_camera_boundary(sprite, BOUND_LEFT);
	physics->check_nearest_touched_camera_boundary(sprite);
	physics->check_touched_stage_boundaries(sprite);
	physics->check_touched_stage_boundary(sprite, BOUND_TOP);
	physics->check_nearest_touched_stage_boundary(sprite);
	physics->raycast(Vector2(-10, 0), Vector2(10, 0), UINT32_MAX);
	physics->check_collision(Vector2(-10, 0), Vector2(10, 0), UINT32_MAX, true, true);
	GdArray details = physics->raycast_with_details(Vector2(-10, 0), Vector2(10, 0), nullptr, UINT32_MAX, true, true);
	GdArray rect_hits = physics->check_collision_rect(Vector2(), Vector2(20, 20), UINT32_MAX);
	GdArray circle_hits = physics->check_collision_circle(Vector2(), 10, UINT32_MAX);
	SpxAbi::free_array(details);
	SpxAbi::free_array(rect_hits);
	SpxAbi::free_array(circle_hits);

	SpxSceneMgr *scene = engine->get_scene();
	const String scene_texture_path = TestUtils::get_data_path("images/icon.png");
	CharString scene_texture_utf8 = scene_texture_path.utf8();
	GdObj render_id = scene->create_render_sprite(scene_texture_utf8.get_data(), Vector2(1, 2), 15, Vector2(1, 1), 2, Vector2());
	REQUIRE(render_id != NULL_OBJECT_ID);
	FloatArray circle{ 6 };
	FloatArray rect{ 10, 20 };
	FloatArray capsule{ 8, 24 };
	FloatArray polygon{ 0, 0, 16, 0, 0, 16 };
	Vector<GdObj> static_ids;
	static_ids.push_back(scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), (GdInt)ColliderType::NONE, Vector2(), nullptr));
	static_ids.push_back(scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), (GdInt)ColliderType::CIRCLE, Vector2(), circle.ptr()));
	static_ids.push_back(scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), (GdInt)ColliderType::RECT, Vector2(), rect.ptr()));
	static_ids.push_back(scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), (GdInt)ColliderType::CAPSULE, Vector2(), capsule.ptr()));
	static_ids.push_back(scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), (GdInt)ColliderType::POLYGON, Vector2(), polygon.ptr()));
	for (GdObj id : static_ids) {
		CHECK(id != NULL_OBJECT_ID);
	}
	GdObj invalid_collider = scene->create_static_sprite(scene_texture_utf8.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), 999, Vector2(), nullptr);
	CHECK(invalid_collider != NULL_OBJECT_ID);
	scene->create_pure_sprite(scene_texture_utf8.get_data(), Vector2(2, 3), 4);
	Vector<ISortableSprite *> sortables;
	scene->collect_sortable_sprites(sortables);
	CHECK(sortables.size() == 8);
	CHECK(scene->get_scene_bounds(fixture.root).has_area());
	CHECK(scene->get_tilemap_bounds(nullptr) == Rect2());
	scene->export_scene_as_png(nullptr);
	Node *empty_scene = memnew(Node);
	scene->export_scene_as_png(empty_scene);
	memdelete(empty_scene);
		SceneTree *tree = SceneTree::get_singleton();
		Node *previous_scene = tree->get_current_scene();
		tree->set_current_scene(fixture.root);
		scene->export_scene_as_png(fixture.root);
		Node2D *export_root = memnew(Node2D);
		TileMapLayer *export_layer = memnew(TileMapLayer);
		Ref<PlaceholderTexture2D> export_texture;
		export_texture.instantiate();
		export_texture->set_size(Vector2(16, 16));
		Ref<TileSetAtlasSource> export_source;
		export_source.instantiate();
		export_source->set_texture(export_texture);
		export_source->set_texture_region_size(Vector2i(16, 16));
		export_source->create_tile(Vector2i());
		Ref<TileSet> export_tile_set;
		export_tile_set.instantiate();
		export_tile_set->set_tile_size(Vector2i(16, 16));
		const int export_source_id = export_tile_set->add_source(export_source);
		export_layer->set_tile_set(export_tile_set);
		export_layer->set_cell(Vector2i(), export_source_id, Vector2i());
		export_root->add_child(export_layer);
		fixture.root->add_child(export_root);
		scene->export_scene_as_png(export_root);
		SpxRenderSprite *uninstantiable = memnew(SpxRenderSprite);
		export_root->add_child(uninstantiable);
		ERR_PRINT_OFF
		scene->export_scene_as_png(export_root);
		ERR_PRINT_ON
		export_root->remove_child(uninstantiable);
		memdelete(uninstantiable);
		tree->set_current_scene(previous_scene);
	scene->destroy_pure_sprite(render_id);
	scene->destroy_pure_sprite(999999);

	SpxDebugMgr *debug = engine->get_debug();
	debug->debug_draw_circle(Vector2(1, 2), 10, Color(1, 0, 0));
	debug->debug_draw_rect(Vector2(3, 4), Vector2(20, 30), Color(0, 1, 0));
	debug->debug_draw_line(Vector2(-1, -2), Vector2(5, 6), Color(0, 0, 1));
	debug->on_update(0.1f);

	SpxNavigationMgr *navigation = engine->get_navigation();
	navigation->setup_path_finder_with_size(Vector2(12, 10), Vector2(8, 8), false, false);
	sprites->set_physics_mode(sprite, SpxSprite::STATIC);
	sprites->set_collider_rect(sprite, Vector2(), Vector2(24, 24));
	navigation->set_obstacle(sprite, true);
	GdArray path = navigation->find_path(Vector2(), Vector2(40, 24), false);
	SpxAbi::free_array(path);
	navigation->set_obstacle(sprite, false);
	navigation->setup_path_finder(true);
	navigation->on_reset(0);
	path = navigation->find_path(Vector2(), Vector2(16, 16), true);
	SpxAbi::free_array(path);

	SpxPenMgr *pens = engine->get_pen();
	pens->set_canvas_size(128, 96);
	GdObj pen = pens->create_pen();
	REQUIRE(pen != NULL_OBJECT_ID);
	pens->move_pen_to(pen, Vector2(1, 2));
	pens->pen_down(pen, false);
	pens->move_pen_to(pen, Vector2(12, 8));
	pens->set_pen_color_to(pen, Color(0.25f, 0.5f, 0.75f, 0.8f));
	pens->change_pen_by(pen, 0, -0.2f);
	pens->set_pen_to(pen, 1, 0.6f);
	pens->change_pen_size_by(pen, 2);
	pens->set_pen_size_to(pen, 4);
	Vector<float> pen_commands;
	pen_commands.push_back(5.0f);
	append_pen_command(pen_commands, 1, pen, 30, 40);
	append_pen_command(pen_commands, 2, pen, 0);
	append_pen_command(pen_commands, 4, pen, 0.1f, 0.2f, 0.3f, 0.4f);
	append_pen_command(pen_commands, 5, pen, 7);
	append_pen_command(pen_commands, 3, pen);
	pens->batch_update_commands(pen_commands.ptr(), pen_commands.size());
	REQUIRE(pens->get_object(pen) != nullptr);
	CHECK(pens->get_object(pen)->get_id() == pen);
	ERR_PRINT_OFF
	const float invalid_count[] = { -1.0f };
	const float invalid_length[] = { 1.0f };
	Vector<float> invalid_command;
	invalid_command.push_back(1.0f);
	append_pen_command(invalid_command, 99, pen);
	Vector<float> missing_pen;
	missing_pen.push_back(1.0f);
	append_pen_command(missing_pen, 1, 999999, 1, 2);
	pens->batch_update_commands(nullptr, 0);
	pens->batch_update_commands(invalid_count, 1);
	pens->batch_update_commands(invalid_length, 1);
	pens->batch_update_commands(invalid_command.ptr(), invalid_command.size());
	pens->batch_update_commands(missing_pen.ptr(), missing_pen.size());
	ERR_PRINT_ON
	pens->pen_stamp_sprite(sprite);
	pens->pen_stamp(pen);
	pens->pen_up(pen);
	pens->flush_all();
	pens->destroy_all_pens();
	pens->destroy_pen(pen);
	scene->clear_pure_sprites();
	scene->destroy_all_sprites();
}

TEST_CASE("[SceneTree][SPX] Platform manager preserves process and persistent settings") {
	EngineFixture fixture;
	SpxPlatformMgr *platform = SpxEngine::get_singleton()->get_platform();
	platform->set_stretch(true, 480, 360);
	platform->set_stretch(false, 0, 0);
	platform->set_debug_mode(true);
	CHECK(platform->is_debug_mode());
	CHECK(platform->is_main_thread());
	platform->set_time_scale(0.75f);
	CHECK(Math::is_equal_approx(platform->get_time_scale(), 0.75f));
	platform->set_max_fps(120);
	CHECK(platform->get_max_fps() == 120);
	platform->set_persistent_data_dir("/tmp/spx-data");
	CHECK(platform->_get_persistent_data_dir() == "/tmp/spx-data");
	CHECK(platform->is_in_persistent_data_dir("/tmp/spx-data/save.json"));
	CHECK_FALSE(platform->is_in_persistent_data_dir("/tmp/other/save.json"));
	CHECK(take_string(platform->get_persistent_data_dir()) == "/tmp/spx-data");
	const Vector2 window_position = platform->get_window_position();
	const Vector2 window_size = platform->get_window_size();
	CHECK(Math::is_finite(window_position.x));
	CHECK(window_size.x > 0);
	CHECK(take_string(platform->get_window_title()).is_empty());
	platform->set_window_position(window_position);
	platform->set_window_size(320, 240, true);
	CHECK(platform->get_window_size().x > 0);
	platform->set_window_size((GdInt)window_size.x, (GdInt)window_size.y, false);
	platform->set_window_title("SPX test");
	platform->set_window_fullscreen(false);
	CHECK_FALSE(platform->is_window_fullscreen());

	platform->set_time_scale(1.0f);
	platform->set_max_fps(0);
	platform->set_debug_mode(false);
}

TEST_CASE("[SceneTree][SPX] Tile map managers place edit and parse native tiles") {
	EngineFixture fixture;
	SpxEngine *engine = SpxEngine::get_singleton();
	SpxTilemapMgr *tiles = engine->get_tilemap();
	const String texture_path = TestUtils::get_data_path("images/icon.png");
	CharString texture_utf8 = texture_path.utf8();
	FloatArray positions{ 0, 0, 16, 0, 0, 16 };
	FloatArray collision{ -8, -8, 8, -8, 8, 8, -8, 8 };

	CHECK(take_string(tiles->get_tile(Vector2())).is_empty());
	tiles->open_draw_tiles_with_size(16);
	tiles->open_draw_tiles();
	tiles->set_layer_index(2);
	tiles->set_layer_offset(2, Vector2(4, 6));
	CHECK(tiles->get_layer_offset(2) == Vector2(4, 6));
	tiles->set_tile(texture_utf8.get_data(), true);
	tiles->place_tile(Vector2(), texture_utf8.get_data());
	CHECK(take_string(tiles->get_tile(Vector2())) == texture_path);
	tiles->place_tile_with_layer(Vector2(16, 16), texture_utf8.get_data(), 3);
	CHECK(take_string(tiles->get_tile_with_layer(Vector2(16, 16), 3)) == texture_path);
	tiles->place_tiles(positions.ptr(), texture_utf8.get_data());
	tiles->place_tiles_with_layer(positions.ptr(), texture_utf8.get_data(), 4);
	tiles->set_tile_with_collision_info(texture_utf8.get_data(), collision.ptr());
	tiles->erase_tile(Vector2());
	tiles->erase_tile_with_layer(Vector2(16, 16), 3);
	tiles->erase_tile_with_layer(Vector2(), -1);

	SpxDrawTiles *draw_tiles = nullptr;
	for (int i = 0; i < fixture.root->get_child_count(); i++) {
		draw_tiles = Object::cast_to<SpxDrawTiles>(fixture.root->get_child(i));
		if (draw_tiles != nullptr) {
			break;
		}
	}
	REQUIRE(draw_tiles != nullptr);
	draw_tiles->enter_editor_mode();
	draw_tiles->set_layer_index(5);
	draw_tiles->set_tile_texture_spx(texture_utf8.get_data(), &SpxDrawTiles::no_collision_array);
	draw_tiles->place_or_erase_tile(Vector2(), false);
	draw_tiles->undo();
	draw_tiles->redo();
	draw_tiles->place_or_erase_tile(Vector2(), true);
	draw_tiles->undo();
	draw_tiles->redo();

	DrawTilesProbe *probe = memnew(DrawTilesProbe);
	fixture.root->add_child(probe);
	probe->set_tile_size(8);
	probe->set_layer_index(0);
	Ref<Image> probe_image = Image::create_empty(8, 8, false, Image::FORMAT_RGBA8);
	probe_image->fill(Color(0.8f, 0.2f, 0.4f));
	Ref<ImageTexture> probe_texture = ImageTexture::create_from_image(probe_image);
	probe->set_texture(probe_texture, true);
	TileMapLayer *probe_layer = Object::cast_to<TileMapLayer>(probe->get_node_or_null(NodePath("spx_draw_tiles_layer_0")));
	REQUIRE(probe_layer != nullptr);
	probe->place_tile(probe_layer, Vector2i());
	CHECK(engine->get_scene()->get_tilemap_bounds(probe_layer).has_area());
	DrawContext draw_context;
	draw_context.set_layer(probe_layer)
			.set_cell_size(Vector2i(8, 8))
			.set_grid_color(Color(1, 1, 0, 0.5f))
			.set_axis_width(2)
			.set_guide_rect_radius(1)
			.set_layer_pos(Vector2())
			.set_mouse_pos(Vector2())
			.set_current_texture(probe_texture)
			.set_flipped_axis(false)
			.set_axis_dragging(false);
	LayerRenderer renderer;
	ERR_PRINT_OFF
	renderer.draw(probe, draw_context);
	draw_context.set_axis_dragging(true);
	renderer.draw(probe, draw_context);
	ERR_PRINT_ON
	probe->enter_editor_mode();
	Ref<InputEventMouseButton> left;
	left.instantiate();
	left->set_button_index(MouseButton::LEFT);
	left->set_pressed(true);
	probe->send_input(left);
	Ref<InputEventMouseMotion> motion;
	motion.instantiate();
	probe->send_input(motion);
	left->set_pressed(false);
	probe->send_input(left);
	probe->set_layer_offset_spx(0, Vector2(1000, 1000));
	left->set_pressed(true);
	probe->send_input(left);
	probe->send_input(motion);
	left->set_pressed(false);
	probe->send_input(left);
	Ref<InputEventKey> ordinary_key;
	ordinary_key.instantiate();
	ordinary_key->set_keycode(Key::A);
	ordinary_key->set_pressed(true);
	probe->send_input(ordinary_key);
	Ref<InputEventKey> export_key;
	export_key.instantiate();
	export_key->set_keycode(Key::E);
	export_key->set_pressed(false);
	probe->send_input(export_key);
	export_key->set_pressed(true);
	probe->send_input(export_key);
	DrawTilesProbe *shortcut_probe = memnew(DrawTilesProbe);
	fixture.root->add_child(shortcut_probe);
	export_key->set_ctrl_pressed(true);
	ERR_PRINT_OFF
	shortcut_probe->send_input(export_key);
	ERR_PRINT_ON
	fixture.root->remove_child(shortcut_probe);
	memdelete(shortcut_probe);
	probe->exit_editor_mode();
	probe->undo();
	probe->redo();
	probe->clear_all_layers();
	draw_tiles->clear_all_layers();
	draw_tiles->undo();
	draw_tiles->redo();

	tiles->close_draw_tiles();
	tiles->close_draw_tiles();
	tiles->open_draw_tiles();
	tiles->exit_tilemap_editor_mode();

	const String json_path = TestUtils::get_temp_path("spx_parsed_tilemap.json");
	const String tile_path = json_path.get_base_dir().path_join("spx_parsed_tile.png");
	Ref<Image> tile_image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
	tile_image->fill(Color(0.2f, 0.7f, 0.4f));
	REQUIRE(tile_image->save_png(tile_path) == OK);
	const String json = R"({
		"version": 1,
		"name": "parsed-map",
		"node_offset": [2, 3],
		"tileset": {
			"tile_size": [16, 16],
			"tile_shape": "square",
			"physics_layers": [{"collision_layer": 2, "collision_mask": 3}],
			"sources": [{
				"id": 1,
				"type": "atlas",
				"texture": "spx_parsed_tile.png",
				"texture_region_size": [16, 16],
				"tiles": [{
					"atlas_coords": [0, 0],
					"size_in_atlas": [1, 1],
					"physics": [{"layer": 0, "polygons": [[0, 0, 16, 0, 0, 16]]}]
				}]
			}, {
				"id": 2,
				"type": "collection"
			}, {
				"id": 3,
				"type": "atlas",
				"texture": ""
			}, {
				"id": 4,
				"type": "atlas",
				"texture": "missing.png",
				"texture_region_size": [16, 16]
			}]
		},
		"layers": [{"name": "ground", "z_index": 2, "offset": [4, 5], "enabled": true, "tile_map_data": "AA=="}]
	})";
	Ref<FileAccess> file = FileAccess::open(json_path, FileAccess::WRITE);
	REQUIRE(file.is_valid());
	file->store_string(json);
	file.unref();

	SpxTilemapparserMgr *parser = engine->get_tilemapparser();
	CharString json_utf8 = json_path.utf8();
	parser->load_tilemap(json_utf8.get_data());
	CHECK(parser->has_tilemap("parsed-map"));
	CHECK(parser->get_tilemap_layer_count("parsed-map") == 1);
	parser->load_tilemap(json_utf8.get_data());
	parser->unload_tilemap("missing-map");
	parser->unload_tilemap("parsed-map");
	CHECK_FALSE(parser->has_tilemap("parsed-map"));
	parser->destroy_all_tilemaps();
	CHECK(DirAccess::remove_absolute(json_path) == OK);
	CHECK(DirAccess::remove_absolute(tile_path) == OK);
}

} // namespace TestSpxManagerApis

#endif // TEST_SPX_MANAGER_APIS_H
