/**************************************************************************/
/*  spx_svg_utils.cpp                                                     */
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

#include "spx_svg_utils.h"

#include "core/os/memory.h"
#include "core/os/mutex.h"
#include "core/templates/hash_map.h"
#include "core/templates/vector.h"
#include "servers/text_server.h"
#include "thirdparty/lunasvg/include/lunasvg.h"
#include "thirdparty/lunasvg/include/plutovg.h"

#include <lunasvg.h>
#ifdef LUNASVG_ENABLE_HARFBUZZ
#include <hb.h>
#endif
#include <algorithm>
#include <cstring>

namespace {

struct StoredSVGFontFace {
	String family;
	Vector<uint8_t> data;
};

static String _ascii_fold_font_family(const String &family) {
	String folded = family;
	for (int i = 0; i < folded.length(); i++) {
		char32_t character = folded[i];
		if (character >= U'A' && character <= U'Z') {
			folded[i] = character + (U'a' - U'A');
		}
	}
	return folded;
}

struct FontRegistrySnapshot {
	Vector<uint8_t> default_font_data;
	Vector<StoredSVGFontFace> named_font_faces;
	Vector<String> font_preferences;
	bool font_preferences_configured = false;
	uint64_t serial = 0;
	uint64_t generation = 0;
};

class FontRegistry {
public:
	void replace_all(const Vector<uint8_t> &p_default_font_data, const Vector<SpxSvgProjectFontFace> &p_named_font_faces, const Vector<String> &p_preferences) {
		HashMap<String, Vector<uint8_t>> named_font_faces;
		for (const SpxSvgProjectFontFace &face : p_named_font_faces) {
			named_font_faces.insert(_ascii_fold_font_family(face.family), face.data);
		}

		MutexLock lock(m_mutex);
		m_default_font_data = p_default_font_data;
		m_named_font_faces = named_font_faces;
		m_font_preferences = p_preferences;
		m_font_preferences_configured = true;
		m_generation++;
		m_serial++;
	}

	void replace_default_font(const Vector<uint8_t> &p_font_data) {
		MutexLock lock(m_mutex);
		m_default_font_data = p_font_data;
		m_serial++;
	}

	void replace_font_face(const String &p_family, const Vector<uint8_t> &p_font_data) {
		MutexLock lock(m_mutex);
		m_named_font_faces.insert(_ascii_fold_font_family(p_family), p_font_data);
		m_serial++;
	}

	void set_font_preferences(const Vector<String> &p_preferences) {
		MutexLock lock(m_mutex);
		m_font_preferences = p_preferences;
		m_font_preferences_configured = true;
		m_serial++;
	}

	void reset() {
		MutexLock lock(m_mutex);
		m_default_font_data.clear();
		m_named_font_faces.clear();
		m_font_preferences.clear();
		m_font_preferences_configured = false;
		m_generation++;
		m_serial++;
	}

	bool snapshot_if_changed(uint64_t p_applied_serial, FontRegistrySnapshot &r_snapshot) {
		MutexLock lock(m_mutex);
		if (m_serial == 0 || m_serial == p_applied_serial) {
			return false;
		}

		r_snapshot.default_font_data = m_default_font_data;
		r_snapshot.font_preferences = m_font_preferences;
		r_snapshot.font_preferences_configured = m_font_preferences_configured;
		r_snapshot.serial = m_serial;
		r_snapshot.generation = m_generation;
		r_snapshot.named_font_faces.clear();
		for (const KeyValue<String, Vector<uint8_t>> &E : m_named_font_faces) {
			StoredSVGFontFace stored_font;
			stored_font.family = E.key;
			stored_font.data = E.value;
			r_snapshot.named_font_faces.push_back(stored_font);
		}
		return true;
	}

