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

#include "../spx_audio.h"
#include "scene/2d/audio_stream_player_2d.h"
#include "servers/audio_server.h"
#include "tests/test_macros.h"

class TestSpxAudioInternalsAccessor {
public:
	static AudioStreamPlayer2D *create_player() {
		return SpxAudio::_create_player();
	}
};

namespace TestSpxAudio {

TEST_CASE("[Audio][SceneTree][SPX] Audio players use stream playback") {
	const int dummy_driver = AudioDriverManager::get_driver_count() - 1;
	REQUIRE(dummy_driver >= 0);
	AudioDriverManager::initialize(dummy_driver);
	AudioServer *audio_server = memnew(AudioServer);
	audio_server->init();

	AudioStreamPlayer2D *player = TestSpxAudioInternalsAccessor::create_player();
	REQUIRE(player != nullptr);
	CHECK_EQ(player->get_playback_type(), AudioServer::PLAYBACK_TYPE_STREAM);
	memdelete(player);

	audio_server->finish();
	memdelete(audio_server);
}

} // namespace TestSpxAudio

#endif // TEST_SPX_AUDIO_H
