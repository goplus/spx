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
#include "tests/test_macros.h"

class TestSpxInternalsAccessor {
public:
	static void set_initialized(bool p_initialized) {
		Spx::initialized = p_initialized;
	}
};

class TestSpxEngineInternalsAccessor {
public:
	static void prepare_destroy_callback_test(SpxEngine *p_engine, SpxBaseMgr *p_manager = nullptr) {
		p_engine->_destroy_all_managers();
		if (p_manager) {
			p_engine->mgrs.append(p_manager);
		}
		p_engine->managers_awake = true;
	}
};

namespace TestSpxLifecycle {

static int shutdown_callback_count = 0;
static int destroyed_callback_count = 0;
static bool spx_was_ready_during_destroy = false;
static bool restart_ran_during_destroy = false;
static bool engine_was_destroyed = false;
static Vector<String> teardown_events;

class LifecycleManager : public SpxBaseMgr {
public:
	void on_destroy() override {
		teardown_events.append("manager_destroy");
	}
};

static void observe_destroy_callback() {
	CHECK(SpxEngine::is_initialized());
	teardown_events.append("destroy");
}

static void shutdown_from_destroy_callback() {
	shutdown_callback_count++;
	SpxEngine::shutdown();
}

static void observe_spx_during_destroy_callback() {
	spx_was_ready_during_destroy = Spx::is_initialized();
	Spx::restart();
	restart_ran_during_destroy = !SpxEngine::get_singleton()->is_reset();
}

static void observe_destroyed_callback() {
	destroyed_callback_count++;
	engine_was_destroyed = !SpxEngine::is_initialized();
	teardown_events.append("destroyed");
}

TEST_CASE(
		"[SPX] Callback proxy releases a one-shot callback before invocation") {
	SpxCallbackProxy *proxy = memnew(SpxCallbackProxy);
	int invocation_count = 0;
	proxy->set_callback([&invocation_count]() { invocation_count++; });

	CHECK(proxy->has_callback());
	proxy->call(SNAME("_on_timeout"));
	CHECK_FALSE(proxy->has_callback());
	CHECK_EQ(invocation_count, 1);
	proxy->call(SNAME("_on_timeout"));
	CHECK_EQ(invocation_count, 1);

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

TEST_CASE("[SPX] Engine shutdown rejects destroy-callback reentry") {
	REQUIRE_FALSE(SpxEngine::is_initialized());

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	REQUIRE(engine != nullptr);
	TestSpxEngineInternalsAccessor::prepare_destroy_callback_test(engine);
	engine->get_callbacks()->func_on_engine_destroy = shutdown_from_destroy_callback;
	shutdown_callback_count = 0;

	SpxEngine::shutdown();

	CHECK_EQ(shutdown_callback_count, 1);
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SPX] SPX becomes unavailable before destroy callbacks run") {
	REQUIRE_FALSE(SpxEngine::is_initialized());

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	REQUIRE(engine != nullptr);
	TestSpxEngineInternalsAccessor::prepare_destroy_callback_test(engine);
	TestSpxInternalsAccessor::set_initialized(true);
	engine->get_callbacks()->func_on_engine_destroy = observe_spx_during_destroy_callback;
	spx_was_ready_during_destroy = true;
	restart_ran_during_destroy = true;

	Spx::on_destroy();

	CHECK_FALSE(spx_was_ready_during_destroy);
	CHECK_FALSE(restart_ran_during_destroy);
	CHECK_FALSE(Spx::is_initialized());
	CHECK_FALSE(SpxEngine::is_initialized());
}

TEST_CASE("[SPX] Engine shutdown notifies destroy before managers and destroyed after teardown") {
	REQUIRE_FALSE(SpxEngine::is_initialized());

	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	REQUIRE(engine != nullptr);
	engine->get_callbacks()->func_on_engine_destroy = observe_destroy_callback;
	engine->get_callbacks()->func_on_engine_destroyed = observe_destroyed_callback;
	destroyed_callback_count = 0;
	engine_was_destroyed = false;
	teardown_events.clear();
	bool managers_awake = false;

	SUBCASE("Shutdown before awake") {}
	SUBCASE("Exit before awake") {
		engine->on_exit(0);
		engine->on_exit(0);
	}
	SUBCASE("Shutdown after awake") {
		TestSpxEngineInternalsAccessor::prepare_destroy_callback_test(engine, memnew(LifecycleManager));
		managers_awake = true;
	}
	SUBCASE("Exit after awake") {
		TestSpxEngineInternalsAccessor::prepare_destroy_callback_test(engine, memnew(LifecycleManager));
		managers_awake = true;
		engine->on_exit(0);
		engine->on_exit(0);
	}

	CHECK(teardown_events.is_empty());
	SpxEngine::shutdown();
	SpxEngine::shutdown();

	REQUIRE_EQ(teardown_events.size(), managers_awake ? 3 : 2);
	CHECK_EQ(teardown_events[0], "destroy");
	if (managers_awake) {
		CHECK_EQ(teardown_events[1], "manager_destroy");
	}
	CHECK_EQ(teardown_events[teardown_events.size() - 1], "destroyed");
	CHECK_EQ(destroyed_callback_count, 1);
	CHECK(engine_was_destroyed);
	CHECK_FALSE(SpxEngine::is_initialized());
}

} // namespace TestSpxLifecycle

#endif // TEST_SPX_LIFECYCLE_H