	uint64_t get_generation() const {
		MutexLock lock(m_mutex);
		return m_generation;
	}

private:
	Mutex m_mutex;
	Vector<uint8_t> m_default_font_data;
	HashMap<String, Vector<uint8_t>> m_named_font_faces;
	Vector<String> m_font_preferences;
	bool m_font_preferences_configured = false;
	uint64_t m_serial = 0;
	uint64_t m_generation = 0;
};

static FontRegistry &get_font_registry() {
	static FontRegistry registry;
	return registry;
}

static size_t _get_grapheme_breaks(const uint32_t *text, size_t length, size_t *breaks, size_t capacity, void *) {
	if (text == nullptr || length == 0 || breaks == nullptr || capacity == 0) {
		return 0;
	}
	TextServerManager *manager = TextServerManager::get_singleton();
	if (manager == nullptr) {
		return 0;
	}
	Ref<TextServer> text_server = manager->get_primary_interface();
	if (text_server.is_null() || !text_server->has_feature(TextServer::FEATURE_BREAK_ITERATORS)) {
		return 0;
	}

	String value(reinterpret_cast<const char32_t *>(text), static_cast<int>(length));
	PackedInt32Array character_breaks = text_server->string_get_character_breaks(value);
	size_t count = std::min(capacity, static_cast<size_t>(character_breaks.size()));
	for (size_t i = 0; i < count; i++) {
		breaks[i] = static_cast<size_t>(character_breaks[static_cast<int>(i)]);
	}
	return count;
}

static bool _copy_font_bytes(const void *font_data, int length, Vector<uint8_t> &r_bytes) {
	if (font_data == nullptr || length <= 0) {
		r_bytes.clear();
		return false;
	}

	r_bytes.resize(length);
	::memcpy(r_bytes.ptrw(), font_data, length);
	return true;
}

static void _destroy_shared_font_bytes(void *p_data) {
	memdelete(static_cast<Vector<uint8_t> *>(p_data));
}

static bool _register_font_bytes_for_current_thread(const String &family, const Vector<uint8_t> &font_data) {
	if (font_data.is_empty()) {
		return false;
	}

	// Godot Vector is copy-on-write. Retaining a Vector here gives the
	// thread-local LunaSVG face immutable ownership without copying the complete
	// font once per rendering thread.
	Vector<uint8_t> *shared_font_data = memnew(Vector<uint8_t>);
	*shared_font_data = font_data;

	CharString utf8_family = family.utf8();
	const char *family_name = family.is_empty() ? "" : utf8_family.get_data();
	return lunasvg_add_font_face_from_data(family_name, false, false, shared_font_data->ptr(), shared_font_data->size(),
			_destroy_shared_font_bytes, shared_font_data);
}

static void _install_grapheme_break_callback_for_current_thread() {
	lunasvg_set_grapheme_break_func(_get_grapheme_breaks, nullptr);
}

static void _apply_font_preferences_to_current_thread(const FontRegistrySnapshot &snapshot) {
	Vector<CharString> utf8_preferences;
	utf8_preferences.resize(snapshot.font_preferences.size());
	for (int i = 0; i < utf8_preferences.size(); i++) {
		utf8_preferences.set(i, snapshot.font_preferences[i].utf8());
	}

	Vector<const char *> preference_names;
	preference_names.resize(utf8_preferences.size());
	for (int i = 0; i < preference_names.size(); i++) {
		preference_names.set(i, utf8_preferences[i].get_data());
	}
	const char *const empty_preference = nullptr;
	const char *const *preference_data = nullptr;
	if (snapshot.font_preferences_configured) {
		// LunaSVG uses nullptr to mean that no project preference was supplied.
		// Keep an explicit empty preference list distinct by passing a non-null
		// sentinel with a zero count.
		preference_data = preference_names.is_empty() ? &empty_preference : preference_names.ptr();
	}
	lunasvg_set_font_preferences(
			preference_data,
			snapshot.font_preferences_configured ? preference_names.size() : 0);
}

static bool _apply_font_registry_snapshot_to_current_thread(const FontRegistrySnapshot &snapshot, uint64_t &r_applied_generation) {
	if (r_applied_generation != snapshot.generation) {
		// A new generation starts at reset. Every rendering thread clears its
		// local cache before applying the first snapshot for the new project.
		lunasvg_clear_font_faces();
		r_applied_generation = snapshot.generation;
	}
	if (!snapshot.default_font_data.is_empty() &&
			!_register_font_bytes_for_current_thread("", snapshot.default_font_data)) {
		lunasvg_clear_font_faces();
		return false;
	}
	for (int i = 0; i < snapshot.named_font_faces.size(); i++) {
		if (!_register_font_bytes_for_current_thread(snapshot.named_font_faces[i].family, snapshot.named_font_faces[i].data)) {
			// Do not render or mark the serial as applied with a partial font set.
			// The next load retries the complete immutable snapshot.
			lunasvg_clear_font_faces();
			return false;
		}
	}
	_apply_font_preferences_to_current_thread(snapshot);
	return true;
}

} // namespace

