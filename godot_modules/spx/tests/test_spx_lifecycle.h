/**************************************************************************/
/*  test_spx_lifecycle.h                                                */
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

#ifndef TEST_SPX_LIFECYCLE_H
#define TEST_SPX_LIFECYCLE_H

#include "../spx.h"
#include "../spx_callback_proxy.h"
#include "../spx_engine.h"
#include "../spx_ext_mgr.h"
#include "../spx_layer_sorter.h"
#include "../spx_platform_mgr.h"
#include "../spx_ui_mgr.h"
#include "../recorder/movie_recorder_manager.h"
#include "main/main_loop_phase_callback_bus.h"
#include "scene/2d/camera_2d.h"
#include "scene/main/canvas_layer.h"
#include "scene/main/window.h"
#include "servers/audio_server.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"

namespace TestSpxLifecycle {

TEST_CASE("[SceneTree][SPX] Managers release owned nodes and preserve an authored camera") {
	if (AudioServer::get_singleton() == nullptr) {
		AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
		memnew(AudioServer)->init();
	}
	SceneTree *tree = SceneTree::get_singleton();
	Node *root = memnew(Node);
	tree->get_root()->add_child(root);
	Camera2D *authored_camera = nullptr;
	SUBCASE("Generated camera belongs to the manager") {}
	SUBCASE("Authored camera belongs to the scene") {
		authored_camera = memnew(Camera2D);
		root->add_child(authored_camera);
	}

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	engine->set_root_node(tree, root);
	engine->on_awake();
	Node *ui_layer = root->get_node_or_null(NodePath("SpxUiMgr"));
	REQUIRE(Object::cast_to<CanvasLayer>(ui_layer) != nullptr);
	if (Object::cast_to<CanvasLayer>(ui_layer) == nullptr) {
		SpxEngine::shutdown();
		memdelete(root);
		return;
	}
	CHECK(root->get_node_or_null(NodePath("SpxResMgr")) == nullptr);
	CHECK(root->get_node_or_null(NodePath("SpxPhysicsMgr")) == nullptr);
	CHECK(root->get_node_or_null(NodePath("SpxExtMgr")) == nullptr);

	engine->get_ui()->on_reset(0);
	Node *replacement = root->get_node_or_null(NodePath("SpxUiMgr"));
	CHECK(replacement != ui_layer);
	CHECK(Object::cast_to<CanvasLayer>(replacement) != nullptr);
	CHECK(ui_layer->is_queued_for_deletion());

	Vector<Node *> owned;
	for (int i = 0; i < root->get_child_count(); i++) {
		Node *child = root->get_child(i);
		if (child != authored_camera) {
			owned.push_back(child);
		}
	}
	SpxEngine::shutdown();
	for (Node *node : owned) {
		CHECK(node->is_queued_for_deletion());
	}
	if (authored_camera != nullptr) {
		CHECK_FALSE(authored_camera->is_queued_for_deletion());
		CHECK(authored_camera->get_parent() == root);
	}
	memdelete(root);
}

static int shutdown_callback_count = 0;
static int engine_start_count = 0;
static int engine_fixed_update_count = 0;
static int engine_update_count = 0;
static int engine_reset_count = 0;
static int engine_pause_count = 0;
static GdInt runtime_reset_code = -1;

#ifdef THREADS_ENABLED
struct PendingControlWorker {
	static void run(void *) {
		Spx::restart();
		Spx::reset(41);
		Spx::pause();
		Spx::resume();
		Spx::next_frame();
	}
};
#endif // THREADS_ENABLED
static void shutdown_from_destroy_callback() {
	shutdown_callback_count++;
	SpxEngine::shutdown();
}

TEST_CASE("[SceneTree][SPX] SPX main-loop lifecycle dispatches controls and callbacks") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	if (AudioServer::get_singleton() == nullptr) {
		AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
		memnew(AudioServer)->init();
	}
	Spx::on_destroy();
	SpxEngine::register_callbacks(nullptr);
	Spx::on_start(nullptr);
	CHECK_FALSE(Spx::is_initialized());

