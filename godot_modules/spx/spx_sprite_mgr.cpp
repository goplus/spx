/**************************************************************************/
/*  spx_sprite_mgr.cpp                                                    */
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

#include "spx_sprite_mgr.h"

#include "core/math/math_funcs.h"
#include "core/os/thread.h"
#include "core/templates/rb_map.h"
#include "core/typedefs.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/physics/physics_body_2d.h"
#include "scene/main/node.h"
#include "scene/main/window.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/packed_scene.h"
#include "servers/rendering_server.h"

#include "spx_coordinate.h"
#include "spx_engine.h"
#include "spx_ext_mgr.h"
#include "spx_layer_sorter.h"
#include "spx_object_access.h"
#include "spx_pen_mgr.h"
#include "spx_physics_mgr.h"
#include "spx_pixel_query.h"
#include "spx_res_mgr.h"
#include "spx_scene_mgr.h"
#include "spx_sprite.h"

#include <algorithm>
#include <vector>

#define DEFAULT_COLLISION_ALPHA_THRESHOLD 0.05


StringName SpxSpriteMgr::default_texture_anim;

// Checked main-thread lookup with each API's existing default return value.
#define SPX_REQUIRE_SPRITE_VOID() \
	SPX_SPRITE_LOOKUP_VOID(obj, __func__)

#define SPX_REQUIRE_SPRITE_RETURN(VALUE) \
	SPX_SPRITE_LOOKUP_RETURN(obj, __func__, VALUE)

#define SPX_REQUIRE_TARGET_SPRITE_VOID(TARGET) \
	SPX_TARGET_SPRITE_LOOKUP_VOID(TARGET, __func__)

#define SPX_REQUIRE_TARGET_SPRITE_RETURN(TARGET, VALUE) \
	SPX_TARGET_SPRITE_LOOKUP_RETURN(TARGET, __func__, VALUE)

static _FORCE_INLINE_ GdFloat color_rgb_distance_squared(const GdColor &p_a, const GdColor &p_b) {
	const GdFloat dr = p_a.r - p_b.r;
	const GdFloat dg = p_a.g - p_b.g;
	const GdFloat db = p_a.b - p_b.b;
	return dr * dr + dg * dg + db * db;
}

namespace {

using SpxPixelQuery::Layer;
using SpxPixelQuery::Snapshot;

bool capture_sprite_pixels(SpxSprite *p_sprite, Snapshot &r_snapshot, bool p_require_visible, bool p_apply_collision_alpha) {
	if (!p_sprite || (p_require_visible && !p_sprite->is_visible_in_tree())) {
		return false;
	}
	return SpxPixelQuery::capture(p_sprite->get_anim2d(), p_apply_collision_alpha, r_snapshot);
}

bool check_pixel_collision(SpxSprite *p_a, SpxSprite *p_b, GdFloat p_alpha_threshold, int p_step, SpxPixelQuery::ImageCache *p_cache = nullptr) {
	Snapshot query_a;
	Snapshot query_b;
	if (!capture_sprite_pixels(p_a, query_a, true, false) ||
			!capture_sprite_pixels(p_b, query_b, true, false)) {
		return false;
	}

	const Rect2i overlap_rect = SpxPixelQuery::overlap(query_a.bounds, query_b.bounds);
	if (!overlap_rect.has_area() || !SpxPixelQuery::load_image(query_a, p_cache) || !SpxPixelQuery::load_image(query_b, p_cache)) {
		return false;
	}
	return SpxPixelQuery::any_pixel_center(overlap_rect, p_step, [&](const Vector2 &sample_pos) {
		Color color_a;
		if (!SpxPixelQuery::sample(query_a, sample_pos, color_a) || color_a.a <= p_alpha_threshold) {
			return false;
		}
		Color color_b;
		return SpxPixelQuery::sample(query_b, sample_pos, color_b) && color_b.a > p_alpha_threshold;
	});
}

} // namespace

void SpxSpriteMgr::on_awake() {
	default_texture_anim = "default";

	// Initialize pixel collision sampling step with default value of 2 (good balance between performance and accuracy)
	pixel_collision_sampling_step = 2;

	dont_destroy_root = memnew(Node2D);
	dont_destroy_root->set_name("dont_destroy_root");
	get_spx_root()->add_child(dont_destroy_root);

	sprite_root = memnew(Node2D);
	sprite_root->set_name("sprite_root");
	get_spx_root()->add_child(sprite_root);
}

void SpxSpriteMgr::on_start() {
	auto nodes = get_root()->find_children("*", "SpxSprite", true, false);
	for (int i = 0; i < nodes.size(); i++) {
		auto sprite = Object::cast_to<SpxSprite>(nodes[i]);
		if (sprite != nullptr) {
			sprite->set_gid(get_unique_id());
			sprite->on_start();
			_register_sprite(sprite);
			auto value = sprite->get_spx_type_name();
			CharString data = value.utf8();
			SPX_CALLBACK->func_on_scene_sprite_instantiated(sprite->get_gid(), data.get_data());
		}
	}
}

