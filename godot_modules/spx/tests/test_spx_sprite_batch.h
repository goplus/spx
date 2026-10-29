/**************************************************************************/
/*  test_spx_sprite_batch.h                                             */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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

#ifndef TEST_SPX_SPRITE_BATCH_H
#define TEST_SPX_SPRITE_BATCH_H

#include "../spx_batch_validation.h"
#include "../spx_engine.h"
#include "../spx_physics_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"

#include <climits>
#include <limits>

namespace TestSpxSpriteBatch {

class SpriteMgrProbe : public SpxSpriteMgr {
public:
	void register_sprite(SpxSprite *p_sprite) { _register_sprite(p_sprite); }
};

TEST_CASE("[SPX] Batch count decoding rejects unsafe float-to-int conversions") {
	int decoded = -1;
	CHECK(SpxBatchValidation::decode_nonnegative_count(0.0f, decoded));
	CHECK_EQ(decoded, 0);
	CHECK(SpxBatchValidation::decode_nonnegative_count(42.0f, decoded));
	CHECK_EQ(decoded, 42);

	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(-1.0f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(1.5f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(std::numeric_limits<float>::infinity(), decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(std::numeric_limits<float>::quiet_NaN(), decoded));
	// INT_MAX rounds up to 2^31 as float32 and must be rejected before cast.
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(static_cast<float>(std::numeric_limits<int>::max()), decoded));
}

TEST_CASE("[SPX] Batch integer decoding accepts signed values and rejects fractions") {
	int decoded = 0;
	CHECK(SpxBatchValidation::decode_int(-4096.0f, decoded));
	CHECK_EQ(decoded, -4096);
	CHECK(SpxBatchValidation::decode_int(4096.0f, decoded));
	CHECK_EQ(decoded, 4096);
	CHECK_FALSE(SpxBatchValidation::decode_int(-0.25f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_int(std::numeric_limits<float>::quiet_NaN(), decoded));
}

TEST_CASE("[SPX] Batch finite preflight checks every selected lane") {
	const float finite_values[] = { 0.0f, -1.0f, 3.5f };
	CHECK(SpxBatchValidation::all_finite(finite_values, 3));

	const float non_finite_values[] = { 0.0f, std::numeric_limits<float>::infinity(), 3.5f };
	CHECK_FALSE(SpxBatchValidation::all_finite(non_finite_values, 3));
}

TEST_CASE("[SPX] Pixel collision sampling rejects unrepresentable bridge steps") {
	SpxSpriteMgr manager;
	manager.set_pixel_collision_sampling_step(3);
	for (GdInt step : { GdInt(INT_MAX) + 1, GdInt(1) << 32, std::numeric_limits<GdInt>::max() }) {
		ERR_PRINT_OFF;
		manager.set_pixel_collision_sampling_step(step);
		ERR_PRINT_ON;
		CHECK(manager.get_pixel_collision_sampling_step() == 3);
	}
	manager.set_pixel_collision_sampling_step(INT_MAX);
	CHECK(manager.get_pixel_collision_sampling_step() == INT_MAX);
}

TEST_CASE("[SceneTree][SPX] Dynamic impulses change momentum once independently of tick rate") {
	SceneTree *tree = SceneTree::get_singleton();
	for (int ticks_per_second : { 30, 60, 120 }) {
		for (real_t mass : { 1.0f, 2.0f, 0.0f }) {
			SpxSprite *sprite = memnew(SpxSprite);
			tree->get_root()->add_child(sprite);
			sprite->set_physics_mode(SpxSprite::DYNAMIC);
			sprite->set_use_gravity(false);
			sprite->set_drag(0);
			sprite->set_friction(0);
			sprite->set_mass(mass);
			sprite->set_velocity(Vector2(3, 4));
			sprite->add_impulse(Vector2(20, -40));
			sprite->add_impulse(Vector2(40, -80));
			const Vector2 expected = Vector2(3, 4) + Vector2(60, -120) / (mass == 0 ? 1 : mass);
			tree->physics_process(1.0 / ticks_per_second);
			CHECK(sprite->get_velocity().is_equal_approx(expected));
			tree->physics_process(1.0 / ticks_per_second);
			CHECK(sprite->get_velocity().is_equal_approx(expected));
			memdelete(sprite);
		}
	}
}

TEST_CASE("[SceneTree][SPX] Position batches use independently sized native buffers") {
	SpriteMgrProbe manager;
	SpxSprite *sprite = memnew(SpxSprite);
	constexpr GdObj id = 0x112233447fc00001;
	sprite->set_gid(id);
	sprite->set_position(Vector2(3, 4));
	SceneTree::get_singleton()->get_root()->add_child(sprite);
	manager.register_sprite(sprite);

	const GdObj ids[] = { id, 999, id };
	float out[] = { 42, 42, 42, 42, 42, 42, 42, 42 };
	CHECK(manager.batch_retrieve_positions(ids, 3, out, 6));
	CHECK_EQ(out[0], 3);
	CHECK_EQ(out[1], -4);
	CHECK(Math::is_nan(out[2]));
	CHECK(Math::is_nan(out[3]));
	CHECK_EQ(out[4], 3);
	CHECK_EQ(out[5], -4);
	CHECK_EQ(out[6], 42);
	CHECK_EQ(out[7], 42);
	CHECK_EQ(ids[0], id);

	sprite->set_position(Vector2(5, 6));
	CHECK(manager.batch_retrieve_positions(ids, 3, out, 6));
	CHECK_EQ(out[0], 5);
	CHECK_EQ(out[1], -6);
	manager.destroy_sprite(id);
}

TEST_CASE("[SceneTree][SPX] Position batches reject invalid output before writing") {
	SpriteMgrProbe manager;
	const GdObj ids[] = { 1, 2 };
	float out[] = { 3, 4, 5 };
	ERR_PRINT_OFF;
	CHECK_FALSE(manager.batch_retrieve_positions(nullptr, 2, out, 4));
	CHECK_FALSE(manager.batch_retrieve_positions(ids, 2, nullptr, 4));
	for (int count : { INT_MAX, 0, -1, 1, 2 }) {
		CHECK_FALSE(manager.batch_retrieve_positions(ids, count, out, 3));
		CHECK_EQ(out[0], 3);
		CHECK_EQ(out[1], 4);
		CHECK_EQ(out[2], 5);
	}
	CHECK(manager.batch_retrieve_positions(nullptr, 0, nullptr, 0));
	ERR_PRINT_ON;
}

struct TransformFixture {
	Node2D *root = memnew(Node2D);
	SpxSpriteMgr *sprites = nullptr;

	TransformFixture() {
		SpxEngine::register_callbacks(nullptr);
		SpxEngine *engine = SpxEngine::get_singleton();
		SceneTree *tree = SceneTree::get_singleton();
		tree->get_root()->add_child(root);
		engine->set_root_node(tree, root);
		sprites = engine->get_sprite();
		sprites->on_awake();
	}

	~TransformFixture() {
		SpxEngine::shutdown();
		memdelete(root);
	}
};

TEST_CASE("[SceneTree][SPX] Single and batch transforms produce the same node state") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	TransformFixture fixture;
	SpxSpriteMgr &manager = *fixture.sprites;
	const GdObj single_id = manager.create_bare_sprite(Vector2());
	const GdObj batch_id = manager.create_bare_sprite(Vector2());
	SpxSprite *single = manager.get_sprite(single_id);
	SpxSprite *batched = manager.get_sprite(batch_id);
	for (SpxSprite *sprite : { single, batched }) {
		REQUIRE(sprite != nullptr);
		sprite->set_physics_mode(SpxSprite::KINEMATIC);
		REQUIRE(sprite->get_collider(false) != nullptr);
		REQUIRE(sprite->get_collider(true) != nullptr);
	}

	// Each row has the eight transform lanes after the ID. Exercise both Y
	// conversions and repeated visibility changes on the same pair of nodes.
	const float transforms[][8] = {
		{ 12.5f, -24.0f, 0.5f, 2.0f, 3.0f, -4.0f, 5.0f, 1.0f },
		{ -7.0f, 9.5f, -0.75f, 0.5f, 1.5f, 6.0f, -8.0f, 0.0f },
		{ 3.0f, -2.0f, 1.25f, 4.0f, 0.25f, -1.0f, 2.0f, 0.0f },
		{ 0.0f, 0.0f, 0.0f, 1.0f, 1.0f, 0.0f, 0.0f, 1.0f },
	};
	for (const auto &values : transforms) {
		const GdVec2 pos(values[0], values[1]);
		const GdVec2 scale(values[3], values[4]);
		const GdVec2 pivot(values[5], values[6]);
		const bool visible = values[7] != 0.0f;
		manager.set_transform(single_id, pos, values[2], scale, visible, pivot);
		const float batch[] = {
			1.0f,
			0.0f,
			static_cast<float>(batch_id),
			values[0],
			values[1],
			values[2],
			values[3],
			values[4],
			values[5],
			values[6],
			values[7],
		};
		manager.batch_update_transforms(batch, sizeof(batch) / sizeof(batch[0]));

		for (SpxSprite *sprite : { single, batched }) {
			CHECK(sprite->get_position() == Vector2(pos.x, -pos.y));
			CHECK(sprite->get_rotation() == doctest::Approx(values[2]));
			CHECK(sprite->get_scale().is_equal_approx(scale));
			CHECK(sprite->get_render_offset() == Vector2(pivot.x, -pivot.y));
			CHECK_EQ(sprite->is_visible(), visible);
			CHECK_EQ(sprite->get_collider(false)->is_disabled(), !visible);
			CHECK_EQ(sprite->get_collider(true)->is_disabled(), !visible);
		}
	}
}

TEST_CASE("[SceneTree][SPX] Transform batch uses destroy-wins semantics") {
	SpriteMgrProbe manager;
	SpxSprite *sprite = memnew(SpxSprite);
	constexpr GdObj id = 7;
	sprite->set_gid(id);
	sprite->set_position(Vector2(3, 4));
	SceneTree::get_singleton()->get_root()->add_child(sprite);
	manager.register_sprite(sprite);

	// A malformed later record must reject the whole packet before the first
	// record changes the node.
	const float malformed_batch[] = {
		2.0f,
		0.0f,
		7.0f,
		50.0f,
		60.0f,
		0.0f,
		1.0f,
		1.0f,
		0.0f,
		0.0f,
		1.0f,
		7.0f,
		std::numeric_limits<float>::quiet_NaN(),
		60.0f,
		0.0f,
		1.0f,
		1.0f,
		0.0f,
		0.0f,
		1.0f,
	};
	ERR_PRINT_OFF
	manager.batch_update_transforms(malformed_batch, sizeof(malformed_batch) / sizeof(malformed_batch[0]));
	ERR_PRINT_ON
	CHECK_EQ(sprite->get_position(), Vector2(3, 4));

	// Header: one update and two duplicate deletes. The update would move the
	// sprite if it were applied before deletion.
	const float transform_batch[] = {
		1.0f,
		2.0f,
		7.0f,
		100.0f,
		200.0f,
		0.0f,
		1.0f,
		1.0f,
		0.0f,
		0.0f,
		1.0f,
		7.0f,
		7.0f,
	};
	manager.batch_update_transforms(transform_batch, sizeof(transform_batch) / sizeof(transform_batch[0]));

	CHECK(sprite->is_queued_for_deletion());
	CHECK_EQ(sprite->get_position(), Vector2(3, 4));
	CHECK_EQ(manager.get_sprite(id), nullptr);
	CHECK_FALSE(manager.is_sprite_alive(id));

	// Later batches in the same frame must also ignore the tombstoned node.
	const float visual_batch[] = {
		1.0f,
		7.0f,
		9.0f,
		9.0f,
		0.0f,
		0.0f,
		0.0f,
		0.0f,
		0.0f,
		0.0f,
	};
	manager.batch_update_visuals(visual_batch, sizeof(visual_batch) / sizeof(visual_batch[0]));
	CHECK_EQ(sprite->get_render_scale(), Vector2(1, 1));
}

TEST_CASE("[SceneTree][SPX] Physics batches preserve raw masks and update all supported properties") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2());
	auto bits = [](uint32_t value) {
		float f;
		memcpy(&f, &value, 4);
		return f;
	};
	const float lo = bits(uint32_t(id)), hi = bits(uint32_t(uint64_t(id) >> 32));
	float data[] = { 14,
		4, lo, hi, bits(SpxSprite::DYNAMIC), 0, 0,
		1, lo, hi, 20, 30, 0,
		2, lo, hi, 50, 0, 0,
		3, lo, hi, 2, 0, 0,
		5, lo, hi, 0, 0, 0,
		6, lo, hi, 0.5, 0, 0,
		7, lo, hi, 0.25, 0, 0,
		8, lo, hi, 0.75, 0, 0,
		9, lo, hi, bits(0x80000001), 0, 0,
		10, lo, hi, bits(0x7fffffff), 0, 0,
		11, lo, hi, bits(0xffffffff), 0, 0,
		12, lo, hi, bits(0x80000000), 0, 0,
		13, lo, hi, 0, 0, 0,
		14, lo, hi, 0, 0, 0 };
	manager->batch_update_physics(data, 85);
	auto *sprite = manager->get_sprite(id);
	CHECK(sprite->get_velocity() == Vector2(20, -30));
	CHECK(sprite->get_physics_mode() == SpxSprite::DYNAMIC);
	CHECK(manager->get_gravity(id) == 50);
	CHECK(manager->get_mass(id) == 2);
	CHECK_FALSE(manager->is_use_gravity(id));
	CHECK(manager->get_gravity_scale(id) == 0.5);
	CHECK(manager->get_drag(id) == 0.25);
	CHECK(manager->get_friction(id) == 0.75);
	CHECK(manager->get_collision_layer(id) == 0x80000001LL);
	CHECK(manager->get_collision_mask(id) == 0x7fffffffLL);
	CHECK(manager->get_trigger_layer(id) == 0xffffffffLL);
	CHECK(manager->get_trigger_mask(id) == 0x80000000LL);
	CHECK_FALSE(manager->is_collision_enabled(id));
	CHECK_FALSE(manager->is_trigger_enabled(id));
	float invalid[] = { 2, 1, lo, hi, 99, 99, 0, 3, lo, hi, std::numeric_limits<float>::quiet_NaN(), 0, 0 };
	ERR_PRINT_OFF;
	manager->batch_update_physics(invalid, 13);
	ERR_PRINT_ON;
	CHECK(sprite->get_velocity() == Vector2(20, -30));
	CHECK(sprite->get_mass() == 2);
}

