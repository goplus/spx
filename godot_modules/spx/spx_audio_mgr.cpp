/**************************************************************************/
/*  spx_audio_mgr.cpp                                                     */
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

#include "spx_audio_mgr.h"

#include "scene/2d/audio_stream_player_2d.h"
#include "scene/2d/camera_2d.h"

#include "spx_audio_bus_pool.h"
#include "spx_camera_mgr.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"
#include "spx_sprite.h"
#include "spx_sprite_mgr.h"

SpxAudio *SpxAudioMgr::_get_aid_audio(GdInt aid) {
	if (aid_audios.has(aid)) {
		return aid_audios[aid];
	}
	return nullptr;
}

void SpxAudioMgr::on_awake() {
	SpxBaseMgr::on_awake();
	// Initialize SpxAudioBusPool
	SpxAudioBusPool::init();
	_create_root("audio_root");
	g_audio_id = 0;
}

void SpxAudioMgr::on_update(float delta) {
	SpxBaseMgr::on_update(delta);
	_update_all(delta);

	// SpxAudio removes finished players during its update. Mirror that cleanup in
	// the manager so subsequent queries do not resolve a stale aid to the owner.
	MutexLock aid_lock(aid_mutex);
	Vector<GdInt> finished_aids;
	for (const auto &[aid, audio] : aid_audios) {
		if (audio == nullptr || !audio->has_audio(aid)) {
			finished_aids.push_back(aid);
		}
	}
	for (const GdInt &aid : finished_aids) {
		aid_audios.erase(aid);
	}
}

void SpxAudioMgr::on_reset(int reset_code) {
	{
		MutexLock aid_lock(aid_mutex);
		aid_audios.clear();
	}
	_reset_all(reset_code);
	SpxAudioBusPool::reset();
}

void SpxAudioMgr::on_destroy() {
	{
		MutexLock aid_lock(aid_mutex);
		aid_audios.clear();
	}
	_destroy_all();
	SpxAudioBusPool::shutdown();
	SpxBaseMgr::on_destroy();
}

GdObj SpxAudioMgr::create_audio() {
	return _create_object();
}

void SpxAudioMgr::stop_all() {
	if (unlikely(!_validate_main_thread(__func__))) {
		return;
	}

	Vector<GdObj> audio_ids;
	for (const KeyValue<GdObj, SpxAudio *> &E : id_objects) {
		audio_ids.push_back(E.key);
	}
	for (GdObj id : audio_ids) {
		SpxAudio *audio = _get_object_unsafe(id);
		if (audio != nullptr) {
			audio->stop_all();
		}
	}
	{
		MutexLock aid_lock(aid_mutex);
		aid_audios.clear();
	}
}

void SpxAudioMgr::destroy_audio(GdObj obj) {
	SpxAudio *audio = get_object(obj);
	if (audio != nullptr) {
		// Remove audio from aid_audios mapping
		MutexLock aid_lock(aid_mutex);
		Vector<GdInt> keys;
		for (const auto &[aid, audio_obj] : aid_audios) {
			if (audio_obj == audio) {
				keys.push_back(aid);
			}
		}
		for (const GdInt &key : keys) {
			aid_audios.erase(key);
		}
	}

	// Now destroy the object using parent class method
	destroy_object(obj);
}

void SpxAudioMgr::set_pitch(GdObj obj, GdFloat pitch) {
	if (!with_object(obj, [pitch](SpxAudio *audio) {
			audio->set_pitch(pitch);
		})) {
		print_error("try to access null SpxAudio object");
	}
}

GdFloat SpxAudioMgr::get_pitch(GdObj obj) {
	return with_object_ret<GdFloat>(obj, 0.0, [](SpxAudio *audio) {
		return audio->get_pitch();
	});
}

void SpxAudioMgr::set_pan(GdObj obj, GdFloat pan) {
	if (!with_object(obj, [pan](SpxAudio *audio) {
			audio->set_pan(pan);
		})) {
		print_error("try to access null SpxAudio object");
	}
}

GdFloat SpxAudioMgr::get_pan(GdObj obj) {
	return with_object_ret<GdFloat>(obj, 0.0, [](SpxAudio *audio) {
		return audio->get_pan();
	});
}

void SpxAudioMgr::set_volume(GdObj obj, GdFloat volume) {
	if (!with_object(obj, [volume](SpxAudio *audio) {
			audio->set_volume(volume);
		})) {
		print_error("try to access null SpxAudio object");
	}
}

GdFloat SpxAudioMgr::get_volume(GdObj obj) {
	return with_object_ret<GdFloat>(obj, 0.0, [](SpxAudio *audio) {
		return audio->get_volume();
	});
}

GdInt SpxAudioMgr::play(GdObj obj, GdString path) {
	return play_with_attenuation(obj, path, 0, 2000, 1);
}

GdInt SpxAudioMgr::play_with_attenuation(GdObj obj, GdString path, GdObj owner_id, GdFloat attenuation, GdFloat max_distance) {
	Node *audio_owner = nullptr;
	if (owner_id == -1) {
		audio_owner = static_cast<Node *>(cameraMgr->get_camera());
	} else {
		audio_owner = static_cast<Node *>(spriteMgr->get_sprite(owner_id));
	}

	if (audio_owner == nullptr) {
		audio_owner = static_cast<Node *>(root);
	}

	SpxAudio *audio = get_object(obj);
	if (audio == nullptr) {
		print_error("try to access null SpxAudio object");
		return 0;
	}

	GdInt aid = 0;
	{
		MutexLock aid_lock(aid_mutex);
		aid = ++g_audio_id;
	}
	if (!audio->play(aid, path, audio_owner, attenuation, max_distance)) {
		return 0;
	}
	{
		MutexLock aid_lock(aid_mutex);
		aid_audios[aid] = audio;
	}
	return aid;
}

GdBool SpxAudioMgr::is_playing(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return false;
	}
	if (!audio->has_audio(aid)) {
		MutexLock aid_lock(aid_mutex);
		aid_audios.erase(aid);
		return false;
	}
	return audio->is_playing(aid);
}

void SpxAudioMgr::pause(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return;
	}
	audio->pause(aid);
}

void SpxAudioMgr::resume(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return;
	}
	audio->resume(aid);
}

void SpxAudioMgr::stop(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
		if (audio != nullptr) {
			aid_audios.erase(aid);
		}
	}
	if (audio == nullptr) {
		return;
	}
	audio->stop(aid);
}

GdBool SpxAudioMgr::restart(GdInt aid) {
	MutexLock aid_lock(aid_mutex);
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio == nullptr) {
		return false;
	}
	GdBool restarted = audio->restart(aid);
	if (!restarted) {
		aid_audios.erase(aid);
	}
	return restarted;
}

void SpxAudioMgr::set_loop(GdInt aid, GdBool loop) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return;
	}
	audio->set_loop(aid, loop);
}

GdBool SpxAudioMgr::get_loop(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return false;
	}
	return audio->get_loop(aid);
}

GdFloat SpxAudioMgr::get_timer(GdInt aid) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return 0.0;
	}
	return audio->get_timer(aid);
}

void SpxAudioMgr::set_timer(GdInt aid, GdFloat time) {
	SpxAudio *audio = nullptr;
	{
		MutexLock aid_lock(aid_mutex);
		audio = _get_aid_audio(aid);
	}
	if (audio == nullptr) {
		return;
	}
	audio->set_timer(aid, time);
}
