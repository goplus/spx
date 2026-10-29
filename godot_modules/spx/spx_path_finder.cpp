/**************************************************************************/
/*  spx_path_finer.cpp                                                    */
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

#include "spx_path_finder.h"

#include "core/math/geometry_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/physics/static_body_2d.h"
#include "scene/2d/tile_map_layer.h"
#include "scene/resources/2d/rectangle_shape_2d.h"

#include "spx_abi.h"
#include "spx_coordinate.h"
#include "spx_engine.h"
#include "spx_scene_mgr.h"
#include "spx_sprite.h"
#include "spx_sprite_mgr.h"

#include <limits>

void SpxPathFinder::_bind_methods() {
	ClassDB::bind_method(D_METHOD("setup_grid", "size", "cell_size", "with_debug"), &SpxPathFinder::setup);
	ClassDB::bind_method(D_METHOD("add_all_obstacles", "root"), &SpxPathFinder::add_all_obstacles);
	ClassDB::bind_method(D_METHOD("find_path", "start", "end"), &SpxPathFinder::find_path);

	ClassDB::bind_method(D_METHOD("get_size"), &SpxPathFinder::get_size);
	ClassDB::bind_method(D_METHOD("get_cell_size"), &SpxPathFinder::get_cell_size);
	ClassDB::bind_method(D_METHOD("is_cell_solid", "cell"), &SpxPathFinder::is_cell_solid);
	ClassDB::bind_method(D_METHOD("cell_to_world_gd", "cell"), &SpxPathFinder::cell_to_world_gd);
}

SpxPathFinder::SpxPathFinder() {
	astar.instantiate();
}

SpxPathFinder::~SpxPathFinder() {
	if (drawer) {
		drawer->queue_free();
		drawer = nullptr;
	}
}

void SpxPathFinder::setup_spx(GdVec2 grid_size, GdVec2 cell_size, GdBool with_debug) {
	setup(grid_size, cell_size, with_debug);
}

void SpxPathFinder::setup(Vector2i grid_size, Vector2i cell_size, bool with_debug) {
	cached_cell_size = cell_size;
	Node *root = nullptr;

	if (SpxEngine::get_singleton()) {
		root = SpxEngine::get_singleton()->get_spx_root();
	}

	if (!root) {
		root = SceneTree::get_singleton()->get_current_scene();
	}

	if (root) {
		astar->clear();
		obstacle_cells.clear();
		cell_occupancy.clear();
		_setup_astar(root, grid_size, cell_size);

		add_all_obstacles(root);

		if (with_debug && !drawer) {
			drawer = memnew(PathDebugDrawer(this));
			root->add_child(drawer);
		}
	}
}

void SpxPathFinder::set_jumping_enabled(bool p_enabled) {
	astar->set_jumping_enabled(p_enabled);
}

void SpxPathFinder::add_all_obstacles(Node *root) {
	if (!root) {
		return;
	}

	List<Node *> stack;
	stack.push_back(root);

	while (!stack.is_empty()) {
		Node *node = stack.back()->get();
		stack.pop_back();

		for (int i = 0; i < node->get_child_count(); i++) {
			Node *child = node->get_child(i);

			if (StaticBody2D *body = Object::cast_to<StaticBody2D>(child)) {
				_process_static_obstacles(body);
			} else if (SpxSprite *sprite = Object::cast_to<SpxSprite>(child)) {
				_process_static_obstacles(sprite, sprite->get_physics_mode() == SpxSprite::PhysicsMode::STATIC);
			} else if (TileMapLayer *layer = Object::cast_to<TileMapLayer>(child)) {
				_process_tilemap_obstacles(layer);
			}

			stack.push_back(child);
		}
	}
}

void SpxPathFinder::set_sprite_obstacle(GdObj obj, bool enabled) {
	_process_sprite_obstacle(obj, enabled);
}

