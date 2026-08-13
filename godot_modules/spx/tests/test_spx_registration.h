/**************************************************************************/
/*  test_spx_registration.h                                               */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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

#ifndef TEST_SPX_REGISTRATION_H
#define TEST_SPX_REGISTRATION_H

#include "../gdextension_spx_ext.h"
#include "../spx_engine.h"
#include "core/extension/gdextension.h"
#include "core/os/thread.h"
#include "tests/test_macros.h"

namespace TestSpxRegistration {

struct SpxEngineShutdownGuard {
	~SpxEngineShutdownGuard() {
		SpxEngine::shutdown();
	}
};

TEST_CASE("[SPX] Module initialization registers extension interface functions") {
	CHECK_NE(GDExtension::get_interface_function("spx_global_register_callbacks"), nullptr);
	CHECK_NE(GDExtension::get_interface_function("spx_platform_is_main_thread"), nullptr);
	CHECK_NE(GDExtension::get_interface_function("spx_sprite_batch_update_visuals"), nullptr);
	CHECK_NE(GDExtension::get_interface_function("spx_pen_set_canvas_size"), nullptr);
	CHECK_NE(GDExtension::get_interface_function("spx_pen_batch_update_commands"), nullptr);
}

#ifdef THREADS_ENABLED
struct MainThreadProbe {
	GDExtensionSpxPlatformIsMainThread check = nullptr;
	GdBool result = true;

	static void run(void *p_data) {
		MainThreadProbe *probe = static_cast<MainThreadProbe *>(p_data);
		probe->check(&probe->result);
	}
};
#endif

TEST_CASE("[SPX] Platform main-thread query follows Godot thread state") {
	SpxEngineShutdownGuard shutdown_guard;
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SpxEngine::register_callbacks(nullptr);
	REQUIRE(SpxEngine::is_initialized());

	auto check = reinterpret_cast<GDExtensionSpxPlatformIsMainThread>(GDExtension::get_interface_function("spx_platform_is_main_thread"));
	REQUIRE_NE(check, nullptr);

	GdBool main_result = false;
	check(&main_result);
	CHECK(main_result);

#ifdef THREADS_ENABLED
	MainThreadProbe probe;
	probe.check = check;
	Thread worker;
	const Thread::ID worker_id = worker.start(&MainThreadProbe::run, &probe);
	if (worker.is_started()) {
		worker.wait_to_finish();
	}

	REQUIRE_NE(worker_id, Thread::UNASSIGNED_ID);
	CHECK_FALSE(probe.result);
#endif
}

} // namespace TestSpxRegistration

#endif // TEST_SPX_REGISTRATION_H
