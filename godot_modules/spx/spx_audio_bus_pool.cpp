/**************************************************************************/
/*  spx_audio_bus_pool.cpp                                                     */
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

#include "spx_audio_bus_pool.h"

#include "servers/audio/effects/audio_effect_panner.h"

#include "spx_audio_mgr.h"
#include "spx_engine.h"
#include "spx_res_mgr.h"

StringName SpxAudioBusPool::STR_BUS_MASTER;
StringName SpxAudioBusPool::STR_BUS_SFX;
StringName SpxAudioBusPool::STR_BUS_MUSIC;

namespace {
constexpr char SPX_BUS_PREFIX[] = "_spx_audio_bus_";
}

SpxAudioBusPool *SpxAudioBusPool::get_singleton() {
	return singleton;
}

void SpxAudioBusPool::init() {
	STR_BUS_MASTER = "Master";
	STR_BUS_SFX = "Sfx";
	STR_BUS_MUSIC = "Music";

	if (singleton == nullptr) {
		singleton = memnew(SpxAudioBusPool);
	}
	reset();
}

void SpxAudioBusPool::reset() {
	if (singleton == nullptr) {
		return;
	}

	AudioServer *audio_server = AudioServer::get_singleton();
	ERR_FAIL_NULL(audio_server);

	singleton->free_buses.clear();
	singleton->active_buses.clear();
	singleton->ensure_bus(STR_BUS_SFX);
	singleton->ensure_bus(STR_BUS_MUSIC);

	// Only reset buses created by this pool. A project bus may intentionally
	// use the same prefix, so inferring ownership from its name is unsafe.
	for (const StringName &bus_name : singleton->created_buses) {
		const int bus_id = audio_server->get_bus_index(bus_name);
		if (bus_id >= 0) {
			singleton->reset_bus(bus_id);
		}
	}
	for (const StringName &bus_name : singleton->pooled_buses) {
		if (audio_server->get_bus_index(bus_name) >= 0) {
			singleton->free_buses.push_back(bus_name);
		}
	}

	if (singleton->free_buses.is_empty()) {
		singleton->expand_buses(DEFAULT_POOLED_BUS_COUNT);
	}
}

void SpxAudioBusPool::shutdown() {
	if (singleton == nullptr) {
		return;
	}

	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server != nullptr) {
		Vector<int> created_bus_indices;
		for (const StringName &bus_name : singleton->created_buses) {
			const int bus_id = audio_server->get_bus_index(bus_name);
			if (bus_id > BUS_MASTER) {
				created_bus_indices.push_back(bus_id);
			}
		}
		created_bus_indices.sort();
		for (int i = created_bus_indices.size() - 1; i >= 0; i--) {
			audio_server->remove_bus(created_bus_indices[i]);
		}
	}

	memdelete(singleton);
	singleton = nullptr;
}

StringName SpxAudioBusPool::alloc() {
	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server == nullptr) {
		return StringName();
	}

	while (true) {
		if (free_buses.is_empty()) {
			expand_buses();
		}
		if (free_buses.is_empty()) {
			return StringName();
		}

		StringName bus_name = free_buses[free_buses.size() - 1];
		free_buses.remove_at(free_buses.size() - 1);
		if (audio_server->get_bus_index(bus_name) < 0) {
			continue;
		}

		active_buses.insert(bus_name);
		return bus_name;
	}
}

void SpxAudioBusPool::free(const StringName &p_name) {
	if (!active_buses.has(p_name)) {
		print_error("Trying to free inactive SPX audio bus: " + String(p_name));
		return;
	}

	active_buses.erase(p_name);
	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server == nullptr) {
		return;
	}
	const int bus_id = audio_server->get_bus_index(p_name);
	if (bus_id < 0) {
		return;
	}

	// Return to the pool
	free_buses.push_back(p_name);
	// reset the bus
	reset_bus(bus_id);
}

void SpxAudioBusPool::set_volume(const StringName &p_name, GdFloat p_volume) {
	const int bus_id = get_valid_bus_id(p_name);
	if (bus_id < 0) {
		return;
	}

	// Convert to decibels (Godot uses decibel scale for volume)
	auto db = Math::linear_to_db(p_volume);
	AudioServer::get_singleton()->set_bus_volume_db(bus_id, db);
}

GdFloat SpxAudioBusPool::get_volume(const StringName &p_name) {
	const int bus_id = get_valid_bus_id(p_name);
	if (bus_id < 0) {
		return 0.0f;
	}

	// Get volume in decibels
	float db = AudioServer::get_singleton()->get_bus_volume_db(bus_id);
	return Math::db_to_linear(db);
}

