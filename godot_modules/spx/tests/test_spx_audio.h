/**************************************************************************/
/*  test_spx_audio.h                                                    */
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

#ifndef TEST_SPX_AUDIO_H
#define TEST_SPX_AUDIO_H

#include "../spx_audio_mgr.h"
#include "../spx_engine.h"
#include "../spx_res_mgr.h"
#include "../spx_camera_mgr.h"
#include "scene/2d/audio_stream_player_2d.h"
#include "scene/main/window.h"
#include "scene/resources/audio_stream_wav.h"
#include "servers/audio_server.h"
#include "servers/physics_server_2d.h"
#include "tests/test_macros.h"

namespace TestSpxAudio {

// Drive the audio mixer explicitly so stopped voices release their streams
// before a subcase tears down the AudioServer.
class AudioDriver : public ::AudioDriver {
public:
	const char *get_name() const override { return "SPX audio test driver"; }
	Error init() override { return OK; }
	void start() override {}
	int get_mix_rate() const override { return 44100; }
	SpeakerMode get_speaker_mode() const override { return SPEAKER_MODE_STEREO; }
	void lock() override {}
	void unlock() override {}
	void finish() override {}
	void mix() {
		int32_t buffer[1024];
		audio_server_process(512, buffer);
	}
};

static void start_stream_playback(AudioStreamPlayer2D *player, float offset) {
	// The test driver mixes streams; production keeps the platform default.
	CHECK_EQ(player->get_playback_type(), AudioServer::PLAYBACK_TYPE_DEFAULT);
	player->stop();
	player->set_playback_type(AudioServer::PLAYBACK_TYPE_STREAM);
	player->play(offset);
	// Publish queued playback so pause assertions exercise an active stream.
	PhysicsServer2D *physics = PhysicsServer2D::get_singleton();
	physics->sync();
	player->notification(Node::NOTIFICATION_INTERNAL_PHYSICS_PROCESS);
	physics->end_sync();
}

TEST_CASE("[SceneTree][SPX] Audio effects preserve playback lifetimes") {
	AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
	::AudioDriver *previous_driver = ::AudioDriver::get_singleton();
	AudioDriver driver;
	driver.set_singleton();
	AudioServer *server = memnew(AudioServer);
	server->init();
	SceneTree *tree = SceneTree::get_singleton();
	Node *root = memnew(Node);
	tree->get_root()->add_child(root);
	struct Cleanup {
		Node *root;
		AudioDriver &driver;
		AudioServer *server;
		::AudioDriver *previous_driver;
		~Cleanup() {
			SpxEngine::shutdown();
			memdelete(root);
			driver.mix();
			server->update();
			server->finish();
			memdelete(server);
			previous_driver->set_singleton();
		}
	} cleanup{ root, driver, server, previous_driver };
	SpxEngine::register_callbacks(nullptr);
	SpxEngine *engine = SpxEngine::get_singleton();
	engine->set_root_node(tree, root);
	engine->on_awake();
	engine->get_res()->set_load_mode(false);

	Ref<AudioStreamWAV> stream;
	stream.instantiate();
	Vector<uint8_t> data;
	data.resize(441000);
	data.fill(64);
	stream->set_data(data);
	stream->set_mix_rate(44100);
	stream->set_path("res://spx_paused_audio_test.wav");

	SpxAudioMgr *audio = engine->get_audio();
	const GdObj object = audio->create_audio();
	const GdInt playback = audio->play_with_attenuation(
			object, "res://spx_paused_audio_test.wav", 0, 0, 100);
	REQUIRE(playback != 0);
	auto players = root->find_children("*", "AudioStreamPlayer2D", true, false);
	REQUIRE_EQ(players.size(), 1);
	AudioStreamPlayer2D *player =
			Object::cast_to<AudioStreamPlayer2D>(players[0]);
	REQUIRE(player != nullptr);
	start_stream_playback(player, 0.5f);
	REQUIRE(audio->is_playing(playback));
	const Ref<AudioStreamPlayback> playing_stream = player->get_stream_playback();
	const GdFloat position = audio->get_timer(playback);
	audio->set_pitch(object, 1.5f);
	CHECK_EQ(audio->get_pitch(object), 1.5f);
	CHECK_EQ(player->get_pitch_scale(), 1.5f);
	CHECK(player->get_stream_playback() == playing_stream);
	CHECK_EQ(audio->get_timer(playback), position);
	CHECK(audio->is_playing(playback));


	audio->pause(playback);
	audio->set_pitch(object, 0.5f);
	CHECK_EQ(player->get_pitch_scale(), 0.5f);
	CHECK(player->get_stream_playback() == playing_stream);
	CHECK_EQ(audio->get_timer(playback), position);
	CHECK(player->get_stream_paused());
	CHECK_FALSE(player->is_playing());
	CHECK(audio->is_playing(playback));
	audio->on_update(0.1f);
	CHECK(audio->is_playing(playback));
	audio->resume(playback);
	CHECK_FALSE(player->get_stream_paused());
	CHECK(audio->is_playing(playback));

	// Allocating an effect bus migrates an existing voice without restarting it.
	audio->set_pan(object, -0.4f);
	CHECK(audio->get_pan(object) == doctest::Approx(-0.4f));
	audio->set_volume(object, 0.25f);
	CHECK(audio->get_volume(object) == doctest::Approx(0.25f));
	CHECK(player->get_bus() != "Master");
	CHECK(player->get_stream_playback() == playing_stream);
	audio->pause(playback);
	SUBCASE("Seek and restart an existing playback") {
		audio->resume(playback);
		audio->set_timer(playback, 0.75f);
		CHECK(audio->get_timer(playback) == doctest::Approx(0.75f));
		CHECK(audio->restart(playback));
		CHECK(audio->get_timer(playback) == doctest::Approx(0.0f));
		audio->stop_all();
	}
	SUBCASE("Worker calls cannot mutate audio ownership or playback") {
		GdInt rejected = -1;
		struct Context {
			SpxAudioMgr *audio;
			GdObj object;
			GdInt *rejected;
		} context{ audio, object, &rejected };
		Thread worker;
		ERR_PRINT_OFF;
		worker.start([](void *data) {
			auto &context = *static_cast<Context *>(data);
			context.audio->on_update(0.1f);
			context.audio->on_reset(0);
			context.audio->on_destroy();
			context.audio->stop_all();
			context.audio->destroy_audio(context.object);
			*context.rejected = context.audio->play(context.object, "res://spx_paused_audio_test.wav");
			context.audio->pause(1);
		}, &context);
		worker.wait_to_finish();
		ERR_PRINT_ON;
		CHECK(rejected == 0);
		CHECK(audio->get_object(object) != nullptr);
		CHECK(audio->is_playing(playback));
		CHECK(player->get_stream_paused());
		audio->stop_all();
	}

	SUBCASE("Invalid playback handles remain inert") {
		audio->set_pitch(0, 4);
		audio->set_pan(0, 1);
		audio->set_volume(0, 0);
		CHECK(audio->play(0, "res://spx_paused_audio_test.wav") == 0);
		ERR_PRINT_OFF;
		CHECK(audio->play(object, "res://missing_spx_audio.wav") == 0);
		ERR_PRINT_ON;
		CHECK(audio->get_pitch(object) == 0.5f);
		CHECK(audio->get_volume(object) == doctest::Approx(0.25f));
		CHECK_FALSE(audio->restart(0));
		CHECK_FALSE(audio->is_playing(0));
		audio->pause(0);
		audio->resume(0);
		audio->stop(0);
		audio->set_loop(0, true);
		CHECK_FALSE(audio->get_loop(0));
		CHECK(audio->get_timer(0) == 0);
		CHECK(audio->get_pitch(0) == 0);
		CHECK(audio->get_pan(0) == 0);
		CHECK(audio->get_volume(0) == 0);
		audio->set_timer(0, 1.0f);
		auto *voice = audio->get_object(object);
		REQUIRE(voice);
		ERR_PRINT_OFF;
		voice->pause(0);
		voice->resume(0);
		voice->set_loop(0, true);
		voice->set_timer(0, 1);
		CHECK_FALSE(voice->is_playing(0));
		CHECK_FALSE(voice->restart(0));
		CHECK_FALSE(voice->get_loop(0));
		CHECK(voice->get_timer(0) == 0);
		ERR_PRINT_ON;
		audio->stop_all();
	}
	SUBCASE("Missing and queued voice owners reject playback without replacing a live voice") {
		auto *voice = audio->get_object(object);
		REQUIRE(voice);
		auto *owner = memnew(Node);
		root->add_child(owner);
		owner->queue_free();
		CHECK_FALSE(voice->play(playback, "res://spx_paused_audio_test.wav", owner));
		CHECK(voice->has_audio(playback));
		SpxAudio detached;
		detached.on_create(0, nullptr);
		CHECK_FALSE(detached.play(1, "res://spx_paused_audio_test.wav"));
		detached.on_destroy();
		audio->stop_all();
	}
	SUBCASE("Stop one paused playback") {
		audio->stop(playback);
	}
	SUBCASE("Stop all paused playbacks") {
		audio->stop_all();
	}
	SUBCASE("Playback finishes") {
		player->stop();
		audio->on_update(0.1f);
	}
	SUBCASE("Destroy the audio object") {
		audio->destroy_audio(object);
	}
	SUBCASE("Pitch updates concurrent, looping and future playbacks") {
		const GdInt looping = audio->play_with_attenuation(
				object, "res://spx_paused_audio_test.wav", 0, 0, 100);
		REQUIRE(looping != 0);
		players = root->find_children("*", "AudioStreamPlayer2D", true, false);
		REQUIRE_EQ(players.size(), 2);
		AudioStreamPlayer2D *loop_player =
				Object::cast_to<AudioStreamPlayer2D>(players[1]);
		REQUIRE(loop_player != nullptr);
		CHECK_EQ(loop_player->get_pitch_scale(), 0.5f);
		start_stream_playback(loop_player, 0.25f);
		audio->set_loop(looping, true);
		const Ref<AudioStreamPlayback> loop_stream = loop_player->get_stream_playback();
		const GdFloat loop_position = audio->get_timer(looping);
		audio->set_pitch(object, 2.0f);
		CHECK_EQ(player->get_pitch_scale(), 2.0f);
		CHECK(player->get_stream_paused());
		CHECK(player->get_stream_playback() == playing_stream);
		CHECK_EQ(loop_player->get_pitch_scale(), 2.0f);
		CHECK(loop_player->get_stream_playback() == loop_stream);
		CHECK_EQ(audio->get_timer(looping), loop_position);
		CHECK(audio->is_playing(looping));
		CHECK(audio->get_loop(looping));
		loop_player->stop();
		audio->on_update(0.1f);
		CHECK(audio->is_playing(looping));
		CHECK_EQ(loop_player->get_pitch_scale(), 2.0f);
		audio->stop_all();
	}
	SUBCASE("Pitch ignores queued players") {
		player->queue_free();
		audio->set_pitch(object, 2.0f);
		CHECK_EQ(player->get_pitch_scale(), 0.5f);
		audio->on_update(0.1f);
	}
	SUBCASE("Pitch tolerates deleted players") {
		memdelete(player);
		audio->set_pitch(object, 2.0f);
		CHECK_FALSE(audio->is_playing(playback));
		audio->on_update(0.1f);
	}
	SUBCASE("Restart retires a voice whose stream was removed") {
		player->set_stream(Ref<AudioStream>());
		CHECK_FALSE(audio->restart(playback));
		CHECK(player->is_queued_for_deletion());
		CHECK_FALSE(audio->get_object(object)->has_audio(playback));
	}
	SUBCASE("Camera-owned sounds follow camera lifetime") {
		GdInt camera_voice = audio->play_with_attenuation(object, "res://spx_paused_audio_test.wav", -1, 0, 100);
		REQUIRE(camera_voice != 0);
		auto children = engine->get_camera()->get_camera()->find_children("*", "AudioStreamPlayer2D", true, false);
		REQUIRE(children.size() == 1);
		auto *camera_player = Object::cast_to<AudioStreamPlayer2D>(children[0]);
		REQUIRE(camera_player);
		camera_player->queue_free();
		CHECK_FALSE(audio->is_playing(camera_voice));
		audio->stop_all();
	}
	CHECK_FALSE(audio->is_playing(playback));
}

} // namespace TestSpxAudio

#endif // TEST_SPX_AUDIO_H
