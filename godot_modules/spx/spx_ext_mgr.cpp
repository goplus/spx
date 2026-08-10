/**************************************************************************/
/*  spx_ext_mgr.cpp                                                    */
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

#include "spx_ext_mgr.h"

#include "spx.h"
#include "spx_engine.h"
#include "spx_layer_sorter.h"

void SpxExtMgr::request_exit(GdInt exit_code) {
	auto callback = SpxEngine::get_singleton()->get_on_runtime_exit();
	if (callback != nullptr) {
		callback(exit_code);
	}

	SpxEngine::get_singleton()->on_exit(exit_code);
	get_tree()->quit(exit_code);
}

void SpxExtMgr::request_reset(GdInt exit_code) {
	Spx::reset(exit_code);
}

void SpxExtMgr::request_restart() {
	Spx::restart();
}

void SpxExtMgr::on_runtime_panic(GdString msg) {
	auto msg_str = SpxStr(msg);
	auto callback = SpxEngine::get_singleton()->get_on_runtime_panic();
	if (callback != nullptr) {
		auto str = SpxReturnStr(msg_str);
		callback(str);
	}
}

// Pause API implementations - delegate to Spx layer
void SpxExtMgr::pause() {
	Spx::pause();
}

void SpxExtMgr::resume() {
	Spx::resume();
}

GdBool SpxExtMgr::is_paused() {
	return Spx::is_paused();
}

void SpxExtMgr::next_frame() {
	Spx::next_frame();
}

void SpxExtMgr::set_layer_sorter_mode(GdInt mode) {
	SpxLayerSorter::instance().set_mode((LayerSortMode)mode);
}
