/**************************************************************************/
/*  test_spx_lifecycle.h                                                  */
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
#include "../spx_ui_mgr.h"
#include "scene/2d/camera_2d.h"
#include "scene/main/canvas_layer.h"
#include "scene/main/window.h"
#include "servers/audio_server.h"
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
	CHECK(Object::cast_to<CanvasLayer>(ui_layer) != nullptr);
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
static void shutdown_from_destroy_callback() {
	shutdown_callback_count++;
	SpxEngine::shutdown();
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

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *registered_engine = SpxEngine::get_singleton();
	REQUIRE(registered_engine != nullptr);

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

} // namespace TestSpxLifecycle

#endif // TEST_SPX_LIFECYCLE_H
