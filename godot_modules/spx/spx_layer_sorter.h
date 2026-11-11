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

#include "core/object/object_id.h"
#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include <vector>

class ISortableSprite;
class Font;

// Values from the latest sort; no scene object outlives the update through this snapshot.
struct SortInfo {
	GdObj id;
	Point2 pos;
	bool is_static;
};

enum class LayerSortMode {
	NONE,
	VERTICAL
};

class SpxLayerSorter {
	ObjectID drawer_id;
	LayerSortMode sort_mode = LayerSortMode::NONE;

	void _clear_drawer();
	SpxLayerSorter() = default;
	~SpxLayerSorter();
	SpxLayerSorter(const SpxLayerSorter &) = delete;
	SpxLayerSorter &operator=(const SpxLayerSorter &) = delete;

public:
	static SpxLayerSorter &instance() {
		static SpxLayerSorter inst;
		return inst;
	}

	void set_mode(LayerSortMode mode);
	bool is_enabled() const { return sort_mode != LayerSortMode::NONE; }
	void update(const Vector<ISortableSprite *> &sortables);
	void reset();
};

class LayerSorterDebugDrawer : public Node2D {
	GDCLASS(LayerSorterDebugDrawer, Node2D);

	std::vector<SortInfo> sorted;
	Rect2 screen_rect;
	Ref<Font> font;

protected:
	static void _bind_methods();
	void _notification(int p_what);
	void _draw();

public:
	void update(const std::vector<SortInfo> &p_sorted, const Rect2 &p_screen_rect);
};

#endif // SPX_LAYER_SORTER_H
