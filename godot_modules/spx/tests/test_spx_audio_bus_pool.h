/**************************************************************************/
/*  test_spx_audio_bus_pool.h                                             */
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

#ifndef TEST_SPX_AUDIO_BUS_POOL_H
#define TEST_SPX_AUDIO_BUS_POOL_H

#include "../spx_audio_bus_pool.h"
#include "core/math/math_funcs.h"
#include "servers/audio/effects/audio_effect_panner.h"
#include "servers/audio_server.h"
#include "tests/test_macros.h"

namespace TestSpxAudioBusPool {

static int add_bus(AudioServer *p_audio_server, const StringName &p_name, const StringName &p_send = "Master") {
	p_audio_server->add_bus();
	const int bus_id = p_audio_server->get_bus_count() - 1;
	p_audio_server->set_bus_name(bus_id, p_name);
	p_audio_server->set_bus_send(bus_id, p_send);
	return bus_id;
}

static void remove_bus(AudioServer *p_audio_server, const StringName &p_name) {
	const int bus_id = p_audio_server->get_bus_index(p_name);
	if (bus_id > 0) {
		p_audio_server->remove_bus(bus_id);
	}
}

TEST_CASE("[Audio][SPX] Audio bus pool borrows project buses without claiming prefix collisions") {
	AudioServer *audio_server = AudioServer::get_singleton();
	REQUIRE(audio_server != nullptr);
	REQUIRE(SpxAudioBusPool::get_singleton() == nullptr);

	const int borrowed_sfx = add_bus(audio_server, "Sfx");
	const int borrowed_music = add_bus(audio_server, "Music");
	const StringName colliding_name = "_spx_audio_bus_0";
	const int colliding_bus = add_bus(audio_server, colliding_name, "Sfx");
	constexpr float borrowed_sfx_volume_db = -3.0f;
	constexpr float colliding_volume_db = -7.0f;
	Ref<AudioEffectPanner> colliding_effect;
	colliding_effect.instantiate();
	colliding_effect->set_pan(0.25f);
	audio_server->set_bus_volume_db(borrowed_sfx, borrowed_sfx_volume_db);
	audio_server->set_bus_volume_db(colliding_bus, colliding_volume_db);
	audio_server->add_bus_effect(colliding_bus, colliding_effect);

	SpxAudioBusPool::init();
	CHECK_EQ(audio_server->get_bus_count(), 5);
	SpxAudioBusPool::reset();

	CHECK_EQ(audio_server->get_bus_index("Sfx"), borrowed_sfx);
	CHECK_EQ(audio_server->get_bus_index("Music"), borrowed_music);
	CHECK_EQ(audio_server->get_bus_index(colliding_name), colliding_bus);
	CHECK(Math::is_equal_approx(audio_server->get_bus_volume_db(borrowed_sfx), borrowed_sfx_volume_db));
	CHECK_EQ(audio_server->get_bus_send(colliding_bus), StringName("Sfx"));
	CHECK(Math::is_equal_approx(audio_server->get_bus_volume_db(colliding_bus), colliding_volume_db));
	CHECK_EQ(audio_server->get_bus_effect_count(colliding_bus), 1);
	CHECK(audio_server->get_bus_effect(colliding_bus, 0) == colliding_effect);

	SpxAudioBusPool::shutdown();
	CHECK_EQ(audio_server->get_bus_count(), 4);
	CHECK_EQ(audio_server->get_bus_index("Sfx"), borrowed_sfx);
	CHECK_EQ(audio_server->get_bus_index("Music"), borrowed_music);
	CHECK_EQ(audio_server->get_bus_index(colliding_name), colliding_bus);
	CHECK(Math::is_equal_approx(audio_server->get_bus_volume_db(colliding_bus), colliding_volume_db));

	remove_bus(audio_server, colliding_name);
	remove_bus(audio_server, "Music");
	remove_bus(audio_server, "Sfx");
	CHECK_EQ(audio_server->get_bus_count(), 1);
}

TEST_CASE("[Audio][SPX] Audio bus pool owns, reuses, resets, and removes its buses") {
	AudioServer *audio_server = AudioServer::get_singleton();
	REQUIRE(audio_server != nullptr);
	REQUIRE(SpxAudioBusPool::get_singleton() == nullptr);
	REQUIRE_EQ(audio_server->get_bus_count(), 1);

	SpxAudioBusPool::init();
	SpxAudioBusPool *pool = SpxAudioBusPool::get_singleton();
	REQUIRE(pool != nullptr);
	CHECK_EQ(audio_server->get_bus_count(), 4);

	const StringName first_bus = pool->alloc();
	REQUIRE(!first_bus.is_empty());
	pool->set_volume(first_bus, 0.25f);
	pool->set_pan(first_bus, 0.4f);
	CHECK(Math::is_equal_approx(pool->get_volume(first_bus), 0.25f));
	CHECK(Math::is_equal_approx(pool->get_pan(first_bus), 0.4f));
	pool->free(first_bus);

	const StringName reused_bus = pool->alloc();
	CHECK_EQ(reused_bus, first_bus);
	CHECK(Math::is_equal_approx(pool->get_volume(reused_bus), 1.0f));
	CHECK(Math::is_zero_approx(pool->get_pan(reused_bus)));
	pool->free(reused_bus);

	const int removed_bus_id = audio_server->get_bus_index(reused_bus);
	REQUIRE(removed_bus_id > 0);
	audio_server->remove_bus(removed_bus_id);
	SpxAudioBusPool::reset();

	const StringName recreated_bus = pool->alloc();
	const StringName second_bus = pool->alloc();
	CHECK_EQ(recreated_bus, reused_bus);
	CHECK(!second_bus.is_empty());
	CHECK(second_bus != recreated_bus);
	pool->set_volume(recreated_bus, 0.5f);
	pool->set_pan(recreated_bus, -0.5f);
	SpxAudioBusPool::reset();

	const StringName reset_bus = pool->alloc();
	REQUIRE(!reset_bus.is_empty());
	CHECK(Math::is_equal_approx(pool->get_volume(reset_bus), 1.0f));
	CHECK(Math::is_zero_approx(pool->get_pan(reset_bus)));
	pool->free(reset_bus);

	SpxAudioBusPool::shutdown();
	CHECK_EQ(audio_server->get_bus_count(), 1);
	CHECK_EQ(audio_server->get_bus_index("Sfx"), -1);
	CHECK_EQ(audio_server->get_bus_index("Music"), -1);

	SpxAudioBusPool::init();
	CHECK_EQ(audio_server->get_bus_count(), 4);
	SpxAudioBusPool::shutdown();
	CHECK_EQ(audio_server->get_bus_count(), 1);
}

} // namespace TestSpxAudioBusPool

#endif // TEST_SPX_AUDIO_BUS_POOL_H
