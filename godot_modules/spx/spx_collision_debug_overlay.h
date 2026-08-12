/**************************************************************************/
/*  spx_collision_debug_overlay.h                                         */
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

#ifndef SPX_COLLISION_DEBUG_OVERLAY_H
#define SPX_COLLISION_DEBUG_OVERLAY_H

#include "scene/2d/node_2d.h"

class CollisionShape2D;

class SpxCollisionDebugOverlay : public Node2D {
	GDCLASS(SpxCollisionDebugOverlay, Node2D);

	ObjectID target_id;
	Color debug_color;
	bool requested_visible = false;

	CollisionShape2D *_get_target() const;
	bool _is_native_collision_debug_visible() const;
	void _sync_visibility(bool p_debug_mode);
	void _on_target_redrawn();
	void _draw_debug_shape();

protected:
	static void _bind_methods() {}
	void _notification(int p_what);

public:
	void configure(CollisionShape2D *p_target, const Color &p_color, bool p_visible);
	void set_debug_color(const Color &p_color);
	Color get_debug_color() const { return debug_color; }
	void set_requested_visible(bool p_visible);
	bool is_requested_visible() const { return requested_visible; }
	void sync_debug_mode(bool p_enabled) { _sync_visibility(p_enabled); }
	CollisionShape2D *get_target() const { return _get_target(); }

	SpxCollisionDebugOverlay();
};

Color spx_collision_debug_shape_color(const Color &p_color, bool p_disabled);
Color spx_collision_debug_one_way_color(const Color &p_color, bool p_disabled);

SpxCollisionDebugOverlay *spx_find_collision_debug_overlay(CollisionShape2D *p_target);
SpxCollisionDebugOverlay *spx_ensure_collision_debug_overlay(CollisionShape2D *p_target, const Color &p_color, bool p_visible);
void spx_collision_debug_mode_changed(bool p_enabled);

#endif // SPX_COLLISION_DEBUG_OVERLAY_H