void SpxSpriteMgr::on_destroy() {
	id_objects.clear();
	bounding_collision_pairs.clear();
	pixel_collision_pairs.clear();
	if (sprite_root != nullptr) {
		sprite_root->queue_free();
		sprite_root = nullptr;
	}
	if (dont_destroy_root != nullptr) {
		dont_destroy_root->queue_free();
		dont_destroy_root = nullptr;
	}
}

void SpxSpriteMgr::on_update(float delta) {
	_check_pixel_collision_events();
	auto &sorter = SpxLayerSorter::instance();
	if (!sorter.is_enabled()) {
		return;
	}

	Vector<ISortableSprite *> all_sortables;

	for (auto &pair : id_objects) {
		if (pair.value && !pair.value->is_queued_for_deletion()) {
			all_sortables.push_back(pair.value);
		}
	}

	sceneMgr->collect_sortable_sprites(all_sortables);
	sorter.update(all_sortables);
}

void SpxSpriteMgr::on_reset(int reset_code) {
	default_texture_anim = "default";
	dont_destroy_root->queue_free();
	dont_destroy_root = memnew(Node2D);
	dont_destroy_root->set_name("dont_destroy_root");
	get_spx_root()->add_child(dont_destroy_root);

	destroy_all_sprites();
}

void SpxSpriteMgr::collect_sortable_sprites(Vector<ISortableSprite *> &out) {
	for (auto &pair : id_objects) {
		if (pair.value && !pair.value->is_queued_for_deletion()) {
			out.push_back(pair.value);
		}
	}
}

SpxSprite *SpxSpriteMgr::get_sprite(GdObj obj) {
	ERR_FAIL_COND_V_MSG(!Thread::is_main_thread(), nullptr, "SPX sprites may only be accessed on the engine main thread.");

	// Use single-lookup pattern: find() returns Element*, avoiding double hash lookup
	auto element = id_objects.find(obj);
	if (element == nullptr) {
		return nullptr;
	}

	SpxSprite *sprite = element->value();
	if (sprite == nullptr || sprite->is_queued_for_deletion()) {
		return nullptr;
	}
	return sprite;
}

void SpxSpriteMgr::_register_sprite(SpxSprite *p_sprite) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "SPX sprites may only be registered on the engine main thread.");
	ERR_FAIL_NULL(p_sprite);
	id_objects[p_sprite->get_gid()] = p_sprite;
}

void SpxSpriteMgr::on_sprite_destroy(SpxSprite *sprite) {
	_remove_collision_pairs_for_sprite(sprite->get_gid());
	if (id_objects.erase(sprite->get_gid())) {
		SPX_CALLBACK->func_on_sprite_destroyed(sprite->get_gid());
	}
}

void SpxSpriteMgr::set_dont_destroy_on_load(GdObj obj) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->get_parent()->remove_child(sprite);
	dont_destroy_root->add_child(sprite);
}

void SpxSpriteMgr::set_process(GdObj obj, GdBool is_on) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_process(is_on);
}

void SpxSpriteMgr::set_physic_process(GdObj obj, GdBool is_on) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_physics_process(is_on);
}
void SpxSpriteMgr::set_type_name(GdObj obj, GdString type_name) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_type_name(type_name);
}

void SpxSpriteMgr::set_child_position(GdObj obj, GdString path, GdVec2 pos) {
	SPX_REQUIRE_SPRITE_VOID()
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		child->set_position(spx_to_godot_vec2(pos));
	}
}

GdVec2 SpxSpriteMgr::get_child_position(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		auto pos = child->get_position();
		return godot_to_spx_vec2(pos);
	}
	return GdVec2();
}

void SpxSpriteMgr::set_child_rotation(GdObj obj, GdString path, GdFloat rot) {
	SPX_REQUIRE_SPRITE_VOID()
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		child->set_rotation(rot);
	}
}

GdFloat SpxSpriteMgr::get_child_rotation(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		return child->get_rotation();
	}
	return 0;
}

void SpxSpriteMgr::set_child_scale(GdObj obj, GdString path, GdVec2 scale) {
	SPX_REQUIRE_SPRITE_VOID()
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		child->set_scale(scale);
	}
}

GdVec2 SpxSpriteMgr::get_child_scale(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	auto child = (Node2D *)sprite->get_node(SpxStr(path));
	if (child != nullptr) {
		return child->get_scale();
	}
	return GdVec2();
}

