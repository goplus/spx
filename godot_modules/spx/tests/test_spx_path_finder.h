/**************************************************************************/
/*  test_spx_path_finder.h                                              */
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

#ifndef TEST_SPX_PATH_FINDER_H
#define TEST_SPX_PATH_FINDER_H

#include "../spx_path_finder.h"
#include "../spx_sprite_mgr.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/physics/static_body_2d.h"
#include "scene/2d/tile_map_layer.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"
#include "scene/resources/placeholder_textures.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"

namespace TestSpxPathFinder {

TEST_CASE("[SceneTree][SPX] Path finder tests tile collision polygons at cell centers") {
	real_t polygon_center_x = 24;
	bool expected_solid = true;
	SUBCASE("A polygon containing the center but no corners blocks the cell") {}
	SUBCASE("A polygon containing only the left corners leaves the cell open") {
		polygon_center_x = 16;
		expected_solid = false;
	}
	SUBCASE("A polygon containing only the right corners leaves the cell open") {
		polygon_center_x = 32;
		expected_solid = false;
	}

	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(16, 16));
	Ref<TileSetAtlasSource> source;
	source.instantiate();
	source->set_texture(texture);
	source->set_texture_region_size(Vector2i(16, 16));
	Ref<TileSet> tile_set;
	tile_set.instantiate();
	tile_set->set_tile_size(Vector2i(16, 16));
	tile_set->add_physics_layer();
	const int source_id = tile_set->add_source(source);
	source->create_tile(Vector2i());
	TileData *tile_data = source->get_tile_data(Vector2i(), 0);
	REQUIRE(tile_data != nullptr);

	// The tile center is (8, 8). A 24-pixel-tall polygon reaches neighboring
	// cells, exercising polygon checks beyond the always-solid source cell.
	PackedVector2Array polygon;
	polygon.push_back(Vector2(polygon_center_x - 12, -12));
	polygon.push_back(Vector2(polygon_center_x - 4, -12));
	polygon.push_back(Vector2(polygon_center_x - 4, 12));
	polygon.push_back(Vector2(polygon_center_x - 12, 12));
	tile_data->set_collision_polygons_count(0, 1);
	tile_data->set_collision_polygon_points(0, 0, polygon);

	SceneTree *tree = SceneTree::get_singleton();
	Node *previous_scene = tree->get_current_scene();
	Node *root = memnew(Node);
	TileMapLayer *layer = memnew(TileMapLayer);
	layer->set_tile_set(tile_set);
	layer->set_cell(Vector2i(), source_id, Vector2i());
	root->add_child(layer);
	tree->get_root()->add_child(root);
	tree->set_current_scene(root);

	Ref<SpxPathFinder> path_finder;
	path_finder.instantiate();
	path_finder->setup(Vector2i(8, 8), Vector2i(16, 16));
	CHECK(path_finder->is_cell_solid(Vector2i(0, 0)));
	CHECK(path_finder->is_cell_solid(Vector2i(1, 0)) == expected_solid);
	CHECK_FALSE(path_finder->is_cell_solid(Vector2i(1, -1)));
	CHECK_FALSE(path_finder->is_cell_solid(Vector2i(1, 1)));

	tree->set_current_scene(previous_scene);
	memdelete(root);
}

class DebugDrawer : public PathDebugDrawer {
public:
	DebugDrawer() = default;
	explicit DebugDrawer(const Ref<SpxPathFinder> &finder) :
			PathDebugDrawer(finder) {}
	using PathDebugDrawer::input;
};