	SceneTree *tree = SceneTree::get_singleton();
	Spx::on_start(tree);
	REQUIRE(Spx::is_initialized());
	SpxEngine *engine = SpxEngine::get_singleton();
	REQUIRE(engine != nullptr);
	Node *engine_node = engine->get_spx_root();
	REQUIRE(engine_node != nullptr);
	CHECK(engine->get_root() == tree->get_root());
	CHECK(engine->get_tree() == tree);
	CHECK(engine->get_unique_id() > 0);

	engine_start_count = 0;
	engine_fixed_update_count = 0;
	engine_update_count = 0;
	engine_reset_count = 0;
	engine_pause_count = 0;
	runtime_reset_code = -1;
	engine->get_callbacks()->func_on_engine_start = []() { engine_start_count++; };
	engine->get_callbacks()->func_on_engine_fixed_update = [](GdFloat) { engine_fixed_update_count++; };
	engine->get_callbacks()->func_on_engine_update = [](GdFloat) { engine_update_count++; };
	engine->get_callbacks()->func_on_engine_reset = []() { engine_reset_count++; };
	engine->get_callbacks()->func_on_engine_pause = [](GdBool) { engine_pause_count++; };
	SpxEngine::register_runtime_reset_callbacks([](GdInt p_code) { runtime_reset_code = p_code; });

	Spx::on_start(tree);
	engine->on_awake();
	CHECK(engine_start_count == 0);
	engine->restart();
	Spx::on_fixed_update(0.02);
	Spx::on_update(0.016);
	CHECK(engine_fixed_update_count == 1);
	CHECK(engine_update_count == 1);

	Spx::pause();
	CHECK(Spx::is_paused());
	Spx::on_fixed_update(0.02);
	Spx::on_update(0.016);
	CHECK(engine_fixed_update_count == 1);
	CHECK(engine_update_count == 1);
	Spx::next_frame();
	Spx::on_update(0.016);
	CHECK(engine_update_count == 2);
	Spx::resume();
	CHECK_FALSE(Spx::is_paused());
	CHECK(engine_pause_count >= 2);

	engine->set_delay_runtime_reset(false);
	SpxExtMgr::request_reset(7);
	CHECK(engine->is_reset());
	CHECK(engine_reset_count == 1);
	CHECK(runtime_reset_code == 7);
	SpxExtMgr::request_restart();
	CHECK_FALSE(engine->is_reset());
	engine->set_delay_runtime_reset(true);
	Spx::reset(8);
	CHECK(engine->is_reset());
	CHECK(engine_reset_count == 2);
	Spx::restart();
	CHECK_FALSE(engine->is_reset());
#ifdef THREADS_ENABLED
	Thread control_worker;
	const Thread::ID control_worker_id = control_worker.start(&PendingControlWorker::run, nullptr);
	if (control_worker.is_started()) {
		control_worker.wait_to_finish();
	}
	REQUIRE_NE(control_worker_id, Thread::UNASSIGNED_ID);
	Spx::on_update(0.016);
	Spx::on_update(0.016);
	Spx::on_update(0.016);
	CHECK(engine->is_reset());
	Spx::restart();
	CHECK_FALSE(engine->is_reset());
#endif // THREADS_ENABLED
	engine->capture_last_frame();
	engine->clear_frozen_frame();

	Spx::on_destroy();
	CHECK_FALSE(Spx::is_initialized());
	CHECK_FALSE(SpxEngine::is_initialized());
	Spx::on_fixed_update(0.02);
	Spx::on_update(0.016);
	Spx::reset(9);
	Spx::restart();
	Spx::pause();
	Spx::resume();
	Spx::next_frame();
	Spx::on_destroy();

	if (engine_node->get_parent() != nullptr) {
		engine_node->get_parent()->remove_child(engine_node);
	}
	memdelete(engine_node);
}

TEST_CASE(
		"[SPX] Callback proxy releases a one-shot callback before invocation") {
	SpxCallbackProxy *proxy = memnew(SpxCallbackProxy);
	int invocation_count = 0;
	proxy->set_callback([&]() {
		invocation_count++;
		if (invocation_count == 1) {
			proxy->call(SNAME("_on_timeout"));
			proxy->set_callback([&]() { invocation_count += 10; });
		}
	});

	proxy->call(SNAME("_on_timeout"));
	CHECK_EQ(invocation_count, 1);
	proxy->call(SNAME("_on_timeout"));
	CHECK_EQ(invocation_count, 11);
	proxy->call(SNAME("_on_timeout"));
	CHECK_EQ(invocation_count, 11);

	memdelete(proxy);
}

