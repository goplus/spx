/**************************************************************************/
/*  test_spx_layer_sorter.h                                                 */
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

#ifndef TEST_SPX_LAYER_SORTER_H
#define TEST_SPX_LAYER_SORTER_H

#include "../spx.h"
#include "../spx_camera_mgr.h"
#include "../spx_engine.h"
#include "../spx_layer_sorter.h"
#include "scene/2d/camera_2d.h"
#include "scene/main/viewport.h"
#include "scene/main/window.h"
#include "tests/test_macros.h"

namespace TestSpxLayerSorter {

struct SortableSprite : ISortableSprite {
	GdObj id;
	Point2 position;
	bool is_static = false;
	int z_index = 0;
	int z_updates = 0;

	SortableSprite(GdObj p_id, Point2 p_position, bool p_static = false) :
			id(p_id), position(p_position), is_static(p_static) {}

	GdObj get_sort_id() const override { return id; }
	Point2 get_sort_position() const override { return position; }
	void set_sort_z_index(int p_z) override {
		z_index = p_z;
		z_updates++;
	}
	int get_sort_z_index() const override { return z_index; }
	bool is_node_valid() const override { return true; }
	bool is_sort_static() const override { return is_static; }
};

struct SorterFixture {
	SpxLayerSorter &sorter = SpxLayerSorter::instance();
	SubViewport *viewport = memnew(SubViewport);
	bool debug_mode = Spx::is_debug_mode();

	SorterFixture() {
		Spx::set_debug_mode(false);
		SceneTree *tree = SceneTree::get_singleton();
		viewport->set_size(Size2i(400, 200));
		tree->get_root()->add_child(viewport);
		Camera2D *camera = memnew(Camera2D);
		camera->set_enabled(false);
		viewport->add_child(camera);

		SpxEngine::register_callbacks(nullptr);
		SpxEngine::get_singleton()->set_root_node(tree, viewport);
		SpxEngine::get_singleton()->get_camera()->on_awake();
		sorter.reset();
		sorter.set_mode(LayerSortMode::VERTICAL);
	}

	~SorterFixture() {
		sorter.reset();
		sorter.set_mode(LayerSortMode::NONE);
		SpxEngine::get_singleton()->get_camera()->on_destroy();
		SpxEngine::shutdown();
		memdelete(viewport);
		Spx::set_debug_mode(debug_mode);
	}
};

TEST_CASE("[SceneTree][SPX] Layer sorter preserves order and cached updates") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SorterFixture fixture;
	SortableSprite back(1, Point2(10, 20));
	SortableSprite right(2, Point2(40, 30));
	SortableSprite fixed(3, Point2(30, 30), true);
	SortableSprite front(4, Point2(20, 40));
	Vector<ISortableSprite *> sprites{ &front, &fixed, &back, &right };

	fixture.sorter.update(sprites);
	CHECK_EQ(fixture.sorter.get_screen_rect(), Rect2(0, 0, 400, 200));
	CHECK_EQ(back.z_index, 1);
	CHECK_EQ(right.z_index, 2);
	CHECK_EQ(fixed.z_index, 3);
	CHECK_EQ(front.z_index, 4);

	fixture.sorter.update(sprites);
	CHECK_EQ(back.z_updates, 1);
	CHECK_EQ(right.z_updates, 1);
	CHECK_EQ(fixed.z_updates, 1);
	CHECK_EQ(front.z_updates, 1);

	// One changed entry among three dynamic sprites takes the incremental path.
	front.position.y = 10;
	fixture.sorter.update(sprites);
	CHECK_EQ(front.z_index, 1);
	CHECK_EQ(back.z_index, 2);
	CHECK_EQ(right.z_index, 3);
	CHECK_EQ(fixed.z_index, 4);
}

TEST_CASE("[SceneTree][SPX] Layer sorter filters offscreen sprites and reinserts returning ones") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SorterFixture fixture;
	SortableSprite left(1, Point2(100, 100));
	SortableSprite right(2, Point2(200, 100));
	Vector<ISortableSprite *> sprites{ &left, &right };

	fixture.sorter.update(sprites);
	CHECK_EQ(right.z_index, 1);
	CHECK_EQ(left.z_index, 2);

	right.position.x = 400;
	fixture.sorter.update(sprites);
	REQUIRE_EQ(fixture.sorter.get_dynamic_sorted().size(), 1);
	CHECK_EQ(fixture.sorter.get_dynamic_sorted()[0].id, left.id);
	CHECK_EQ(right.z_updates, 1);

	right.position.x = 50;
	fixture.sorter.update(sprites);
	REQUIRE_EQ(fixture.sorter.get_dynamic_sorted().size(), 2);
	CHECK_EQ(fixture.sorter.get_dynamic_sorted()[0].id, left.id);
	CHECK_EQ(fixture.sorter.get_dynamic_sorted()[1].id, right.id);
	CHECK_EQ(left.z_index, 1);
	CHECK_EQ(right.z_index, 2);
}

TEST_CASE("[SceneTree][SPX] Layer sorter reset drops cached sprites") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SorterFixture fixture;
	SortableSprite fixed(1, Point2(10, 10), true);
	SortableSprite moving(2, Point2(20, 20));
	Vector<ISortableSprite *> sprites{ &moving, &fixed };
	fixture.sorter.update(sprites);
	REQUIRE_EQ(fixture.sorter.get_static_sorted().size(), 1);
	REQUIRE_EQ(fixture.sorter.get_dynamic_sorted().size(), 1);

	fixture.sorter.reset();
	CHECK(fixture.sorter.get_static_sorted().empty());
	CHECK(fixture.sorter.get_dynamic_sorted().empty());

	SortableSprite replacement(2, Point2(30, 30));
	Vector<ISortableSprite *> replacements{ &replacement };
	fixture.sorter.update(replacements);
	CHECK(fixture.sorter.get_static_sorted().empty());
	REQUIRE_EQ(fixture.sorter.get_dynamic_sorted().size(), 1);
	CHECK_EQ(fixture.sorter.get_dynamic_sorted()[0].sortable, &replacement);
	CHECK_EQ(replacement.z_index, 1);
	CHECK_EQ(fixed.z_updates, 1);
	CHECK_EQ(moving.z_updates, 1);
}

} // namespace TestSpxLayerSorter

#endif // TEST_SPX_LAYER_SORTER_H
