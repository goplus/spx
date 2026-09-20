/**************************************************************************/
/*  spx_audio.cpp                                                     */
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

#include "spx_audio.h"

#include "scene/2d/audio_stream_player_2d.h"
#include "scene/main/node.h"

#include "gdextension_spx_ext.h"
#include "spx_audio_bus_pool.h"
#include "spx_audio_mgr.h"
#include "spx_engine.h"
#include "spx_object_guard.h"
#include "spx_res_mgr.h"

AudioStreamPlayer2D *SpxAudio::_get_aid_audio(GdInt aid) const {
	const Voice *voice = voices.getptr(aid);
	if (voice == nullptr) {
		return nullptr;
	}
	AudioStreamPlayer2D *player = Object::cast_to<AudioStreamPlayer2D>(ObjectDB::get_instance(voice->player_id));
	return player != nullptr && !player->is_queued_for_deletion() ? player : nullptr;
}

void SpxAudio::_release_voice(GdInt aid) {
	AudioStreamPlayer2D *player = _get_aid_audio(aid);
	// Remove the handle before interacting with the scene-owned player. All
	// cleanup paths are also safe after its parent has already deleted it.
	voices.erase(aid);
	if (player != nullptr) {
		player->stop();
		player->queue_free();
	}
}

void SpxAudio::on_create(GdInt p_id, Node *p_root) {
	root_id = p_root != nullptr ? p_root->get_instance_id() : ObjectID();
	bus_name = SpxAudioBusPool::STR_BUS_SFX;
	owns_dedicated_bus = false;
}

void SpxAudio::stop_all() {
	while (!voices.is_empty()) {
		_release_voice(voices.begin()->key);
	}
}

void SpxAudio::on_destroy() {
	on_reset(0);
}

void SpxAudio::on_update(float delta) {
	Vector<GdInt> finished_aids;
	for (const KeyValue<GdInt, Voice> &entry : voices) {
		AudioStreamPlayer2D *player = _get_aid_audio(entry.key);
		if (player == nullptr || player->get_stream().is_null()) {
			finished_aids.push_back(entry.key);
		} else if (!player->is_playing() && !player->get_stream_paused()) {
			if (entry.value.loop) {
				player->play();
			} else {
				finished_aids.push_back(entry.key);
			}
		}
	}
	for (GdInt aid : finished_aids) {
		_release_voice(aid);
	}
}

void SpxAudio::on_reset(int reset_code) {
	stop_all();
	// free the bus
	if (owns_dedicated_bus) {
		audioPool->free(bus_name);
	}
	bus_name = SpxAudioBusPool::STR_BUS_SFX;
	owns_dedicated_bus = false;
}

bool SpxAudio::play(GdInt aid, GdString path, Node *owner, GdFloat attenuation, GdFloat max_distance) {
	auto path_str = SpxStr(path);
	Ref<AudioStream> stream = resMgr->load_audio(path_str);
	if (stream.is_null()) {
		return false;
	}
	Node *parent = owner != nullptr ? owner : Object::cast_to<Node>(ObjectDB::get_instance(root_id));
	if (parent == nullptr || parent->is_queued_for_deletion()) {
		return false;
	}
	_release_voice(aid);
	auto *audio = memnew(AudioStreamPlayer2D);
	parent->add_child(audio);
	audio->set_bus(bus_name);
	audio->set_stream(stream);
	audio->set_max_distance(max_distance);
	audio->set_attenuation(attenuation);
	audio->set_name(path_str);
	audio->set_pitch_scale(get_pitch());
	audio->play();
	voices[aid] = { audio->get_instance_id(), false };
	return true;
}

bool SpxAudio::has_audio(GdInt aid) const {
	return _get_aid_audio(aid) != nullptr;
}

GdBool SpxAudio::is_playing(GdInt aid) {
	SPX_AUDIO_GUARD_RETURN(aid, __func__, false)
	return audio->is_playing();
}

void SpxAudio::pause(GdInt aid) {
	SPX_AUDIO_GUARD_VOID(aid, __func__)
	audio->set_stream_paused(true);
}

void SpxAudio::resume(GdInt aid) {
	SPX_AUDIO_GUARD_VOID(aid, __func__)
	audio->set_stream_paused(false);
}

void SpxAudio::stop(GdInt aid) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "SPX audio voices may only be stopped on the engine main thread.");
	_release_voice(aid);
}

GdBool SpxAudio::restart(GdInt aid) {
	SPX_AUDIO_GUARD_RETURN(aid, __func__, false)
	if (audio->is_queued_for_deletion() || !audio->get_stream().is_valid()) {
		return false;
	}
	audio->play(0.0f);
	return audio->is_playing();
}

void SpxAudio::set_loop(GdInt aid, GdBool loop) {
	SPX_AUDIO_GUARD_VOID(aid, __func__)
	voices[aid].loop = loop;
}

GdBool SpxAudio::get_loop(GdInt aid) {
	SPX_AUDIO_GUARD_RETURN(aid, __func__, false)
	return voices[aid].loop;
}

GdFloat SpxAudio::get_timer(GdInt aid) {
	SPX_AUDIO_GUARD_RETURN(aid, __func__, 0)
	return audio->get_playback_position();
}

void SpxAudio::set_timer(GdInt aid, GdFloat time) {
	SPX_AUDIO_GUARD_VOID(aid, __func__)
	audio->seek(time);
}

void SpxAudio::set_pitch(GdFloat pitch) {
	cur_pitch = pitch;
	// is need to update the pitch of the all audios ?
}

GdFloat SpxAudio::get_pitch() {
	return cur_pitch;
}

void SpxAudio::set_pan(GdFloat pan) {
	if (!ensure_dedicated_bus()) {
		return;
	}
	audioPool->set_pan(bus_name, pan);
}

GdFloat SpxAudio::get_pan() {
	return audioPool->get_pan(bus_name);
}

void SpxAudio::set_volume(GdFloat volume) {
	if (!ensure_dedicated_bus()) {
		return;
	}
	audioPool->set_volume(bus_name, volume);
}

GdFloat SpxAudio::get_volume() {
	return audioPool->get_volume(bus_name);
}

bool SpxAudio::ensure_dedicated_bus() {
	if (!owns_dedicated_bus) {
		const StringName allocated_bus = audioPool->alloc();
		if (allocated_bus.is_empty()) {
			return false;
		}
		bus_name = allocated_bus;
		owns_dedicated_bus = true;

		for (const KeyValue<GdInt, Voice> &entry : voices) {
			AudioStreamPlayer2D *player = _get_aid_audio(entry.key);
			if (player != nullptr) {
				player->set_bus(bus_name);
			}
		}
	}
	return true;
}
