/**************************************************************************/
/*  spx_layer_sorter.h                                                    */
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

#ifndef SPX_LAYER_SORTER_H
#define SPX_LAYER_SORTER_H

#include "core/object/object.h"
#include "scene/2d/node_2d.h"
#include "spx_sprite.h"
#include <algorithm>
#include <functional>
#include <unordered_set>
#include <vector>

class ISortableSprite;
class LayerSorterDebugDrawer;
class Font;

struct SortInfo {
	GdObj id;
	Point2 pos;
	ISortableSprite *sortable;
};

enum class LayerSortMode {
	NONE,
	VERTICAL
};

class SpxLayerSorter {
public:
	using VisibilityCallback = std::function<void(ISortableSprite *, bool visible)>;

	static SpxLayerSorter &instance() {
		static SpxLayerSorter inst;
		return inst;
	}

	void set_mode(LayerSortMode mode);
	void update(const Vector<ISortableSprite *> &sortables);

	void add_static_sprite(ISortableSprite *sp);
	void remove_static_sprite(ISortableSprite *sp);

	_FORCE_INLINE_ void reset() {
		static_sorted.clear();
		dynamic_sorted.clear();
		dynamic_dirty.clear();
		dynamic_dirty_ids.clear();
		visible_ids.clear();
		_clear_drawer();
	}
	_FORCE_INLINE_ void set_screen_rect(const Rect2 &rect) {
		screen_rect = rect;
	}
	_FORCE_INLINE_ void set_visibility_callback(VisibilityCallback cb) {
		visibility_callback = std::move(cb);
	}

private:
	std::vector<SortInfo> static_sorted; // never frequently re-sorted
	std::vector<SortInfo> dynamic_sorted; // updated incrementally
	std::vector<SortInfo> dynamic_dirty;
	std::unordered_set<GdObj> dynamic_dirty_ids;

	Rect2 screen_rect;
	std::unordered_set<GdObj> visible_ids;
	VisibilityCallback visibility_callback;

	LayerSorterDebugDrawer *drawer = nullptr;

	static inline LayerSortMode sort_mode = LayerSortMode::NONE;

	static inline float full_sort_ratio = 0.3f;
	static inline bool static_initialized = false;
	static inline bool sprite_cmp(const SortInfo &a, const SortInfo &b) {
		if (a.pos.y == b.pos.y) {
			return a.pos.x > b.pos.x;
		}
		return a.pos.y < b.pos.y;
	}

	void _mark_dirty(ISortableSprite *sp);
	void _update_visibility(const Vector<ISortableSprite *> &sortables);
	void _collect_sprites(const Vector<ISortableSprite *> &sortables);
	void _incremental_sort_dynamic();
	void _full_sort_dynamic();
	void _apply_z_index_merged();

	void _create_drawer();
	void _clear_drawer();

private:
	SpxLayerSorter() {
		reset();
	}
	~SpxLayerSorter();

	SpxLayerSorter(const SpxLayerSorter &) = delete;
	SpxLayerSorter &operator=(const SpxLayerSorter &) = delete;

public:
	_FORCE_INLINE_ const Rect2 &get_screen_rect() const {
		return screen_rect;
	}
	_FORCE_INLINE_ const std::vector<SortInfo> &get_static_sorted() const {
		return static_sorted;
	}
	_FORCE_INLINE_ const std::vector<SortInfo> &get_dynamic_sorted() const {
		return dynamic_sorted;
	}
	_FORCE_INLINE_ void unlink_drawer() {
		drawer = nullptr;
	}
};

class LayerSorterDebugDrawer : public Node2D {
	GDCLASS(LayerSorterDebugDrawer, Node2D);

private:
	SpxLayerSorter *sorter = nullptr;
	Ref<Font> font;

protected:
	LayerSorterDebugDrawer() = default;
	~LayerSorterDebugDrawer() = default;

	static void _bind_methods();
	void _notification(int p_what);
	void _ready();
	void _draw();
	void _exit_tree();

public:
	explicit LayerSorterDebugDrawer(SpxLayerSorter *p_sorter) {
		sorter = p_sorter;
	}

	_FORCE_INLINE_ void update() {
		queue_redraw();
	}
};

#endif //SPX_LAYER_SORTER_H