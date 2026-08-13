/**************************************************************************/
/*  spx_sprite_mgr.h                                                      */
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

#ifndef SPX_SPRITE_MGR_H
#define SPX_SPRITE_MGR_H

#include "core/templates/hash_map.h"
#include "gdextension_spx_ext.h"
#include "scene/2d/animated_sprite_2d.h"
#include "spx_base_mgr.h"
#include "spx_layer_sorter.h"
#include <functional>
#include <unordered_set>

typedef std::function<bool(GdColor, GdColor)> ColorCheckFunc;

class SpxSprite;
class ISortableSprite;

class TriggerPair {
public:
	GdObj id1;
	GdObj id2;

public:
	TriggerPair() :
			id1(0), id2(0) {}

	TriggerPair(GdObj p_id1, GdObj p_id2) {
		id1 = p_id1;
		id2 = p_id2;
		if (p_id1 > p_id2) {
			id1 = p_id2;
			id2 = p_id1;
		}
	}
	bool operator<(const TriggerPair &p_other) const {
		return id1 < p_other.id1 || (id1 == p_other.id1 && id2 < p_other.id2);
	}

	bool operator==(const TriggerPair &p_other) const {
		return id1 == p_other.id1 && id2 == p_other.id2;
	}

	bool operator!=(const TriggerPair &p_other) const {
		return !(*this == p_other);
	}

	size_t std_hash() const {
		return static_cast<size_t>(id1) ^ static_cast<size_t>(id2);
	}
};

namespace std {
template <>
struct hash<TriggerPair> {
	size_t operator()(const TriggerPair &pair) const {
		return pair.std_hash();
	}
};
} //namespace std

class SpxSpriteMgr : public SpxBaseMgr {
	SPXCLASS(SpxSpriteMgr, SpxBaseMgr)
public:
	virtual ~SpxSpriteMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor

private:
	RBMap<GdObj, SpxSprite *> id_objects;

	std::unordered_set<TriggerPair> bounding_collision_pairs;
	std::unordered_set<TriggerPair> pixel_collision_pairs;

	Node *dont_destroy_root;
	Node *sprite_root;

	// Pixel-perfect collision sampling step: check every N pixels instead of every pixel
	// Higher values = better performance but lower accuracy
	// 1 = check every pixel (most accurate, slowest)
	// 2 = check every 2 pixels (good balance)
	// 3+ = faster but may miss small collisions
	int pixel_collision_sampling_step;

	Ref<Image> _get_current_frame_image(AnimatedSprite2D *sprite);
	Rect2 _get_sprite_aabb(AnimatedSprite2D *anim2d);
	GdBool _check_collision(GdObj obj, ColorCheckFunc check_func);
	GdBool _check_scene_color_collision(GdObj obj, ColorCheckFunc check_func);
	bool _check_pixel_collision_between(SpxSprite *sprite_a, SpxSprite *sprite_b, GdFloat alpha_threshold);
	void _notify_pixel_collision_enter(const TriggerPair &pair);
	void _notify_pixel_collision_exit(const TriggerPair &pair, GdObj skip_id = NULL_OBJECT_ID);
	bool _erase_pixel_collision_pair(const TriggerPair &pair);
	void _remove_collision_pairs_for_sprite(GdObj obj);
	void _check_pixel_collision_events();
	void _batch_write_positions(const GdObj *ids, int count, float *out, int out_len);

protected:
	void _register_sprite(SpxSprite *p_sprite);

public:
	static StringName default_texture_anim;
	void on_awake() override;
	void on_start() override;
	void on_destroy() override;
	void on_update(float delta) override;
	void on_reset(int reset_code) override;

	SpxSprite *get_sprite(GdObj obj);
	void on_sprite_destroy(SpxSprite *sprite);
	void on_trigger_enter(GdInt self_id, GdInt other_id);
	void on_trigger_exit(GdInt self_id, GdInt other_id);
	GdObj _create_sprite(GdString path, GdVec2 pos, GdBool is_backdrop);
	void destroy_all_sprites();
	void collect_sortable_sprites(Vector<ISortableSprite *> &out);

public:
	SPX_API void set_dont_destroy_on_load(GdObj obj);
	// process
	SPX_API void set_process(GdObj obj, GdBool is_on);
	SPX_API void set_physic_process(GdObj obj, GdBool is_on);

	SPX_API void set_type_name(GdObj obj, GdString type_name);

	SPX_API void set_pivot(GdObj obj, GdVec2 pivot);
	SPX_API GdVec2 get_pivot(GdObj obj);

	// children
	SPX_API void set_child_position(GdObj obj, GdString path, GdVec2 pos);
	SPX_API GdVec2 get_child_position(GdObj obj, GdString path);
	SPX_API void set_child_rotation(GdObj obj, GdString path, GdFloat rot);
	SPX_API GdFloat get_child_rotation(GdObj obj, GdString path);
	SPX_API void set_child_scale(GdObj obj, GdString path, GdVec2 scale);
	SPX_API GdVec2 get_child_scale(GdObj obj, GdString path);

