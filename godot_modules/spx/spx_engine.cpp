/**************************************************************************/
/*  spx_engine.cpp                                                        */
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

#include "spx_engine.h"

#include "core/os/memory.h"
#include "core/os/thread.h"
#include "scene/gui/texture_rect.h"
#include "scene/main/canvas_layer.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "servers/display_server.h"

#include "gdextension_spx_ext.h"
#include "spx_audio_mgr.h"
#include "spx_callback_proxy.h"
#include "spx_camera_mgr.h"
#include "spx_debug_mgr.h"
#include "spx_ext_mgr.h"
#include "spx_input_mgr.h"
#include "spx_navigation_mgr.h"
#include "spx_pen_mgr.h"
#include "spx_physics_mgr.h"
#include "spx_platform_mgr.h"
#include "spx_res_mgr.h"
#include "spx_scene_mgr.h"
#include "spx_sprite_mgr.h"
#include "spx_tilemap_mgr.h"
#include "spx_tilemapparser_mgr.h"
#include "spx_ui_mgr.h"
#include "svg_mgr.h"

void SpxEngine::register_runtime_panic_callbacks(GDExtensionSpxGlobalRuntimePanicCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_panic, callback, __func__);
}

void SpxEngine::register_runtime_exit_callbacks(GDExtensionSpxGlobalRuntimeExitCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_exit, callback, __func__);
}

void SpxEngine::register_runtime_reset_callbacks(GDExtensionSpxGlobalRuntimeResetCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_reset, callback, __func__);
}

static SpxCallbackInfo get_default_spx_callbacks() {
	SpxCallbackInfo callbacks;
	callbacks.func_on_engine_start = []() {};
	callbacks.func_on_engine_fixed_update = [](GdFloat delta) {};
	callbacks.func_on_engine_update = [](GdFloat delta) {};
	callbacks.func_on_engine_destroy = []() {};
	callbacks.func_on_engine_reset = []() {};
	callbacks.func_on_engine_pause = [](GdBool is_paused) {};
	callbacks.func_on_scene_sprite_instantiated = [](GdObj obj, GdString type_name) {};
	callbacks.func_on_sprite_ready = [](GdObj obj) {};
	callbacks.func_on_sprite_updated = [](GdFloat delta) {};
	callbacks.func_on_sprite_fixed_updated = [](GdFloat delta) {};
	callbacks.func_on_sprite_destroyed = [](GdObj obj) {};
	callbacks.func_on_sprite_frames_set_changed = [](GdObj obj) {};
	callbacks.func_on_sprite_animation_changed = [](GdObj obj) {};
	callbacks.func_on_sprite_frame_changed = [](GdObj obj) {};
	callbacks.func_on_sprite_animation_looped = [](GdObj obj) {};
	callbacks.func_on_sprite_animation_finished = [](GdObj obj) {};
	callbacks.func_on_sprite_vfx_finished = [](GdObj obj) {};
	callbacks.func_on_sprite_screen_exited = [](GdObj obj) {};
	callbacks.func_on_sprite_screen_entered = [](GdObj obj) {};
	callbacks.func_on_mouse_pressed = [](GdInt keyid) {};
	callbacks.func_on_mouse_released = [](GdInt keyid) {};
	callbacks.func_on_key_pressed = [](GdInt keyid) {};
	callbacks.func_on_key_released = [](GdInt keyid) {};
	callbacks.func_on_action_pressed = [](GdString action_name) {};
	callbacks.func_on_action_just_pressed = [](GdString action_name) {};
	callbacks.func_on_action_just_released = [](GdString action_name) {};
	callbacks.func_on_axis_changed = [](GdString action_name, GdFloat value) {};
	callbacks.func_on_collision_enter = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_collision_stay = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_collision_exit = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_trigger_enter = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_trigger_stay = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_trigger_exit = [](GdInt self_id, GdInt other_id) {};
	callbacks.func_on_ui_ready = [](GdObj obj) {};
	callbacks.func_on_ui_updated = [](GdObj obj) {};
	callbacks.func_on_ui_destroyed = [](GdObj obj) {};
	callbacks.func_on_ui_pressed = [](GdObj obj) {};
	callbacks.func_on_ui_released = [](GdObj obj) {};
	callbacks.func_on_ui_hovered = [](GdObj obj) {};
	callbacks.func_on_ui_clicked = [](GdObj obj) {};
	callbacks.func_on_ui_toggle = [](GdObj obj, GdBool is_on) {};
	callbacks.func_on_ui_text_changed = [](GdObj obj, GdString text) {};
	return callbacks;
}

