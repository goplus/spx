/**************************************************************************/
/*  spx_navigation_mgr.cpp                                                */
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

#include "spx_navigation_mgr.h"

void SpxNavigationMgr::on_reset(int reset_code) {
	if (path_finder.is_valid()) {
		path_finder->reset();
		path_finder = nullptr;
	}
}
void SpxNavigationMgr::setup_path_finder_with_size(GdVec2 grid_size, GdVec2 cell_size, GdBool with_jump, GdBool with_debug) {
	if (path_finder.is_null() || !path_finder.is_valid()) {
		path_finder.instantiate();
		path_finder->setup_spx(grid_size, cell_size, with_debug);
		path_finder->set_jumping_enabled(with_jump);
	}
}

void SpxNavigationMgr::setup_path_finder(GdBool with_jump) {
	setup_path_finder_with_size(default_grid_size, default_cell_size, with_jump, false);
}

void SpxNavigationMgr::set_obstacle(GdObj obj, GdBool enabled) {
	if (path_finder.is_valid()) {
		path_finder->set_sprite_obstacle(obj, enabled);
	}
}

GdArray SpxNavigationMgr::find_path(GdVec2 p_from, GdVec2 p_to, GdBool with_jump) {
	if (path_finder.is_null() || !path_finder.is_valid()) {
		setup_path_finder(with_jump);
	}

	return path_finder->find_path_spx(p_from, p_to);
}