	SPX_API GdBool check_collision(GdObj obj, GdObj target, GdBool is_src_trigger, GdBool is_dst_trigger);
	SPX_API GdBool check_collision_with_point(GdObj obj, GdVec2 point, GdBool is_click_query);
	SPX_API void set_debug_collision_visible(GdObj obj, GdBool visible);
	SPX_API GdBool is_debug_collision_visible(GdObj obj);
	SPX_API GdObj create_backdrop(GdString path);
	SPX_API GdObj create_sprite(GdString path, GdVec2 pos);
	SPX_API GdObj create_bare_sprite(GdVec2 pos);
	SPX_API GdObj clone_sprite(GdObj obj);
	SPX_API GdBool destroy_sprite(GdObj obj);
	SPX_API GdBool is_sprite_alive(GdObj obj);
	SPX_API void set_position(GdObj obj, GdVec2 pos);
	SPX_API void set_transform(GdObj obj, GdVec2 pos, GdFloat rot, GdVec2 scale, GdBool visible, GdVec2 pivot);
	SPX_API GdVec2 get_position(GdObj obj);
	SPX_API void set_rotation(GdObj obj, GdFloat rot);
	SPX_API GdFloat get_rotation(GdObj obj);
	SPX_API void set_scale(GdObj obj, GdVec2 scale);
	SPX_API GdVec2 get_scale(GdObj obj);
	SPX_API void set_render_scale(GdObj obj, GdVec2 scale);
	SPX_API GdVec2 get_render_scale(GdObj obj);
	SPX_API void set_color(GdObj obj, GdColor color);
	SPX_API GdColor get_color(GdObj obj);

	SPX_API void set_material_shader(GdObj obj, GdString path);
	SPX_API GdString get_material_shader(GdObj obj);
	SPX_API void set_material_params(GdObj obj, GdString effect, GdFloat amount);
	SPX_API GdFloat get_material_params(GdObj obj, GdString effect);

	SPX_API void set_material_params_vec(GdObj obj, GdString effect, GdFloat x, GdFloat y, GdFloat z, GdFloat w);

	SPX_API void set_material_params_vec4(GdObj obj, GdString effect, GdVec4 vec4);
	SPX_API GdVec4 get_material_params_vec4(GdObj obj, GdString effect);

	SPX_API void set_material_params_color(GdObj obj, GdString effect, GdColor color);
	SPX_API GdColor get_material_params_color(GdObj obj, GdString effect);

	SPX_API void set_texture_atlas(GdObj obj, GdString path, GdRect2 rect2);
	SPX_API void set_texture(GdObj obj, GdString path);
	SPX_API void set_texture_atlas_direct(GdObj obj, GdString path, GdRect2 rect2);
	SPX_API void set_texture_direct(GdObj obj, GdString path);

	SPX_API GdString get_texture(GdObj obj);
	SPX_API void set_visible(GdObj obj, GdBool visible);
	SPX_API GdBool get_visible(GdObj obj);
	SPX_API GdInt get_z_index(GdObj obj);
	SPX_API void set_z_index(GdObj obj, GdInt z);

	// animation
	SPX_API void play_anim(GdObj obj, GdString p_name, GdFloat p_speed, GdBool isLoop, GdBool p_revert);
	SPX_API void play_backwards_anim(GdObj obj, GdString p_name);
	SPX_API void pause_anim(GdObj obj);
	SPX_API void stop_anim(GdObj obj);
	SPX_API GdBool is_playing_anim(GdObj obj);
	SPX_API void set_anim(GdObj obj, GdString p_name);
	SPX_API GdString get_anim(GdObj obj);
	SPX_API void set_anim_frame(GdObj obj, GdInt p_frame);
	SPX_API GdInt get_anim_frame(GdObj obj);
	SPX_API void set_anim_speed_scale(GdObj obj, GdFloat p_speed_scale);
	SPX_API GdFloat get_anim_speed_scale(GdObj obj);
	SPX_API GdFloat get_anim_playing_speed(GdObj obj);
	SPX_API void set_anim_centered(GdObj obj, GdBool p_center);
	SPX_API GdBool is_anim_centered(GdObj obj);
	SPX_API void set_anim_offset(GdObj obj, GdVec2 p_offset);
	SPX_API GdVec2 get_anim_offset(GdObj obj);
	SPX_API void set_anim_flip_h(GdObj obj, GdBool p_flip);
	SPX_API GdBool is_anim_flipped_h(GdObj obj);
	SPX_API void set_anim_flip_v(GdObj obj, GdBool p_flip);
	SPX_API GdBool is_anim_flipped_v(GdObj obj);
	SPX_API GdString get_current_anim_name(GdObj obj);