TEST_CASE("[SceneTree][SPX] Navigation debug gestures retain a usable grid and detach safely") {
	TestSpx::EngineScene scene;
	Ref<SpxPathFinder> finder;
	finder.instantiate();
	finder->setup(Vector2i(8, 8), Vector2i(16, 16), true);
	CHECK(finder->get_size() == Vector2i(8, 8));
	CHECK(finder->get_cell_size() == Vector2(16, 16));
	CHECK(finder->cell_to_world_gd(Vector2i(1, 1)) == Vector2(24, 24));
	int count = scene.root->get_child_count();
	finder->setup(Vector2i(8, 8), Vector2i(16, 16), true);
	CHECK(scene.root->get_child_count() == count);
	finder->reset();
	auto *viewport = memnew(SubViewport);
	viewport->set_size(Vector2i(256, 256));
	scene.root->add_child(viewport);
	finder->setup(Vector2i(8, 8), Vector2i(16, 16), false);
	auto *drawer = memnew(DebugDrawer(finder));
	viewport->add_child(drawer);
	drawer->set_process_input(false);
	CHECK(drawer->get_z_index() == 1000);
	CHECK_FALSE(drawer->is_z_relative());
	auto motion = [&](Vector2 pos) {
		Ref<InputEventMouseMotion> e;
		e.instantiate();
		e->set_position(pos);
		viewport->push_input(e, true);
		drawer->input(e);
		MessageQueue::get_singleton()->flush();
	};
	auto click = [&](MouseButton button, bool pressed) {
		Ref<InputEventMouseButton> e;
		e.instantiate();
		e->set_button_index(button);
		e->set_pressed(pressed);
		drawer->input(e);
		MessageQueue::get_singleton()->flush();
	};
	click(MouseButton::MIDDLE, true);
	click(MouseButton::MIDDLE, false);
	SUBCASE("Choose the end point before the start point") {
		motion(Vector2(40, -8));
		click(MouseButton::RIGHT, true);
		click(MouseButton::RIGHT, false);
	}
	motion(Vector2(-40, -8));
	click(MouseButton::LEFT, true);
	click(MouseButton::LEFT, false);
	motion(Vector2(40, -8));
	click(MouseButton::RIGHT, true);
	click(MouseButton::RIGHT, false);
	click(MouseButton::LEFT, true); // A distant start point is not draggable here.
	click(MouseButton::LEFT, false);
	auto path = finder->find_path(Vector2(-40, -8), Vector2(40, -8));
	REQUIRE(path.size() == 6);
	drawer->set_path(path);
	MessageQueue::get_singleton()->flush();
	click(MouseButton::RIGHT, true);
	motion(Vector2(24, 24));
	click(MouseButton::RIGHT, false);
	motion(Vector2(-40, -8));
	click(MouseButton::RIGHT, true); // A distant end point is not draggable here.
	click(MouseButton::RIGHT, false);
	click(MouseButton::LEFT, true);
	motion(Vector2(-24, 24));
	click(MouseButton::LEFT, false);
	drawer->set_path_finder(Ref<SpxPathFinder>());
	motion(Vector2());
	click(MouseButton::LEFT, true);
	memdelete(drawer);
	auto *unconfigured = memnew(DebugDrawer);
	viewport->add_child(unconfigured);
	unconfigured->input(Ref<InputEvent>());
	MessageQueue::get_singleton()->flush();
	memdelete(unconfigured);
	finder->reset();
	CHECK(finder->get_size() == Vector2i());
}

TEST_CASE("[SceneTree][SPX] Path finder ignores unsupported, missing, and disabled obstacle geometry") {
	TestSpx::EngineScene scene;
	auto *body = memnew(StaticBody2D);
	scene.root->add_child(body);
	body->add_child(memnew(CollisionShape2D));
	auto *circle = memnew(CollisionShape2D);
	Ref<CircleShape2D> shape;
	shape.instantiate();
	shape->set_radius(32);
	circle->set_shape(shape);
	body->add_child(circle);
	Ref<SpxPathFinder> finder;
	finder.instantiate();
	finder->setup(Vector2i(8, 8), Vector2i(16, 16));
	finder->add_all_obstacles(nullptr);
	const auto direct = finder->find_path(Vector2(-24, 8), Vector2(24, 8));
	REQUIRE(direct.size() == 4);

	auto *sprites = scene.engine->get_sprite();
	const GdObj dynamic_sprite = sprites->create_bare_sprite(Vector2(8, -8));
	sprites->set_collider_rect(dynamic_sprite, Vector2(), Vector2(16, 16));
	ERR_PRINT_OFF;
	finder->set_sprite_obstacle(NULL_OBJECT_ID, true);
	finder->set_sprite_obstacle(dynamic_sprite, true);
	ERR_PRINT_ON;
	CHECK(finder->find_path(Vector2(-24, 8), Vector2(24, 8)) == direct);
	for (const Vector2 &point : direct) {
		CHECK_FALSE(finder->is_cell_solid(Vector2i((point / Vector2(16, 16)).floor())));
	}

	// A supported shape must stop blocking when disabled, without removing its body.
	auto *rectangle = memnew(CollisionShape2D);
	Ref<RectangleShape2D> rectangle_shape;
	rectangle_shape.instantiate();
	rectangle_shape->set_size(Vector2(8, 8));
	rectangle->set_shape(rectangle_shape);
	rectangle->set_position(Vector2(8, 8));
	body->add_child(rectangle);
	finder->add_all_obstacles(scene.root);
	CHECK(finder->is_cell_solid(Vector2i()));
	rectangle->set_disabled(true);
	finder->add_all_obstacles(scene.root);
	CHECK(finder->find_path(Vector2(-24, 8), Vector2(24, 8)) == direct);
}