void SpxEngine::register_callbacks(GDExtensionSpxCallbackInfoPtr callback_ptr) {
	if (singleton != nullptr) {
		print_error("SpxEngine::register_callbacks failed, already initialized!");
		return;
	}
	singleton = memnew(SpxEngine);
	singleton->_initialize_managers();
	singleton->callbacks = (callback_ptr != nullptr) ? *(SpxCallbackInfo *)callback_ptr : get_default_spx_callbacks();
	singleton->global_id = 1;
	singleton->is_spx_paused = false;
	singleton->should_execute_single_frame = false;
}

void SpxEngine::shutdown() {
	if (singleton == nullptr || singleton->shutting_down) {
		return;
	}

	SpxEngine *engine = singleton;
	engine->shutting_down = true;
	engine->on_destroy();
	singleton = nullptr;
	memdelete(engine);
}

SpxCallbackInfo *SpxEngine::get_callbacks() {
	return &callbacks;
}

GdInt SpxEngine::get_unique_id() {
	return global_id++;
}

Node *SpxEngine::get_spx_root() {
	return spx_root;
}

SceneTree *SpxEngine::get_tree() {
	return tree;
}

Window *SpxEngine::get_root() {
	if (tree == nullptr) {
		return nullptr;
	}
	return tree->get_root();
}

void SpxEngine::set_root_node(SceneTree *p_tree, Node *p_node) {
	tree = p_tree;
	spx_root = p_node;
	if (tree != nullptr && tree->get_root() != nullptr && delay_proxy == nullptr) {
		delay_proxy = memnew(SpxCallbackProxy);
		tree->get_root()->add_child(delay_proxy);
		on_timeout_callable = Callable(delay_proxy, "_on_timeout");
	}
}

void SpxEngine::on_awake() {
	if (has_exit || managers_awake) {
		return;
	}

	managers_awake = true;
	_notify_managers_awake();
	_notify_managers_start();

	if (callbacks.func_on_engine_start) {
		callbacks.func_on_engine_start();
	}
}

void SpxEngine::on_fixed_update(float delta) {
	if (has_exit) {
		return;
	}

	if (is_spx_paused && !should_execute_single_frame) {
		return;
	}

	_notify_managers_fixed_update(delta);

	if (callbacks.func_on_engine_fixed_update) {
		callbacks.func_on_engine_fixed_update(delta);
	}
}

void SpxEngine::on_update(float delta) {
	if (has_exit) {
		return;
	}

	if (is_spx_paused && !should_execute_single_frame) {
		return;
	}

	if (should_execute_single_frame) {
		should_execute_single_frame = false;
	}

	if (is_defer_call_pause) {
		_on_godot_pause_changed(defer_pause_value);
		is_defer_call_pause = false;
	}

	_notify_managers_update(delta);

	if (callbacks.func_on_engine_update) {
		callbacks.func_on_engine_update(delta);
	}

	if (pen) {
		pen->flush_all();
	}

	if (is_spx_paused && !tree->is_paused()) {
		if (Thread::is_main_thread()) {
			tree->set_pause(true);
		} else {
			tree->call_deferred("set_pause", true);
		}
	}
}

void SpxEngine::on_destroy() {
	_disconnect_reset_timer();
	clear_frozen_frame();

	const bool was_awake = managers_awake;
	if (was_awake) {
		_notify_managers_destroy();
		managers_awake = false;
	}

	if (was_awake && !has_exit) {
		if (callbacks.func_on_engine_destroy) {
			callbacks.func_on_engine_destroy();
		}
	}

	if (delay_proxy) {
		delay_proxy->clear_callback();
		delay_proxy->queue_free();
		delay_proxy = nullptr;
	}
	on_timeout_callable = Callable();

	callbacks = get_default_spx_callbacks();
	on_runtime_panic = nullptr;
	on_runtime_exit = nullptr;
	on_runtime_reset = nullptr;
	SvgManager::destroy_singleton();
	_destroy_all_managers();
	tree = nullptr;
	spx_root = nullptr;
}

void SpxEngine::on_exit(int exit_code) {
	if (has_exit) {
		return;
	}

	has_exit = true;

	_notify_managers_exit(exit_code);

	callbacks = get_default_spx_callbacks();
}