GdArray SpxPathFinder::find_path_spx(GdVec2 p_from, GdVec2 p_to) {
	auto path_points = find_path(spx_to_godot_vec2(p_from), spx_to_godot_vec2(p_to));
	auto count = path_points.size();
	if (count > std::numeric_limits<int32_t>::max() / 2) {
		return nullptr;
	}
	const int32_t output_count = static_cast<int32_t>(count) * 2;
	GdArray result = SpxAbi::create_array(GD_ARRAY_TYPE_FLOAT, output_count);
	if (result == nullptr || output_count == 0) {
		return result;
	}
	float *output = SpxAbi::get_array<float>(result, 0);
	if (output == nullptr) {
		SpxAbi::free_array(result);
		return nullptr;
	}

	for (auto i = 0; i < count; i++) {
		auto idx = i * 2;
		const Vector2 spx_point = godot_to_spx_vec2(path_points[i]);
		output[idx] = spx_point.x;
		output[idx + 1] = spx_point.y;
	}

	return result;
}

PackedVector2Array SpxPathFinder::find_path(Vector2 start, Vector2 end) {
	_prune_removed_obstacles();
	PackedVector2Array path;

	Vector2i from = _world_to_cell(start);
	Vector2i to = _world_to_cell(end);

	auto cell_path = astar->get_id_path(from, to, true);
	for (int i = 0; i < cell_path.size(); i++) {
		path.push_back(_cell_to_world(cell_path[i]));
	}

	return path;
}

Vector2i SpxPathFinder::_world_to_cell(const Vector2 &pos) const {
	return Vector2i(
			(int)Math::floor(pos.x / cached_cell_size.x),
			(int)Math::floor(pos.y / cached_cell_size.y));
}

Vector2 SpxPathFinder::_cell_to_world(const Vector2i &cell) const {
	return Vector2(
			cell.x * cached_cell_size.x + cached_cell_size.x * 0.5,
			cell.y * cached_cell_size.y + cached_cell_size.y * 0.5);
}

void SpxPathFinder::_replace_obstacle_cells(ObjectID p_owner, const HashSet<Vector2i> &p_cells) {
	if (const auto *previous = obstacle_cells.getptr(p_owner)) {
		for (const Vector2i &cell : *previous) {
			if (--cell_occupancy[cell] == 0) {
				cell_occupancy.erase(cell);
				astar->set_point_solid(cell, false);
			}
		}
	}
	if (p_cells.is_empty()) {
		obstacle_cells.erase(p_owner);
		return;
	}
	obstacle_cells[p_owner] = p_cells;
	for (const Vector2i &cell : p_cells) {
		cell_occupancy[cell]++;
		astar->set_point_solid(cell, true);
	}
}

void SpxPathFinder::_prune_removed_obstacles() {
	for (auto it = obstacle_cells.begin(); it;) {
		const ObjectID owner = it->key;
		++it;
		auto *node = Object::cast_to<Node>(ObjectDB::get_instance(owner));
		if (!node || !node->is_inside_tree() || node->is_queued_for_deletion()) {
			_replace_obstacle_cells(owner, {});
		}
	}
}

void SpxPathFinder::_setup_astar(Node *root, Vector2i &grid_size, Vector2i &cell_size) {
	Rect2 scene_bounds = _get_scene_bounds(root);
	Vector2 scene_center = scene_bounds.get_center();
	Vector2i scene_cells = _world_to_cell(scene_bounds.size);
	Vector2 center_cell = _world_to_cell(scene_center);

	Vector2i final_grid_size(
			MIN(scene_cells.x, grid_size.x),
			MIN(scene_cells.y, grid_size.y));

	if (final_grid_size.x <= 0 || final_grid_size.y <= 0) {
		final_grid_size = grid_size;
	}

	Vector2i final_grid_pos = (center_cell - final_grid_size / 2).floor();

	astar->set_region({ final_grid_pos, final_grid_size });
	astar->set_cell_size(cell_size);
	astar->set_diagonal_mode(AStarGrid2D::DIAGONAL_MODE_NEVER);
	astar->update();
}