TEST_CASE("[SceneTree][SPX] Navigation retains overlapping obstacle ownership across updates and deletion") {
	TestSpx::EngineScene scene;
	Ref<SpxPathFinder> finder;
	finder.instantiate();
	finder->setup(Vector2i(16, 16), Vector2i(16, 16));
	auto *sprites = scene.engine->get_sprite();
	auto create_obstacle = [&](Vector2 position) {
		GdObj id = sprites->create_bare_sprite(position);
		sprites->set_physics_mode(id, SpxSprite::STATIC);
		sprites->set_collider_rect(id, Vector2(), Vector2(16, 16));
		return id;
	};
	GdObj first = create_obstacle(Vector2(8, -8));
	GdObj second = create_obstacle(Vector2(24, -8));
	finder->set_sprite_obstacle(first, true);
	finder->set_sprite_obstacle(first, true);
	finder->set_sprite_obstacle(second, true);
	CHECK(finder->is_cell_solid(Vector2i(0, 0)));
	CHECK(finder->is_cell_solid(Vector2i(1, 0)));
	finder->set_sprite_obstacle(first, false);
	finder->set_sprite_obstacle(first, false);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(0, 0)));
	CHECK(finder->is_cell_solid(Vector2i(1, 0)));
	finder->set_sprite_obstacle(second, false);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(1, 0)));

	finder->set_sprite_obstacle(first, true);
	sprites->get_sprite(first)->set_position(Vector2(56, 8));
	finder->set_sprite_obstacle(first, true);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(0, 0)));
	CHECK(finder->is_cell_solid(Vector2i(3, 0)));
	// Removal uses the stored footprint even after further movement or mode changes.
	sprites->get_sprite(first)->set_position(Vector2(-40, 8));
	sprites->set_physics_mode(first, SpxSprite::NO_PHYSICS);
	finder->set_sprite_obstacle(first, false);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(3, 0)));
	sprites->set_physics_mode(first, SpxSprite::STATIC);
	finder->set_sprite_obstacle(first, true);
	CHECK(finder->is_cell_solid(Vector2i(-3, 0)));
	sprites->set_physics_mode(first, SpxSprite::NO_PHYSICS);
	finder->add_all_obstacles(scene.root);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(-3, 0)));

	sprites->set_physics_mode(first, SpxSprite::STATIC);
	sprites->get_sprite(first)->set_position(Vector2(24, 8));
	finder->set_sprite_obstacle(first, true);
	finder->set_sprite_obstacle(second, true);
	sprites->destroy_sprite(first);
	finder->find_path(Vector2(-40, 8), Vector2(72, 8));
	CHECK(finder->is_cell_solid(Vector2i(1, 0)));
	sprites->destroy_sprite(second);
	const auto direct = finder->find_path(Vector2(-40, 8), Vector2(72, 8));
	CHECK_FALSE(finder->is_cell_solid(Vector2i(1, 0)));
	CHECK(direct.size() == 8);
}