GdBool SpxSpriteMgr::check_collision(GdObj obj, GdObj target, GdBool is_src_trigger, GdBool is_dst_trigger) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	SPX_REQUIRE_TARGET_SPRITE_RETURN(target, false)
	return sprite->check_collision(sprite_target, is_src_trigger, is_dst_trigger);
}

GdBool SpxSpriteMgr::check_collision_with_point(GdObj obj, GdVec2 point, GdBool is_click_query) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	point = spx_to_godot_vec2(point);

	if (is_click_query && !sprite->is_visible_in_tree()) {
		return false;
	}

	Snapshot query;
	// Scratch keeps point sensing and click picking distinct:
	// - click queries respect visibility and ghost alpha
	// - sensing queries ignore both and use the sprite silhouette directly
	if (capture_sprite_pixels(sprite, query, is_click_query, is_click_query) && SpxPixelQuery::load_image(query)) {
		Color color;
		return SpxPixelQuery::sample(query, point, color) && color.a > 0.0f;
	}

	// Keep point-query fallbacks aligned with the trigger footprint used by SPX's
	// mouse/tap detection, even when the pixel path is unavailable.
	return sprite->check_collision_with_point(point, true);
}

void SpxSpriteMgr::set_debug_collision_visible(GdObj obj, GdBool visible) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_debug_collision_visible(visible);
}

GdBool SpxSpriteMgr::is_debug_collision_visible(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_debug_collision_visible();
}

GdInt SpxSpriteMgr::create_backdrop(GdString path) {
	return _create_sprite(path, GdVec2(), true);
}

GdInt SpxSpriteMgr::create_sprite(GdString path, GdVec2 pos) {
	return _create_sprite(path, pos, false);
}

GdObj SpxSpriteMgr::create_bare_sprite(GdVec2 pos) {
	return _create_sprite("", pos, false);
}

// sprite
GdInt SpxSpriteMgr::_create_sprite(GdString path, GdVec2 pos, GdBool is_backdrop) {
	ERR_FAIL_COND_V_MSG(!Thread::is_main_thread(), NULL_OBJECT_ID, "SPX sprites may only be created on the engine main thread.");

	const String path_str = SpxStr(path);
	SpxSprite *sprite = nullptr;
	if (path_str == "") {
		sprite = memnew(SpxSprite);
		sprite->set_position(spx_to_godot_vec2(pos));
		Node2D *render_root = memnew(Node2D);
		render_root->set_name("RenderRoot");
		sprite->add_child(render_root);
		AnimatedSprite2D *animated_sprite = memnew(AnimatedSprite2D);
		animated_sprite->set_name("Anim2D");
		render_root->add_child(animated_sprite);
		Area2D *area = memnew(Area2D);
		area->set_name("Area2D");
		sprite->add_child(area);
		CollisionShape2D *area_collision_shape = memnew(CollisionShape2D);
		area_collision_shape->set_name("Trigger2D");
		const Ref<CircleShape2D> area_shape = memnew(CircleShape2D);
		area_shape->set_radius(10.0f);
		area_collision_shape->set_shape(area_shape);
		area->add_child(area_collision_shape);
		CollisionShape2D *body_collision_shape = memnew(CollisionShape2D);
		body_collision_shape->set_name("Collider2D");
		const Ref<CircleShape2D> body_shape = memnew(CircleShape2D);
		body_shape->set_radius(10.0f);
		body_collision_shape->set_shape(body_shape);
		sprite->add_child(body_collision_shape);
	} else {
		// load from path
		Ref<PackedScene> scene = ResourceLoader::load(path_str);
		if (scene.is_null()) {
			print_error("Failed to load sprite scene " + path_str);
			return NULL_OBJECT_ID;
		} else {
			Node *node = scene->instantiate();
			if (node == nullptr) {
				print_error("Failed to instantiate sprite scene " + path_str);
				return NULL_OBJECT_ID;
			}
			sprite = dynamic_cast<SpxSprite *>(node);
			if (sprite == nullptr) {
				print_error("Failed to load sprite scene, type invalid " + path_str);
				memdelete(node);
				return NULL_OBJECT_ID;
			}
		}
	}

	sprite->set_backdrop(is_backdrop);
	sprite->set_gid(get_unique_id());
	sprite_root->add_child(sprite);
	sprite->on_start();
	_register_sprite(sprite);
	SPX_CALLBACK->func_on_sprite_ready(sprite->get_gid());
	return sprite->get_gid();
}

void SpxSpriteMgr::destroy_all_sprites() {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "SPX sprites may only be destroyed on the engine main thread.");

	sprite_root->queue_free();
	sprite_root = memnew(Node2D);
	sprite_root->set_name("sprite_root");
	get_spx_root()->add_child(sprite_root);

	id_objects.clear();
	bounding_collision_pairs.clear();
	pixel_collision_pairs.clear();
}

