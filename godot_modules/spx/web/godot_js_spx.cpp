/**************************************************************************/
/*  godot_js_spx.cpp                                               */
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

#include "../gdextension_spx_ext.h"
#include "core/extension/gdextension.h"
#include "core/extension/gdextension_special_compat_hashes.h"
#include "core/variant/variant.h"
#include "core/extension/gdextension_interface.h"
#include "scene/main/window.h"
#include "../spx_engine.h"
#include "../spx_audio_mgr.h"
#include "../spx_camera_mgr.h"
#include "../spx_debug_mgr.h"
#include "../spx_ext_mgr.h"
#include "../spx_input_mgr.h"
#include "../spx_navigation_mgr.h"
#include "../spx_pen_mgr.h"
#include "../spx_physics_mgr.h"
#include "../spx_platform_mgr.h"
#include "../spx_res_mgr.h"
#include "../spx_scene_mgr.h"
#include "../spx_sprite_mgr.h"
#include "../spx_tilemap_mgr.h"
#include "../spx_tilemapparser_mgr.h"
#include "../spx_ui_mgr.h"

#include <emscripten.h>
#define audioMgr SpxEngine::get_singleton()->get_audio()
#define cameraMgr SpxEngine::get_singleton()->get_camera()
#define debugMgr SpxEngine::get_singleton()->get_debug()
#define extMgr SpxEngine::get_singleton()->get_ext()
#define inputMgr SpxEngine::get_singleton()->get_input()
#define navigationMgr SpxEngine::get_singleton()->get_navigation()
#define penMgr SpxEngine::get_singleton()->get_pen()
#define physicsMgr SpxEngine::get_singleton()->get_physics()
#define platformMgr SpxEngine::get_singleton()->get_platform()
#define resMgr SpxEngine::get_singleton()->get_res()
#define sceneMgr SpxEngine::get_singleton()->get_scene()
#define spriteMgr SpxEngine::get_singleton()->get_sprite()
#define tilemapMgr SpxEngine::get_singleton()->get_tilemap()
#define tilemapparserMgr SpxEngine::get_singleton()->get_tilemapparser()
#define uiMgr SpxEngine::get_singleton()->get_ui()

