/**************************************************************************/
/*  spx.cpp                                                               */
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

#include "spx.h"

#include "core/io/dir_access.h"
#include "core/object/class_db.h"
#include "core/os/thread.h"
#include "main/main_loop_phase_callback_bus.h"
#include "scene/main/node.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"

#include "gdextension_spx_ext.h"
#include "spx_callback_proxy.h"
#include "spx_collision_debug_overlay.h"
#include "spx_draw_tiles.h"
#include "spx_engine.h"
#include "spx_input_proxy.h"
#include "spx_path_finder.h"
#include "spx_sprite.h"
#include "spx_ui.h"
#include "spx_list_monitor.h"

// Simple node class for initialization
class SpxEngineNode : public Node {
	GDCLASS(SpxEngineNode, Node);
};

#define SPX_ENGINE SpxEngine::get_singleton()

namespace {
MainLoopPhaseCallbackBus::RegistrationID main_loop_callback_registration = MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;

void _spx_main_loop_start(void *, MainLoop *p_main_loop) {
	Spx::on_start(p_main_loop);
}

void _spx_main_loop_fixed_update(void *, double p_delta) {
	Spx::on_fixed_update(p_delta);
}

void _spx_main_loop_update(void *, double p_delta) {
	Spx::on_update(p_delta);
}

void _spx_main_loop_destroy(void *) {
	Spx::on_destroy();
}

inline bool _is_spx_engine_ready() {
	return Spx::is_initialized() && SpxEngine::is_initialized();
}
} // namespace

void Spx::register_extension_functions() {
	if (extension_functions_registered) {
		return;
	}

	// Module CORE initialization runs immediately before Godot builds the base
	// GDExtension interface table. Register the SPX-owned entries here so core
	// no longer needs an SPX callback or alias functions.
	gdextension_spx_setup_interface();
	extension_functions_registered = true;
}

void Spx::unregister_extension_functions() {
	// Godot's GDExtension interface registry is process-scoped and has no
	// matching removal API. Keep this guard set so a module reinitialization
	// cannot try to insert the same SPX function names a second time.
}

void Spx::register_main_loop_callbacks() {
	if (main_loop_callback_registration != MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID) {
		return;
	}

	MainLoopPhaseCallbackBus::Callbacks callbacks;
	callbacks.start = &_spx_main_loop_start;
	callbacks.fixed_update = &_spx_main_loop_fixed_update;
	callbacks.update = &_spx_main_loop_update;
	callbacks.destroy = &_spx_main_loop_destroy;
	main_loop_callback_registration = get_main_loop_phase_callback_bus().register_callbacks(callbacks);
}

void Spx::unregister_main_loop_callbacks() {
	if (main_loop_callback_registration == MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID) {
		return;
	}

	get_main_loop_phase_callback_bus().unregister_callbacks(main_loop_callback_registration);
	main_loop_callback_registration = MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;
}

bool Spx::has_main_loop_callbacks_registered() {
	return main_loop_callback_registration != MainLoopPhaseCallbackBus::INVALID_REGISTRATION_ID;
}

void Spx::set_debug_mode(bool enable) {
	debug_mode = enable;
	spx_collision_debug_mode_changed(enable);
}

void Spx::register_types() {
	ClassDB::register_class<SpxListMonitor>();
	ClassDB::register_class<SpxSprite>();
	ClassDB::register_internal_class<SpxCollisionDebugOverlay>();
	ClassDB::register_class<SpxInputProxy>();
	ClassDB::register_class<SpxDrawTiles>();
	ClassDB::register_class<SpxPathFinder>();
	ClassDB::register_class<PathDebugDrawer>();
	ClassDB::register_class<SpxCallbackProxy>();
}

void Spx::on_start(MainLoop *p_main_loop) {
	if (initialized.is_set() || !SpxEngine::is_initialized()) {
		return;
	}

	SceneTree *tree = Object::cast_to<SceneTree>(p_main_loop);
	if (tree == nullptr) {
		return;
	}
	Window *root = tree->get_root();
	if (root == nullptr) {
		return;
	}

	pending_controls.set_accepting(false);
	SpxEngineNode *new_node = memnew(SpxEngineNode);
	new_node->set_name("SpxEngineNode");
	root->add_child(new_node);
	SPX_ENGINE->set_root_node(tree, new_node);
	initialized.set();
	pending_controls.set_accepting(true);
	SPX_ENGINE->on_awake();
}

void Spx::on_fixed_update(double delta) {
	if (!_is_spx_engine_ready()) {
		return;
	}

	SPX_ENGINE->on_fixed_update(delta);
}

void Spx::on_update(double delta) {
	if (!_is_spx_engine_ready()) {
		return;
	}

	// Consume each phase immediately before execution. Requests made by a
	// callback for a later phase still run in this update, as before.
	if (pending_controls.take(SpxPendingControls::RESTART)) {
		SPX_ENGINE->restart();
		return;
	}
	int exit_code = 0;
	if (pending_controls.take(SpxPendingControls::RESET, &exit_code)) {
		SPX_ENGINE->on_reset(exit_code);
		return;
	}
	if (pending_controls.take(SpxPendingControls::PAUSE)) {
		SPX_ENGINE->pause();
	}
	if (pending_controls.take(SpxPendingControls::RESUME)) {
		SPX_ENGINE->resume();
	}
	if (pending_controls.take(SpxPendingControls::NEXT_FRAME)) {
		SPX_ENGINE->next_frame();
	}

	SPX_ENGINE->on_update(delta);
}

void Spx::on_destroy() {
	// Runtime callbacks invoked during shutdown must observe SPX as unavailable;
	// otherwise they can re-enter ordinary APIs while managers are tearing down.
	initialized.clear();
	pending_controls.set_accepting(false);

	if (SpxEngine::is_initialized()) {
		SpxEngine::shutdown();
	}
}

void Spx::reset(int exit_code) {
	if (!Thread::is_main_thread()) {
		pending_controls.submit(SpxPendingControls::RESET, exit_code);
		return;
	}
	if (_is_spx_engine_ready()) {
		SPX_ENGINE->on_reset(exit_code);
	}
}

void Spx::restart() {
	if (!Thread::is_main_thread()) {
		pending_controls.submit(SpxPendingControls::RESTART);
		return;
	}
	if (_is_spx_engine_ready()) {
		SPX_ENGINE->restart();
	}
}

void Spx::pause() {
	if (!Thread::is_main_thread()) {
		pending_controls.submit(SpxPendingControls::PAUSE);
		return;
	}
	if (_is_spx_engine_ready()) {
		SPX_ENGINE->pause();
	}
}

void Spx::resume() {
	if (!Thread::is_main_thread()) {
		pending_controls.submit(SpxPendingControls::RESUME);
		return;
	}
	if (_is_spx_engine_ready()) {
		SPX_ENGINE->resume();
	}
}

void Spx::next_frame() {
	if (!Thread::is_main_thread()) {
		pending_controls.submit(SpxPendingControls::NEXT_FRAME);
		return;
	}
	if (_is_spx_engine_ready()) {
		SPX_ENGINE->next_frame();
	}
}

bool Spx::is_paused() {
	return pending_controls.is_paused();
}