GdInt SpxSpriteMgr::clone_sprite(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(NULL_OBJECT_ID)
	SpxSprite *cloned = dynamic_cast<SpxSprite *>(sprite->duplicate());
	if (unlikely(!cloned)) {
		ERR_PRINT("Failed to clone sprite with GID: " + itos(obj));
		return NULL_OBJECT_ID;
	}
	cloned->set_gid(get_unique_id());
	sprite_root->add_child(cloned);
	_register_sprite(cloned);
	cloned->on_start();
	SPX_CALLBACK->func_on_sprite_ready(cloned->get_gid());
	return cloned->get_gid();
}

GdBool SpxSpriteMgr::destroy_sprite(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	sprite->set_block_signals(true);
	sprite->queue_free();
	return true;
}

GdBool SpxSpriteMgr::is_sprite_alive(GdObj obj) {
	return get_sprite(obj) != nullptr;
}

void SpxSpriteMgr::set_position(GdObj obj, GdVec2 pos) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_position(spx_to_godot_vec2(pos));
}

void SpxSpriteMgr::set_transform(GdObj obj, GdVec2 pos, GdFloat rot, GdVec2 scale, GdBool visible, GdVec2 pivot) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_position(spx_to_godot_vec2(pos));
	sprite->set_rotation(rot);
	sprite->set_scale(scale);
	sprite->set_visible(visible);
	sprite->on_set_visible(visible);
	sprite->set_render_offset(spx_to_godot_vec2(pivot));
}

void SpxSpriteMgr::set_rotation(GdObj obj, GdFloat rot) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_rotation(rot);
}

void SpxSpriteMgr::set_scale(GdObj obj, GdVec2 scale) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_scale(scale);
}

GdVec2 SpxSpriteMgr::get_position(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	auto pos = sprite->get_position();
	return godot_to_spx_vec2(pos);
}

GdFloat SpxSpriteMgr::get_rotation(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_rotation();
}

GdVec2 SpxSpriteMgr::get_scale(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return sprite->get_scale();
}

void SpxSpriteMgr::set_render_scale(GdObj obj, GdVec2 scale) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_render_scale(scale);
}
GdVec2 SpxSpriteMgr::get_render_scale(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return sprite->get_render_scale();
}

void SpxSpriteMgr::set_color(GdObj obj, GdColor color) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_color(color);
}

GdColor SpxSpriteMgr::get_color(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdColor())
	return sprite->get_color();
}

void SpxSpriteMgr::set_material_shader(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_material_shader(path);
}

GdString SpxSpriteMgr::get_material_shader(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdString())
	return sprite->get_material_shader();
}

void SpxSpriteMgr::set_material_params(GdObj obj, GdString effect, GdFloat amount) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_material_params(effect, amount);
}

GdFloat SpxSpriteMgr::get_material_params(GdObj obj, GdString effect) {
	SPX_REQUIRE_SPRITE_RETURN(GdFloat())
	return sprite->get_material_params(effect);
}

void SpxSpriteMgr::set_material_params_vec4(GdObj obj, GdString effect, GdVec4 vec4) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_material_params_vec4(effect, vec4);
}

void SpxSpriteMgr::set_material_params_vec(GdObj obj, GdString effect, GdFloat x, GdFloat y, GdFloat z, GdFloat w) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_material_params_vec4(effect, GdVec4(x, y, z, w));
}

GdVec4 SpxSpriteMgr::get_material_params_vec4(GdObj obj, GdString effect) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec4())
	return sprite->get_material_params_vec4(effect);
}

void SpxSpriteMgr::set_material_params_color(GdObj obj, GdString effect, GdColor color) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_material_params_color(effect, color);
}

GdColor SpxSpriteMgr::get_material_params_color(GdObj obj, GdString effect) {
	SPX_REQUIRE_SPRITE_RETURN(GdColor())
	return sprite->get_material_params_color(effect);
}

void SpxSpriteMgr::set_texture_atlas(GdObj obj, GdString path, GdRect2 rect2) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_texture_atlas(path, rect2);
}

void SpxSpriteMgr::set_texture(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_texture(path);
}

void SpxSpriteMgr::set_texture_atlas_direct(GdObj obj, GdString path, GdRect2 rect2) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_texture_atlas_direct(path, rect2, true);
}

void SpxSpriteMgr::set_texture_direct(GdObj obj, GdString path) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_texture_direct(path, true);
}

GdString SpxSpriteMgr::get_texture(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdString())
	return sprite->get_texture();
}

void SpxSpriteMgr::set_visible(GdObj obj, GdBool visible) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_visible(visible);
	sprite->on_set_visible(visible);
}

GdBool SpxSpriteMgr::get_visible(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_visible();
}