void SpxPathFinder::_process_rectangle_shape(Node2D *owner, CollisionShape2D *shape, HashSet<Vector2i> &r_cells) {
	if (shape->is_disabled() || !shape->get_shape().is_valid()) {
		return;
	}

	RectangleShape2D *rect = Object::cast_to<RectangleShape2D>(shape->get_shape().ptr());
	if (!rect) {
		return;
	}

	// Keep the original corner arithmetic before flooring grid boundaries.
	const Vector2 half_size = rect->get_size() * 0.5f;
	const Vector2 corners[] = {
		Vector2(-half_size.x, -half_size.y), Vector2(half_size.x, -half_size.y),
		Vector2(half_size.x, half_size.y), Vector2(-half_size.x, half_size.y),
	};
	const Transform2D transform = owner->get_global_transform() * shape->get_transform();
	Vector2 min_p = transform.xform(corners[0]);
	Vector2 max_p = min_p;
	for (int i = 1; i < 4; i++) {
		const Vector2 point = transform.xform(corners[i]);
		min_p = min_p.min(point);
		max_p = max_p.max(point);
	}
	const Rect2i region = astar->get_region();
	const Vector2i start = _world_to_cell(min_p).max(region.position);
	const Vector2i end = _world_to_cell(max_p).min(region.get_end() - Vector2i(1, 1));

	for (int x = start.x; x <= end.x; x++) {
		for (int y = start.y; y <= end.y; y++) {
			r_cells.insert(Vector2i(x, y));
		}
	}
}

void SpxPathFinder::_process_static_obstacles(Node2D *body, bool add) {
	HashSet<Vector2i> cells;
	if (add && body->is_visible_in_tree()) {
		for (int j = 0; j < body->get_child_count(); j++) {
			if (auto *shape = Object::cast_to<CollisionShape2D>(body->get_child(j))) {
				_process_rectangle_shape(body, shape, cells);
			}
		}
	}
	_replace_obstacle_cells(body->get_instance_id(), cells);
}

void SpxPathFinder::_process_tilemap_obstacles(TileMapLayer *layer, int p_layer_id) {
	if (!layer) {
		return;
	}

	HashSet<Vector2i> cells;
	Ref<TileSet> tileset = layer->get_tile_set();
	if (tileset.is_null() || p_layer_id < 0 || p_layer_id >= tileset->get_physics_layers_count()) {
		_replace_obstacle_cells(layer->get_instance_id(), cells);
		return;
	}
	const Rect2i region = astar->get_region();
	Array used_cells = layer->get_used_cells();

	for (int i = 0; i < used_cells.size(); ++i) {
		Vector2i cell = used_cells[i];
		TileData *td = layer->get_cell_tile_data(cell);
		if (!td) {
			continue;
		}

		int poly_count = td->get_collision_polygons_count(p_layer_id);

		if (poly_count <= 0) {
			continue;
		}

		Vector2 cell_local = layer->map_to_local(cell);
		Vector2 cell_global = layer->to_global(cell_local);
		Vector2i center_cell = _world_to_cell(cell_global);
		if (region.has_point(center_cell)) {
			cells.insert(center_cell);
		}

		Transform2D xform = layer->get_global_transform();

		for (int j = 0; j < poly_count; ++j) {
			Vector<Vector2> poly = td->get_collision_polygon_points(p_layer_id, j);
			if (poly.is_empty()) {
				continue;
			}

			// Even a small local polygon can cover neighboring grid cells after
			// the layer transform (for example, a scaled tilemap) is applied.
			Vector<Vector2> world_poly;
			world_poly.resize(poly.size());
			for (int k = 0; k < poly.size(); ++k) {
				world_poly.write[k] = xform.xform(cell_local + poly[k]);
			}

			Vector2 min_w = world_poly[0];
			Vector2 max_w = world_poly[0];
			for (int k = 1; k < world_poly.size(); ++k) {
				min_w = min_w.min(world_poly[k]);
				max_w = max_w.max(world_poly[k]);
			}

			Vector2i min_cell = _world_to_cell(min_w).max(region.position);
			Vector2i max_cell = _world_to_cell(max_w).min(region.get_end() - Vector2i(1, 1));

			for (int cx = min_cell.x; cx <= max_cell.x; ++cx) {
				for (int cy = min_cell.y; cy <= max_cell.y; ++cy) {
					Vector2i candidate(cx, cy);
					if (Geometry2D::is_point_in_polygon(_cell_to_world(candidate), world_poly)) {
						cells.insert(candidate);
					}
				}
			}
		}
	}
	_replace_obstacle_cells(layer->get_instance_id(), cells);
}

void SpxPathFinder::_process_sprite_obstacle(GdObj obj, bool add) {
	auto sprite = spriteMgr->get_sprite(obj);
	if (!sprite) {
		print_error("Try to get property of a null sprite gid = " + itos(obj));
		return;
	}

	if (add && sprite->get_physics_mode() != SpxSprite::PhysicsMode::STATIC) {
		print_error("Skip processing non-static obstacles.");
		return;
	}
	_process_static_obstacles(sprite, add);
}