void SpxEngine::on_reset(int reset_code) {
	if (is_spx_reset) {
		return;
	}

	is_spx_reset = true;
	capture_last_frame();
	_do_reset(reset_code);
}

bool SpxEngine::is_reset() {
	return is_spx_reset;
}

void SpxEngine::restart() {
	if (!is_spx_reset) {
		return;
	}

	_disconnect_reset_timer();
	clear_frozen_frame();
	_resume_pure();
	is_spx_reset = false;

	_notify_managers_start();
}

void SpxEngine::set_delay_runtime_reset(bool p_delay) {
	should_delay_runtime_reset = p_delay;
}

void SpxEngine::capture_last_frame() {
	if (is_frozen_frame || !tree) {
		return;
	}

	Ref<Image> img = _get_viewport_image();
	if (img.is_null()) {
		return;
	}

	freeze_screen = _create_freeze_texture(img);
	_attach_freeze_node(freeze_screen);
	is_frozen_frame = true;
}

void SpxEngine::clear_frozen_frame() {
	if (!is_frozen_frame) {
		return;
	}

	if (freeze_screen) {
		freeze_screen->queue_free();
		freeze_screen = nullptr;
	}

	if (freeze_layer) {
		freeze_layer->queue_free();
		freeze_layer = nullptr;
	}

	is_frozen_frame = false;
}

void SpxEngine::pause() {
	if (!tree) {
		return;
	}

	if (Thread::is_main_thread()) {
		tree->set_pause(true);
		_on_godot_pause_changed(true);
	} else {
		tree->call_deferred("set_pause", true);
		is_defer_call_pause = true;
		defer_pause_value = true;
	}
}

void SpxEngine::resume() {
	if (!tree) {
		return;
	}

	if (Thread::is_main_thread()) {
		tree->set_pause(false);
		_on_godot_pause_changed(false);
	} else {
		tree->call_deferred("set_pause", false);
		is_defer_call_pause = true;
		defer_pause_value = false;
	}
}

bool SpxEngine::is_paused() const {
	return is_spx_paused;
}

void SpxEngine::next_frame() {
	if (!is_spx_paused) {
		return;
	}

	if (!tree) {
		return;
	}

	if (Thread::is_main_thread()) {
		tree->set_pause(false);
		should_execute_single_frame = true;
	} else {
		tree->call_deferred("set_pause", false);
		should_execute_single_frame = true;
	}
}

void SpxEngine::_do_reset(int reset_code) {
	if (callbacks.func_on_engine_reset) {
		callbacks.func_on_engine_reset();
	}

	_notify_managers_reset(reset_code);

	SvgManager::get_singleton()->reset(true);

	if (should_delay_runtime_reset) {
		_invoke_runtime_reset_delayed(reset_code);
	} else {
		_invoke_runtime_reset(reset_code);
	}
}

void SpxEngine::_invoke_runtime_reset(int reset_code) {
	_pause_pure();
	auto callback = get_on_runtime_reset();
	if (callback) {
		callback(reset_code);
	}
}

void SpxEngine::_invoke_runtime_reset_delayed(int reset_code) {
	if (!tree || !delay_proxy) {
		return;
	}

	_disconnect_reset_timer();
	reset_timer = tree->create_timer(RESET_PAUSE_DELAY_SEC);
	delay_proxy->set_callback([this, reset_code]() {
		_invoke_runtime_reset(reset_code);
	});
	reset_timer->connect("timeout", on_timeout_callable);
}

void SpxEngine::_disconnect_reset_timer() {
	if (reset_timer.is_valid() && reset_timer->is_connected("timeout", on_timeout_callable)) {
		reset_timer->disconnect("timeout", on_timeout_callable);
	}
	reset_timer.unref();
	if (delay_proxy != nullptr) {
		delay_proxy->clear_callback();
	}
}

void SpxEngine::_on_godot_pause_changed(bool is_godot_paused) {
	if (is_godot_paused != is_spx_paused) {
		is_spx_paused = is_godot_paused;

		_notify_managers_pause(is_spx_paused);

		if (callbacks.func_on_engine_pause) {
			callbacks.func_on_engine_pause(is_spx_paused);
		}
	}
}

void SpxEngine::_pause_pure() {
	if (!tree) {
		is_spx_paused = true;
		return;
	}

	if (Thread::is_main_thread()) {
		tree->set_pause(true);
	} else {
		tree->call_deferred("set_pause", true);
	}

	is_spx_paused = true;
}