	// physics
	SPX_API void set_velocity(GdObj obj, GdVec2 velocity);
	SPX_API GdVec2 get_velocity(GdObj obj);
	SPX_API GdBool is_on_floor(GdObj obj);
	SPX_API GdBool is_on_floor_only(GdObj obj);
	SPX_API GdBool is_on_wall(GdObj obj);
	SPX_API GdBool is_on_wall_only(GdObj obj);
	SPX_API GdBool is_on_ceiling(GdObj obj);
	SPX_API GdBool is_on_ceiling_only(GdObj obj);
	SPX_API GdVec2 get_last_motion(GdObj obj);
	SPX_API GdVec2 get_position_delta(GdObj obj);
	SPX_API GdVec2 get_floor_normal(GdObj obj);
	SPX_API GdVec2 get_wall_normal(GdObj obj);
	SPX_API GdVec2 get_real_velocity(GdObj obj);
	SPX_API void move_and_slide(GdObj obj);

	SPX_API void set_gravity(GdObj obj, GdFloat gravity);
	SPX_API GdFloat get_gravity(GdObj obj);
	SPX_API void set_mass(GdObj obj, GdFloat mass);
	SPX_API GdFloat get_mass(GdObj obj);
	SPX_API void add_force(GdObj obj, GdVec2 force);
	SPX_API void add_impulse(GdObj obj, GdVec2 impulse);

	SPX_API void set_physics_mode(GdObj obj, GdInt mode);
	SPX_API GdInt get_physics_mode(GdObj obj);
	SPX_API void set_use_gravity(GdObj obj, GdBool enabled);
	SPX_API GdBool is_use_gravity(GdObj obj);
	SPX_API void set_gravity_scale(GdObj obj, GdFloat scale);
	SPX_API GdFloat get_gravity_scale(GdObj obj);
	SPX_API void set_drag(GdObj obj, GdFloat drag);
	SPX_API GdFloat get_drag(GdObj obj);
	SPX_API void set_friction(GdObj obj, GdFloat friction);
	SPX_API GdFloat get_friction(GdObj obj);

	SPX_API void set_collision_layer(GdObj obj, GdInt layer);
	SPX_API GdInt get_collision_layer(GdObj obj);
	SPX_API void set_collision_mask(GdObj obj, GdInt mask);
	SPX_API GdInt get_collision_mask(GdObj obj);

	SPX_API void set_trigger_layer(GdObj obj, GdInt layer);
	SPX_API GdInt get_trigger_layer(GdObj obj);
	SPX_API void set_trigger_mask(GdObj obj, GdInt mask);
	SPX_API GdInt get_trigger_mask(GdObj obj);

	SPX_API void set_collider_rect(GdObj obj, GdVec2 center, GdVec2 size);
	SPX_API void set_collider_circle(GdObj obj, GdVec2 center, GdFloat radius);
	SPX_API void set_collider_capsule(GdObj obj, GdVec2 center, GdVec2 size);
	SPX_API void set_collider_polygon(GdObj obj, GdVec2 center, GdArray points);
	SPX_API void set_collision_enabled(GdObj obj, GdBool enabled);
	SPX_API GdBool is_collision_enabled(GdObj obj);

	SPX_API void set_trigger_rect(GdObj obj, GdVec2 center, GdVec2 size);
	SPX_API void set_trigger_circle(GdObj obj, GdVec2 center, GdFloat radius);
	SPX_API void set_trigger_capsule(GdObj obj, GdVec2 center, GdVec2 size);
	SPX_API void set_trigger_polygon(GdObj obj, GdVec2 center, GdArray points);
	SPX_API void set_trigger_enabled(GdObj obj, GdBool trigger);
	SPX_API GdBool is_trigger_enabled(GdObj obj);

	// misc
	// Matches any opaque source pixel against the target pixel color; the source pixel color itself is not filtered.
	SPX_API GdBool check_collision_by_color(GdObj obj, GdColor color, GdFloat color_threshold, GdFloat alpha_threshold);
	// Matches both source and target pixel colors using the same RGBA distance threshold on each side.
	SPX_API GdBool check_collision_by_colors(GdObj obj, GdColor sprite_color, GdColor target_color, GdFloat color_threshold, GdFloat alpha_threshold);
	SPX_API GdBool check_collision_by_alpha(GdObj obj, GdFloat alpha_threshold);
	SPX_API GdBool check_collision_with_sprite(GdObj obj, GdObj obj_b, GdFloat alpha_threshold, GdBool use_pixel_perfect);

	// pixel collision sampling configuration
	SPX_API void set_pixel_collision_sampling_step(GdInt step);
	SPX_API GdInt get_pixel_collision_sampling_step();

	// batch sync
	SPX_API void batch_update_transforms(const float *buffer_data, int len);
	SPX_API void batch_update_visuals(const float *buffer_data, int len);
	SPX_API void batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len);
	SPX_API void batch_update_physics(const float *buffer_data, int len);
};

#endif // SPX_SPRITE_MGR_H
