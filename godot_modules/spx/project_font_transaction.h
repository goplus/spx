/**************************************************************************/
/*  project_font_transaction.h                                            */
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

#ifndef PROJECT_FONT_TRANSACTION_H
#define PROJECT_FONT_TRANSACTION_H

#include "gdextension_spx_ext.h"
#include "scene/resources/font.h"

class SpxResMgr;

namespace ProjectFonts {

struct FaceSpec {
	String path;
	String family;
	String family_key;
};

struct Request {
	String default_path;
	Vector<FaceSpec> faces;
	Vector<String> preferences;
};

struct PreparedFace {
	FaceSpec spec;
	Vector<uint8_t> data;
	Ref<FontFile> display_font;
};

struct Prepared {
	Vector<uint8_t> default_data;
	Ref<FontFile> default_font;
	Vector<PreparedFace> faces;
	HashMap<String, Ref<FontFile>> display_fonts;
	Ref<Font> theme_font;
	Vector<String> preferences;
};

String fold_family(const String &p_family);
bool strings_from_array(GdArray p_values, const String &p_name, Vector<String> &r_values, String &r_error);
Vector<String> preferences_from_array(GdArray p_preferences);
bool load_font_data(const String &p_path, const String &p_engine_path, Vector<uint8_t> &r_font_data, String *r_error = nullptr);
Ref<FontFile> create_display_font(const Vector<uint8_t> &p_font_data);
Ref<Font> build_display_font_chain(const HashMap<String, Ref<FontFile>> &p_fonts, const Vector<String> &p_preferences);
bool decode_request(GdString p_default_font_path, GdArray p_font_paths, GdArray p_font_families, GdArray p_preferences, Request &r_request, String &r_error);
bool validate_request(Request &r_request, String &r_error);
bool prepare(const Request &p_request, SpxResMgr &p_res_mgr, Prepared &r_prepared, String &r_error);

} // namespace ProjectFonts

#endif // PROJECT_FONT_TRANSACTION_H
