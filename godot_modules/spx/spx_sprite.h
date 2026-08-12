/**************************************************************************/
/*  spx_sprite.h                                                          */
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

#ifndef SPX_SPRITE_H
#define SPX_SPRITE_H

#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include "scene/2d/physics/character_body_2d.h"
#include "scene/2d/physics/physics_body_2d.h"
#include "scene/2d/physics/static_body_2d.h"
#include "scene/2d/sprite_2d.h"
#include "spx.h"

class SpriteFrames;
class AnimatedSprite2D;
class Area2D;
class CollisionShape2D;
class ShaderMaterial;
class Texture2D;
class VisibleOnScreenNotifier2D;

// Interface for sortable sprites
class ISortableSprite {
public:
	virtual ~ISortableSprite() = default;
	virtual GdObj get_sort_id() const = 0;
	virtual Point2 get_sort_position() const = 0;
	virtual void set_sort_z_index(int z) = 0;
	virtual int get_sort_z_index() const = 0;
	virtual bool is_node_valid() const = 0;
	virtual bool is_sort_static() const { return false; }
};

// SpxRenderSprite - Wrapper for Sprite2D with sortable interface
class SpxRenderSprite : public Sprite2D, public ISortableSprite {
	GDCLASS(SpxRenderSprite, Sprite2D);

public:
	SpxRenderSprite() = default;
	~SpxRenderSprite() override = default;

	void set_sort_id(GdObj p_id) { sort_id = p_id; }
	GdObj get_sort_id_internal() const { return sort_id; }
	void set_pivot(GdVec2 p_pivot) { pivot_offset = p_pivot; }
	GdVec2 get_pivot() { return pivot_offset; }

	// ISortableSprite interface implementation
	GdObj get_sort_id() const override { return sort_id; }
	Point2 get_sort_position() const override { return get_global_position() - pivot_offset; }
	void set_sort_z_index(int p_z_index) override { set_z_index(p_z_index); }
	int get_sort_z_index() const override { return get_z_index(); }
	bool is_node_valid() const override { return is_inside_tree(); }
	bool is_sort_static() const override { return true; }

private:
	GdObj sort_id = 0;
	Vector2 pivot_offset;
};

// SpxStaticSprite - Wrapper for StaticBody2D with sortable interface
class SpxStaticSprite : public StaticBody2D, public ISortableSprite {
	GDCLASS(SpxStaticSprite, StaticBody2D);

public:
	void set_sort_id(GdObj p_id) { sort_id = p_id; }
	GdObj get_sort_id_internal() const { return sort_id; }
	void set_collider(CollisionShape2D *p_collider);
	CollisionShape2D *get_collider() const { return collider2d; }
	void set_pivot(GdVec2 p_pivot) { pivot_offset = p_pivot; }
	GdVec2 get_pivot() { return pivot_offset; }

	// ISortableSprite interface implementation
	GdObj get_sort_id() const override { return sort_id; }
	Point2 get_sort_position() const override { return get_global_position() - pivot_offset; }
	void set_sort_z_index(int p_z_index) override { set_z_index(p_z_index); }
	int get_sort_z_index() const override { return get_z_index(); }
	bool is_node_valid() const override { return is_inside_tree(); }
	bool is_sort_static() const override { return true; }

private:
	GdObj sort_id = 0;
	Vector2 pivot_offset;
	CollisionShape2D *collider2d = nullptr;
};

class SpxSprite : public CharacterBody2D, public ISortableSprite {
	GDCLASS(SpxSprite, CharacterBody2D);

public:
	enum PhysicsMode {
		NO_PHYSICS = 0,
		KINEMATIC = 1,
		DYNAMIC = 2,
		STATIC = 3,
	};

	static void _bind_methods();
	SpxSprite();
	~SpxSprite() override;

	// Lifecycle
	void on_start();
	void on_destroy_call();

	void set_block_signals(bool p_block);

	// Metadata
	void set_gid(GdObj p_id);
	GdObj get_gid();
	void set_type_name(GdString p_type_name);
	void set_spx_type_name(String p_type_name);
	String get_spx_type_name();
	void set_backdrop(GdBool p_is_backdrop) { is_backdrop = p_is_backdrop; }
	bool is_backdrop_sprite() const { return is_backdrop; }

	// Components
	AnimatedSprite2D *get_anim2d() const { return anim2d; }
	Area2D *get_area2d() const { return area2d; }
	CollisionShape2D *get_trigger() const { return trigger2d; }

