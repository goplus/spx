/**************************************************************************/
/*  test_spx_scene_mgr.h                                                */
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

#ifndef TEST_SPX_SCENE_MGR_H
#define TEST_SPX_SCENE_MGR_H

#include "../spx_physics_mgr.h"
#include "../spx_scene_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/2d/sprite_2d.h"
#include "scene/2d/tile_map_layer.h"
#include "scene/resources/2d/capsule_shape_2d.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/2d/convex_polygon_shape_2d.h"
#include "scene/resources/2d/rectangle_shape_2d.h"
#include "scene/resources/placeholder_textures.h"
#include "spx_test_fixtures.h"

namespace TestSpxSceneMgr {

TEST_CASE("[SceneTree][SPX] Scene manager owns render sprites and static colliders") {
	TestSpx::TempDirectory files("spx_scene");
	files.write_png("sprite.png", Vector2i(16, 8), Color(1, 0, 0));
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_scene();
	const CharString path = files.file("sprite.png").utf8();
	const GdObj render = manager->create_render_sprite(path.get_data(), Vector2(10, 20), 30, Vector2(2, 3), 4, Vector2(1, 2));
	REQUIRE(render != NULL_OBJECT_ID);
	REQUIRE(manager->id_pure_sprites.has(render));
	auto *node = dynamic_cast<Node2D *>(manager->id_pure_sprites[render]);
	REQUIRE(node != nullptr);
	CHECK(node->get_parent() == manager->pure_sprite_root);
	CHECK(node->get_position() == Vector2(10, -20));
	CHECK(node->get_rotation_degrees() == doctest::Approx(30));
	CHECK(node->get_scale() == Vector2(2, 3));
	CHECK(node->get_z_index() == 4);
	auto *render_node = Object::cast_to<Sprite2D>(node);
	REQUIRE(render_node != nullptr);
	REQUIRE(render_node->get_texture().is_valid());
	CHECK(render_node->get_texture()->get_size() == Vector2(16, 8));

	float dimensions[] = { 6, 10 };
	GdArrayInfo parameters = {};
	parameters.type = GD_ARRAY_TYPE_FLOAT;
	parameters.size = 2;
	parameters.data = dimensions;
	const GdObj body = manager->create_static_sprite(path.get_data(), Vector2(5, 7), 15, Vector2(1, 2), 3, Vector2(2, 4), (int)ColliderType::RECT, Vector2(3, 5), &parameters);
	REQUIRE(body != NULL_OBJECT_ID);
	auto *static_node = dynamic_cast<Node2D *>(manager->id_pure_sprites[body]);
	REQUIRE(static_node != nullptr);
	CHECK(static_node->get_position() == Vector2(5, -7));
	CHECK(static_node->get_scale() == Vector2(1, 2));
	REQUIRE(static_node->get_child_count() == 2);
	auto *visual = Object::cast_to<Sprite2D>(static_node->get_child(0));
	auto *collider = Object::cast_to<CollisionShape2D>(static_node->get_child(1));
	REQUIRE(visual != nullptr);
	REQUIRE(collider != nullptr);
	CHECK(visual->get_position() == Vector2(2, -4));
	CHECK(visual->get_z_index() == 3);
	Ref<RectangleShape2D> shape = collider->get_shape();
	REQUIRE(shape.is_valid());
	CHECK(shape->get_size() == Vector2(6, 10));
	CHECK(collider->get_position() == Vector2(3, -5));
	Vector<ISortableSprite *> sprites;
	manager->collect_sortable_sprites(sprites);
	CHECK(sprites.size() == 2);
	// Clearing managed actors must leave authored render/static sprites intact.
	auto *actors = scene.engine->get_sprite();
	const GdObj actor = actors->create_bare_sprite(Vector2());
	auto *actor_node = actors->get_sprite(actor);
	REQUIRE(actor_node);
	if (!actor_node) {
		return;
	}
	Node *actor_root = actor_node->get_parent();
	manager->destroy_all_sprites();
	CHECK(actors->get_sprite(actor) == nullptr);
	CHECK(actor_root->is_queued_for_deletion());
	CHECK(manager->id_pure_sprites.size() == 2);
	const GdObj replacement = actors->create_bare_sprite(Vector2(3, 4));
	auto *replacement_node = actors->get_sprite(replacement);
	REQUIRE(replacement_node);
	if (!replacement_node) {
		return;
	}
	CHECK(replacement_node->get_parent() != actor_root);
	manager->destroy_pure_sprite(render);
	CHECK_FALSE(manager->id_pure_sprites.has(render));
	CHECK(node->is_queued_for_deletion());
	manager->destroy_pure_sprite(render);
	Node *old_root = manager->pure_sprite_root;
	manager->on_reset(0);
	CHECK(manager->id_pure_sprites.is_empty());
	CHECK(old_root->is_queued_for_deletion());
	CHECK(manager->pure_sprite_root != old_root);
	CHECK(manager->pure_sprite_root->get_parent() == scene.root);
}

TEST_CASE("[SceneTree][SPX] Scene bounds and export snapshots include nested transformed tile layers") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_scene();
	const int initial_children = scene.root->get_child_count();
	ERR_PRINT_OFF;
	manager->export_scene_as_png(nullptr);
	manager->export_scene_as_png(scene.root);
	ERR_PRINT_ON;
	CHECK(scene.root->get_child_count() == initial_children);
	CHECK(manager->get_tilemap_bounds(nullptr) == Rect2());
	Node2D *group = memnew(Node2D);
	scene.root->add_child(group);
	group->set_position(Vector2(10, 20));
	TileMapLayer *layer = memnew(TileMapLayer);
	group->add_child(layer);
	CHECK(manager->get_tilemap_bounds(layer) == Rect2());
	Ref<TileSet> tileset;
	tileset.instantiate();
	tileset->set_tile_size(Vector2i(16, 16));
	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(16, 16));
	Ref<TileSetAtlasSource> atlas;
	atlas.instantiate();
	atlas->set_texture(texture);
	atlas->set_texture_region_size(Vector2i(16, 16));
	atlas->create_tile(Vector2i());
	tileset->add_source(atlas, 0);
	layer->set_tile_set(tileset);
	layer->set_cell(Vector2i(-1, 0), 0, Vector2i());
	layer->set_cell(Vector2i(1, 1), 0, Vector2i());
	manager->set_cached_cell_size(Vector2(16, 16));
	CHECK(manager->get_tilemap_bounds(layer) == Rect2(-16, 0, 48, 32));
	CHECK(manager->get_scene_bounds(scene.root) == Rect2(-6, 20, 48, 32));
	layer->set_scale(Vector2(2, 2));
	CHECK(manager->get_scene_bounds(scene.root) == Rect2(-22, 20, 96, 64));
	const int authored_children = scene.root->get_child_count(false);
	const int all_children = scene.root->get_child_count(true);
	ERR_PRINT_OFF;
	manager->export_scene_as_png(scene.root); // Runtime helpers cannot be duplicated.
	ERR_PRINT_ON;
	CHECK(scene.root->get_child_count(true) == all_children);

	manager->export_scene_as_png(group);
	auto *first = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(first);
	CHECK(first->get_size() == Vector2i(96, 64));
	CHECK(first->get_child_count() == 1);
	manager->on_update(0.25);
	CHECK_FALSE(first->is_queued_for_deletion());
	CHECK(manager->get_scene_bounds(scene.root) == Rect2(-22, 20, 96, 64));
	CHECK(scene.root->get_child_count(false) == authored_children);
	manager->export_scene_as_png(group);
	auto *second = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(second);
	CHECK(second != first);
	CHECK(second->get_size() == Vector2i(96, 64));
	REQUIRE(second->get_child_count() == 1);
	Node *copy = second->get_child(0);
	REQUIRE(copy);
	CHECK(copy->find_children("*", "SubViewport", true, false).is_empty());
	CHECK(first->is_queued_for_deletion());
	CHECK(first->get_parent() == nullptr);
	memdelete(first);
	manager->on_reset(0);
	CHECK(second->is_queued_for_deletion());
	manager->on_update(1.0); // Reset cancels saving the pending snapshot.
	memdelete(second);

	manager->export_scene_as_png(group);
	auto *removed = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(removed);
	memdelete(removed);
	manager->on_update(1.0); // Scene removal must not leave a dangling viewport.
	manager->export_scene_as_png(group);
	auto *completed = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(completed);
	ERR_PRINT_OFF;
	manager->on_update(1.0); // Headless rendering cannot provide an image to save.
	ERR_PRINT_ON;
	CHECK(completed->is_queued_for_deletion());
	memdelete(completed);
	manager->on_update(1.0); // A completed export is consumed exactly once.
	manager->export_scene_as_png(group);
	auto *detached = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(detached);
	scene.root->remove_child(detached);
	manager->on_reset(0); // Cancellation also owns a snapshot removed from the tree.
	CHECK(detached->is_queued_for_deletion());
	CHECK(detached->get_parent() == nullptr);
	manager->on_update(1.0);
	memdelete(detached);
	manager->export_scene_as_png(group);
	auto *last = Object::cast_to<SubViewport>(scene.root->get_child(-1, true));
	REQUIRE(last);
	manager->on_destroy();
	CHECK(last->is_queued_for_deletion());
	manager->on_update(1.0);
	memdelete(last);
}

