/**************************************************************************/
/*  test_spx_main_loop_phase_callback_bus.h                               */
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

#ifndef TEST_SPX_MAIN_LOOP_PHASE_CALLBACK_BUS_H
#define TEST_SPX_MAIN_LOOP_PHASE_CALLBACK_BUS_H

#include "main/main_loop_phase_callback_bus.h"
#include "../spx.h"
#include "tests/test_macros.h"

namespace TestSpxMainLoopPhaseCallbackBus {

TEST_CASE("[SPX] Module main loop phase callback registration is idempotent") {
	MainLoopPhaseCallbackBus &bus = get_main_loop_phase_callback_bus();
	const bool was_registered = Spx::has_main_loop_callbacks_registered();
	if (!was_registered) {
		Spx::register_main_loop_callbacks();
	}

	const uint32_t registered_count = bus.get_registration_count();
	CHECK(Spx::has_main_loop_callbacks_registered());
	Spx::register_main_loop_callbacks();
	CHECK(bus.get_registration_count() == registered_count);

	Spx::unregister_main_loop_callbacks();
	CHECK_FALSE(Spx::has_main_loop_callbacks_registered());
	CHECK(bus.get_registration_count() + 1 == registered_count);
	Spx::unregister_main_loop_callbacks();
	CHECK(bus.get_registration_count() + 1 == registered_count);

	Spx::register_main_loop_callbacks();
	CHECK(Spx::has_main_loop_callbacks_registered());
	CHECK(bus.get_registration_count() == registered_count);
	if (!was_registered) {
		Spx::unregister_main_loop_callbacks();
	}
}

} // namespace TestSpxMainLoopPhaseCallbackBus

#endif // TEST_SPX_MAIN_LOOP_PHASE_CALLBACK_BUS_H
