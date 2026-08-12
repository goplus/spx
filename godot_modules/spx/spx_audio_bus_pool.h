/**************************************************************************/
/*  spx_audio_bus_pool.h                                                       */
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

#ifndef SPX_AUDIO_BUS_POOL_H
#define SPX_AUDIO_BUS_POOL_H

#include "core/string/string_name.h"
#include "core/templates/hash_set.h"
#include "core/templates/vector.h"
#include "gdextension_spx_ext.h"

class SpxAudioBusPool {
	static inline SpxAudioBusPool *singleton = nullptr;

public:
	static const int BUS_MASTER = 0;
	static StringName STR_BUS_MASTER;
	static StringName STR_BUS_SFX;
	static StringName STR_BUS_MUSIC;

private:
	static const int DEFAULT_POOLED_BUS_COUNT = 1;
	static const int BUS_EXPANSION_SIZE = 4;

	Vector<StringName> free_buses;
	HashSet<StringName> active_buses;
	HashSet<StringName> pooled_buses;
	HashSet<StringName> created_buses;

private:
	int ensure_bus(const StringName &p_name);
	void expand_buses(int p_count = BUS_EXPANSION_SIZE);
	void reset_bus(int p_id);
	int get_valid_bus_id(const StringName &p_name) const;

public:
	static SpxAudioBusPool *get_singleton();
	static void init();
	static void reset();
	static void shutdown();
	StringName alloc();
	void free(const StringName &p_name);
	void set_volume(const StringName &p_name, GdFloat p_volume);
	GdFloat get_volume(const StringName &p_name);
	void set_pan(const StringName &p_name, GdFloat p_pan);
	GdFloat get_pan(const StringName &p_name);
};
#endif // SPX_AUDIO_BUS_POOL_H
