#ifndef GDEXTENSION_SPX_WRAP_H
#define GDEXTENSION_SPX_WRAP_H

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#define NOT_GODOT_ENGINE
#include "gdextension_spx_ext.h"
#ifndef __cplusplus
typedef uint32_t char32_t;
typedef uint16_t char16_t;
#endif

#ifdef __cplusplus
extern "C" {
#endif

typedef uintptr_t pointer;

extern void initialize(void *userdata, GDExtensionInitializationLevel p_level);
extern void deinitialize(void *userdata, GDExtensionInitializationLevel p_level);

static inline void initialization(GDExtensionInitialization *p_init) {
	p_init->initialize = initialize;
	p_init->deinitialize = deinitialize;
}

static inline void *get_proc_address(uintptr_t fn, const char* p_name) {
	return (void *)((GDExtensionInterfaceGetProcAddress)fn)(p_name);
}

// engine
extern void func_on_engine_start();  
extern void func_on_engine_update(GdFloat delta);  
extern void func_on_engine_fixed_update(GdFloat delta);  
extern void func_on_engine_destroy();  
extern void func_on_engine_destroyed();
extern void func_on_engine_reset();
extern void func_on_engine_pause(GdBool is_paused);  

extern void func_on_scene_sprite_instantiated(GdInt id,GdString type_name);  

// sprite
extern void func_on_sprite_ready(GdInt id);  
extern void func_on_sprite_updated(GdFloat id);  
extern void func_on_sprite_fixed_updated(GdFloat id);  
extern void func_on_sprite_destroyed(GdInt id);  

extern void func_on_sprite_screen_entered(GdInt id);
extern void func_on_sprite_screen_exited(GdInt id);
extern void func_on_sprite_vfx_finished(GdInt id);
extern void func_on_sprite_animation_finished(GdInt id);
extern void func_on_sprite_animation_looped(GdInt id);
extern void func_on_sprite_frame_changed(GdInt id);
extern void func_on_sprite_animation_changed(GdInt id);
extern void func_on_sprite_frames_set_changed(GdInt id);

// input
extern void func_on_mouse_pressed(GdInt keyid);  
extern void func_on_mouse_released(GdInt keyid);  
extern void func_on_key_pressed(GdInt keyid);  
extern void func_on_key_released(GdInt keyid);  
extern void func_on_action_pressed(GdString action_name);  
extern void func_on_action_just_pressed(GdString action_name);  
extern void func_on_action_just_released(GdString action_name);  
extern void func_on_axis_changed(GdString action_name, GdFloat value);  
// physics
extern void func_on_collision_enter(GdInt self_id, GdInt other_id);  
extern void func_on_collision_stay(GdInt self_id, GdInt other_id);  
extern void func_on_collision_exit(GdInt self_id, GdInt other_id);  
extern void func_on_trigger_enter(GdInt self_id, GdInt other_id);  
extern void func_on_trigger_stay(GdInt self_id, GdInt other_id);  
extern void func_on_trigger_exit(GdInt self_id, GdInt other_id); 
// ui 
extern void func_on_ui_ready(GdInt id);  
extern void func_on_ui_updated(GdInt id);  
extern void func_on_ui_destroyed(GdInt id); 

extern void func_on_ui_pressed(GdInt id);  
extern void func_on_ui_released(GdInt id);  
extern void func_on_ui_hovered(GdInt id);  
extern void func_on_ui_clicked(GdInt id);  
extern void func_on_ui_toggle(GdInt id, GdBool is_on);  
extern void func_on_ui_text_changed(GdInt id, GdString text);  

static inline void spx_global_register_callbacks(pointer fn) {
	SpxCallbackInfo info = {0};
	// engine
	info.func_on_engine_start = func_on_engine_start;
	info.func_on_engine_update = func_on_engine_update;
	info.func_on_engine_fixed_update = func_on_engine_fixed_update;
	info.func_on_engine_destroy = func_on_engine_destroy;
	info.func_on_engine_destroyed = func_on_engine_destroyed;
	info.func_on_engine_reset = func_on_engine_reset;
	info.func_on_engine_pause = func_on_engine_pause;

	info.func_on_scene_sprite_instantiated = func_on_scene_sprite_instantiated;

	// sprite
	info.func_on_sprite_ready = func_on_sprite_ready;
	info.func_on_sprite_updated = func_on_sprite_updated;
	info.func_on_sprite_fixed_updated = func_on_sprite_fixed_updated;
	info.func_on_sprite_destroyed = func_on_sprite_destroyed;
	// animation
	info.func_on_sprite_frames_set_changed = func_on_sprite_frames_set_changed;
	info.func_on_sprite_animation_changed = func_on_sprite_animation_changed;
	info.func_on_sprite_frame_changed = func_on_sprite_frame_changed;
	info.func_on_sprite_animation_looped = func_on_sprite_animation_looped;
	info.func_on_sprite_animation_finished = func_on_sprite_animation_finished;
	// vfx
	info.func_on_sprite_vfx_finished = func_on_sprite_vfx_finished;
	// visibility
	info.func_on_sprite_screen_exited = func_on_sprite_screen_exited;
	info.func_on_sprite_screen_entered = func_on_sprite_screen_entered;

	// input
	info.func_on_mouse_pressed = func_on_mouse_pressed;
	info.func_on_mouse_released = func_on_mouse_released;
	info.func_on_key_pressed = func_on_key_pressed;
	info.func_on_key_released = func_on_key_released;
	info.func_on_action_pressed = func_on_action_pressed;
	info.func_on_action_just_pressed = func_on_action_just_pressed;
	info.func_on_action_just_released = func_on_action_just_released;
	info.func_on_axis_changed = func_on_axis_changed;
	// physics
	info.func_on_collision_enter = func_on_collision_enter;
	info.func_on_collision_stay = func_on_collision_stay;
	info.func_on_collision_exit = func_on_collision_exit;
	info.func_on_trigger_enter = func_on_trigger_enter;
	info.func_on_trigger_stay = func_on_trigger_stay;
	info.func_on_trigger_exit = func_on_trigger_exit;
	// ui
	info.func_on_ui_ready = func_on_ui_ready;
	info.func_on_ui_updated = func_on_ui_updated;
	info.func_on_ui_destroyed = func_on_ui_destroyed;

	info.func_on_ui_pressed = func_on_ui_pressed;
	info.func_on_ui_released = func_on_ui_released;
	info.func_on_ui_hovered = func_on_ui_hovered;
	info.func_on_ui_clicked = func_on_ui_clicked;
	info.func_on_ui_toggle = func_on_ui_toggle;
	info.func_on_ui_text_changed = func_on_ui_text_changed;
	((GDExtensionSpxGlobalRegisterCallbacks)fn)((GDExtensionSpxCallbackInfoPtr)&info);
}


#ifdef __cplusplus
}
#endif

#endif // GDEXTENSION_SPX_WRAP_H