bool SpxSvgUtils::is_font_data_valid(const Vector<uint8_t> &font_data) {
	if (font_data.is_empty()) {
		return false;
	}

	plutovg_font_face_t *face = plutovg_font_face_load_from_data(font_data.ptr(), font_data.size(), 0, nullptr, nullptr);
	if (face == nullptr) {
		return false;
	}
	plutovg_font_face_destroy(face);

#ifdef LUNASVG_ENABLE_HARFBUZZ
	hb_blob_t *blob = hb_blob_create(reinterpret_cast<const char *>(font_data.ptr()), font_data.size(), HB_MEMORY_MODE_READONLY, nullptr, nullptr);
	hb_face_t *hb_face = hb_face_create(blob, 0);
	const bool harfbuzz_valid = hb_face_get_upem(hb_face) > 0 && hb_face_get_glyph_count(hb_face) > 0;
	hb_face_destroy(hb_face);
	hb_blob_destroy(blob);
	if (!harfbuzz_valid) {
		return false;
	}
#endif
	return true;
}

void SpxSvgUtils::apply_font_registry(const Vector<uint8_t> &default_font_data, const Vector<SpxSvgProjectFontFace> &named_font_faces, const Vector<String> &preferences) {
	get_font_registry().replace_all(default_font_data, named_font_faces, preferences);
}

uint64_t SpxSvgUtils::get_font_registry_generation() {
	return get_font_registry().get_generation();
}

void SpxSvgUtils::set_default_font(const void *font_data, int length) {
	Vector<uint8_t> font_bytes;
	if (!_copy_font_bytes(font_data, length, font_bytes)) {
		return;
	}

	get_font_registry().replace_default_font(font_bytes);
}

void SpxSvgUtils::add_font_face(const String &family, const void *font_data, int length) {
	if (family.is_empty()) {
		return;
	}

	Vector<uint8_t> font_bytes;
	if (!_copy_font_bytes(font_data, length, font_bytes)) {
		return;
	}

	get_font_registry().replace_font_face(family, font_bytes);
}

void SpxSvgUtils::set_font_preferences(const Vector<String> &preferences) {
	get_font_registry().set_font_preferences(preferences);
}

void SpxSvgUtils::reset_font_registry() {
	get_font_registry().reset();
}

bool SpxSvgUtils::ensure_font_faces_registered() {
	thread_local uint64_t applied_serial = 0;
	thread_local uint64_t applied_generation = 0;
	_install_grapheme_break_callback_for_current_thread();

	FontRegistrySnapshot snapshot;
	if (!get_font_registry().snapshot_if_changed(applied_serial, snapshot)) {
		return true;
	}
	if (_apply_font_registry_snapshot_to_current_thread(snapshot, applied_generation)) {
		applied_serial = snapshot.serial;
		return true;
	}
	return false;
}
