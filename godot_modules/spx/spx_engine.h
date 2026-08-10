/**************************************************************************/
/*  spx_engine.h                                                          */
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

#ifndef SPX_ENGINE_H
#define SPX_ENGINE_H

#include "core/variant/callable.h"
#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

#include <cstdint>
class SceneTree;
class Window;
class Node;
class Image;
class TextureRect;
class CanvasLayer;
class SpxInputMgr;
class SpxAudioMgr;
class SpxPhysicsMgr;
class SpxSpriteMgr;
class SpxUiMgr;
class SpxSceneMgr;
class SpxCameraMgr;
class SpxPlatformMgr;
class SpxResMgr;
class SpxExtMgr;
class SpxDebugMgr;
class SpxNavigationMgr;
class SpxPenMgr;
class SpxTilemapMgr;
class SpxTilemapparserMgr;
class SpxCallbackProxy;

typedef void (*GDExtensionSpxGlobalRuntimePanicCallback)(GdString msg);
typedef void (*GDExtensionSpxGlobalRuntimeExitCallback)(GdInt code);
typedef void (*GDExtensionSpxGlobalRuntimeResetCallback)(GdInt code);

class SpxEngine : SpxBaseMgr {
	static inline SpxEngine *singleton = nullptr;

public:
	static SpxEngine *get_singleton() { return singleton; }
	static bool has_initialed() { return singleton != nullptr; }
	static void register_callbacks(GDExtensionSpxCallbackInfoPtr callback);
	static void register_runtime_panic_callbacks(GDExtensionSpxGlobalRuntimePanicCallback callback);
	static void register_runtime_exit_callbacks(GDExtensionSpxGlobalRuntimeExitCallback callback);
	static void register_runtime_reset_callbacks(GDExtensionSpxGlobalRuntimeResetCallback callback);
	virtual ~SpxEngine() = default;

private:
	Vector<SpxBaseMgr *> mgrs;
	SpxInputMgr *input = nullptr;
	SpxAudioMgr *audio = nullptr;
	SpxPhysicsMgr *physics = nullptr;
	SpxSpriteMgr *sprite = nullptr;
	SpxUiMgr *ui = nullptr;
	SpxSceneMgr *scene = nullptr;
	SpxCameraMgr *camera = nullptr;
	SpxPlatformMgr *platform = nullptr;
	SpxResMgr *res = nullptr;
	SpxExtMgr *ext = nullptr;
	SpxDebugMgr *debug = nullptr;
	SpxNavigationMgr *navigation = nullptr;
	SpxPenMgr *pen = nullptr;
	SpxTilemapMgr *tilemap = nullptr;
	SpxTilemapparserMgr *tilemapparser = nullptr;

	SpxCallbackProxy *delay_proxy = nullptr;

	template <typename T>
	T *create_manager() {
		T *mgr = memnew(T);
		mgrs.append(static_cast<SpxBaseMgr *>(mgr));
		return mgr;
	}

	void _destroy_all_managers();

public:
	SpxInputMgr *get_input() { return input; }
	SpxAudioMgr *get_audio() { return audio; }
	SpxPhysicsMgr *get_physics() { return physics; }
	SpxSpriteMgr *get_sprite() { return sprite; }
	SpxUiMgr *get_ui() { return ui; }
	SpxSceneMgr *get_scene() { return scene; }
	SpxCameraMgr *get_camera() { return camera; }
	SpxPlatformMgr *get_platform() { return platform; }
	SpxResMgr *get_res() { return res; }
	SpxExtMgr *get_ext() { return ext; }
	SpxDebugMgr *get_debug() { return debug; }
	SpxNavigationMgr *get_navigation() { return navigation; }
	SpxPenMgr *get_pen() { return pen; }
	SpxTilemapMgr *get_tilemap() { return tilemap; }
	SpxTilemapparserMgr *get_tilemapparser() { return tilemapparser; }

private:
	SceneTree *tree = nullptr;
	Node *spx_root = nullptr;
	GdInt global_id = 0;
	SpxCallbackInfo callbacks = {};
	GDExtensionSpxGlobalRuntimePanicCallback on_runtime_panic = nullptr;
	GDExtensionSpxGlobalRuntimeExitCallback on_runtime_exit = nullptr;
	GDExtensionSpxGlobalRuntimeResetCallback on_runtime_reset = nullptr;

	template <typename Callback>
	static void _register_runtime_callback(
			Callback SpxEngine::*member,
			Callback callback,
			const char *func_name) {
		if (singleton == nullptr) {
			print_error(vformat("%s failed, engine not initialized!", func_name));
			return;
		}

		singleton->*member = callback;
	}

	CanvasLayer *freeze_layer = nullptr;
	TextureRect *freeze_screen = nullptr;
	bool is_frozen_frame = false;

	Ref<SceneTreeTimer> reset_timer;
	Callable on_timeout_callable;
	const double RESET_PAUSE_DELAY_SEC = 5.0f;
	bool should_delay_runtime_reset = false;

	bool has_exit = false;
	bool is_spx_reset = true;
	bool is_spx_paused = false;
	bool is_defer_call_pause = false;
	bool defer_pause_value = false;
	bool should_execute_single_frame = false;

public:
	SpxCallbackInfo *get_callbacks();
	GDExtensionSpxGlobalRuntimePanicCallback get_on_runtime_panic() { return on_runtime_panic; }
	GDExtensionSpxGlobalRuntimeExitCallback get_on_runtime_exit() { return on_runtime_exit; }
	GDExtensionSpxGlobalRuntimeResetCallback get_on_runtime_reset() { return on_runtime_reset; }

public:
	GdInt get_unique_id() override;
	Node *get_spx_root() override;
	SceneTree *get_tree() override;
	Window *get_root() override;
	void set_root_node(SceneTree *p_tree, Node *p_node);

	void on_awake() override;
	void on_fixed_update(float delta) override;
	void on_update(float delta) override;
	void on_destroy() override;
	void on_exit(int exit_code) override;
	void on_reset(int reset_code) override;

	bool is_reset();
	void restart();
	void set_delay_runtime_reset(bool p_delay);

	void capture_last_frame();
	void clear_frozen_frame();

	void pause();
	void resume();
	bool is_paused() const;
	void next_frame();

private:
	void _do_reset(int reset_code);
	void _invoke_runtime_reset(int reset_code);
	void _invoke_runtime_reset_delayed(int reset_code);
	void _disconnect_reset_timer();

	void _on_godot_pause_changed(bool is_godot_paused);
	void _pause_pure();
	void _resume_pure();

	Ref<Image> _get_viewport_image() const;
	TextureRect *_create_freeze_texture(const Ref<Image> &img) const;
	void _attach_freeze_node(TextureRect *screen);

	void _initialize_managers();
	void _notify_managers_awake();
	void _notify_managers_start();
	void _notify_managers_fixed_update(float delta);
	void _notify_managers_update(float delta);
	void _notify_managers_destroy();
	void _notify_managers_exit(int exit_code);
	void _notify_managers_reset(int reset_code);
	void _notify_managers_pause(bool paused);
};

#endif // SPX_ENGINE_H
