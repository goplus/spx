/**************************************************************************/
/*  spx_pen_surface.h                                                     */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including  */
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

#ifndef SPX_PEN_SURFACE_H
#define SPX_PEN_SURFACE_H

#include "core/templates/vector.h"
#include "scene/2d/node_2d.h"
#include "scene/2d/sprite_2d.h"
#include "scene/main/viewport.h"

class SpxPenCanvas : public Node2D {
	GDCLASS(SpxPenCanvas, Node2D);

private:
	struct DrawCommand {
		enum Type {
			LINE,
			STAMP,
		};

		Type type = LINE;
		Vector2 from;
		Vector2 to;
		Color color;
		float width = 1.0f;
		bool draw_start_cap = true;
		Ref<Texture2D> texture;
		float rotation = 0.0f;
		Vector2 scale = Vector2(1.0f, 1.0f);
	};

	Vector<DrawCommand> pending_commands;
	void _draw_line_batch(int p_begin, int p_end);

protected:
	static void _bind_methods();
	void _notification(int p_what);

public:
	void add_line(const Vector2 &p_from, const Vector2 &p_to, float p_width, const Color &p_color, bool p_draw_start_cap);
	void add_stamp(const Ref<Texture2D> &p_texture, const Vector2 &p_position, float p_rotation, const Vector2 &p_scale);
	void discard_pending();
};

// A Scratch-style shared pen layer. Drawing commands are rasterized by the
// renderer into a persistent transparent SubViewport instead of touching
// Image pixels on the CPU and uploading the whole stage texture every frame.
class SpxPenSurface : public Node2D {
	GDCLASS(SpxPenSurface, Node2D);

private:
	SubViewport *render_target = nullptr;
	SpxPenCanvas *canvas = nullptr;
	Sprite2D *canvas_sprite = nullptr;
	Size2i canvas_size;
	bool dirty = false;
	bool clear_requested = true;

protected:
	static void _bind_methods();

public:
	void initialize(const Size2i &p_size);
	void set_canvas_size(const Size2i &p_size);
	void draw_line(const Vector2 &p_from, const Vector2 &p_to, float p_width, const Color &p_color, bool p_draw_start_cap);
	void draw_stamp(const Ref<Texture2D> &p_texture, const Vector2 &p_position, float p_rotation, const Vector2 &p_scale);
	void clear();
	void flush();
	Size2i get_canvas_size() const;

	~SpxPenSurface();
};

#endif // SPX_PEN_SURFACE_H