	// Rendering
	void set_render_offset(GdVec2 p_render_offset);
	GdVec2 get_render_offset() const { return render_offset; }
	void set_render_scale(GdVec2 p_scale);
	GdVec2 get_render_scale();
	void set_material_shader(GdString p_path);
	GdString get_material_shader();
	void set_color(GdColor p_color);
	GdColor get_color();
	void set_material_params(GdString p_effect, GdFloat p_amount);
	GdFloat get_material_params(GdString p_effect);
	void set_material_params_vec4(GdString p_effect, GdVec4 p_vec4);
	void set_uv_remap(const GdVec4 &p_uv_remap);
	GdVec4 get_material_params_vec4(GdString p_effect);
	void set_material_params_color(GdString p_effect, GdColor p_color);
	GdColor get_material_params_color(GdString p_effect);
	void set_texture_atlas(GdString p_path, GdRect2 p_region);
	void set_texture(GdString p_path);
	void set_texture_atlas_direct(GdString p_path, GdRect2 p_region, GdBool p_direct);
	void set_texture_direct(GdString p_path, GdBool p_direct);
	GdString get_texture();
	Rect2 get_rect() const;
	GdString get_current_anim_name();
	void on_set_visible(GdBool p_visible);

	// Animation
	void play_anim(GdString p_name, GdFloat p_speed = 1.0, GdBool p_is_loop = false, GdBool p_from_end = false);
	void play_backwards_anim(GdString p_name);
	void pause_anim();
	void stop_anim();
	GdBool is_playing_anim() const;
	void set_anim(GdString p_name);
	GdString get_anim() const;
	void set_anim_frame(GdInt p_frame);
	GdInt get_anim_frame() const;
	void set_anim_speed_scale(GdFloat p_speed_scale);
	GdFloat get_anim_speed_scale() const;
	GdFloat get_anim_playing_speed() const;
	void set_anim_centered(GdBool p_center);
	GdBool is_anim_centered() const;
	void set_anim_offset(GdVec2 p_offset);
	GdVec2 get_anim_offset() const;
	void set_anim_flip_h(GdBool p_flip);
	GdBool is_anim_flipped_h() const;
	void set_anim_flip_v(GdBool p_flip);
	GdBool is_anim_flipped_v() const;
	void set_dynamic_frame_offset_enabled(GdBool p_enabled);
	GdBool is_dynamic_frame_offset_enabled() const;

	// Physics
	void set_physics_mode(GdInt p_mode);
	GdInt get_physics_mode() const;
	void set_use_gravity(GdBool p_enabled);
	GdBool is_use_gravity() const;
	void set_gravity_scale(GdFloat p_scale);
	GdFloat get_gravity_scale() const;
	void set_drag(GdFloat p_drag);
	GdFloat get_drag() const;
	void set_friction(GdFloat p_friction);
	GdFloat get_friction() const;
	void set_gravity(GdFloat p_gravity);
	GdFloat get_gravity();
	void set_mass(GdFloat p_mass);
	GdFloat get_mass();
	void add_force(GdVec2 p_force);
	void add_impulse(GdVec2 p_impulse);
	void set_trigger_layer(GdInt p_layer);
	GdInt get_trigger_layer();
	void set_trigger_mask(GdInt p_mask);
	GdInt get_trigger_mask();
	void set_collider_rect(GdVec2 p_center, GdVec2 p_size);
	void set_collider_circle(GdVec2 p_center, GdFloat p_radius);
	void set_collider_capsule(GdVec2 p_center, GdVec2 p_size);
	void set_collider_polygon(GdVec2 p_center, GdArray p_points);
	void set_collision_enabled(GdBool p_enabled);
	GdBool is_collision_enabled();
	void set_trigger_rect(GdVec2 p_center, GdVec2 p_size);
	void set_trigger_circle(GdVec2 p_center, GdFloat p_radius);
	void set_trigger_capsule(GdVec2 p_center, GdVec2 p_size);
	void set_trigger_polygon(GdVec2 p_center, GdArray p_points);
	void set_trigger_enabled(GdBool p_enabled);
	GdBool is_trigger_enabled();

	// Collision
	CollisionShape2D *get_collider(bool p_is_trigger = false);
	GdBool check_collision(SpxSprite *p_other, GdBool p_is_src_trigger = true, GdBool p_is_dst_trigger = true);
	GdBool check_collision_with_point(GdVec2 p_point, GdBool p_is_trigger = true);
	void set_debug_collision_visible(GdBool p_enabled);
	GdBool is_debug_collision_visible() const;

