/**************************************************************************/
/*  spx_svg_utils.h                                                       */
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

#ifndef SPX_SVG_UTILS_H
#define SPX_SVG_UTILS_H

#include "core/string/ustring.h"
#include "core/templates/vector.h"

struct SpxSvgProjectFontFace {
	String family;
	Vector<uint8_t> data;
};

class SpxSvgUtils {
public:
	// Validates the same font bytes LunaSVG will consume without changing the
	// current thread's face cache or the process-wide project font registry.
	static bool is_font_data_valid(const Vector<uint8_t> &font_data);
	// Atomically replaces the complete project font snapshot. Rendering threads
	// see either the previous generation or this complete generation; they never
	// observe individual faces being registered.
	static void apply_font_registry(const Vector<uint8_t> &default_font_data, const Vector<SpxSvgProjectFontFace> &named_font_faces, const Vector<String> &preferences);
	// Exposes the currently published project-font generation for diagnostics
	// and transaction tests.
	static uint64_t get_font_registry_generation();
	static void set_default_font(const void *font_data, int length);
	static void add_font_face(const String &family, const void *font_data, int length);
	static void set_font_preferences(const Vector<String> &preferences);
	static void reset_font_registry();
	// Returns false when the complete thread-local snapshot could not be
	// installed. Callers must not render with a partial font generation.
	static bool ensure_font_faces_registered();
};

#endif // SPX_SVG_UTILS_H