TEST_CASE("[SceneTree][SPX] Visual batches apply scales and optional z-order atomically") {
	TestSpx::TempDirectory files("spx_batch_uv");
	files.write("sprite.gdshader", "shader_type canvas_item; uniform vec4 uv_remap;");
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2());
	REQUIRE(GdObj(float(id)) == id);
	auto *sprite = manager->get_sprite(id);
	const auto shader_path = files.file("sprite.gdshader").utf8();
	manager->set_material_shader(id, shader_path.get_data());
	float values[] = { 1, float(id), 2, 3, 7, 1, 0, 0, 1, 1 };
	manager->batch_update_visuals(values, 10);
	CHECK(Vector2(sprite->get_render_scale()) == Vector2(2, 3));
	CHECK(sprite->get_z_index() == 7);
	values[2] = 4;
	values[3] = 5;
	values[4] = 100;
	values[5] = 0;
	manager->batch_update_visuals(values, 10);
	CHECK(Vector2(sprite->get_render_scale()) == Vector2(4, 5));
	CHECK(sprite->get_z_index() == 7);
	values[5] = 2;
	values[6] = 0.25f;
	values[7] = 0.5f;
	values[8] = 0.75f;
	values[9] = 1;
	manager->batch_update_visuals(values, 10);
	auto uv = sprite->get_material_params_vec4("uv_remap");
	CHECK(Vector4(uv.x, uv.y, uv.z, uv.w) == Vector4(0.25, 0.5, 0.75, 1));
	const float invalid_flags[] = { 4, 0.5, std::numeric_limits<float>::infinity() };
	for (float flag : invalid_flags) {
		values[2] = 9;
		values[5] = flag;
		ERR_PRINT_OFF;
		manager->batch_update_visuals(values, 10);
		ERR_PRINT_ON;
		CHECK(Vector2(sprite->get_render_scale()) == Vector2(4, 5));
	}
	values[5] = 1;
	values[4] = 100000;
	ERR_PRINT_OFF;
	manager->batch_update_visuals(values, 10);
	ERR_PRINT_ON;
	CHECK(sprite->get_z_index() == 7);
	manager->set_pixel_collision_sampling_step(3);
	CHECK(manager->get_pixel_collision_sampling_step() == 3);
	ERR_PRINT_OFF;
	manager->set_pixel_collision_sampling_step(0);
	ERR_PRINT_ON;
	CHECK(manager->get_pixel_collision_sampling_step() == 1);
	const GdObj ids[] = { id, 0 };
	float positions[4];
	CHECK(manager->batch_retrieve_positions(ids, 2, positions, 4));
	CHECK(positions[0] == 0);
	CHECK(positions[1] == 0);
	CHECK(std::isnan(positions[2]));
	CHECK(std::isnan(positions[3]));
}

