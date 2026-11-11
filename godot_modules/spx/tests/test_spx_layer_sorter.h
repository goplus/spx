/**************************************************************************/
/*  test_spx_layer_sorter.h                                             */
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
#include "../spx_image_texture.h"
#include "../spx_layer_sorter.h"
#include "../spx_sprite.h"
#include "scene/2d/camera_2d.h"
#include "scene/main/viewport.h"
#include "scene/main/window.h"
#include "scene/resources/theme.h"
#include "scene/theme/theme_db.h"
#include "servers/rendering_server.h"
#include "tests/test_macros.h"

namespace TestSpxLayerSorter {

struct SortableSprite : ISortableSprite {
	GdObj id;
	Point2 position;
	bool is_static = false;
	bool valid = true;
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
	bool is_node_valid() const override { return valid; }
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

TEST_CASE("[SceneTree][SPX] Layer sorter preserves order without redundant z updates") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SorterFixture fixture;
	SortableSprite back(1, Point2(10, 20));
	SortableSprite right(2, Point2(40, 30));
	SortableSprite fixed(3, Point2(30, 30), true);
	SortableSprite front(4, Point2(20, 40));
	Vector<ISortableSprite *> sprites{ &front, &fixed, &back, &right };

	fixture.sorter.update(sprites);
	CHECK_EQ(back.z_index, 1);
	CHECK_EQ(right.z_index, 2);
	CHECK_EQ(fixed.z_index, 3);
	CHECK_EQ(front.z_index, 4);

	fixture.sorter.update(sprites);
	CHECK_EQ(back.z_updates, 1);
	CHECK_EQ(right.z_updates, 1);
	CHECK_EQ(fixed.z_updates, 1);
	CHECK_EQ(front.z_updates, 1);

	front.position.y = 10;
	fixture.sorter.update(sprites);
	CHECK_EQ(front.z_index, 1);
	CHECK_EQ(back.z_index, 2);
	CHECK_EQ(right.z_index, 3);
	CHECK_EQ(fixed.z_index, 4);
}

TEST_CASE("[SceneTree][SPX] Layer sorter keeps offscreen anchors ordered as viewport and membership change") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	SorterFixture fixture;
	SortableSprite left(1, Point2(100, 100));
	SortableSprite right(2, Point2(200, 100));
	SUBCASE("Dynamic sprite") {}
	SUBCASE("Static sprite") {
		right.is_static = true;
	}
	Vector<ISortableSprite *> sprites{ &left, &right };

	fixture.sorter.update(sprites);
	CHECK_EQ(right.z_index, 1);
	CHECK_EQ(left.z_index, 2);

	// A large sprite may still be visible after its sort anchor leaves the viewport.
	fixture.viewport->set_size(Size2i(150, 200));
	REQUIRE_FALSE(SpxEngine::get_singleton()->get_camera()->get_global_camera_rect().has_point(right.position));
	SortableSprite entering(3, Point2(120, 50));
	sprites.push_back(&entering);
	fixture.sorter.update(sprites);
	CHECK_EQ(entering.z_index, 1);
	CHECK_EQ(right.z_index, 2);
	CHECK_EQ(left.z_index, 3);

	right.position.x = 400;
	fixture.sorter.update(sprites);
	CHECK_EQ(right.z_updates, 2);
	CHECK_EQ(left.z_index, 3);

	right.position.x = 50;
	fixture.sorter.update(sprites);
	CHECK_EQ(left.z_index, 2);
	CHECK_EQ(right.z_index, 3);
}

