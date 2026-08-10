/**************************************************************************/
/*  spx_debug_mgr.cpp                                                     */
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

#include "spx_debug_mgr.h"

#include <cmath>

#include "scene/2d/line_2d.h"

#include "spx_coordinate.h"

Mutex SpxDebugMgr::lock;

void SpxDebugMgr::on_awake() {
	SpxBaseMgr::on_awake();

	debug_root = memnew(Node2D);
	debug_root->set_name("debug_root");
	debug_root->set_z_index(1000);
	debug_root->set_z_as_relative(false);
	get_spx_root()->add_child(debug_root);
}

void SpxDebugMgr::on_update(float delta) {
	SpxBaseMgr::on_update(delta);

	lock.lock();
	_clear_debug_shapes();
	lock.unlock();
}

void SpxDebugMgr::on_destroy() {
	lock.lock();
	_clear_debug_shapes();
	if (debug_root) {
		debug_root->queue_free();
		debug_root = nullptr;
	}
	lock.unlock();
	SpxBaseMgr::on_destroy();
}

void SpxDebugMgr::on_reset(int reset_code) {
	_clear_debug_shapes();
}

void SpxDebugMgr::_clear_debug_shapes() {
	for (const DebugShape &shape : debug_shapes) {
		if (shape.node && shape.node->is_inside_tree()) {
			shape.node->queue_free();
		}
	}
	debug_shapes.clear();
}

void SpxDebugMgr::debug_draw_circle(GdVec2 pos, GdFloat radius, GdColor color) {
	if (!debug_root) {
		return;
	}

	pos = spx_to_godot_vec2(pos);
	Line2D *circle = memnew(Line2D);
	circle->set_default_color(color);
	circle->set_width(2.0f);

	PackedVector2Array points;
	int segments = MAX(16, (int)(radius * 0.8f));
	for (int i = 0; i <= segments; i++) {
		float angle = i * 6.283185f / segments;
		points.append(Vector2(std::cos(angle) * radius, std::sin(angle) * radius));
	}
	circle->set_points(points);
	circle->set_position(pos);

	debug_root->add_child(circle);

	DebugShape shape;
	shape.type = DebugShape::CIRCLE;
	shape.position = pos;
	shape.radius = radius;
	shape.color = color;
	shape.node = circle;
	debug_shapes.push_back(shape);
}

void SpxDebugMgr::debug_draw_rect(GdVec2 pos, GdVec2 size, GdColor color) {
	if (!debug_root) {
		return;
	}

	Line2D *rect = memnew(Line2D);
	rect->set_default_color(color);
	rect->set_width(2.0f);

	pos = spx_to_godot_vec2(pos);
	size = size * 0.5;
	PackedVector2Array points;
	points.append(Vector2(-size.x, -size.y));
	points.append(Vector2(size.x, -size.y));
	points.append(Vector2(size.x, size.y));
	points.append(Vector2(-size.x, size.y));
	points.append(Vector2(-size.x, -size.y));
	rect->set_points(points);
	rect->set_position(pos);

	debug_root->add_child(rect);

	DebugShape shape;
	shape.type = DebugShape::RECT;
	shape.position = pos;
	shape.size = size;
	shape.color = color;
	shape.node = rect;
	debug_shapes.push_back(shape);
}

void SpxDebugMgr::debug_draw_line(GdVec2 from, GdVec2 to, GdColor color) {
	if (!debug_root) {
		return;
	}

	from = spx_to_godot_vec2(from);
	to = spx_to_godot_vec2(to);

	Line2D *line = memnew(Line2D);
	line->set_default_color(color);
	line->set_width(2.0f);

	PackedVector2Array points;
	points.append(Vector2(0, 0));
	points.append(Vector2(to.x - from.x, to.y - from.y));
	line->set_points(points);
	line->set_position(from);

	debug_root->add_child(line);

	DebugShape shape;
	shape.type = DebugShape::LINE;
	shape.position = from;
	shape.to_position = to;
	shape.color = color;
	shape.node = line;
	debug_shapes.push_back(shape);
}
