/**************************************************************************/
/*  spx_pen_mgr.h                                                         */
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

#ifndef SPX_PEN_MGR_H
#define SPX_PEN_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_object_mgr.h"
#include "spx_pen.h"

class SpxPenSurface;
class SpxPenMgr : public SpxObjectMgr<SpxPen> {
	SPXCLASS(SpxPenMgr, SpxObjectMgr<SpxPen>)

private:
	SpxPenSurface *surface = nullptr;

public:
	virtual ~SpxPenMgr() = default;

	void on_awake() override;
	void on_update(float delta) override;
	void on_destroy() override;
	void on_reset(int reset_code) override;

	SPX_API void destroy_all_pens();
	SPX_API void set_canvas_size(GdInt width, GdInt height);
	void flush_all();
	SPX_API GdObj create_pen();
	SPX_API void destroy_pen(GdObj obj);
	SPX_API void batch_update_commands(const float *buffer_data, int len);
	// Pen operation methods
	SPX_API void pen_stamp(GdObj obj);
	SPX_API void move_pen_to(GdObj obj, GdVec2 position);
	SPX_API void pen_down(GdObj obj, GdBool move_by_mouse);
	SPX_API void pen_up(GdObj obj);
	SPX_API void set_pen_color_to(GdObj obj, GdColor color);
	SPX_API void change_pen_by(GdObj obj, GdInt property, GdFloat amount);
	SPX_API void set_pen_to(GdObj obj, GdInt property, GdFloat value);
	SPX_API void change_pen_size_by(GdObj obj, GdFloat amount);
	SPX_API void set_pen_size_to(GdObj obj, GdFloat size);
	SPX_API void set_pen_stamp_texture(GdObj obj, GdString texture_path);
	// rotation_radians is in radians.
	SPX_API void pen_stamp_with_transform(GdObj obj, GdString texture_path, GdVec2 position, GdFloat rotation_radians, GdVec2 scale);
};

#endif // SPX_PEN_MGR_H