extern "C" {
// memory allocator for wrap codes
EMSCRIPTEN_KEEPALIVE
void* cmalloc(int size) {
	auto ptr = malloc(size);
	return ptr;
}
EMSCRIPTEN_KEEPALIVE
void cfree(void* ptr) {
	free(ptr);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_stop_all() {
	 audioMgr->stop_all();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_create_audio(GdObj *ret_val) {
	*ret_val = audioMgr->create_audio();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_destroy_audio(GdObj *obj) {
	 audioMgr->destroy_audio(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_set_pitch(GdObj *obj, GdFloat *pitch) {
	 audioMgr->set_pitch(*obj, *pitch);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_get_pitch(GdObj *obj, GdFloat *ret_val) {
	*ret_val = audioMgr->get_pitch(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_set_pan(GdObj *obj, GdFloat *pan) {
	 audioMgr->set_pan(*obj, *pan);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_get_pan(GdObj *obj, GdFloat *ret_val) {
	*ret_val = audioMgr->get_pan(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_set_volume(GdObj *obj, GdFloat *volume) {
	 audioMgr->set_volume(*obj, *volume);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_get_volume(GdObj *obj, GdFloat *ret_val) {
	*ret_val = audioMgr->get_volume(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_play_with_attenuation(GdObj *obj, GdString *path, GdObj *owner_id, GdFloat *attenuation, GdFloat *max_distance, GdInt *ret_val) {
	*ret_val = audioMgr->play_with_attenuation(*obj, *path, *owner_id, *attenuation, *max_distance);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_play(GdObj *obj, GdString *path, GdInt *ret_val) {
	*ret_val = audioMgr->play(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_pause(GdInt *aid) {
	 audioMgr->pause(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_resume(GdInt *aid) {
	 audioMgr->resume(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_stop(GdInt *aid) {
	 audioMgr->stop(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_restart(GdInt *aid, GdBool *ret_val) {
	*ret_val = audioMgr->restart(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_set_loop(GdInt *aid, GdBool *loop) {
	 audioMgr->set_loop(*aid, *loop);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_get_loop(GdInt *aid, GdBool *ret_val) {
	*ret_val = audioMgr->get_loop(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_get_timer(GdInt *aid, GdFloat *ret_val) {
	*ret_val = audioMgr->get_timer(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_set_timer(GdInt *aid, GdFloat *time) {
	 audioMgr->set_timer(*aid, *time);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_audio_is_playing(GdInt *aid, GdBool *ret_val) {
	*ret_val = audioMgr->is_playing(*aid);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_get_camera_position(GdVec2 *ret_val) {
	*ret_val = cameraMgr->get_camera_position();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_set_camera_position(GdVec2 *position) {
	 cameraMgr->set_camera_position(*position);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_get_camera_zoom(GdVec2 *ret_val) {
	*ret_val = cameraMgr->get_camera_zoom();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_set_camera_zoom(GdVec2 *size) {
	 cameraMgr->set_camera_zoom(*size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_get_viewport_rect(GdRect2 *ret_val) {
	*ret_val = cameraMgr->get_viewport_rect();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_get_global_camera_rect(GdRect2 *ret_val) {
	*ret_val = cameraMgr->get_global_camera_rect();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_get_stage_limits_rect(GdRect2 *ret_val) {
	*ret_val = cameraMgr->get_stage_limits_rect();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_set_camera_limit(GdInt *side, GdInt *limit) {
	 cameraMgr->set_camera_limit(*side, *limit);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_camera_set_camera_smoothing(GdBool *enabled) {
	 cameraMgr->set_camera_smoothing(*enabled);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_debug_debug_draw_circle(GdVec2 *pos, GdFloat *radius, GdColor *color) {
	 debugMgr->debug_draw_circle(*pos, *radius, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_debug_debug_draw_rect(GdVec2 *pos, GdVec2 *size, GdColor *color) {
	 debugMgr->debug_draw_rect(*pos, *size, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_debug_debug_draw_line(GdVec2 *from, GdVec2 *to, GdColor *color) {
	 debugMgr->debug_draw_line(*from, *to, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_request_exit(GdInt *exit_code) {
	 extMgr->request_exit(*exit_code);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_request_reset(GdInt *exit_code) {
	 extMgr->request_reset(*exit_code);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_request_restart() {
	 extMgr->request_restart();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_on_runtime_panic(GdString *msg) {
	 extMgr->on_runtime_panic(*msg);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_pause() {
	 extMgr->pause();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_resume() {
	 extMgr->resume();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_is_paused(GdBool *ret_val) {
	*ret_val = extMgr->is_paused();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_next_frame() {
	 extMgr->next_frame();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ext_set_layer_sorter_mode(GdInt *mode) {
	 extMgr->set_layer_sorter_mode(*mode);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_global_mouse_pos(GdVec2 *ret_val) {
	*ret_val = inputMgr->get_global_mouse_pos();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_key(GdInt *key, GdBool *ret_val) {
	*ret_val = inputMgr->get_key(*key);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_mouse_state(GdInt *mouse_id, GdBool *ret_val) {
	*ret_val = inputMgr->get_mouse_state(*mouse_id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_key_state(GdInt *key, GdInt *ret_val) {
	*ret_val = inputMgr->get_key_state(*key);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_axis(GdString *neg_action, GdString *pos_action, GdFloat *ret_val) {
	*ret_val = inputMgr->get_axis(*neg_action, *pos_action);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_pressed(GdString *action, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_pressed(*action);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_just_pressed(GdString *action, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_just_pressed(*action);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_just_released(GdString *action, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_just_released(*action);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_register_action(GdString *action, GdInt *ret_val) {
	*ret_val = inputMgr->register_action(*action);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_get_axis_id(GdInt *neg_action_id, GdInt *pos_action_id, GdFloat *ret_val) {
	*ret_val = inputMgr->get_axis_id(*neg_action_id, *pos_action_id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_pressed_id(GdInt *action_id, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_pressed_id(*action_id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_just_pressed_id(GdInt *action_id, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_just_pressed_id(*action_id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_is_action_just_released_id(GdInt *action_id, GdBool *ret_val) {
	*ret_val = inputMgr->is_action_just_released_id(*action_id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_input_write_snapshot(float *out, int len) {
	inputMgr->write_snapshot(out, len);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_navigation_setup_path_finder_with_size(GdVec2 *grid_size, GdVec2 *cell_size, GdBool *with_jump, GdBool *with_debug) {
	 navigationMgr->setup_path_finder_with_size(*grid_size, *cell_size, *with_jump, *with_debug);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_navigation_setup_path_finder(GdBool *with_jump) {
	 navigationMgr->setup_path_finder(*with_jump);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_navigation_set_obstacle(GdObj *obj, GdBool *enabled) {
	 navigationMgr->set_obstacle(*obj, *enabled);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_navigation_find_path(GdVec2 *p_from, GdVec2 *p_to, GdBool *with_jump, GdArray *ret_val) {
	*ret_val = navigationMgr->find_path(*p_from, *p_to, *with_jump);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_destroy_all_pens() {
	 penMgr->destroy_all_pens();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_set_canvas_size(GdInt *width, GdInt *height) {
	 penMgr->set_canvas_size(*width, *height);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_create_pen(GdObj *ret_val) {
	*ret_val = penMgr->create_pen();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_destroy_pen(GdObj *obj) {
	 penMgr->destroy_pen(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_batch_update_commands(const float *buffer_data, int len) {
	penMgr->batch_update_commands(buffer_data, len);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_pen_stamp(GdObj *obj) {
	 penMgr->pen_stamp(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_move_pen_to(GdObj *obj, GdVec2 *position) {
	 penMgr->move_pen_to(*obj, *position);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_pen_down(GdObj *obj, GdBool *move_by_mouse) {
	 penMgr->pen_down(*obj, *move_by_mouse);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_pen_up(GdObj *obj) {
	 penMgr->pen_up(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_set_pen_color_to(GdObj *obj, GdColor *color) {
	 penMgr->set_pen_color_to(*obj, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_change_pen_by(GdObj *obj, GdInt *property, GdFloat *amount) {
	 penMgr->change_pen_by(*obj, *property, *amount);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_set_pen_to(GdObj *obj, GdInt *property, GdFloat *value) {
	 penMgr->set_pen_to(*obj, *property, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_change_pen_size_by(GdObj *obj, GdFloat *amount) {
	 penMgr->change_pen_size_by(*obj, *amount);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_set_pen_size_to(GdObj *obj, GdFloat *size) {
	 penMgr->set_pen_size_to(*obj, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_set_pen_stamp_texture(GdObj *obj, GdString *texture_path) {
	 penMgr->set_pen_stamp_texture(*obj, *texture_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_pen_pen_stamp_with_transform(GdObj *obj, GdString *texture_path, GdVec2 *position, GdFloat *rotation_radians, GdVec2 *scale) {
	 penMgr->pen_stamp_with_transform(*obj, *texture_path, *position, *rotation_radians, *scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_raycast(GdVec2 *from, GdVec2 *to, GdInt *collision_mask, GdObj *ret_val) {
	*ret_val = physicsMgr->raycast(*from, *to, *collision_mask);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_collision(GdVec2 *from, GdVec2 *to, GdInt *collision_mask, GdBool *collide_with_areas, GdBool *collide_with_bodies, GdBool *ret_val) {
	*ret_val = physicsMgr->check_collision(*from, *to, *collision_mask, *collide_with_areas, *collide_with_bodies);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_touched_camera_boundaries(GdObj *obj, GdInt *ret_val) {
	*ret_val = physicsMgr->check_touched_camera_boundaries(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_touched_camera_boundary(GdObj *obj, GdInt *board_type, GdBool *ret_val) {
	*ret_val = physicsMgr->check_touched_camera_boundary(*obj, *board_type);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_nearest_touched_camera_boundary(GdObj *obj, GdInt *ret_val) {
	*ret_val = physicsMgr->check_nearest_touched_camera_boundary(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_touched_stage_boundaries(GdObj *obj, GdInt *ret_val) {
	*ret_val = physicsMgr->check_touched_stage_boundaries(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_touched_stage_boundary(GdObj *obj, GdInt *board_type, GdBool *ret_val) {
	*ret_val = physicsMgr->check_touched_stage_boundary(*obj, *board_type);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_nearest_touched_stage_boundary(GdObj *obj, GdInt *ret_val) {
	*ret_val = physicsMgr->check_nearest_touched_stage_boundary(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_set_collision_system_type(GdBool *is_collision_by_alpha) {
	 physicsMgr->set_collision_system_type(*is_collision_by_alpha);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_set_global_gravity(GdFloat *gravity) {
	 physicsMgr->set_global_gravity(*gravity);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_get_global_gravity(GdFloat *ret_val) {
	*ret_val = physicsMgr->get_global_gravity();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_set_global_friction(GdFloat *friction) {
	 physicsMgr->set_global_friction(*friction);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_get_global_friction(GdFloat *ret_val) {
	*ret_val = physicsMgr->get_global_friction();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_set_global_air_drag(GdFloat *air_drag) {
	 physicsMgr->set_global_air_drag(*air_drag);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_get_global_air_drag(GdFloat *ret_val) {
	*ret_val = physicsMgr->get_global_air_drag();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_collision_rect(GdVec2 *pos, GdVec2 *size, GdInt *collision_mask, GdArray *ret_val) {
	*ret_val = physicsMgr->check_collision_rect(*pos, *size, *collision_mask);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_check_collision_circle(GdVec2 *pos, GdFloat *radius, GdInt *collision_mask, GdArray *ret_val) {
	*ret_val = physicsMgr->check_collision_circle(*pos, *radius, *collision_mask);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_physics_raycast_with_details(GdVec2 *from, GdVec2 *to, GdArray *ignore_sprites, GdInt *collision_mask, GdBool *collide_with_areas, GdBool *collide_with_bodies, GdArray *ret_val) {
	*ret_val = physicsMgr->raycast_with_details(*from, *to, *ignore_sprites, *collision_mask, *collide_with_areas, *collide_with_bodies);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_stretch_mode(GdBool *enable) {
	 platformMgr->set_stretch_mode(*enable);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_stretch_aspect(GdBool *is_keep) {
	 platformMgr->set_stretch_aspect(*is_keep);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_stretch_content_scale(GdInt *width, GdInt *height) {
	 platformMgr->set_stretch_content_scale(*width, *height);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_window_position(GdVec2 *pos) {
	 platformMgr->set_window_position(*pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_window_position(GdVec2 *ret_val) {
	*ret_val = platformMgr->get_window_position();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_window_size(GdInt *width, GdInt *height, GdBool *with_content_scale) {
	 platformMgr->set_window_size(*width, *height, *with_content_scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_window_size(GdVec2 *ret_val) {
	*ret_val = platformMgr->get_window_size();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_window_title(GdString *title) {
	 platformMgr->set_window_title(*title);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_window_title(GdString *ret_val) {
	*ret_val = platformMgr->get_window_title();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_window_fullscreen(GdBool *enable) {
	 platformMgr->set_window_fullscreen(*enable);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_is_window_fullscreen(GdBool *ret_val) {
	*ret_val = platformMgr->is_window_fullscreen();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_debug_mode(GdBool *enable) {
	 platformMgr->set_debug_mode(*enable);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_is_debug_mode(GdBool *ret_val) {
	*ret_val = platformMgr->is_debug_mode();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_is_main_thread(GdBool *ret_val) {
	*ret_val = platformMgr->is_main_thread();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_time_scale(GdFloat *ret_val) {
	*ret_val = platformMgr->get_time_scale();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_time_scale(GdFloat *time_scale) {
	 platformMgr->set_time_scale(*time_scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_max_fps(GdInt *ret_val) {
	*ret_val = platformMgr->get_max_fps();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_max_fps(GdInt *fps) {
	 platformMgr->set_max_fps(*fps);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_get_persistant_data_dir(GdString *ret_val) {
	*ret_val = platformMgr->get_persistant_data_dir();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_set_persistant_data_dir(GdString *path) {
	 platformMgr->set_persistant_data_dir(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_platform_is_in_persistant_data_dir(GdString *path, GdBool *ret_val) {
	*ret_val = platformMgr->is_in_persistant_data_dir(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_create_animation(GdString *p_sprite_type, GdString *p_anim_name, GdString *p_json_ctx, GdInt *fps, GdBool *is_atlas) {
	 resMgr->create_animation(*p_sprite_type, *p_anim_name, *p_json_ctx, *fps, *is_atlas);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_set_load_mode(GdBool *is_direct_mode) {
	 resMgr->set_load_mode(*is_direct_mode);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_get_load_mode(GdBool *ret_val) {
	*ret_val = resMgr->get_load_mode();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_get_bound_from_alpha(GdString *p_path, GdRect2 *ret_val) {
	*ret_val = resMgr->get_bound_from_alpha(*p_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_get_image_size(GdString *p_path, GdVec2 *ret_val) {
	*ret_val = resMgr->get_image_size(*p_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_read_all_text(GdString *p_path, GdString *ret_val) {
	*ret_val = resMgr->read_all_text(*p_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_has_file(GdString *p_path, GdBool *ret_val) {
	*ret_val = resMgr->has_file(*p_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_list_directories(GdString *p_path, GdString *ret_val) {
	*ret_val = resMgr->list_directories(*p_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_reload_texture(GdString *path) {
	 resMgr->reload_texture(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_free_str(GdString *str) {
	(void)str;
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_apply_project_fonts(GdString *default_font_path, GdArray *font_paths, GdArray *font_families, GdArray *preferences, GdString *ret_val) {
	*ret_val = resMgr->apply_project_fonts(*default_font_path, *font_paths, *font_families, *preferences);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_set_default_font(GdString *font_path) {
	 resMgr->set_default_font(*font_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_register_font_face(GdString *font_path, GdString *family) {
	 resMgr->register_font_face(*font_path, *family);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_res_set_font_preferences(GdArray *preferences) {
	 resMgr->set_font_preferences(*preferences);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_change_scene_to_file(GdString *path) {
	 sceneMgr->change_scene_to_file(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_destroy_all_sprites() {
	 sceneMgr->destroy_all_sprites();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_reload_current_scene(GdInt *ret_val) {
	*ret_val = sceneMgr->reload_current_scene();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_unload_current_scene() {
	 sceneMgr->unload_current_scene();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_clear_pure_sprites() {
	 sceneMgr->clear_pure_sprites();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_create_pure_sprite(GdString *texture_path, GdVec2 *pos, GdInt *zindex) {
	 sceneMgr->create_pure_sprite(*texture_path, *pos, *zindex);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_destroy_pure_sprite(GdObj *id) {
	 sceneMgr->destroy_pure_sprite(*id);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_create_render_sprite(GdString *texture_path, GdVec2 *pos, GdFloat *degree, GdVec2 *scale, GdInt *zindex, GdVec2 *pivot, GdObj *ret_val) {
	*ret_val = sceneMgr->create_render_sprite(*texture_path, *pos, *degree, *scale, *zindex, *pivot);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_scene_create_static_sprite(GdString *texture_path, GdVec2 *pos, GdFloat *degree, GdVec2 *scale, GdInt *zindex, GdVec2 *pivot, GdInt *collider_type, GdVec2 *collider_pivot, GdArray *collider_params, GdObj *ret_val) {
	*ret_val = sceneMgr->create_static_sprite(*texture_path, *pos, *degree, *scale, *zindex, *pivot, *collider_type, *collider_pivot, *collider_params);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_dont_destroy_on_load(GdObj *obj) {
	 spriteMgr->set_dont_destroy_on_load(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_process(GdObj *obj, GdBool *is_on) {
	 spriteMgr->set_process(*obj, *is_on);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_physic_process(GdObj *obj, GdBool *is_on) {
	 spriteMgr->set_physic_process(*obj, *is_on);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_type_name(GdObj *obj, GdString *type_name) {
	 spriteMgr->set_type_name(*obj, *type_name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_pivot(GdObj *obj, GdVec2 *pivot) {
	 spriteMgr->set_pivot(*obj, *pivot);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_pivot(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_pivot(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_child_position(GdObj *obj, GdString *path, GdVec2 *pos) {
	 spriteMgr->set_child_position(*obj, *path, *pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_child_position(GdObj *obj, GdString *path, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_child_position(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_child_rotation(GdObj *obj, GdString *path, GdFloat *rot) {
	 spriteMgr->set_child_rotation(*obj, *path, *rot);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_child_rotation(GdObj *obj, GdString *path, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_child_rotation(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_child_scale(GdObj *obj, GdString *path, GdVec2 *scale) {
	 spriteMgr->set_child_scale(*obj, *path, *scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_child_scale(GdObj *obj, GdString *path, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_child_scale(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision(GdObj *obj, GdObj *target, GdBool *is_src_trigger, GdBool *is_dst_trigger, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision(*obj, *target, *is_src_trigger, *is_dst_trigger);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision_with_point(GdObj *obj, GdVec2 *point, GdBool *is_click_query, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision_with_point(*obj, *point, *is_click_query);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_debug_collision_visible(GdObj *obj, GdBool *visible) {
	 spriteMgr->set_debug_collision_visible(*obj, *visible);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_debug_collision_visible(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_debug_collision_visible(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_create_backdrop(GdString *path, GdObj *ret_val) {
	*ret_val = spriteMgr->create_backdrop(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_create_sprite(GdString *path, GdVec2 *pos, GdObj *ret_val) {
	*ret_val = spriteMgr->create_sprite(*path, *pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_create_bare_sprite(GdVec2 *pos, GdObj *ret_val) {
	*ret_val = spriteMgr->create_bare_sprite(*pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_clone_sprite(GdObj *obj, GdObj *ret_val) {
	*ret_val = spriteMgr->clone_sprite(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_destroy_sprite(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->destroy_sprite(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_sprite_alive(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_sprite_alive(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_position(GdObj *obj, GdVec2 *pos) {
	 spriteMgr->set_position(*obj, *pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_transform(GdObj *obj, GdVec2 *pos, GdFloat *rot, GdVec2 *scale, GdBool *visible, GdVec2 *pivot) {
	 spriteMgr->set_transform(*obj, *pos, *rot, *scale, *visible, *pivot);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_position(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_position(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_rotation(GdObj *obj, GdFloat *rot) {
	 spriteMgr->set_rotation(*obj, *rot);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_rotation(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_rotation(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_scale(GdObj *obj, GdVec2 *scale) {
	 spriteMgr->set_scale(*obj, *scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_scale(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_scale(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_render_scale(GdObj *obj, GdVec2 *scale) {
	 spriteMgr->set_render_scale(*obj, *scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_render_scale(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_render_scale(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_color(GdObj *obj, GdColor *color) {
	 spriteMgr->set_color(*obj, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_color(GdObj *obj, GdColor *ret_val) {
	*ret_val = spriteMgr->get_color(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_material_shader(GdObj *obj, GdString *path) {
	 spriteMgr->set_material_shader(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_material_shader(GdObj *obj, GdString *ret_val) {
	*ret_val = spriteMgr->get_material_shader(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_material_params(GdObj *obj, GdString *effect, GdFloat *amount) {
	 spriteMgr->set_material_params(*obj, *effect, *amount);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_material_params(GdObj *obj, GdString *effect, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_material_params(*obj, *effect);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_material_params_vec(GdObj *obj, GdString *effect, GdFloat *x, GdFloat *y, GdFloat *z, GdFloat *w) {
	 spriteMgr->set_material_params_vec(*obj, *effect, *x, *y, *z, *w);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_material_params_vec4(GdObj *obj, GdString *effect, GdVec4 *vec4) {
	 spriteMgr->set_material_params_vec4(*obj, *effect, *vec4);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_material_params_vec4(GdObj *obj, GdString *effect, GdVec4 *ret_val) {
	*ret_val = spriteMgr->get_material_params_vec4(*obj, *effect);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_material_params_color(GdObj *obj, GdString *effect, GdColor *color) {
	 spriteMgr->set_material_params_color(*obj, *effect, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_material_params_color(GdObj *obj, GdString *effect, GdColor *ret_val) {
	*ret_val = spriteMgr->get_material_params_color(*obj, *effect);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_texture_atlas(GdObj *obj, GdString *path, GdRect2 *rect2) {
	 spriteMgr->set_texture_atlas(*obj, *path, *rect2);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_texture(GdObj *obj, GdString *path) {
	 spriteMgr->set_texture(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_texture_atlas_direct(GdObj *obj, GdString *path, GdRect2 *rect2) {
	 spriteMgr->set_texture_atlas_direct(*obj, *path, *rect2);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_texture_direct(GdObj *obj, GdString *path) {
	 spriteMgr->set_texture_direct(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_texture(GdObj *obj, GdString *ret_val) {
	*ret_val = spriteMgr->get_texture(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_visible(GdObj *obj, GdBool *visible) {
	 spriteMgr->set_visible(*obj, *visible);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_visible(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->get_visible(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_z_index(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_z_index(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_z_index(GdObj *obj, GdInt *z) {
	 spriteMgr->set_z_index(*obj, *z);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_play_anim(GdObj *obj, GdString *p_name, GdFloat *p_speed, GdBool *isLoop, GdBool *p_revert) {
	 spriteMgr->play_anim(*obj, *p_name, *p_speed, *isLoop, *p_revert);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_play_backwards_anim(GdObj *obj, GdString *p_name) {
	 spriteMgr->play_backwards_anim(*obj, *p_name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_pause_anim(GdObj *obj) {
	 spriteMgr->pause_anim(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_stop_anim(GdObj *obj) {
	 spriteMgr->stop_anim(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_playing_anim(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_playing_anim(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim(GdObj *obj, GdString *p_name) {
	 spriteMgr->set_anim(*obj, *p_name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_anim(GdObj *obj, GdString *ret_val) {
	*ret_val = spriteMgr->get_anim(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_frame(GdObj *obj, GdInt *p_frame) {
	 spriteMgr->set_anim_frame(*obj, *p_frame);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_anim_frame(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_anim_frame(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_speed_scale(GdObj *obj, GdFloat *p_speed_scale) {
	 spriteMgr->set_anim_speed_scale(*obj, *p_speed_scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_anim_speed_scale(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_anim_speed_scale(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_anim_playing_speed(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_anim_playing_speed(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_centered(GdObj *obj, GdBool *p_center) {
	 spriteMgr->set_anim_centered(*obj, *p_center);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_anim_centered(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_anim_centered(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_offset(GdObj *obj, GdVec2 *p_offset) {
	 spriteMgr->set_anim_offset(*obj, *p_offset);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_anim_offset(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_anim_offset(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_flip_h(GdObj *obj, GdBool *p_flip) {
	 spriteMgr->set_anim_flip_h(*obj, *p_flip);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_anim_flipped_h(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_anim_flipped_h(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_anim_flip_v(GdObj *obj, GdBool *p_flip) {
	 spriteMgr->set_anim_flip_v(*obj, *p_flip);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_anim_flipped_v(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_anim_flipped_v(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_current_anim_name(GdObj *obj, GdString *ret_val) {
	*ret_val = spriteMgr->get_current_anim_name(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_velocity(GdObj *obj, GdVec2 *velocity) {
	 spriteMgr->set_velocity(*obj, *velocity);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_velocity(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_velocity(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_floor(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_floor(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_floor_only(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_floor_only(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_wall(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_wall(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_wall_only(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_wall_only(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_ceiling(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_ceiling(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_on_ceiling_only(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_on_ceiling_only(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_last_motion(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_last_motion(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_position_delta(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_position_delta(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_floor_normal(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_floor_normal(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_wall_normal(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_wall_normal(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_real_velocity(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = spriteMgr->get_real_velocity(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_move_and_slide(GdObj *obj) {
	 spriteMgr->move_and_slide(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_gravity(GdObj *obj, GdFloat *gravity) {
	 spriteMgr->set_gravity(*obj, *gravity);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_gravity(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_gravity(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_mass(GdObj *obj, GdFloat *mass) {
	 spriteMgr->set_mass(*obj, *mass);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_mass(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_mass(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_add_force(GdObj *obj, GdVec2 *force) {
	 spriteMgr->add_force(*obj, *force);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_add_impulse(GdObj *obj, GdVec2 *impulse) {
	 spriteMgr->add_impulse(*obj, *impulse);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_physics_mode(GdObj *obj, GdInt *mode) {
	 spriteMgr->set_physics_mode(*obj, *mode);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_physics_mode(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_physics_mode(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_use_gravity(GdObj *obj, GdBool *enabled) {
	 spriteMgr->set_use_gravity(*obj, *enabled);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_use_gravity(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_use_gravity(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_gravity_scale(GdObj *obj, GdFloat *scale) {
	 spriteMgr->set_gravity_scale(*obj, *scale);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_gravity_scale(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_gravity_scale(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_drag(GdObj *obj, GdFloat *drag) {
	 spriteMgr->set_drag(*obj, *drag);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_drag(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_drag(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_friction(GdObj *obj, GdFloat *friction) {
	 spriteMgr->set_friction(*obj, *friction);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_friction(GdObj *obj, GdFloat *ret_val) {
	*ret_val = spriteMgr->get_friction(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collision_layer(GdObj *obj, GdInt *layer) {
	 spriteMgr->set_collision_layer(*obj, *layer);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_collision_layer(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_collision_layer(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collision_mask(GdObj *obj, GdInt *mask) {
	 spriteMgr->set_collision_mask(*obj, *mask);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_collision_mask(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_collision_mask(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_layer(GdObj *obj, GdInt *layer) {
	 spriteMgr->set_trigger_layer(*obj, *layer);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_trigger_layer(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_trigger_layer(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_mask(GdObj *obj, GdInt *mask) {
	 spriteMgr->set_trigger_mask(*obj, *mask);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_trigger_mask(GdObj *obj, GdInt *ret_val) {
	*ret_val = spriteMgr->get_trigger_mask(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collider_rect(GdObj *obj, GdVec2 *center, GdVec2 *size) {
	 spriteMgr->set_collider_rect(*obj, *center, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collider_circle(GdObj *obj, GdVec2 *center, GdFloat *radius) {
	 spriteMgr->set_collider_circle(*obj, *center, *radius);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collider_capsule(GdObj *obj, GdVec2 *center, GdVec2 *size) {
	 spriteMgr->set_collider_capsule(*obj, *center, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collider_polygon(GdObj *obj, GdVec2 *center, GdArray *points) {
	 spriteMgr->set_collider_polygon(*obj, *center, *points);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_collision_enabled(GdObj *obj, GdBool *enabled) {
	 spriteMgr->set_collision_enabled(*obj, *enabled);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_collision_enabled(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_collision_enabled(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_rect(GdObj *obj, GdVec2 *center, GdVec2 *size) {
	 spriteMgr->set_trigger_rect(*obj, *center, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_circle(GdObj *obj, GdVec2 *center, GdFloat *radius) {
	 spriteMgr->set_trigger_circle(*obj, *center, *radius);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_capsule(GdObj *obj, GdVec2 *center, GdVec2 *size) {
	 spriteMgr->set_trigger_capsule(*obj, *center, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_polygon(GdObj *obj, GdVec2 *center, GdArray *points) {
	 spriteMgr->set_trigger_polygon(*obj, *center, *points);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_trigger_enabled(GdObj *obj, GdBool *trigger) {
	 spriteMgr->set_trigger_enabled(*obj, *trigger);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_is_trigger_enabled(GdObj *obj, GdBool *ret_val) {
	*ret_val = spriteMgr->is_trigger_enabled(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision_by_color(GdObj *obj, GdColor *color, GdFloat *color_threshold, GdFloat *alpha_threshold, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision_by_color(*obj, *color, *color_threshold, *alpha_threshold);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision_by_colors(GdObj *obj, GdColor *sprite_color, GdColor *target_color, GdFloat *color_threshold, GdFloat *alpha_threshold, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision_by_colors(*obj, *sprite_color, *target_color, *color_threshold, *alpha_threshold);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision_by_alpha(GdObj *obj, GdFloat *alpha_threshold, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision_by_alpha(*obj, *alpha_threshold);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_check_collision_with_sprite(GdObj *obj, GdObj *obj_b, GdFloat *alpha_threshold, GdBool *use_pixel_perfect, GdBool *ret_val) {
	*ret_val = spriteMgr->check_collision_with_sprite(*obj, *obj_b, *alpha_threshold, *use_pixel_perfect);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_set_pixel_collision_sampling_step(GdInt *step) {
	 spriteMgr->set_pixel_collision_sampling_step(*step);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_get_pixel_collision_sampling_step(GdInt *ret_val) {
	*ret_val = spriteMgr->get_pixel_collision_sampling_step();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_batch_update_transforms(const float *buffer_data, int len) {
	spriteMgr->batch_update_transforms(buffer_data, len);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_batch_update_visuals(const float *buffer_data, int len) {
	spriteMgr->batch_update_visuals(buffer_data, len);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_batch_update_physics(const float *buffer_data, int len) {
	spriteMgr->batch_update_physics(buffer_data, len);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_open_draw_tiles_with_size(GdInt *tile_size) {
	 tilemapMgr->open_draw_tiles_with_size(*tile_size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_open_draw_tiles() {
	 tilemapMgr->open_draw_tiles();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_set_layer_index(GdInt *index) {
	 tilemapMgr->set_layer_index(*index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_set_tile(GdString *texture_path, GdBool *with_collision) {
	 tilemapMgr->set_tile(*texture_path, *with_collision);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_set_tile_with_collision_info(GdString *texture_path, GdArray *collision_points) {
	 tilemapMgr->set_tile_with_collision_info(*texture_path, *collision_points);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_set_layer_offset(GdInt *index, GdVec2 *offset) {
	 tilemapMgr->set_layer_offset(*index, *offset);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_get_layer_offset(GdInt *index, GdVec2 *ret_val) {
	*ret_val = tilemapMgr->get_layer_offset(*index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_place_tiles(GdArray *positions, GdString *texture_path) {
	 tilemapMgr->place_tiles(*positions, *texture_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_place_tiles_with_layer(GdArray *positions, GdString *texture_path, GdInt *layer_index) {
	 tilemapMgr->place_tiles_with_layer(*positions, *texture_path, *layer_index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_place_tile(GdVec2 *pos, GdString *texture_path) {
	 tilemapMgr->place_tile(*pos, *texture_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_place_tile_with_layer(GdVec2 *pos, GdString *texture_path, GdInt *layer_index) {
	 tilemapMgr->place_tile_with_layer(*pos, *texture_path, *layer_index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_erase_tile(GdVec2 *pos) {
	 tilemapMgr->erase_tile(*pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_erase_tile_with_layer(GdVec2 *pos, GdInt *layer_index) {
	 tilemapMgr->erase_tile_with_layer(*pos, *layer_index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_get_tile(GdVec2 *pos, GdString *ret_val) {
	*ret_val = tilemapMgr->get_tile(*pos);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_get_tile_with_layer(GdVec2 *pos, GdInt *layer_index, GdString *ret_val) {
	*ret_val = tilemapMgr->get_tile_with_layer(*pos, *layer_index);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_close_draw_tiles() {
	 tilemapMgr->close_draw_tiles();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemap_exit_tilemap_editor_mode() {
	 tilemapMgr->exit_tilemap_editor_mode();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemapparser_load_tilemap(GdString *json_path) {
	 tilemapparserMgr->load_tilemap(*json_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemapparser_unload_tilemap(GdString *name) {
	 tilemapparserMgr->unload_tilemap(*name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemapparser_destroy_all_tilemaps() {
	 tilemapparserMgr->destroy_all_tilemaps();
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemapparser_has_tilemap(GdString *name, GdBool *ret_val) {
	*ret_val = tilemapparserMgr->has_tilemap(*name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_tilemapparser_get_tilemap_layer_count(GdString *name, GdInt *ret_val) {
	*ret_val = tilemapparserMgr->get_tilemap_layer_count(*name);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_bind_node(GdObj *obj, GdString *rel_path, GdObj *ret_val) {
	*ret_val = uiMgr->bind_node(*obj, *rel_path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_node(GdString *path, GdObj *ret_val) {
	*ret_val = uiMgr->create_node(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_button(GdString *path, GdString *text, GdObj *ret_val) {
	*ret_val = uiMgr->create_button(*path, *text);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_label(GdString *path, GdString *text, GdObj *ret_val) {
	*ret_val = uiMgr->create_label(*path, *text);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_image(GdString *path, GdObj *ret_val) {
	*ret_val = uiMgr->create_image(*path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_toggle(GdString *path, GdBool *value, GdObj *ret_val) {
	*ret_val = uiMgr->create_toggle(*path, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_slider(GdString *path, GdFloat *value, GdObj *ret_val) {
	*ret_val = uiMgr->create_slider(*path, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_create_input(GdString *path, GdString *text, GdObj *ret_val) {
	*ret_val = uiMgr->create_input(*path, *text);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_destroy_node(GdObj *obj, GdBool *ret_val) {
	*ret_val = uiMgr->destroy_node(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_type(GdObj *obj, GdInt *ret_val) {
	*ret_val = uiMgr->get_type(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_text(GdObj *obj, GdString *text) {
	 uiMgr->set_text(*obj, *text);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_text(GdObj *obj, GdString *ret_val) {
	*ret_val = uiMgr->get_text(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_texture(GdObj *obj, GdString *path) {
	 uiMgr->set_texture(*obj, *path);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_texture(GdObj *obj, GdString *ret_val) {
	*ret_val = uiMgr->get_texture(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_color(GdObj *obj, GdColor *color) {
	 uiMgr->set_color(*obj, *color);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_color(GdObj *obj, GdColor *ret_val) {
	*ret_val = uiMgr->get_color(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_font_size(GdObj *obj, GdInt *size) {
	 uiMgr->set_font_size(*obj, *size);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_font_size(GdObj *obj, GdInt *ret_val) {
	*ret_val = uiMgr->get_font_size(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_visible(GdObj *obj, GdBool *visible) {
	 uiMgr->set_visible(*obj, *visible);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_visible(GdObj *obj, GdBool *ret_val) {
	*ret_val = uiMgr->get_visible(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_interactable(GdObj *obj, GdBool *interactable) {
	 uiMgr->set_interactable(*obj, *interactable);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_interactable(GdObj *obj, GdBool *ret_val) {
	*ret_val = uiMgr->get_interactable(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_rect(GdObj *obj, GdRect2 *rect) {
	 uiMgr->set_rect(*obj, *rect);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_rect(GdObj *obj, GdRect2 *ret_val) {
	*ret_val = uiMgr->get_rect(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_layout_direction(GdObj *obj, GdInt *ret_val) {
	*ret_val = uiMgr->get_layout_direction(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_layout_direction(GdObj *obj, GdInt *value) {
	 uiMgr->set_layout_direction(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_layout_mode(GdObj *obj, GdInt *ret_val) {
	*ret_val = uiMgr->get_layout_mode(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_layout_mode(GdObj *obj, GdInt *value) {
	 uiMgr->set_layout_mode(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_anchors_preset(GdObj *obj, GdInt *ret_val) {
	*ret_val = uiMgr->get_anchors_preset(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_anchors_preset(GdObj *obj, GdInt *value) {
	 uiMgr->set_anchors_preset(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_scale(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = uiMgr->get_scale(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_scale(GdObj *obj, GdVec2 *value) {
	 uiMgr->set_scale(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_position(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = uiMgr->get_position(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_position(GdObj *obj, GdVec2 *value) {
	 uiMgr->set_position(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_size(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = uiMgr->get_size(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_size(GdObj *obj, GdVec2 *value) {
	 uiMgr->set_size(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_global_position(GdObj *obj, GdVec2 *ret_val) {
	*ret_val = uiMgr->get_global_position(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_global_position(GdObj *obj, GdVec2 *value) {
	 uiMgr->set_global_position(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_rotation(GdObj *obj, GdFloat *ret_val) {
	*ret_val = uiMgr->get_rotation(*obj);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_rotation(GdObj *obj, GdFloat *value) {
	 uiMgr->set_rotation(*obj, *value);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_get_flip(GdObj *obj, GdBool *horizontal, GdBool *ret_val) {
	*ret_val = uiMgr->get_flip(*obj, *horizontal);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_ui_set_flip(GdObj *obj, GdBool *horizontal, GdBool *is_flip) {
	 uiMgr->set_flip(*obj, *horizontal, *is_flip);
}
EMSCRIPTEN_KEEPALIVE
void gdspx_sprite_batch_retrieve_positions(const GdObj *ids, int count, float *out, int out_len) {
	spriteMgr->batch_retrieve_positions(ids, count, out, out_len);
}

}
