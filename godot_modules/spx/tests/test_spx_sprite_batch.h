/**************************************************************************/
/*  test_spx_sprite_batch.h                                               */
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

#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "tests/test_macros.h"

#include <climits>
#include <limits>

namespace TestSpxSpriteBatch {

class SpriteMgrProbe : public SpxSpriteMgr {
public:
	void register_sprite(SpxSprite *p_sprite) { _register_sprite(p_sprite); }
};

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

	const GdObj ids[] = {id, 999, id};
	float out[] = {42, 42, 42, 42, 42, 42, 42, 42};
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
	const GdObj ids[] = {1, 2};
	float out[] = {3, 4, 5};
	ERR_PRINT_OFF;
	CHECK_FALSE(manager.batch_retrieve_positions(nullptr, 2, out, 4));
	CHECK_FALSE(manager.batch_retrieve_positions(ids, 2, nullptr, 4));
	CHECK_FALSE(manager.batch_retrieve_positions(ids, INT_MAX, out, 3));
	for (int count : {0, -1, 2}) {
		CHECK_FALSE(manager.batch_retrieve_positions(ids, count, out, 3));
		CHECK_EQ(out[0], 3);
		CHECK_EQ(out[1], 4);
		CHECK_EQ(out[2], 5);
	}
	CHECK_FALSE(manager.batch_retrieve_positions(ids, 1, out, 3));
	CHECK_EQ(out[0], 3);
	CHECK_EQ(out[1], 4);
	CHECK_EQ(out[2], 5);
	CHECK(manager.batch_retrieve_positions(nullptr, 0, nullptr, 0));
	ERR_PRINT_ON;
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

} // namespace TestSpxSpriteBatch

#endif // TEST_SPX_SPRITE_BATCH_H
