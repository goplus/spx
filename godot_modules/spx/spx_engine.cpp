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
#include "spx_callback_defaults.gen.h"

#include "core/os/memory.h"
#include "core/os/thread.h"
#include "scene/gui/texture_rect.h"
#include "scene/main/canvas_layer.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "servers/display_server.h"

#include "gdextension_spx_ext.h"
#include "spx.h"
#include "spx_audio_mgr.h"
#include "spx_callback_proxy.h"
#include "spx_camera_mgr.h"
#include "spx_debug_mgr.h"
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
#ifdef WEB_ENABLED
#include "web/spx_web_session.h"
#endif

void SpxEngine::register_runtime_panic_callbacks(GDExtensionSpxGlobalRuntimePanicCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_panic, callback, __func__);
}

void SpxEngine::register_runtime_exit_callbacks(GDExtensionSpxGlobalRuntimeExitCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_exit, callback, __func__);
}

void SpxEngine::register_runtime_reset_callbacks(GDExtensionSpxGlobalRuntimeResetCallback callback) {
	_register_runtime_callback(&SpxEngine::on_runtime_reset, callback, __func__);
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
	const auto on_engine_destroyed = engine->callbacks.func_on_engine_destroyed;
	if (engine->callbacks.func_on_engine_destroy) {
		engine->callbacks.func_on_engine_destroy();
	}
	engine->on_destroy();
	singleton = nullptr;
	memdelete(engine);
	if (on_engine_destroyed) {
		on_engine_destroyed();
	}
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
	_notify_managers(&SpxManager::on_awake);
	_notify_managers(&SpxManager::on_start);

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

	_notify_managers(&SpxManager::on_fixed_update, delta);

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

	_notify_managers(&SpxManager::on_update, delta);

	if (callbacks.func_on_engine_update) {
		callbacks.func_on_engine_update(delta);
	}

	if (pen) {
		pen->flush_all();
	}

	if (is_spx_paused && tree && !tree->is_paused()) {
		tree->set_pause(true);
	}
}

void SpxEngine::on_destroy() {
	_disconnect_reset_timer();
	clear_frozen_frame();

	if (managers_awake) {
		_notify_managers(&SpxManager::on_destroy);
		managers_awake = false;
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
	_destroy_all_managers();
	tree = nullptr;
	spx_root = nullptr;
}

void SpxEngine::on_exit(int exit_code) {
	if (has_exit) {
		return;
	}

	has_exit = true;

	_notify_managers(&SpxManager::on_exit, exit_code);

	// Stop runtime events now; shutdown still owns both teardown callbacks.
	const auto on_engine_destroy = callbacks.func_on_engine_destroy;
	const auto on_engine_destroyed = callbacks.func_on_engine_destroyed;
	callbacks = get_default_spx_callbacks();
	callbacks.func_on_engine_destroy = on_engine_destroy;
	callbacks.func_on_engine_destroyed = on_engine_destroyed;
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
	_set_paused_pure(false);
	is_spx_reset = false;
#ifdef WEB_ENABLED
	godot_js_spx_contact_session_start();
#endif

	_notify_managers(&SpxManager::on_start);
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
	ERR_FAIL_COND(!Thread::is_main_thread());
	if (tree) {
		tree->set_pause(true);
		_on_godot_pause_changed(true);
	}
}

void SpxEngine::resume() {
	ERR_FAIL_COND(!Thread::is_main_thread());
	if (tree) {
		tree->set_pause(false);
		_on_godot_pause_changed(false);
	}
}

bool SpxEngine::is_paused() const {
	return is_spx_paused;
}

void SpxEngine::next_frame() {
	ERR_FAIL_COND(!Thread::is_main_thread());
	if (is_spx_paused && tree) {
		tree->set_pause(false);
		should_execute_single_frame = true;
	}
}

void SpxEngine::_do_reset(int reset_code) {
	if (callbacks.func_on_engine_reset) {
		callbacks.func_on_engine_reset();
	}

	_notify_managers(&SpxManager::on_reset, reset_code);

	if (should_delay_runtime_reset) {
		_invoke_runtime_reset_delayed(reset_code);
	} else {
		_invoke_runtime_reset(reset_code);
	}
}

void SpxEngine::_invoke_runtime_reset(int reset_code) {
	_set_paused_pure(true);
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
		Spx::pending_controls.set_paused(is_spx_paused);

		_notify_managers(is_spx_paused ? &SpxManager::on_pause : &SpxManager::on_resume);

		if (callbacks.func_on_engine_pause) {
			callbacks.func_on_engine_pause(is_spx_paused);
		}
	}
}

void SpxEngine::_set_paused_pure(bool p_paused) {
	ERR_FAIL_COND(!Thread::is_main_thread());
	if (tree) {
		tree->set_pause(p_paused);
	}
	is_spx_paused = p_paused;
	Spx::pending_controls.set_paused(p_paused);
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
	// Reset disables content scaling, so keep the captured viewport's aspect ratio.
	screen->set_expand_mode(TextureRect::EXPAND_IGNORE_SIZE);
	screen->set_stretch_mode(TextureRect::STRETCH_KEEP_ASPECT_CENTERED);
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
	input = _create_manager<SpxInputMgr>();
	audio = _create_manager<SpxAudioMgr>();
	physics = _create_manager<SpxPhysicsMgr>();

	sprite = _create_manager<SpxSpriteMgr>();
	ui = _create_manager<SpxUiMgr>();
	scene = _create_manager<SpxSceneMgr>();
	camera = _create_manager<SpxCameraMgr>();

	platform = _create_manager<SpxPlatformMgr>();
	res = _create_manager<SpxResMgr>();
	debug = _create_manager<SpxDebugMgr>();

	navigation = _create_manager<SpxNavigationMgr>();
	pen = _create_manager<SpxPenMgr>();
	tilemap = _create_manager<SpxTilemapMgr>();
	tilemapparser = _create_manager<SpxTilemapparserMgr>();
}

void SpxEngine::_destroy_all_managers() {
	for (int i = managers.size() - 1; i >= 0; --i) {
		memdelete(managers[i]);
	}
	managers.clear();

	input = nullptr;
	audio = nullptr;
	physics = nullptr;
	sprite = nullptr;
	ui = nullptr;
	scene = nullptr;
	camera = nullptr;
	platform = nullptr;
	res = nullptr;
	debug = nullptr;
	navigation = nullptr;
	pen = nullptr;
	tilemap = nullptr;
	tilemapparser = nullptr;
}
