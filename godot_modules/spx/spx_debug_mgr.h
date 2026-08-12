/**************************************************************************/
/*  spx_debug_mgr.h                                                       */
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

#ifndef SPX_DEBUG_MGR_H
#define SPX_DEBUG_MGR_H

#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include "spx_base_mgr.h"

struct DebugShape {
	enum Type {
		CIRCLE,
		RECT,
		LINE
	};
	Type type;
	GdVec2 position;
	GdVec2 size;
	GdFloat radius;
	GdVec2 to_position;
	GdColor color;
	Node2D *node;
};

class SpxDebugMgr : public SpxBaseMgr {
	SPXCLASS(SpxDebugMgr, SpxBaseMgr)
public:
	virtual ~SpxDebugMgr() = default;

private:
	Vector<DebugShape> debug_shapes;
	Node2D *debug_root;

	void _clear_debug_shapes();
	static Mutex lock;

public:
	void on_awake() override;
	void on_update(float delta) override;
	void on_destroy() override;
	void on_reset(int reset_code) override;

	SPX_API void debug_draw_circle(GdVec2 pos, GdFloat radius, GdColor color);
	SPX_API void debug_draw_rect(GdVec2 pos, GdVec2 size, GdColor color);
	SPX_API void debug_draw_line(GdVec2 from, GdVec2 to, GdColor color);
};

#endif // SPX_DEBUG_MGR_H