GdInt SpxSpriteMgr::get_z_index(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_z_index();
}

void SpxSpriteMgr::set_z_index(GdObj obj, GdInt z) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_z_index(z);
}

void SpxSpriteMgr::play_anim(GdObj obj, GdString p_name, GdFloat p_speed, GdBool isLoop, GdBool p_revert) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->play_anim(p_name, p_speed, isLoop, p_revert);
}

void SpxSpriteMgr::play_backwards_anim(GdObj obj, GdString p_name) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->play_backwards_anim(p_name);
}

void SpxSpriteMgr::pause_anim(GdObj obj) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->pause_anim();
}

void SpxSpriteMgr::stop_anim(GdObj obj) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->stop_anim();
}

GdBool SpxSpriteMgr::is_playing_anim(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_playing_anim();
}

void SpxSpriteMgr::set_anim(GdObj obj, GdString p_name) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim(p_name);
}

GdString SpxSpriteMgr::get_anim(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdString())
	return sprite->get_anim();
}

void SpxSpriteMgr::set_anim_frame(GdObj obj, GdInt p_frame) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_frame(p_frame);
}

GdInt SpxSpriteMgr::get_anim_frame(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_anim_frame();
}

void SpxSpriteMgr::set_anim_speed_scale(GdObj obj, GdFloat p_speed_scale) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_speed_scale(p_speed_scale);
}

GdFloat SpxSpriteMgr::get_anim_speed_scale(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(1.0)
	return sprite->get_anim_speed_scale();
}

GdFloat SpxSpriteMgr::get_anim_playing_speed(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(1.0)
	return sprite->get_anim_playing_speed();
}

void SpxSpriteMgr::set_anim_centered(GdObj obj, GdBool p_center) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_centered(p_center);
}

GdBool SpxSpriteMgr::is_anim_centered(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_anim_centered();
}

void SpxSpriteMgr::set_anim_offset(GdObj obj, GdVec2 p_offset) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_offset(spx_to_godot_vec2(p_offset));
}

GdVec2 SpxSpriteMgr::get_anim_offset(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_anim_offset());
}

void SpxSpriteMgr::set_anim_flip_h(GdObj obj, GdBool p_flip) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_flip_h(p_flip);
}

GdBool SpxSpriteMgr::is_anim_flipped_h(GdObj obj) {
	auto sprite = get_sprite(obj);
	if (sprite == nullptr) {
		print_error("try to get property of a null sprite" + itos(obj));
		return false;
	}
	return sprite->is_anim_flipped_h();
}

void SpxSpriteMgr::set_anim_flip_v(GdObj obj, GdBool p_flip) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_anim_flip_v(p_flip);
}

GdBool SpxSpriteMgr::is_anim_flipped_v(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_anim_flipped_v();
}
GdString SpxSpriteMgr::get_current_anim_name(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdString())
	return sprite->get_current_anim_name();
}

void SpxSpriteMgr::set_velocity(GdObj obj, GdVec2 velocity) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_velocity(spx_to_godot_vec2(velocity));
}

GdVec2 SpxSpriteMgr::get_velocity(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	auto val = sprite->get_velocity();
	return godot_to_spx_vec2(val);
}

GdBool SpxSpriteMgr::is_on_floor(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_floor();
}

GdBool SpxSpriteMgr::is_on_floor_only(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_floor_only();
}

GdBool SpxSpriteMgr::is_on_wall(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_wall();
}

GdBool SpxSpriteMgr::is_on_wall_only(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_wall_only();
}

GdBool SpxSpriteMgr::is_on_ceiling(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_ceiling();
}

GdBool SpxSpriteMgr::is_on_ceiling_only(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_on_ceiling_only();
}

GdVec2 SpxSpriteMgr::get_last_motion(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_last_motion());
}

GdVec2 SpxSpriteMgr::get_position_delta(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_position_delta());
}

GdVec2 SpxSpriteMgr::get_floor_normal(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_floor_normal());
}

GdVec2 SpxSpriteMgr::get_wall_normal(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_wall_normal());
}

GdVec2 SpxSpriteMgr::get_real_velocity(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_real_velocity());
}

void SpxSpriteMgr::move_and_slide(GdObj obj) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->move_and_slide();
}

void SpxSpriteMgr::set_gravity(GdObj obj, GdFloat gravity) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_gravity(gravity);
}

GdFloat SpxSpriteMgr::get_gravity(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_gravity();
}

void SpxSpriteMgr::set_mass(GdObj obj, GdFloat mass) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_mass(mass);
}

GdFloat SpxSpriteMgr::get_mass(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_mass();
}

void SpxSpriteMgr::add_force(GdObj obj, GdVec2 force) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->add_force(spx_to_godot_vec2(force));
}

