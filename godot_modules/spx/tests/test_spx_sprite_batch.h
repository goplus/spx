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

#include <limits>

namespace TestSpxSpriteBatch {

class SpriteMgrProbe : public SpxSpriteMgr {
public:
	void register_sprite(SpxSprite *p_sprite) { _register_sprite(p_sprite); }
};

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
