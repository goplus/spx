/**************************************************************************/
/*  project_font_transaction.cpp                                          */
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

#include "project_font_transaction.h"

#include "core/io/file_access.h"
#include "core/io/resource_loader.h"
#include "spx_res_mgr.h"
#include "spx_svg_utils.h"

#include <limits>

namespace ProjectFonts {

static bool _fail(const String &p_message, String *r_error) {
	if (r_error != nullptr) {
		*r_error = p_message;
	} else {
		ERR_PRINT(p_message);
	}
	return false;
}

String fold_family(const String &p_family) {
	String folded = p_family;
	for (int i = 0; i < folded.length(); i++) {
		char32_t character = folded[i];
		if (character >= U'A' && character <= U'Z') {
			folded[i] = character + (U'a' - U'A');
		}
	}
	return folded;
}

bool strings_from_array(GdArray p_values, const String &p_name, Vector<String> &r_values, String &r_error) {
	r_values.clear();
	if (p_values == nullptr) {
		r_error = p_name + " must be a GdArray of strings.";
		return false;
	}
	if (p_values->type != GD_ARRAY_TYPE_STRING) {
		r_error = p_name + " must contain strings.";
		return false;
	}
	if (p_values->size < 0 || (p_values->size > 0 && p_values->data == nullptr)) {
		r_error = p_name + " has an invalid array payload.";
		return false;
	}
	r_values.resize(p_values->size);
	for (int i = 0; i < p_values->size; i++) {
		auto value = SpxBaseMgr::get_array<GdString>(p_values, i);
		if (value == nullptr || *value == nullptr) {
			r_values.clear();
			r_error = p_name + " contains an invalid string at index " + itos(i) + ".";
			return false;
		}
		r_values.write[i] = SpxStr(*value);
	}
	return true;
}

Vector<String> preferences_from_array(GdArray p_preferences) {
	// The legacy native bridge encodes an empty []string as nullptr.
	if (p_preferences == nullptr) {
		return Vector<String>();
	}
	Vector<String> families;
	String error;
	if (!strings_from_array(p_preferences, "Font preferences", families, error)) {
		ERR_PRINT(error);
		families.clear();
	}
	return families;
}

bool load_font_data(const String &p_path, const String &p_engine_path, Vector<uint8_t> &r_font_data, String *r_error) {
	if (ResourceLoader::exists(p_path, "FontFile")) {
		Ref<FontFile> imported_font = ResourceLoader::load(p_path, "FontFile");
		if (!imported_font.is_null()) {
			r_font_data = imported_font->get_data();
			if (r_font_data.is_empty()) {
				return _fail("Loaded font resource has no data: " + p_path, r_error);
			}
			return true;
		}
	}

	Ref<FileAccess> file = FileAccess::open(p_engine_path, FileAccess::READ);
	if (file.is_null()) {
		return _fail("Can not open font file: " + p_path + " engine_path= " + p_engine_path, r_error);
	}

	uint64_t font_size = file->get_length();
	if (font_size == 0) {
		return _fail("Font file is empty: " + p_path + " engine_path= " + p_engine_path, r_error);
	}
	if (font_size > uint64_t(std::numeric_limits<int>::max())) {
		return _fail("Font file is too large: " + p_path + " engine_path= " + p_engine_path, r_error);
	}

	r_font_data.resize((int)font_size);
	uint64_t read_bytes = file->get_buffer(r_font_data.ptrw(), font_size);
	if (read_bytes != font_size) {
		r_font_data.resize(0);
		return _fail("Can not read full font file: " + p_path + " engine_path= " + p_engine_path, r_error);
	}
	return true;
}

Ref<FontFile> create_display_font(const Vector<uint8_t> &p_font_data) {
	Ref<FontFile> font;
	font.instantiate();
	font->set_data(p_font_data);
	font->set_font_style(0);
	font->set_antialiasing(TextServer::FONT_ANTIALIASING_GRAY);
	font->set_force_autohinter(false);
	font->set_hinting(TextServer::HINTING_LIGHT);
	font->set_subpixel_positioning(TextServer::SUBPIXEL_POSITIONING_AUTO);
	font->set_multichannel_signed_distance_field(false);
	font->set_generate_mipmaps(false);
	font->set_fixed_size(0);
	font->set_allow_system_fallback(false);
	return font;
}

Ref<Font> build_display_font_chain(const HashMap<String, Ref<FontFile>> &p_fonts, const Vector<String> &p_preferences) {
	Ref<Font> primary;
	TypedArray<Font> fallbacks;
	for (const String &family : p_preferences) {
		const Ref<FontFile> *font = p_fonts.getptr(fold_family(family));
		if (font == nullptr || font->is_null()) {
			continue;
		}
		if (primary.is_null()) {
			primary = *font;
		} else {
			fallbacks.push_back(*font);
		}
	}

	if (primary.is_valid()) {
		Ref<FontVariation> composite;
		composite.instantiate();
		composite->set_base_font(primary);
		composite->set_fallbacks(fallbacks);
		return composite;
	}

	Ref<FontFile> empty_font;
	empty_font.instantiate();
	empty_font->set_allow_system_fallback(false);
	return empty_font;
}

bool decode_request(GdString p_default_font_path, GdArray p_font_paths, GdArray p_font_families, GdArray p_preferences, Request &r_request, String &r_error) {
	if (p_default_font_path == nullptr) {
		r_error = "Default font path must be a string.";
		return false;
	}
	r_request = Request();
	r_request.default_path = SpxStr(p_default_font_path);
	if (r_request.default_path.is_empty()) {
		r_error = "Default font path must not be empty.";
		return false;
	}

	Vector<String> paths;
	Vector<String> families;
	if (!strings_from_array(p_font_paths, "Font paths", paths, r_error) ||
			!strings_from_array(p_font_families, "Font families", families, r_error) ||
			!strings_from_array(p_preferences, "Font preferences", r_request.preferences, r_error)) {
		return false;
	}
	if (paths.size() != families.size()) {
		r_error = "Font paths and font families must have the same length.";
		return false;
	}
	r_request.faces.resize(paths.size());
	for (int i = 0; i < paths.size(); i++) {
		r_request.faces.write[i] = { paths[i], families[i], String() };
	}
	return true;
}

bool validate_request(Request &r_request, String &r_error) {
	HashMap<String, String> available_families;
	available_families.insert("default", "default");
	for (int i = 0; i < r_request.faces.size(); i++) {
		FaceSpec &face = r_request.faces.write[i];
		if (face.path.is_empty()) {
			r_error = "Font path at index " + itos(i) + " must not be empty.";
			return false;
		}
		if (face.family.is_empty()) {
			r_error = "Font family at index " + itos(i) + " must not be empty.";
			return false;
		}
		face.family_key = fold_family(face.family);
		if (face.family_key == "default") {
			r_error = "Font family at index " + itos(i) + " uses the reserved name default.";
			return false;
		}
		if (available_families.has(face.family_key)) {
			r_error = "Font family " + face.family + " is duplicated after ASCII case folding.";
			return false;
		}
		available_families.insert(face.family_key, face.family);
	}

	HashMap<String, bool> seen_preferences;
	for (int i = 0; i < r_request.preferences.size(); i++) {
		const String &preference = r_request.preferences[i];
		if (preference.is_empty()) {
			r_error = "Font preference at index " + itos(i) + " must not be empty.";
			return false;
		}
		const String folded_preference = fold_family(preference);
		if (!available_families.has(folded_preference)) {
			r_error = "Font preference " + preference + " is not an available font family.";
			return false;
		}
		if (seen_preferences.has(folded_preference)) {
			r_error = "Font preference " + preference + " is duplicated after ASCII case folding.";
			return false;
		}
		seen_preferences.insert(folded_preference, true);
	}
	return true;
}

bool prepare(const Request &p_request, SpxResMgr &p_res_mgr, Prepared &r_prepared, String &r_error) {
	r_prepared = Prepared();
	r_prepared.preferences = p_request.preferences;
	if (!load_font_data(p_request.default_path, p_res_mgr._to_engine_path(p_request.default_path), r_prepared.default_data, &r_error)) {
		return false;
	}
	r_prepared.default_font = create_display_font(r_prepared.default_data);
	if (r_prepared.default_font.is_null() || r_prepared.default_font->get_face_count() <= 0) {
		r_error = "Default font file is not a supported font: " + p_request.default_path;
		return false;
	}
	if (!SpxSvgUtils::is_font_data_valid(r_prepared.default_data)) {
		r_error = "Default font file is not supported by LunaSVG: " + p_request.default_path;
		return false;
	}

	r_prepared.display_fonts.insert("default", r_prepared.default_font);
	r_prepared.faces.resize(p_request.faces.size());
	for (int i = 0; i < p_request.faces.size(); i++) {
		PreparedFace &prepared_face = r_prepared.faces.write[i];
		prepared_face.spec = p_request.faces[i];
		if (!load_font_data(prepared_face.spec.path, p_res_mgr._to_engine_path(prepared_face.spec.path), prepared_face.data, &r_error)) {
			return false;
		}
		prepared_face.display_font = create_display_font(prepared_face.data);
		if (prepared_face.display_font.is_null() || prepared_face.display_font->get_face_count() <= 0) {
			r_error = "Project font file is not a supported font: " + prepared_face.spec.path;
			return false;
		}
		if (!SpxSvgUtils::is_font_data_valid(prepared_face.data)) {
			r_error = "Project font file is not supported by LunaSVG: " + prepared_face.spec.path;
			return false;
		}
		r_prepared.display_fonts.insert(prepared_face.spec.family_key, prepared_face.display_font);
	}
	r_prepared.theme_font = build_display_font_chain(r_prepared.display_fonts, p_request.preferences);
	return true;
}

} // namespace ProjectFonts