TEST_CASE("[SPX] Engine callback registration has one idempotent owner") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SpxEngine::register_runtime_exit_callbacks(nullptr);
	SpxEngine::register_runtime_reset_callbacks(nullptr);
	SpxEngine::register_runtime_panic_callbacks(nullptr);

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *registered_engine = SpxEngine::get_singleton();
	REQUIRE(registered_engine != nullptr);
	CHECK(registered_engine->get_root() == nullptr);
	// A platform reset can arrive before the scene root has been attached.
	registered_engine->get_platform()->set_stretch(true, 480, 360);
	registered_engine->get_platform()->on_reset(0);
	CHECK(registered_engine->get_root() == nullptr);
	registered_engine->capture_last_frame();
	registered_engine->pause();
	registered_engine->resume();
	registered_engine->next_frame();
	CHECK_FALSE(registered_engine->is_paused());

	ERR_PRINT_OFF
	SpxEngine::register_callbacks(nullptr);
	ERR_PRINT_ON
	CHECK_EQ(SpxEngine::get_singleton(), registered_engine);

	SpxEngine::shutdown();
	CHECK_FALSE(SpxEngine::is_initialized());
	SpxEngine::shutdown();
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SPX] Runtime panic callbacks borrow the caller string") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SpxEngine::register_callbacks(nullptr);
	static GdString received;
	received = nullptr;
	SpxEngine::register_runtime_panic_callbacks([](GdString msg) { received = msg; });
	const char message[] = "borrowed panic";
	SpxExtMgr::on_runtime_panic(message);
	CHECK_EQ(received, message);
	SpxEngine::shutdown();
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SPX] Engine shutdown rejects destroy-callback reentry") {
	REQUIRE_FALSE(SpxEngine::is_initialized());

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	REQUIRE(engine != nullptr);
	engine->get_callbacks()->func_on_engine_destroy = shutdown_from_destroy_callback;
	shutdown_callback_count = 0;

	SpxEngine::shutdown();

	CHECK_EQ(shutdown_callback_count, 1);
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SPX] Engine teardown callbacks observe engine lifetime") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	static int phase;
	phase = 0;
	engine->get_callbacks()->func_on_engine_destroy = []() {
		CHECK(SpxEngine::is_initialized());
		CHECK_FALSE(Spx::is_initialized());
		CHECK_EQ(phase, 0);
		phase = 1;
	};
	engine->get_callbacks()->func_on_engine_destroyed = []() {
		CHECK_FALSE(SpxEngine::is_initialized());
		CHECK_EQ(phase, 1);
		phase = 2;
	};
	SUBCASE("Exit before shutdown preserves teardown callbacks") {
		engine->on_exit(0);
		engine->on_exit(0);
	}
	SUBCASE("Shutdown without exit") {}
	Spx::on_destroy();
	Spx::on_destroy();
	CHECK_EQ(phase, 2);
}

TEST_CASE("[SPX] Control mailbox coalesces reset parameters and preserves later requests") {
	SpxPendingControls controls;
	controls.submit(SpxPendingControls::RESET, 1);
	CHECK_FALSE(controls.take(SpxPendingControls::RESET));
	controls.set_accepting(true);
	controls.submit(SpxPendingControls::RESET, 10);
	controls.submit(SpxPendingControls::RESET, 20);
	controls.submit(SpxPendingControls::RESTART);
	controls.submit(SpxPendingControls::PAUSE);
	controls.submit(SpxPendingControls::RESUME);
	CHECK(controls.take(SpxPendingControls::RESTART));
	int code = 0;
	CHECK(controls.take(SpxPendingControls::RESET, &code));
	CHECK_EQ(code, 20);
	controls.submit(SpxPendingControls::RESET, 30);
	CHECK(controls.take(SpxPendingControls::PAUSE));
	CHECK(controls.take(SpxPendingControls::RESUME));
	CHECK(controls.take(SpxPendingControls::RESET, &code));
	CHECK_EQ(code, 30);
	CHECK_FALSE(controls.take(SpxPendingControls::RESET));
	controls.submit(SpxPendingControls::NEXT_FRAME);
	controls.set_paused(true);
	CHECK(controls.is_paused());
	controls.set_accepting(false);
	controls.submit(SpxPendingControls::PAUSE);
	controls.set_paused(true);
	CHECK_FALSE(controls.is_paused());
	controls.set_accepting(true);
	CHECK_FALSE(controls.take(SpxPendingControls::NEXT_FRAME));
	CHECK_FALSE(controls.take(SpxPendingControls::PAUSE));
}

