/**************************************************************************/
/*  spx_layer_sorter.cpp                                                  */
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

#include "spx_layer_sorter.h"

#include "core/templates/hash_map.h"
#include "scene/2d/camera_2d.h"
#include "scene/main/viewport.h"
#include "scene/theme/theme_db.h"
#include "servers/rendering_server.h"
#include "spx.h"
#include "spx_camera_mgr.h"
#include "spx_engine.h"
#include "spx_sprite.h"
#include <algorithm>

void SpxLayerSorter::set_mode(LayerSortMode mode) {
	sort_mode = mode;
	if (!is_enabled()) {
		reset();
	}
}

void SpxLayerSorter::update(const Vector<ISortableSprite *> &sortables) {
	if (!is_enabled()) {
		return;
	}

	std::vector<SortInfo> sorted;
	sorted.reserve(sortables.size());
	HashMap<GdObj, ISortableSprite *> sprites;
	for (ISortableSprite *sprite : sortables) {
		if (!sprite || !sprite->is_node_valid()) {
			continue;
		}
		const Point2 pos = sprite->get_sort_position();
		const GdObj id = sprite->get_sort_id();
		if (sprites.has(id)) {
			continue;
		}
		sprites.insert(id, sprite);
		sorted.push_back({ id, pos, sprite->is_sort_static() });
	}
	// Only visible members need a unique z slot when the full scene exceeds
	// Godot's positive z range. Use rendered bounds, never the sort anchor.
	if (sorted.size() > RS::CANVAS_ITEM_Z_MAX) {
		const Camera2D *camera = SpxEngine::get_singleton()->get_camera()->get_camera();
		const Viewport *viewport = camera ? camera->get_viewport() : nullptr;
		const Rect2 screen = viewport ? viewport->get_canvas_transform().affine_inverse().xform(viewport->get_visible_rect()) : Rect2();
		sorted.erase(std::remove_if(sorted.begin(), sorted.end(), [&](const SortInfo &info) {
			Rect2 bounds;
			return viewport && sprites[info.id]->get_sort_bounds(bounds) && !screen.intersects(bounds, true);
		}), sorted.end());
	}
	ERR_FAIL_COND_MSG(sorted.size() > RS::CANVAS_ITEM_Z_MAX,
			"Layer sorting exceeds Godot's 4096 visible sprite limit; no z indices were changed.");

	std::sort(sorted.begin(), sorted.end(), [](const SortInfo &a, const SortInfo &b) {
		if (a.pos.y != b.pos.y) {
			return a.pos.y < b.pos.y;
		}
		if (a.pos.x != b.pos.x) {
			return a.pos.x > b.pos.x;
		}
		if (a.is_static != b.is_static) {
			return a.is_static;
		}
		return a.id < b.id;
	});

	int z = 1;
	for (const SortInfo &info : sorted) {
		ISortableSprite *sprite = sprites[info.id];
		if (sprite->get_sort_z_index() != z) {
			sprite->set_sort_z_index(z);
		}
		++z;
	}

	if (!Spx::is_debug_mode()) {
		_clear_drawer();
		return;
	}
	LayerSorterDebugDrawer *drawer = Object::cast_to<LayerSorterDebugDrawer>(ObjectDB::get_instance(drawer_id));
	if (!drawer || drawer->is_queued_for_deletion()) {
		Node *root = SpxEngine::get_singleton()->get_spx_root();
		if (!root) {
			return;
		}
		drawer = memnew(LayerSorterDebugDrawer);
		drawer_id = drawer->get_instance_id();
		root->add_child(drawer);
	}
	drawer->update(sorted, SpxEngine::get_singleton()->get_camera()->get_global_camera_rect());
}

void SpxLayerSorter::reset() {
	_clear_drawer();
}

void SpxLayerSorter::_clear_drawer() {
	if (Node *drawer = Object::cast_to<Node>(ObjectDB::get_instance(drawer_id))) {
		drawer->queue_free();
	}
	drawer_id = ObjectID();
}

SpxLayerSorter::~SpxLayerSorter() {
	_clear_drawer();
}

void LayerSorterDebugDrawer::_bind_methods() {
}

void LayerSorterDebugDrawer::_notification(int p_what) {
	if (p_what == NOTIFICATION_READY) {
		set_z_index(1000);
		set_z_as_relative(false);
		font = ThemeDB::get_singleton()->get_default_theme()->get_default_font();
	} else if (p_what == NOTIFICATION_DRAW) {
		_draw();
	}
}

void LayerSorterDebugDrawer::update(const std::vector<SortInfo> &p_sorted, const Rect2 &p_screen_rect) {
	sorted = p_sorted;
	screen_rect = p_screen_rect;
	queue_redraw();
}

void LayerSorterDebugDrawer::_draw() {
	if (font.is_null()) {
		return;
	}
	int static_index = 0;
	int dynamic_index = 0;
	for (const SortInfo &info : sorted) {
		const Color dot_color = info.is_static ? Color(0.4, 0.8, 1) : Color(1, 0.4, 0.4);
		const Color text_color = info.is_static ? Color(0.6, 0.9, 1) : Color(1, 0.6, 0.6);
		const String label = vformat("%s:%d", info.is_static ? "S" : "D", info.is_static ? static_index++ : dynamic_index++);
		draw_circle(info.pos, 8, dot_color);
		draw_string(font, info.pos + Vector2(6, -2), label, HORIZONTAL_ALIGNMENT_LEFT, -1, 24, text_color);
	}
	draw_rect(screen_rect, Color(0, 0, 1, 1), false, 4);
}
