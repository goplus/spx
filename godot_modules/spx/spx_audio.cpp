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

AudioStreamPlayer2D *SpxAudio::_get_aid_audio(GdInt aid) {
	if (aid_audios.has(aid)) {
		return aid_audios[aid];
	}
	return nullptr;
}

AudioStreamPlayer2D *SpxAudio::_create_player() {
	auto *audio = memnew(AudioStreamPlayer2D);
	// Web exports default to sample playback. Positional samples start before
	// their first panning update, which can make very short sounds play at zero
	// volume. Stream playback starts after that update and matches native SPX.
	audio->set_playback_type(AudioServer::PLAYBACK_TYPE_STREAM);
	return audio;
}

void SpxAudio::on_create(GdInt p_id, Node *p_root) {
	root = p_root;
	id = p_id;
	bus_name = SpxAudioBusPool::STR_BUS_SFX;
	owns_dedicated_bus = false;
}

void SpxAudio::stop_all() {
	for (List<AudioStreamPlayer2D *>::Element *item = audios.front(); item;) {
		item->get()->queue_free();
		item = item->next();
	}
	audios.clear();

	for (List<AudioStreamPlayer2D *>::Element *item = loop_audios.front(); item;) {
		item->get()->queue_free();
		item = item->next();
	}
	loop_audios.clear();

	aid_audios.clear();
}

void SpxAudio::on_destroy() {
	on_reset(0);
}

void SpxAudio::on_update(float delta) {
	// check the audio is done
	for (auto item = audios.front(); item;) {
		const auto audio = item->get();
		auto *next = item->next();
		if (!audio->is_playing()) {
			audio->queue_free();
			audios.erase(item);
			for (const auto &[aid, audio_player] : aid_audios) {
				if (audio_player == audio) {
					aid_audios.erase(aid);
					break;
				}
			}
		}
		item = next;
	}

	for (auto item = loop_audios.front(); item;) {
		const auto audio = item->get();
		auto *next = item->next();
		if (audio->get_stream().is_valid() && !audio->is_playing() && !audio->get_stream_paused()) {
			audio->play();
		}
		item = next;
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
	auto *audio = _create_player();
	if (owner != nullptr) {
		owner->add_child(audio);
	} else {
		root->add_child(audio);
	}
	audio->set_bus(bus_name);
	audio->set_stream(stream);
	audio->set_max_distance(max_distance);
	audio->set_attenuation(attenuation);
	audio->play();
	audio->set_name(path_str);
	audio->set_pitch_scale(get_pitch());
	audios.push_back(audio);
	aid_audios[aid] = audio;
	return true;
}

bool SpxAudio::has_audio(GdInt aid) const {
	return aid_audios.has(aid);
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
	SPX_AUDIO_GUARD_VOID(aid, __func__)
	audios.erase(audio.get());
	loop_audios.erase(audio.get());
	aid_audios.erase(aid);
	audio->stop();
	audio->queue_free();
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
	if (loop) {
		auto succ = audios.erase(audio.get());
		if (succ) {
			loop_audios.push_back(audio.get());
		}
	} else {
		auto succ = loop_audios.erase(audio.get());
		if (succ) {
			audios.push_back(audio.get());
		}
	}
}

GdBool SpxAudio::get_loop(GdInt aid) {
	SPX_AUDIO_GUARD_RETURN(aid, __func__, false)
	return loop_audios.find(audio.get()) != nullptr;
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

		for (auto item = audios.front(); item;) {
			item->get()->set_bus(bus_name);
			item = item->next();
		}

		for (auto item = loop_audios.front(); item;) {
			item->get()->set_bus(bus_name);
			item = item->next();
		}
	}
	return true;
}
