/**************************************************************************/
/*  spx_navigation_mgr.h                                                  */
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

#ifndef SPX_NAVIGATION_MGR_H
#define SPX_NAVIGATION_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"
#include "spx_path_finder.h"

class SpxNavigationMgr : public SpxBaseMgr {
	SPXCLASS(SpxNavigationMgr, SpxBaseMgr)
public:
	virtual ~SpxNavigationMgr() = default;
	void on_reset(int reset_code) override;

private:
	Ref<SpxPathFinder> path_finder;
	const GdVec2 default_grid_size{ 100, 100 };
	const GdVec2 default_cell_size{ 16, 16 };

public:
	SPX_API void setup_path_finder_with_size(GdVec2 grid_size, GdVec2 cell_size, GdBool with_jump, GdBool with_debug);
	SPX_API void setup_path_finder(GdBool with_jump);
	SPX_API void set_obstacle(GdObj obj, GdBool enabled);
	SPX_API GdArray find_path(GdVec2 p_from, GdVec2 p_to, GdBool with_jump);
};

#endif // SPX_NAVIGATION_MGR_H