TEST_CASE("[SceneTree][SPX] Public engine controls serialize worker requests on the main loop") {
	REQUIRE_FALSE(Spx::is_initialized());
	REQUIRE_FALSE(SpxEngine::is_initialized());
	if (!AudioServer::get_singleton()) {
		AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
		memnew(AudioServer)->init();
	}
	auto *tree = SceneTree::get_singleton();
	const bool paused = tree->is_paused();
	struct Cleanup {
		SceneTree *tree;
		bool paused;
		~Cleanup() {
			Spx::on_destroy();
			auto *node = tree->get_root()->get_node_or_null(NodePath("SpxEngineNode"));
			if (node) {
				memdelete(node);
			}
			tree->set_pause(paused);
		}
	} cleanup{ tree, paused };
	SpxEngine::register_callbacks(nullptr);
	auto *engine = SpxEngine::get_singleton();
	static int updates, fixed_updates, resets, reset_code;
	updates = 0;
	fixed_updates = 0;
	resets = 0;
	reset_code = -1;
	engine->get_callbacks()->func_on_engine_update = [](GdFloat) { ++updates; };
	engine->get_callbacks()->func_on_engine_fixed_update = [](GdFloat) { ++fixed_updates; };
	engine->get_callbacks()->func_on_engine_reset = []() { ++resets; };
	engine->get_callbacks()->func_on_engine_start = nullptr;
	SpxEngine::register_runtime_reset_callbacks([](GdInt code) { reset_code = code; });
	engine->set_delay_runtime_reset(false);
	Spx::on_start(nullptr);
	CHECK_FALSE(Spx::is_initialized());
	auto &bus = get_main_loop_phase_callback_bus();
	const auto registrations = bus.get_registration_count();
	Spx::register_main_loop_callbacks();
	CHECK(bus.get_registration_count() == registrations);
	Spx::register_extension_functions();
	bus.notify_start(tree);
	Spx::on_start(tree);
	REQUIRE(Spx::is_initialized());
	Spx::restart();
	bus.notify_update(0.016);
	bus.notify_fixed_update(0.016);
	CHECK(updates == 1);
	CHECK(fixed_updates == 1);
	Spx::pause();
	CHECK(Spx::is_paused());
	Spx::on_update(0.016);
	Spx::on_fixed_update(0.016);
	CHECK(updates == 1);
	CHECK(fixed_updates == 1);
	Spx::next_frame();
	Spx::on_fixed_update(0.016);
	Spx::on_update(0.016);
	CHECK(updates == 2);
	CHECK(fixed_updates == 2);
	CHECK(Spx::is_paused());
	CHECK(tree->is_paused());
	Thread worker;
	worker.start([](void *) { Spx::resume(); }, nullptr);
	worker.wait_to_finish();
	CHECK(Spx::is_paused());
	Spx::on_update(0.016);
	CHECK_FALSE(Spx::is_paused());
	CHECK(updates == 3);
	worker.start([](void *) {
		Spx::pause();
		Spx::next_frame();
	},
			nullptr);
	worker.wait_to_finish();
	Spx::on_update(0.016);
	CHECK(Spx::is_paused());
	CHECK(updates == 4);
	worker.start([](void *) { Spx::reset(17); }, nullptr);
	worker.wait_to_finish();
	CHECK(resets == 0);
	Spx::on_update(0.016);
	CHECK(resets == 1);
	CHECK(reset_code == 17);
	CHECK(engine->is_reset());
	Spx::reset(18);
	CHECK(resets == 1);
	worker.start([](void *) { Spx::restart(); }, nullptr);
	worker.wait_to_finish();
	Spx::on_update(0.016);
	CHECK_FALSE(engine->is_reset());
	CHECK_FALSE(Spx::is_paused());
	Spx::restart();
	SpxExtMgr::pause();
	CHECK(SpxExtMgr::is_paused());
	SpxExtMgr::next_frame();
	SpxExtMgr::resume();
	CHECK_FALSE(SpxExtMgr::is_paused());
	SpxExtMgr::request_reset(19);
	CHECK(reset_code == 19);
	SpxExtMgr::request_restart();
	CHECK_FALSE(engine->is_reset());
	static String panic;
	panic = "";
	SpxEngine::register_runtime_panic_callbacks([](GdString message) { panic = SpxStr(message); });
	SpxExtMgr::on_runtime_panic("test panic");
	CHECK(panic == "test panic");
	SpxExtMgr::set_layer_sorter_mode((GdInt)LayerSortMode::NONE);
	engine->get_callbacks()->func_on_engine_update = nullptr;
	engine->get_callbacks()->func_on_engine_fixed_update = nullptr;
	engine->get_callbacks()->func_on_engine_pause = nullptr;
	engine->get_callbacks()->func_on_engine_reset = nullptr;
	SpxEngine::register_runtime_reset_callbacks(nullptr);
	SpxEngine::register_runtime_panic_callbacks(nullptr);
	SpxExtMgr::on_runtime_panic("no listener");
	CHECK(panic == "test panic");
	Spx::on_update(0.016);
	Spx::on_fixed_update(0.016);
	ERR_PRINT_OFF;
	worker.start([](void *data) {
		auto *engine = static_cast<SpxEngine *>(data);
		engine->pause();
		engine->resume();
		engine->next_frame();
	}, engine);
	worker.wait_to_finish();
	ERR_PRINT_ON;
	CHECK_FALSE(engine->is_paused());
	Spx::pause();
	CHECK(engine->is_paused());
	Spx::resume();
	Spx::reset(20);
	CHECK(engine->is_reset());
	CHECK(reset_code == 19);
	bus.notify_destroy();
	MovieRecorderManager::initialize();
	CHECK_FALSE(Spx::is_initialized());
	Spx::on_update(0.016);
	Spx::on_fixed_update(0.016);
	Spx::pause();
	Spx::resume();
	Spx::next_frame();
	Spx::restart();
	Spx::reset(0);
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SceneTree][SPX] Delayed resets fire once and are canceled by restart or shutdown") {
	TestSpx::EngineScene scene;
	auto *engine = scene.engine;
	auto *tree = SceneTree::get_singleton();
	struct RestorePause {
		SceneTree *tree;
		bool paused = tree->is_paused();
		~RestorePause() { tree->set_pause(paused); }
	} restore{ tree };
	static int received;
	received = 0;
	SpxEngine::register_runtime_reset_callbacks([](GdInt code) { received = code; });
	engine->restart();
	engine->set_delay_runtime_reset(true);
	engine->on_reset(71);
	CHECK(received == 0);
	engine->restart();
	tree->process(6.0);
	CHECK(received == 0);
	CHECK_FALSE(engine->is_paused());
	engine->on_reset(72);
	tree->process(6.0);
	CHECK(received == 72);
	CHECK(engine->is_paused());
	tree->process(6.0);
	CHECK(received == 72);
	engine->restart();
	engine->on_reset(73);
	SpxEngine::shutdown();
	tree->process(6.0);
	CHECK(received == 72);
}

TEST_CASE("[SceneTree][SPX] Exit callbacks receive the code before runtime events stop") {
	TestSpx::EngineScene scene;
	struct RestoreExitCode {
		int code = OS::get_singleton()->get_exit_code();
		~RestoreExitCode() { OS::get_singleton()->set_exit_code(code); }
	} restore;
	static int received;
	received = 0;
	SpxEngine::register_runtime_exit_callbacks([](GdInt code) { received = code; });
	SpxExtMgr::request_exit(17);
	CHECK(received == 17);
	CHECK(OS::get_singleton()->get_exit_code() == 17);
	const int children = scene.root->get_child_count();
	scene.engine->on_awake();
	scene.engine->on_update(0.016);
	scene.engine->on_fixed_update(0.016);
	CHECK(scene.root->get_child_count() == children);
}

} // namespace TestSpxLifecycle

#endif // TEST_SPX_LIFECYCLE_H
