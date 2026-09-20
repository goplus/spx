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

namespace TestSpxLifecycle {

static int shutdown_callback_count = 0;
static void shutdown_from_destroy_callback() {
	shutdown_callback_count++;
	SpxEngine::shutdown();
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
