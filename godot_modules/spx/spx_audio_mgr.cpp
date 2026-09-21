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

void SpxAudioMgr::on_awake() {
	SpxAudioBusPool::init();
	_create_root("audio_root");
	g_audio_id = 0;
}

void SpxAudioMgr::on_update(float delta) {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	_update_all(delta);

	Vector<GdInt> finished_aids;
	for (const KeyValue<GdInt, GdObj> &entry : aid_owners) {
		SpxAudio *audio = _find_object(entry.value);
		if (audio == nullptr || !audio->has_audio(entry.key)) {
			finished_aids.push_back(entry.key);
		}
	}
	for (GdInt aid : finished_aids) {
		aid_owners.erase(aid);
	}
}

void SpxAudioMgr::on_reset(int reset_code) {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	aid_owners.clear();
	_reset_objects(reset_code);
	SpxAudioBusPool::reset();
}

void SpxAudioMgr::on_destroy() {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	aid_owners.clear();
	_destroy_objects_and_root();
	SpxAudioBusPool::shutdown();
}

GdObj SpxAudioMgr::create_audio() {
	return _create_object();
}

void SpxAudioMgr::stop_all() {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}

	Vector<GdObj> audio_ids;
	for (const KeyValue<GdObj, SpxAudio *> &entry : id_objects) {
		audio_ids.push_back(entry.key);
	}
	for (GdObj id : audio_ids) {
		SpxAudio *audio = _find_object(id);
		if (audio != nullptr) {
			audio->stop_all();
		}
	}
	aid_owners.clear();
}

void SpxAudioMgr::destroy_audio(GdObj obj) {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	Vector<GdInt> aids;
	for (const KeyValue<GdInt, GdObj> &entry : aid_owners) {
		if (entry.value == obj) {
			aids.push_back(entry.key);
		}
	}
	for (GdInt aid : aids) {
		aid_owners.erase(aid);
	}
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
	if (unlikely(!_require_main_thread(__func__))) {
		return 0;
	}
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

	const GdInt aid = ++g_audio_id;
	if (!audio->play(aid, path, audio_owner, attenuation, max_distance)) {
		return 0;
	}
	aid_owners[aid] = obj;
	return aid;
}

GdBool SpxAudioMgr::is_playing(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	return audio != nullptr && audio->is_playing(aid);
}

void SpxAudioMgr::pause(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio != nullptr) {
		audio->pause(aid);
	}
}

void SpxAudioMgr::resume(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio != nullptr) {
		audio->resume(aid);
	}
}

void SpxAudioMgr::stop(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio != nullptr) {
		aid_owners.erase(aid);
		audio->stop(aid);
	}
}

GdBool SpxAudioMgr::restart(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio == nullptr) {
		return false;
	}
	if (!audio->restart(aid)) {
		audio->stop(aid);
		aid_owners.erase(aid);
		return false;
	}
	return true;
}

void SpxAudioMgr::set_loop(GdInt aid, GdBool loop) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio != nullptr) {
		audio->set_loop(aid, loop);
	}
}

GdBool SpxAudioMgr::get_loop(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	return audio != nullptr && audio->get_loop(aid);
}

GdFloat SpxAudioMgr::get_timer(GdInt aid) {
	SpxAudio *audio = _get_aid_audio(aid);
	return audio != nullptr ? audio->get_timer(aid) : 0.0;
}

void SpxAudioMgr::set_timer(GdInt aid, GdFloat time) {
	SpxAudio *audio = _get_aid_audio(aid);
	if (audio != nullptr) {
		audio->set_timer(aid, time);
	}
}

SpxAudio *SpxAudioMgr::_get_aid_audio(GdInt aid) {
	if (unlikely(!_require_main_thread(__func__))) {
		return nullptr;
	}
	const GdObj *owner = aid_owners.getptr(aid);
	if (owner == nullptr) {
		return nullptr;
	}
	SpxAudio *audio = _find_object(*owner);
	if (audio == nullptr || !audio->has_audio(aid)) {
		if (audio != nullptr) {
			audio->stop(aid);
		}
		aid_owners.erase(aid);
		return nullptr;
	}
	return audio;
}
