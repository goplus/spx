/**************************************************************************/
/*  spx_pen.h                                                             */
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

#ifndef SPX_PEN_H
#define SPX_PEN_H

#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

class SpxSprite;
class SpxPenSurface;
class SpxPen {
private:
	GdObj id;
	SpxPenSurface *surface = nullptr;
	Vector2 last_draw_pos;
	bool has_last_draw_pos = false;
	bool needs_start_cap = true;
	bool is_pen_down = false;
	float min_draw_distance = 1.0f;

	struct PenProperties {
		Color color = Color(0, 0, 0, 1); // BLACK
		float size = 2.0f;
		float saturation = 1.0f;
		float brightness = 1.0f;
		float transparency = 0.0f;
	} pen_properties;

	Vector2 current_pen_pos;
	bool move_by_mouse = false;

	Ref<Texture2D> stamp_texture;
	String stamp_texture_path;

private:
	void _draw_line(GdVec2 from, GdVec2 to, float size, Color color, bool draw_start_cap);
	GdVec2 _get_draw_position(GdVec2 position, float size) const;
	void _start_new_line();
	void _append_current_point_if_needed(GdVec2 position);
	Color _get_current_color() const;
	void _stamp_texture(const Ref<Texture2D> &texture, GdVec2 position, GdFloat rotation_radians, GdVec2 scale);
	Ref<Texture2D> _resolve_stamp_texture(const String &texture_path);

public:
	void on_create(GdInt p_id, Node *p_root);
	void on_destroy();
	void on_update(float delta);
	void on_reset(int reset_code);

public:
	// Pen APIs
	void on_erase_all();
	GdObj get_id();
	void on_down(GdBool move_by_mouse);
	void on_up();
	void stamp();
	void move_to(GdVec2 position);
	void set_color_to(GdColor color);
	void change_by(GdInt property, GdFloat amount);
	void set_to(GdInt property, GdFloat value);
	void change_size_by(GdFloat amount);
	void set_size_to(GdFloat size);
	void set_stamp_texture(GdString texture_path);
	// rotation_radians is in radians.
	void stamp_with_transform(GdString texture_path, GdVec2 position, GdFloat rotation_radians, GdVec2 scale);
};

#endif // SPX_PEN_H
