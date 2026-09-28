/**************************************************************************/
/*  test_spx_audio.h                                                      */
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

TEST_CASE("[SceneTree][SPX] Paused audio retains its playback lifetime") {
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
	player->stop();
	player->set_playback_type(AudioServer::PLAYBACK_TYPE_STREAM);
	player->play();
	// Publish pending playback before pausing: queued playback alone reports
	// playing even when paused, hiding the active-stream lifetime regression.
	PhysicsServer2D::get_singleton()->sync();
	player->notification(Node::NOTIFICATION_INTERNAL_PHYSICS_PROCESS);
	PhysicsServer2D::get_singleton()->end_sync();
	REQUIRE(audio->is_playing(playback));

	audio->pause(playback);
	CHECK(player->get_stream_paused());
	CHECK_FALSE(player->is_playing());
	CHECK(audio->is_playing(playback));
	audio->on_update(0.1f);
	CHECK(audio->is_playing(playback));
	audio->resume(playback);
	CHECK_FALSE(player->get_stream_paused());
	CHECK(audio->is_playing(playback));

	audio->pause(playback);
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
	CHECK_FALSE(audio->is_playing(playback));
}

} // namespace TestSpxAudio

#endif // TEST_SPX_AUDIO_H