TEST_CASE("[SceneTree][SPX] Static sprite creation supports circle, capsule and polygon colliders") {
	TestSpx::TempDirectory files("spx_static_shapes");
	files.write_png("sprite.png", Vector2i(16, 16), Color(1, 1, 1));
	const CharString path = files.file("sprite.png").utf8();
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_scene();
	float coords[] = { 6, 16, 0, 0, -6, 16 };
	GdArrayInfo params = {};
	params.data = coords;
	params.size = 6;
	params.type = GD_ARRAY_TYPE_FLOAT;
	for (auto type : { ColliderType::CIRCLE, ColliderType::CAPSULE, ColliderType::POLYGON }) {
		auto id = manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(type), Vector2(2, 3), &params);
		REQUIRE(id != 0);
		auto *node = dynamic_cast<Node2D *>(manager->id_pure_sprites[id]);
		REQUIRE(node);
		auto *collider = Object::cast_to<CollisionShape2D>(node->get_child(1));
		REQUIRE(collider);
		CHECK(collider->get_position() == Vector2(2, -3));
		if (type == ColliderType::CIRCLE) {
			Ref<CircleShape2D> shape = collider->get_shape();
			REQUIRE(shape.is_valid());
			CHECK(shape->get_radius() == 6);
		} else if (type == ColliderType::CAPSULE) {
			Ref<CapsuleShape2D> shape = collider->get_shape();
			REQUIRE(shape.is_valid());
			CHECK(shape->get_radius() == 3);
			CHECK(shape->get_height() == 16);
		} else {
			Ref<ConvexPolygonShape2D> shape = collider->get_shape();
			REQUIRE(shape.is_valid());
			CHECK(shape->get_points() == Vector<Vector2>({ Vector2(6, -16), Vector2(), Vector2(-6, -16) }));
		}
	}
	auto id = manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(ColliderType::NONE), Vector2(), nullptr);
	REQUIRE(id != 0);
	CHECK(dynamic_cast<SpxRenderSprite *>(manager->id_pure_sprites[id]) != nullptr);
	int before = manager->id_pure_sprites.size();
	params.size = -1;
	ERR_PRINT_OFF;
	CHECK(manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(ColliderType::RECT), Vector2(), &params) == 0);
	ERR_PRINT_ON;
	CHECK(manager->id_pure_sprites.size() == before);
	params.size = 2;
	params.data = nullptr;
	ERR_PRINT_OFF;
	CHECK(manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(ColliderType::RECT), Vector2(), &params) == 0);
	ERR_PRINT_ON;
	CHECK(manager->id_pure_sprites.size() == before);
	manager->create_pure_sprite(path.get_data(), Vector2(), 0);
	CHECK(manager->id_pure_sprites.size() == before + 1);
	for (auto type : { ColliderType::CIRCLE, ColliderType::RECT, ColliderType::CAPSULE }) {
		const auto default_id = manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(type), Vector2(), nullptr);
		REQUIRE(default_id != 0);
		auto *body = dynamic_cast<Node2D *>(manager->id_pure_sprites[default_id]);
		REQUIRE(body);
		auto *shape = Object::cast_to<CollisionShape2D>(body->get_child(1));
		REQUIRE(shape);
		CHECK(shape->get_shape().is_valid());
	}
	manager->on_destroy();
	manager->create_pure_sprite(path.get_data(), Vector2(), 0);
	CHECK(manager->create_render_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2()) == 0);
	CHECK(manager->create_static_sprite(path.get_data(), Vector2(), 0, Vector2(1, 1), 0, Vector2(), int(ColliderType::RECT), Vector2(), nullptr) == 0);
	manager->clear_pure_sprites();
	CHECK(manager->id_pure_sprites.is_empty());
}

