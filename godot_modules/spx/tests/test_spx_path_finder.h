/**************************************************************************/
/*  test_spx_path_finder.h                                                */
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
#include "scene/2d/tile_map_layer.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "scene/resources/placeholder_textures.h"
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

} // namespace TestSpxPathFinder

#endif // TEST_SPX_PATH_FINDER_H