Rect2 SpxPathFinder::_get_scene_bounds(Node *node) {
	if (!SpxEngine::get_singleton()) {
		return Rect2();
	}
	return sceneMgr->get_scene_bounds(node);
}

void SpxPathFinder::_destroy_drawer() {
	if (drawer) {
		drawer->queue_free();
		drawer = nullptr;
	}
}

void PathDebugDrawer::_bind_methods() {
	ClassDB::bind_method(D_METHOD("set_path_finder", "path_finder"), &PathDebugDrawer::set_path_finder);
	ClassDB::bind_method(D_METHOD("set_path", "path"), &PathDebugDrawer::set_path);
}

void PathDebugDrawer::_notification(int p_what) {
	if (p_what == NOTIFICATION_READY) {
		_ready();
	}

	if (p_what == NOTIFICATION_DRAW) {
		_draw();
	}

	if (p_what == NOTIFICATION_EXIT_TREE) {
		_exit_tree();
	}
}

void PathDebugDrawer::_ready() {
	set_process_input(true);
	set_z_index(1000);
	set_z_as_relative(false);
}

void PathDebugDrawer::_draw() {
	if (path_finder.is_null()) {
		return;
	}

	Rect2i region = path_finder->get_region();
	Vector2 cell_size = path_finder->get_cell_size();

	for (int x = 0; x < region.size.x; x++) {
		for (int y = 0; y < region.size.y; y++) {
			Vector2i cell(x + region.position.x, y + region.position.y);
			Vector2 world_pos = path_finder->cell_to_world_gd(cell);
			Vector2 top_left = world_pos - cell_size * 0.5;

			Color c = path_finder->is_cell_solid(cell) ? Color(1, 0, 0, 0.5) : Color(0, 1, 0, 0.1);
			draw_rect(Rect2(top_left, cell_size), c, true);
			draw_rect(Rect2(top_left, cell_size), Color(0, 0, 0), false);
		}
	}

	if (start_set) {
		draw_circle(start, 8, Color(0, 1, 1));
	}

	if (end_set) {
		draw_circle(end, 8, Color(1, 1, 0));
	}

	if (path.size() > 1) {
		draw_polyline(path, Color(0, 0, 1), 3.0, true);
	}
}

void PathDebugDrawer::_exit_tree() {
	if (path_finder.is_valid()) {
		path_finder->clear_drawer();
	}
}

void PathDebugDrawer::input(const Ref<InputEvent> &p_event) {
	if (path_finder.is_null()) {
		return;
	}

	Ref<InputEventMouseButton> mb = p_event;
	Ref<InputEventMouseMotion> mm = p_event;

	if (mb.is_valid()) {
		Vector2 mouse_pos = get_global_mouse_position();

		if (mb->is_pressed()) {
			if (mb->get_button_index() == MouseButton::LEFT) {
				if (!start_set) {
					start = mouse_pos;
					start_set = true;
				} else if (_is_near(mouse_pos, start)) {
					dragging = DRAG_START;
				}
			} else if (mb->get_button_index() == MouseButton::RIGHT) {
				if (!end_set) {
					end = mouse_pos;
					end_set = true;
				} else if (_is_near(mouse_pos, end)) {
					dragging = DRAG_END;
				}
			}

		} else {
			dragging = NONE;
		}

		_update_path();
	}

	if (mm.is_valid() && dragging != NONE) {
		Vector2 mouse_pos = get_global_mouse_position();
		if (dragging == DRAG_START) {
			start = mouse_pos;
		} else {
			end = mouse_pos;
		}

		_update_path();
	}
}

void PathDebugDrawer::set_path_finder(const Ref<SpxPathFinder> &p_path_finder) {
	path_finder = p_path_finder;
	queue_redraw();
}

void PathDebugDrawer::set_path(const PackedVector2Array &p_path) {
	path = p_path;
	queue_redraw();
}

void PathDebugDrawer::_update_path() {
	if (start_set || end_set) {
		queue_redraw();
	}

	if (!start_set || !end_set) {
		return;
	}
	path = path_finder->find_path(start, end);
	queue_redraw();
}

bool PathDebugDrawer::_is_near(const Vector2 &p1, const Vector2 &p2) const {
	return p1.distance_to(p2) <= drag_threshold;
}