void SpxSpriteMgr::add_impulse(GdObj obj, GdVec2 impulse) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->add_impulse(spx_to_godot_vec2(impulse));
}

void SpxSpriteMgr::set_physics_mode(GdObj obj, GdInt mode) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_physics_mode(mode);
}

GdInt SpxSpriteMgr::get_physics_mode(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_physics_mode();
}

void SpxSpriteMgr::set_use_gravity(GdObj obj, GdBool enabled) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_use_gravity(enabled);
}

GdBool SpxSpriteMgr::is_use_gravity(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_use_gravity();
}

void SpxSpriteMgr::set_gravity_scale(GdObj obj, GdFloat scale) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_gravity_scale(scale);
}

GdFloat SpxSpriteMgr::get_gravity_scale(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(1.0f)
	return sprite->get_gravity_scale();
}

void SpxSpriteMgr::set_drag(GdObj obj, GdFloat drag) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_drag(drag);
}

GdFloat SpxSpriteMgr::get_drag(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0.0f)
	return sprite->get_drag();
}

void SpxSpriteMgr::set_friction(GdObj obj, GdFloat friction) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_friction(friction);
}

GdFloat SpxSpriteMgr::get_friction(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0.0f)
	return sprite->get_friction();
}

void SpxSpriteMgr::set_collision_layer(GdObj obj, GdInt layer) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collision_layer((uint32_t)layer);
}

GdInt SpxSpriteMgr::get_collision_layer(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_collision_layer();
}

void SpxSpriteMgr::set_collision_mask(GdObj obj, GdInt mask) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collision_mask((uint32_t)mask);
}

GdInt SpxSpriteMgr::get_collision_mask(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_collision_mask();
}

void SpxSpriteMgr::set_trigger_layer(GdObj obj, GdInt layer) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_layer(layer);
}

GdInt SpxSpriteMgr::get_trigger_layer(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_trigger_layer();
}

void SpxSpriteMgr::set_trigger_mask(GdObj obj, GdInt mask) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_mask(mask);
}

GdInt SpxSpriteMgr::get_trigger_mask(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(0)
	return sprite->get_trigger_mask();
}

void SpxSpriteMgr::set_collider_rect(GdObj obj, GdVec2 center, GdVec2 size) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collider_rect(center, size);
}

void SpxSpriteMgr::set_collider_circle(GdObj obj, GdVec2 center, GdFloat radius) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collider_circle(center, radius);
}

void SpxSpriteMgr::set_collider_capsule(GdObj obj, GdVec2 center, GdVec2 size) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collider_capsule(center, size);
}

void SpxSpriteMgr::set_collider_polygon(GdObj obj, GdVec2 center, GdArray points) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collider_polygon(center, points);
}

void SpxSpriteMgr::set_collision_enabled(GdObj obj, GdBool enabled) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_collision_enabled(enabled);
}

GdBool SpxSpriteMgr::is_collision_enabled(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_collision_enabled();
}

void SpxSpriteMgr::set_trigger_rect(GdObj obj, GdVec2 center, GdVec2 size) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_rect(center, size);
}

void SpxSpriteMgr::set_trigger_circle(GdObj obj, GdVec2 center, GdFloat radius) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_circle(center, radius);
}

void SpxSpriteMgr::set_trigger_capsule(GdObj obj, GdVec2 center, GdVec2 size) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_capsule(center, size);
}

void SpxSpriteMgr::set_trigger_polygon(GdObj obj, GdVec2 center, GdArray points) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_polygon(center, points);
}

void SpxSpriteMgr::set_trigger_enabled(GdObj obj, GdBool trigger) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_trigger_enabled(trigger);
}
GdBool SpxSpriteMgr::is_trigger_enabled(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	return sprite->is_trigger_enabled();
}

GdBool SpxSpriteMgr::check_collision_with_sprite(GdObj obj, GdObj obj_b, GdFloat alpha_threshold, GdBool use_pixel_perfect) {
	SPX_REQUIRE_SPRITE_RETURN(false)
	SPX_REQUIRE_TARGET_SPRITE_RETURN(obj_b, false)
	if (!sprite->is_visible_in_tree() || !sprite_target->is_visible_in_tree()) {
		return false;
	}

	// If not using pixel-perfect collision, use simple collider2d collision detection
	if (!use_pixel_perfect) {
		return sprite->check_collision(sprite_target, false, false);
	}

	return check_pixel_collision(sprite, sprite_target, alpha_threshold, pixel_collision_sampling_step);
}

GdBool SpxSpriteMgr::check_collision_by_color(GdObj obj, GdColor color, GdFloat color_threshold, GdFloat alpha_threshold) {
	const GdFloat threshold_sq = color_threshold * color_threshold;
	return _check_scene_color_collision(obj,
			[=](GdColor self) { return !(self.a <= alpha_threshold); },
			[=](GdColor scene) { return color_rgb_distance_squared(color, scene) < threshold_sq; });
}