void SpxAudioBusPool::set_pan(const StringName &p_name, GdFloat p_pan) {
	const int bus_id = get_valid_bus_id(p_name);
	if (bus_id < 0) {
		return;
	}

	// Clamp pan between -1 and 1
	p_pan = CLAMP(p_pan, -1.0f, 1.0f);

	// Check if there's already a panner effect
	int effect_count = AudioServer::get_singleton()->get_bus_effect_count(bus_id);
	int panner_idx = -1;

	for (int i = 0; i < effect_count; i++) {
		Ref<AudioEffect> effect = AudioServer::get_singleton()->get_bus_effect(bus_id, i);
		if (effect.is_valid() && effect->is_class("AudioEffectPanner")) {
			panner_idx = i;
			break;
		}
	}

	// Create a panner effect if not exists
	Ref<AudioEffectPanner> panner;
	if (panner_idx == -1) {
		panner.instantiate();
		AudioServer::get_singleton()->add_bus_effect(bus_id, panner);
	} else {
		panner = AudioServer::get_singleton()->get_bus_effect(bus_id, panner_idx);
	}

	// Set the pan value
	panner->set_pan(p_pan);
}

GdFloat SpxAudioBusPool::get_pan(const StringName &p_name) {
	const int bus_id = get_valid_bus_id(p_name);
	if (bus_id < 0) {
		return 0.0f;
	}

	// Find panner effect
	int effect_count = AudioServer::get_singleton()->get_bus_effect_count(bus_id);

	for (int i = 0; i < effect_count; i++) {
		Ref<AudioEffect> effect = AudioServer::get_singleton()->get_bus_effect(bus_id, i);
		if (effect.is_valid() && effect->is_class("AudioEffectPanner")) {
			Ref<AudioEffectPanner> panner = effect;
			return panner->get_pan();
		}
	}

	return 0.0f; // Default pan value if no panner found
}

int SpxAudioBusPool::ensure_bus(const StringName &p_name) {
	AudioServer *audio_server = AudioServer::get_singleton();
	ERR_FAIL_NULL_V(audio_server, -1);

	int bus_id = audio_server->get_bus_index(p_name);
	if (bus_id >= 0) {
		return bus_id;
	}

	audio_server->add_bus();
	bus_id = audio_server->get_bus_count() - 1;
	audio_server->set_bus_name(bus_id, p_name);
	audio_server->set_bus_send(bus_id, STR_BUS_MASTER);
	created_buses.insert(p_name);
	return bus_id;
}

void SpxAudioBusPool::expand_buses(int p_count) {
	AudioServer *audio_server = AudioServer::get_singleton();
	ERR_FAIL_NULL(audio_server);

	int name_index = 0;
	for (int i = 0; i < p_count; i++) {
		String bus_name;
		do {
			bus_name = String(SPX_BUS_PREFIX) + itos(name_index++);
		} while (audio_server->get_bus_index(bus_name) >= 0);

		audio_server->add_bus();
		const int bus_id = audio_server->get_bus_count() - 1;
		audio_server->set_bus_name(bus_id, bus_name);
		audio_server->set_bus_send(bus_id, STR_BUS_MASTER);
		created_buses.insert(bus_name);
		pooled_buses.insert(bus_name);
		free_buses.push_back(bus_name);
	}
}

void SpxAudioBusPool::reset_bus(int p_id) {
	AudioServer *audio_server = AudioServer::get_singleton();
	ERR_FAIL_NULL(audio_server);
	ERR_FAIL_INDEX(p_id, audio_server->get_bus_count());

	audio_server->set_bus_volume_db(p_id, 0.0);
	const int effect_count = audio_server->get_bus_effect_count(p_id);
	for (int i = 0; i < effect_count; i++) {
		Ref<AudioEffect> effect = audio_server->get_bus_effect(p_id, i);
		if (effect.is_valid() && effect->is_class("AudioEffectPanner")) {
			Ref<AudioEffectPanner> panner = effect;
			panner->set_pan(0.0);
			break;
		}
	}
}

int SpxAudioBusPool::get_valid_bus_id(const StringName &p_name) const {
	AudioServer *audio_server = AudioServer::get_singleton();
	if (audio_server == nullptr) {
		return -1;
	}

	const int bus_id = audio_server->get_bus_index(p_name);
	if (bus_id < 0) {
		print_error("Unknown SPX audio bus: " + String(p_name));
		return -1;
	}

	if (p_name != STR_BUS_SFX && p_name != STR_BUS_MUSIC && !active_buses.has(p_name)) {
		print_error("SPX audio bus is not active: " + String(p_name));
		return -1;
	}

	return bus_id;
}