void SpxEngine::_resume_pure() {
	if (!tree) {
		is_spx_paused = false;
		return;
	}

	if (Thread::is_main_thread()) {
		tree->set_pause(false);
	} else {
		tree->call_deferred("set_pause", false);
	}

	is_spx_paused = false;
}

Ref<Image> SpxEngine::_get_viewport_image() const {
	Viewport *vp = tree->get_root();
	if (!vp) {
		return Ref<Image>();
	}

	DisplayServer *display_server = DisplayServer::get_singleton();
	if (!display_server || !display_server->window_can_draw()) {
		return Ref<Image>();
	}

	return vp->get_texture()->get_image();
}

TextureRect *SpxEngine::_create_freeze_texture(const Ref<Image> &img) const {
	Ref<ImageTexture> tex = ImageTexture::create_from_image(img);
	TextureRect *screen = memnew(TextureRect);
	screen->set_texture(tex);
	screen->set_stretch_mode(TextureRect::STRETCH_SCALE);
	screen->set_anchors_and_offsets_preset(Control::PRESET_FULL_RECT);
	screen->set_mouse_filter(Control::MOUSE_FILTER_IGNORE);
	return screen;
}

void SpxEngine::_attach_freeze_node(TextureRect *screen) {
	Viewport *vp = tree->get_root();
	if (!vp || !screen) {
		return;
	}

	if (!freeze_layer) {
		freeze_layer = memnew(CanvasLayer);
		freeze_layer->set_layer(1);
		vp->add_child(freeze_layer);
	}

	freeze_layer->add_child(screen);
}

void SpxEngine::_initialize_managers() {
	input = create_manager<SpxInputMgr>();
	audio = create_manager<SpxAudioMgr>();
	physics = create_manager<SpxPhysicsMgr>();

	sprite = create_manager<SpxSpriteMgr>();
	ui = create_manager<SpxUiMgr>();
	scene = create_manager<SpxSceneMgr>();
	camera = create_manager<SpxCameraMgr>();

	platform = create_manager<SpxPlatformMgr>();
	res = create_manager<SpxResMgr>();
	ext = create_manager<SpxExtMgr>();
	debug = create_manager<SpxDebugMgr>();

	navigation = create_manager<SpxNavigationMgr>();
	pen = create_manager<SpxPenMgr>();
	tilemap = create_manager<SpxTilemapMgr>();
	tilemapparser = create_manager<SpxTilemapparserMgr>();
}

void SpxEngine::_notify_managers_awake() {
	for (auto *mgr : mgrs) {
		mgr->on_awake();
	}
}

void SpxEngine::_notify_managers_start() {
	for (auto *mgr : mgrs) {
		mgr->on_start();
	}
}

void SpxEngine::_notify_managers_fixed_update(float delta) {
	for (auto *mgr : mgrs) {
		mgr->on_fixed_update(delta);
	}
}

void SpxEngine::_notify_managers_update(float delta) {
	for (auto *mgr : mgrs) {
		mgr->on_update(delta);
	}
}

void SpxEngine::_notify_managers_destroy() {
	for (auto *mgr : mgrs) {
		mgr->on_destroy();
	}
}

void SpxEngine::_notify_managers_exit(int exit_code) {
	for (auto *mgr : mgrs) {
		mgr->on_exit(exit_code);
	}
}

void SpxEngine::_notify_managers_reset(int reset_code) {
	for (auto *mgr : mgrs) {
		mgr->on_reset(reset_code);
	}
}

void SpxEngine::_notify_managers_pause(bool paused) {
	for (auto *mgr : mgrs) {
		if (paused) {
			mgr->on_pause();
		} else {
			mgr->on_resume();
		}
	}
}

void SpxEngine::_destroy_all_managers() {
	for (int i = mgrs.size() - 1; i >= 0; --i) {
		memdelete(mgrs[i]);
	}
	mgrs.clear();

	input = nullptr;
	audio = nullptr;
	physics = nullptr;
	sprite = nullptr;
	ui = nullptr;
	scene = nullptr;
	camera = nullptr;
	platform = nullptr;
	res = nullptr;
	ext = nullptr;
	debug = nullptr;
	navigation = nullptr;
	pen = nullptr;
	tilemap = nullptr;
	tilemapparser = nullptr;
}