TEST_CASE("[SceneTree][SPX] Layer sorting reserves z slots for rendered bounds under capacity pressure") {
	SorterFixture fixture;
	const Ref<Image> image = Image::create_empty(32, 32, false, Image::FORMAT_RGBA8);
	const Ref<Texture2D> texture = SpxImageTexture::create_from_image(image);
	Vector<ISortableSprite *> sprites;
	for (int i = 0; i < RS::CANVAS_ITEM_Z_MAX; ++i) {
		auto *sprite = memnew(SpxRenderSprite);
		sprite->set_sort_id(i + 1);
		sprite->set_texture(texture);
		sprite->set_position(Point2(-1000, -1000));
		fixture.viewport->add_child(sprite);
		sprites.push_back(sprite);
	}
	auto *large = memnew(SpxRenderSprite);
	large->set_sort_id(RS::CANVAS_ITEM_Z_MAX + 1);
	large->set_texture(texture);
	large->set_position(Point2(-20, 50));
	large->set_scale(Vector2(4, 2));
	large->set_rotation(Math_PI / 4);
	fixture.viewport->add_child(large);
	sprites.push_back(large);
	auto *front = memnew(SpxRenderSprite);
	front->set_sort_id(RS::CANVAS_ITEM_Z_MAX + 2);
	front->set_texture(texture);
	front->set_position(Point2(20, 70));
	fixture.viewport->add_child(front);
	sprites.push_back(front);

	const Rect2 screen = SpxEngine::get_singleton()->get_camera()->get_global_camera_rect();
	CHECK_FALSE(screen.has_point(large->get_sort_position()));
	Rect2 bounds;
	REQUIRE(large->get_sort_bounds(bounds));
	CHECK(screen.intersects(bounds));
	fixture.sorter.update(sprites);
	CHECK_EQ(large->get_z_index(), 1);
	CHECK_EQ(front->get_z_index(), 2);

	// A newly visible object receives an order immediately; no offscreen cache is retained.
	auto *returning = static_cast<SpxRenderSprite *>(sprites[0]);
	returning->set_position(Point2(20, 10));
	fixture.sorter.update(sprites);
	CHECK_EQ(returning->get_z_index(), 1);
	CHECK_EQ(large->get_z_index(), 2);
	CHECK_EQ(front->get_z_index(), 3);

	// A genuinely over-capacity batch must not be partly applied.
	std::vector<SortableSprite> unknown_bounds;
	SUBCASE("All bounds intersect the viewport") {
		for (ISortableSprite *sprite : sprites) {
			static_cast<SpxRenderSprite *>(sprite)->set_position(Point2(20, 20));
		}
	}
	SUBCASE("Unknown bounds remain eligible") {
		for (ISortableSprite *sprite : sprites) {
			static_cast<SpxRenderSprite *>(sprite)->set_texture(Ref<Texture2D>());
		}
	}
	SUBCASE("Custom sortable sprites without bounds remain eligible") {
		unknown_bounds.reserve(sprites.size());
		for (int i = 0; i < sprites.size(); ++i) {
			unknown_bounds.emplace_back(i + 1, Point2(-1000, -1000));
		}
		sprites.clear();
		for (SortableSprite &sprite : unknown_bounds) {
			sprites.push_back(&sprite);
		}
	}
	SUBCASE("A missing camera keeps offscreen bounds eligible") {
		SpxEngine::get_singleton()->get_camera()->on_destroy();
	}
	ERR_PRINT_OFF;
	fixture.sorter.update(sprites);
	ERR_PRINT_ON;
	CHECK_EQ(returning->get_z_index(), 1);
	CHECK_EQ(large->get_z_index(), 2);
	CHECK_EQ(front->get_z_index(), 3);
	for (const SortableSprite &sprite : unknown_bounds) {
		CHECK(sprite.z_updates == 0);
	}
}

TEST_CASE("[SceneTree][SPX] Static-only layers update z-order when members disappear") {
	SorterFixture fixture;
	SortableSprite back(1, Point2(10, 10), true), front(2, Point2(20, 20), true);
	fixture.sorter.update(Vector<ISortableSprite *>({ nullptr, &front, &front, &back }));
	CHECK(back.z_index == 1);
	CHECK(front.z_index == 2);
	back.valid = false;
	fixture.sorter.update(Vector<ISortableSprite *>({ &front, &back, nullptr }));
	CHECK(front.z_index == 1);
	front.valid = false;
	fixture.sorter.update(Vector<ISortableSprite *>({ &front, &back }));
	CHECK(back.z_updates == 1);
	CHECK(front.z_updates == 2);
	fixture.sorter.set_mode(LayerSortMode::NONE);
	front.valid = true;
	front.position = Point2(1, 1);
	int count = front.z_updates;
	fixture.sorter.update(Vector<ISortableSprite *>({ &front }));
	CHECK(front.z_updates == count);
}