TEST_CASE("[SceneTree][SPX] Tile collision occupancy survives sprite removal and tracks layer updates") {
	TestSpx::EngineScene scene;
	Ref<SpxPathFinder> finder;
	finder.instantiate();
	finder->setup(Vector2i(8, 8), Vector2i(16, 16));
	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(16, 16));
	Ref<TileSetAtlasSource> atlas;
	atlas.instantiate();
	atlas->set_texture(texture);
	atlas->set_texture_region_size(Vector2i(16, 16));
	atlas->create_tile(Vector2i());
	Ref<TileSet> tileset;
	tileset.instantiate();
	tileset->set_tile_size(Vector2i(16, 16));
	tileset->add_physics_layer();
	tileset->add_source(atlas, 0);
	auto *data = atlas->get_tile_data(Vector2i(), 0);
	REQUIRE(data);
	data->set_collision_polygons_count(0, 1);
	data->set_collision_polygon_points(0, 0, Vector<Vector2>({ Vector2(-8, -8), Vector2(8, -8), Vector2(8, 8), Vector2(-8, 8) }));
	auto *layer = memnew(TileMapLayer);
	layer->set_tile_set(tileset);
	layer->set_cell(Vector2i(), 0, Vector2i());
	scene.root->add_child(layer);
	auto *sprites = scene.engine->get_sprite();
	GdObj sprite = sprites->create_bare_sprite(Vector2(8, -8));
	sprites->set_physics_mode(sprite, SpxSprite::STATIC);
	sprites->set_collider_rect(sprite, Vector2(), Vector2(16, 16));
	finder->add_all_obstacles(scene.root);
	finder->add_all_obstacles(scene.root);
	finder->set_sprite_obstacle(sprite, false);
	CHECK(finder->is_cell_solid(Vector2i()));
	CHECK_FALSE(finder->is_cell_solid(Vector2i(1, 0)));
	// A one-cell local polygon spans multiple navigation cells when scaled.
	layer->set_scale(Vector2(3, 3));
	finder->add_all_obstacles(scene.root);
	CHECK(finder->is_cell_solid(Vector2i(2, 2)));
	CHECK_FALSE(finder->is_cell_solid(Vector2i(3, 2)));
	layer->set_scale(Vector2(1, 1));
	finder->add_all_obstacles(scene.root);
	CHECK_FALSE(finder->is_cell_solid(Vector2i(2, 2)));
	sprites->get_sprite(sprite)->hide();
	layer->erase_cell(Vector2i());
	finder->add_all_obstacles(scene.root);
	CHECK_FALSE(finder->is_cell_solid(Vector2i()));
	layer->set_cell(Vector2i(), 0, Vector2i());
	finder->add_all_obstacles(scene.root);
	CHECK(finder->is_cell_solid(Vector2i()));
	data->set_collision_polygons_count(0, 0);
	finder->add_all_obstacles(scene.root);
	CHECK_FALSE(finder->is_cell_solid(Vector2i()));
	data->set_collision_polygons_count(0, 1);
	data->set_collision_polygon_points(0, 0, Vector<Vector2>({ Vector2(-8, -8), Vector2(8, -8), Vector2(8, 8), Vector2(-8, 8) }));
	layer->set_cell(Vector2i(100, 100), 0, Vector2i());
	layer->set_cell(Vector2i(1, 0), 99, Vector2i());
	auto *empty_layer = memnew(TileMapLayer);
	scene.root->add_child(empty_layer);
	finder->add_all_obstacles(scene.root);
	CHECK(finder->is_cell_solid(Vector2i()));
	CHECK_FALSE(finder->is_cell_solid(Vector2i(1, 0)));
	Ref<TileSet> empty_tileset;
	empty_tileset.instantiate();
	empty_layer->set_tile_set(empty_tileset);
	finder->add_all_obstacles(scene.root);
	scene.root->remove_child(layer);
	finder->find_path(Vector2(-24, 8), Vector2(24, 8));
	CHECK_FALSE(finder->is_cell_solid(Vector2i()));
	scene.root->add_child(layer);
	finder->add_all_obstacles(scene.root);
	CHECK(finder->is_cell_solid(Vector2i()));
	SUBCASE("Immediate deletion releases occupancy on the next query") {}
	SUBCASE("Queued deletion releases occupancy before the node leaves the tree") {
		layer->queue_free();
		finder->find_path(Vector2(-24, 8), Vector2(24, 8));
		CHECK_FALSE(finder->is_cell_solid(Vector2i()));
	}
	memdelete(layer);
	finder->find_path(Vector2(-24, 8), Vector2(24, 8));
	CHECK_FALSE(finder->is_cell_solid(Vector2i()));
}

} // namespace TestSpxPathFinder

#endif // TEST_SPX_PATH_FINDER_H