TEST_CASE("[SceneTree][SPX] Physics modes integrate forces gravity and drag without leaking impulses") {
	TestSpx::EngineScene scene;
	auto *sprites = scene.engine->get_sprite();
	GdObj id = sprites->create_bare_sprite(Vector2());
	auto *sprite = sprites->get_sprite(id);
	auto *tree = SceneTree::get_singleton();
	const double delta = 0.1;
	sprite->set_physics_mode(SpxSprite::NO_PHYSICS);
	sprite->set_velocity(Vector2(10, 20));
	sprite->add_force(Vector2(100, 100));
	sprite->add_impulse(Vector2(100, 100));
	tree->physics_process(delta);
	CHECK(sprite->get_position().is_equal_approx(Vector2(1, 2)));
	CHECK(sprite->get_velocity() == Vector2(10, 20));
	sprite->set_physics_mode(SpxSprite::STATIC);
	tree->physics_process(delta);
	CHECK(sprite->get_velocity() == Vector2());
	CHECK(sprite->get_position().is_equal_approx(Vector2(1, 2)));
	sprite->set_physics_mode(SpxSprite::KINEMATIC);
	sprite->set_velocity(Vector2(5, 0));
	tree->physics_process(delta);
	CHECK(sprite->get_velocity() == Vector2(5, 0));
	sprite->set_physics_mode(SpxSprite::DYNAMIC);
	sprite->set_velocity(Vector2());
	sprite->set_use_gravity(true);
	sprite->set_gravity(20);
	sprite->set_gravity_scale(2);
	sprite->set_drag(0);
	sprite->add_force(Vector2(10, 0));
	tree->physics_process(delta);
	Vector2 expected(1, 4 * SpxPhysicsDefine::get_global_gravity());
	CHECK(sprite->get_velocity().is_equal_approx(expected));
	sprite->set_use_gravity(false);
	sprite->set_drag(0.5f);
	expected += Vector2(1, 0);
	expected = expected.move_toward(Vector2(), 0.5 * expected.length() * delta * SpxPhysicsDefine::get_global_air_drag());
	tree->physics_process(delta);
	CHECK(sprite->get_velocity().is_equal_approx(expected));
	sprite->set_physics_mode(SpxSprite::STATIC);
	tree->physics_process(delta);
	sprite->set_physics_mode(SpxSprite::DYNAMIC);
	sprite->set_drag(0);
	tree->physics_process(delta);
	CHECK(sprite->get_velocity() == Vector2());
}