GdBool SpxSpriteMgr::check_collision_by_colors(GdObj obj, GdColor sprite_color, GdColor target_color, GdFloat color_threshold, GdFloat alpha_threshold) {
	const GdFloat threshold_sq = color_threshold * color_threshold;
	return _check_scene_color_collision(obj,
			[=](GdColor self) { return !(self.a <= alpha_threshold) && !(color_rgb_distance_squared(sprite_color, self) >= threshold_sq); },
			[=](GdColor scene) { return color_rgb_distance_squared(target_color, scene) < threshold_sq; });
}

GdBool SpxSpriteMgr::check_collision_by_alpha(GdObj obj, GdFloat alpha_threshold) {
	return _check_collision(obj, [alpha_threshold](GdColor a, GdColor b) -> bool {
		return a.a > alpha_threshold && b.a > alpha_threshold;
	});
}

GdBool SpxSpriteMgr::_check_scene_color_collision(GdObj obj, ColorMatchFunc self_matches, ColorMatchFunc scene_matches) {
	SPX_REQUIRE_SPRITE_RETURN(false)

	Snapshot self_query;
	// Scratch uses the caller's silhouette/color as the query mask even when ghosted or hidden.
	if (!capture_sprite_pixels(sprite, self_query, false, false)) {
		return false;
	}
	if (!SpxPixelQuery::load_image(self_query)) {
		return false;
	}
	std::vector<Layer> scene_queries;
	scene_queries.reserve((size_t)id_objects.size() + 1);
	for (const auto &item : id_objects) {
		SpxSprite *candidate = item.value;
		if (candidate == nullptr || candidate == sprite || candidate->is_queued_for_deletion()) {
			continue;
		}

		Layer query;
		if (!capture_sprite_pixels(candidate, query.pixel_query, true, true)) {
			continue;
		}
		if (!SpxPixelQuery::overlap(self_query.bounds, query.pixel_query.bounds).has_area()) {
			continue;
		}
		if (!SpxPixelQuery::load_image(query.pixel_query)) {
			continue;
		}

		query.z_index = candidate->get_z_index();
		query.order = candidate->is_backdrop_sprite() ? Layer::BACKDROP : Layer::SPRITE;
		query.tree_index = candidate->get_index();
		scene_queries.push_back(std::move(query));
	}
	SpxEngine *engine = SpxEngine::get_singleton();
	SpxPenMgr *pen = engine != nullptr ? engine->get_pen() : nullptr;
	Layer pen_layer;
	if (pen != nullptr && pen->capture(self_query.bounds, pen_layer.pixel_query)) {
		pen_layer.order = Layer::PEN;
		scene_queries.push_back(std::move(pen_layer));
	}
	std::sort(scene_queries.begin(), scene_queries.end(), [](const Layer &a, const Layer &b) {
		return a.in_front_of(b);
	});

	return SpxPixelQuery::any_pixel_center(SpxPixelQuery::pixel_centers(self_query.bounds), pixel_collision_sampling_step, [&](const Vector2 &sample_pos) {
		Color self_color;
		return SpxPixelQuery::sample_premultiplied(self_query, sample_pos, self_color) &&
				self_matches(self_color) && scene_matches(SpxPixelQuery::composite(scene_queries, sample_pos));
	});
}

GdBool SpxSpriteMgr::_check_collision(GdObj obj, ColorCheckFunc check_func) {
	SPX_REQUIRE_SPRITE_RETURN(false) // Ensure sprite exists

	Snapshot query1;
	// Scratch uses the caller's silhouette/color as the mask even when ghosted or hidden.
	if (!capture_sprite_pixels(sprite, query1, false, false)) {
		return false;
	}
	if (!SpxPixelQuery::load_image(query1)) {
		return false;
	}

	// Iterate through all objects
	for (const auto &item : id_objects) {
		SpxSprite *sp2 = item.value;
		if (sprite == sp2 || sp2 == nullptr || sp2->is_queued_for_deletion()) {
			continue; // Skip itself
		}

		Snapshot query2;
		if (!capture_sprite_pixels(sp2, query2, true, true)) {
			continue;
		}

		const Rect2i overlap_rect = SpxPixelQuery::overlap(query1.bounds, query2.bounds);
		if (!overlap_rect.has_area()) {
			continue;
		}
		if (!SpxPixelQuery::load_image(query2)) {
			continue;
		}
		if (SpxPixelQuery::any_pixel_center(overlap_rect, pixel_collision_sampling_step, [&](const Vector2 &sample_pos) {
				Color color1;
				Color color2;
				return SpxPixelQuery::sample(query1, sample_pos, color1) &&
						SpxPixelQuery::sample(query2, sample_pos, color2) && check_func(color1, color2);
			})) {
			return true;
		}
	}
	return false;
}