TEST_CASE("[SceneTree][SPX] Layer sorting debug overlay follows the scene lifecycle") {
	SorterFixture fixture;
	Spx::set_debug_mode(true);
	SortableSprite fixed(1, Point2(20, 20), true), moving(2, Point2(40, 40));
	Vector<ISortableSprite *> sprites{ &fixed, &moving };
	SUBCASE("The default theme provides a font") {
		fixture.sorter.update(sprites);
	}
	SUBCASE("A theme without a font still permits sorting and drawer cleanup") {
		Ref<Theme> theme = ThemeDB::get_singleton()->get_default_theme();
		const Ref<Font> previous_font = theme->get_default_font();
		theme->set_default_font(Ref<Font>());
		fixture.sorter.update(sprites);
		theme->set_default_font(previous_font);
	}
	auto drawers = fixture.viewport->find_children("*", "Node2D", true, false);
	Node2D *drawer = nullptr;
	for (int i = 0; i < drawers.size(); ++i) {
		auto *node = Object::cast_to<Node2D>(drawers[i]);
		if (node->get_z_index() == 1000) {
			drawer = node;
		}
	}
	REQUIRE(drawer);
	CHECK_FALSE(drawer->is_z_relative());
	MessageQueue::get_singleton()->flush();
	CHECK(fixed.z_index == 1);
	CHECK(moving.z_index == 2);
	moving.position = Point2(10, 10);
	fixture.sorter.update(sprites);
	MessageQueue::get_singleton()->flush();
	CHECK(moving.z_index == 1);
	CHECK(fixed.z_index == 2);
	drawer->queue_free(); // External deletion leaves the sorter's ID valid until replacement.
	CHECK(drawer->is_queued_for_deletion());
	fixture.sorter.update(sprites);
	Node *replacement = fixture.viewport->get_child(-1);
	REQUIRE(replacement != drawer);
	memdelete(drawer); // An old drawer's delayed deletion must not unlink the replacement.
	fixture.sorter.update(sprites);
	CHECK(fixture.viewport->get_child(-1) == replacement);
	fixture.sorter.reset();
	CHECK(replacement->is_queued_for_deletion());
	memdelete(replacement);
	fixture.sorter.update(sprites);
	Node *external = fixture.viewport->get_child(-1);
	memdelete(external);
	fixture.sorter.update(sprites);
	CHECK(Object::cast_to<LayerSorterDebugDrawer>(fixture.viewport->get_child(-1)) != nullptr);
	Spx::set_debug_mode(false);
	fixture.sorter.update(sprites);
	CHECK(fixture.viewport->get_child(-1)->is_queued_for_deletion());

	// Scene teardown must not prevent sorting or recreate an overlay without a root.
	fixture.sorter.reset();
	auto *engine = SpxEngine::get_singleton();
	engine->set_root_node(SceneTree::get_singleton(), nullptr);
	Spx::set_debug_mode(true);
	const int children = fixture.viewport->get_child_count();
	moving.position.y = 50;
	fixture.sorter.update(sprites);
	CHECK(fixed.z_index == 1);
	CHECK(moving.z_index == 2);
	CHECK(fixture.viewport->get_child_count() == children);
	engine->set_root_node(SceneTree::get_singleton(), fixture.viewport);
}

TEST_CASE("[SceneTree][SPX] Moving static sprites refreshes their order against both groups") {
	SorterFixture fixture;
	SortableSprite back(1, Point2(10, 10), true);
	SortableSprite front(2, Point2(20, 30), true);
	SortableSprite moving(3, Point2(15, 20));
	Vector<ISortableSprite *> sprites{ &back, &front, &moving };
	fixture.sorter.update(sprites);
	CHECK(back.z_index == 1);
	CHECK(moving.z_index == 2);
	CHECK(front.z_index == 3);

	front.position.y = 5;
	fixture.sorter.update(sprites);
	CHECK(front.z_index == 1);
	CHECK(back.z_index == 2);
	CHECK(moving.z_index == 3);

	fixture.sorter.update(sprites);
	CHECK(front.z_updates == 2);
	CHECK(back.z_updates == 2);
	CHECK(moving.z_updates == 2);
}

TEST_CASE("[SceneTree][SPX] Layer sorting follows current membership and physics mode with stable ties") {
	SorterFixture fixture;
	SortableSprite a(1, Point2(20, 20)), b(2, Point2(20, 20)), c(3, Point2(20, 20), true);
	Vector<ISortableSprite *> sprites{ &b, &c, &a, &a };
	fixture.sorter.update(sprites);
	CHECK(c.z_index == 1);
	CHECK(a.z_index == 2);
	CHECK(b.z_index == 3);
	a.is_static = true;
	c.is_static = false;
	fixture.sorter.update(sprites);
	CHECK(a.z_index == 1);
	CHECK(b.z_index == 2);
	CHECK(c.z_index == 3);
	fixture.sorter.update(Vector<ISortableSprite *>{ &c });
	CHECK(c.z_index == 1);
}

TEST_CASE("[SceneTree][SPX] Layer sorting and deferred drawing never retain deleted sprites") {
	SorterFixture fixture;
	Spx::set_debug_mode(true);
	auto *deleted = memnew(SpxRenderSprite);
	deleted->set_sort_id(1);
	deleted->set_position(Point2(20, 20));
	fixture.viewport->add_child(deleted);
	SortableSprite survivor(2, Point2(30, 30));
	fixture.sorter.update(Vector<ISortableSprite *>{ deleted, &survivor });
	CHECK(survivor.z_index == 2);
	memdelete(deleted);
	MessageQueue::get_singleton()->flush(); // Redraw still sees only copied values.
	fixture.sorter.update(Vector<ISortableSprite *>{ &survivor });
	CHECK(survivor.z_index == 1);
}

} // namespace TestSpxLayerSorter

#endif // TEST_SPX_LAYER_SORTER_H