TEST_CASE("[SceneTree][SPX] Malformed batch envelopes never mutate or delete live sprites") {
	TestSpx::EngineScene scene;
	auto *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2(3, 4));
	auto *sprite = manager->get_sprite(id);
	const float nan = std::numeric_limits<float>::quiet_NaN();
	using Batch = void (SpxSpriteMgr::*)(const float *, int);
	for (Batch apply : { &SpxSpriteMgr::batch_update_transforms, &SpxSpriteMgr::batch_update_visuals, &SpxSpriteMgr::batch_update_physics }) {
		ERR_PRINT_OFF;
		(manager->*apply)(nullptr, 0);
		float header[] = { nan, 0 };
		(manager->*apply)(header, 0);
		(manager->*apply)(header, 2);
		header[0] = 2;
		(manager->*apply)(header, 2);
		ERR_PRINT_ON;
		CHECK(manager->is_sprite_alive(id));
		CHECK(Vector2(manager->get_position(id)) == Vector2(3, 4));
	}
	float transform[] = { 0, 1, -1 };
	ERR_PRINT_OFF;
	manager->batch_update_transforms(transform, 3);
	ERR_PRINT_ON;
	CHECK(manager->is_sprite_alive(id));
	float visual[] = { 1, float(id), 1, 1, 0, 2, 0, 0, 1, 1 };
	for (int field : { 1, 2, 6 }) {
		float old = visual[field];
		visual[field] = nan;
		ERR_PRINT_OFF;
		manager->batch_update_visuals(visual, 10);
		ERR_PRINT_ON;
		CHECK(Vector2(sprite->get_render_scale()) == Vector2(1, 1));
		visual[field] = old;
	}
	float physics[] = { 1, 0, 0, 0, 0, 0, 0 };
	uint32_t low = uint32_t(id), high = uint32_t(uint64_t(id) >> 32);
	memcpy(&physics[2], &low, 4);
	memcpy(&physics[3], &high, 4);
	for (float command : { nan, 999.0f, 1.0f }) {
		physics[1] = command;
		physics[4] = nan;
		ERR_PRINT_OFF;
		manager->batch_update_physics(physics, 7);
		ERR_PRINT_ON;
		CHECK(sprite->get_velocity() == Vector2());
	}
	CHECK(Vector2(manager->get_position(id)) == Vector2(3, 4));
}

} // namespace TestSpxSpriteBatch

#endif // TEST_SPX_SPRITE_BATCH_H