TEST_CASE("[SceneTree][SPX] Scene manager switches reloads and unloads authored scenes") {
	TestSpx::TempDirectory files("spx_scene_switch");
	files.write("level.tscn", "[gd_scene format=3]\n[node name=\"Level\" type=\"Node2D\"]\nposition = Vector2(3,4)\n");
	TestSpx::EngineScene scene;
	auto *tree = SceneTree::get_singleton();
	Node *previous = tree->get_current_scene();
	tree->set_current_scene(nullptr);
	struct Restore {
		SceneTree *tree;
		Node *previous;
		~Restore() {
			tree->unload_current_scene();
			tree->process(0.0);
			tree->set_current_scene(previous);
		}
	} restore{ tree, previous };
	auto *manager = scene.engine->get_scene();
	auto path = files.file("level.tscn").utf8();
	manager->change_scene_to_file(path.get_data());
	tree->process(0.0);
	auto *level = Object::cast_to<Node2D>(tree->get_current_scene());
	REQUIRE(level);
	CHECK(level->get_name() == StringName("Level"));
	CHECK(level->get_position() == Vector2(3, 4));
	auto first_id = level->get_instance_id();
	level->set_position(Vector2(10, 20));
	REQUIRE(manager->reload_current_scene() == OK);
	tree->process(0.0);
	level = Object::cast_to<Node2D>(tree->get_current_scene());
	REQUIRE(level);
	CHECK(level->get_instance_id() != first_id);
	CHECK(level->get_position() == Vector2(3, 4));
	manager->unload_current_scene();
	tree->process(0.0);
	CHECK(tree->get_current_scene() == nullptr);
}

} // namespace TestSpxSceneMgr

#endif // TEST_SPX_SCENE_MGR_H