void SpxSpriteMgr::on_trigger_enter(GdInt self_id, GdInt other_id) {
	if (physicsMgr->is_collision_by_pixel) {
		bounding_collision_pairs.insert(TriggerPair(self_id, other_id));
	} else {
		SPX_CALLBACK->func_on_trigger_enter(self_id, other_id);
	}
}
void SpxSpriteMgr::on_trigger_exit(GdInt self_id, GdInt other_id) {
	if (physicsMgr->is_collision_by_pixel) {
		const TriggerPair pair(self_id, other_id);
		// Trigger separation ends the broad-phase candidate pair, so pixel collision
		// tracking must stop immediately instead of waiting for another pixel check.
		bounding_collision_pairs.erase(pair);
		if (_erase_pixel_collision_pair(pair)) {
			_notify_pixel_collision_exit(pair);
		}
	} else {
		SPX_CALLBACK->func_on_trigger_exit(self_id, other_id);
	}
}

void SpxSpriteMgr::_notify_pixel_collision_enter(const TriggerPair &pair) {
	SPX_CALLBACK->func_on_trigger_enter(pair.id1, pair.id2);
	SPX_CALLBACK->func_on_trigger_enter(pair.id2, pair.id1);
}

void SpxSpriteMgr::_notify_pixel_collision_exit(const TriggerPair &pair, GdObj skip_id) {
	if (pair.id1 != skip_id) {
		SPX_CALLBACK->func_on_trigger_exit(pair.id1, pair.id2);
	}
	if (pair.id2 != skip_id) {
		SPX_CALLBACK->func_on_trigger_exit(pair.id2, pair.id1);
	}
}

bool SpxSpriteMgr::_erase_pixel_collision_pair(const TriggerPair &pair) {
	return pixel_collision_pairs.erase(pair) > 0;
}

void SpxSpriteMgr::_remove_collision_pairs_for_sprite(GdObj obj) {
	Vector<TriggerPair> exit_triggers;
	for (auto it = pixel_collision_pairs.begin(); it != pixel_collision_pairs.end();) {
		if (it->id1 == obj || it->id2 == obj) {
			exit_triggers.push_back(*it);
			it = pixel_collision_pairs.erase(it);
		} else {
			++it;
		}
	}

	for (auto it = bounding_collision_pairs.begin(); it != bounding_collision_pairs.end();) {
		if (it->id1 == obj || it->id2 == obj) {
			it = bounding_collision_pairs.erase(it);
		} else {
			++it;
		}
	}

	for (const auto &pair : exit_triggers) {
		_notify_pixel_collision_exit(pair, obj);
	}
}

void SpxSpriteMgr::_check_pixel_collision_events() {
	if (!physicsMgr->is_collision_by_pixel || bounding_collision_pairs.empty()) {
		return;
	}

	Vector<TriggerPair> enter_triggers;
	Vector<TriggerPair> exit_triggers;
	SpxPixelQuery::ImageCache image_cache;

	for (auto it = bounding_collision_pairs.begin(); it != bounding_collision_pairs.end();) {
		const TriggerPair trigger = *it;
		SpxSprite *sprite1 = get_sprite(trigger.id1);
		SpxSprite *sprite2 = get_sprite(trigger.id2);
		if (sprite1 == nullptr || sprite2 == nullptr) {
			_erase_pixel_collision_pair(trigger);
			it = bounding_collision_pairs.erase(it);
			continue;
		}

		if (check_pixel_collision(sprite1, sprite2, DEFAULT_COLLISION_ALPHA_THRESHOLD, pixel_collision_sampling_step, &image_cache)) {
			if (pixel_collision_pairs.insert(trigger).second) {
				enter_triggers.push_back(trigger);
			}
		} else if (_erase_pixel_collision_pair(trigger)) {
			exit_triggers.push_back(trigger);
		}

		++it;
	}

	for (const auto &trigger : exit_triggers) {
		_notify_pixel_collision_exit(trigger);
	}
	for (const auto &trigger : enter_triggers) {
		_notify_pixel_collision_enter(trigger);
	}
}

void SpxSpriteMgr::set_pivot(GdObj obj, GdVec2 pivot) {
	SPX_REQUIRE_SPRITE_VOID()
	sprite->set_render_offset(spx_to_godot_vec2(pivot));
}
GdVec2 SpxSpriteMgr::get_pivot(GdObj obj) {
	SPX_REQUIRE_SPRITE_RETURN(GdVec2())
	return godot_to_spx_vec2(sprite->get_render_offset());
}