	// ISortableSprite
	GdObj get_sort_id() const override { return gid; }
	Point2 get_sort_position() const override { return get_global_position(); }
	void set_sort_z_index(int p_z_index) override { set_z_index(p_z_index); }
	int get_sort_z_index() const override { return get_z_index(); }
	bool is_node_valid() const override { return is_inside_tree(); }
	bool is_sort_static() const override { return get_physics_mode() == PhysicsMode::STATIC; }

protected:
	void _notification(int p_what);
	void _physics_process(double p_delta);
	void _handle_dynamic_physics(double p_delta);
	void _handle_kinematic_physics(double p_delta);
	void _handle_static_physics(double p_delta);
	void _handle_no_physics(double p_delta);

private:
	// Component helpers
	template <typename T>
	T *_get_component(Node *p_node, GdBool p_recursive = false);
	template <typename T>
	T *_get_component(GdBool p_recursive = false);

	// Property bindings
	void _set_use_default_frames(bool p_enabled);
	bool _get_use_default_frames();

	// Runtime initialization
	void _resolve_runtime_components();
	void _initialize_default_frames();
	void _ensure_visible_notifier();
	void _connect_runtime_signals();
	void _update_collision_debug_overlays();

	// Signal callbacks
	void _on_area_entered(Node *p_node);
	void _on_area_exited(Node *p_node);
	void _on_sprite_frames_set_changed();
	void _on_sprite_animation_changed();
	void _on_sprite_frame_changed();
	void _on_sprite_animation_looped();
	void _on_sprite_animation_finished();
	void _on_sprite_vfx_finished();
	void _on_sprite_screen_exited();
	void _on_sprite_screen_entered();

	// Runtime updates
	bool _can_enable_collider() const;
	void _update_collider_disabled_state();
	void _update_trigger_disabled_state();
	void _update_physics_mode();
	void _enable_collision();
	void _disable_collision();

	bool _ensure_material_ready(const char *p_context, GdBool p_create_if_missing = false);
	bool _update_anim_scale();
	bool _update_svg_scale_content(int p_target_scale);
	void _update_svg_animation_scale(int p_target_scale);
	void _on_frame_changed();
	void _update_current_frame_shader_uv_rect();
	void _play_single_image_animation(Ref<Texture2D> p_texture);
	Vector2 _get_actual_render_scale();
	int _get_actual_match_render_scale();

	// State
	GdObj gid = 0;
	Vector2 render_offset;

	PhysicsMode physics_mode = NO_PHYSICS;
	bool use_gravity = true;
	float gravity_scale = 1.0f;
	float mass_value = 1.0f;
	float drag_value = 0.0f;
	float friction_value = 300.0f;
	Vector2 external_forces = Vector2();
	Vector2 applied_forces = Vector2();
	float _gravity = 980.0f;

	bool _is_collision_enabled = true;
	bool _is_trigger_enabled = true;
	bool debug_collision_visible = true;
	bool use_default_frames = false;
	bool enable_dynamic_frame_offset = true;
	bool is_single_image_mode = false;
	bool is_svg_mode = false;
	bool is_backdrop = false;

	int current_svg_scale = 1;

	Vector2 base_offset = Vector2(0, 0);
	Vector2 _render_scale = Vector2(1.0f, 1.0f);

	String spx_type_name;
	String current_svg_path;
	String current_svg_anim_key;
	String current_anim_name = "";

	Ref<SpriteFrames> default_sprite_frames;
	Ref<ShaderMaterial> default_material;

	Area2D *area2d = nullptr;
	CollisionShape2D *trigger2d = nullptr;
	CollisionShape2D *collider2d = nullptr;
	VisibleOnScreenNotifier2D *visible_notifier = nullptr;
	AnimatedSprite2D *anim2d = nullptr;
	Node2D *render_root = nullptr;
};

template <typename T>
T *SpxSprite::_get_component(Node *p_node, GdBool p_recursive) {
	for (int i = 0; i < p_node->get_child_count(); ++i) {
		Node *child = p_node->get_child(i);
		T *component = Object::cast_to<T>(child);
		if (component != nullptr) {
			return component;
		}
		if (p_recursive) {
			component = _get_component<T>(child, true);
			if (component != nullptr) {
				return component;
			}
		}
	}
	return nullptr;
}

template <typename T>
T *SpxSprite::_get_component(GdBool p_recursive) {
	return _get_component<T>(this, p_recursive);
}
#endif // SPX_SPRITE_H
