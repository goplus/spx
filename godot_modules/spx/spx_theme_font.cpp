/**************************************************************************/
/*  spx_theme_font.cpp                                                    */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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

#include "spx_theme_font.h"

#include "scene/resources/theme.h"
#include "scene/theme/theme_db.h"

void spx_get_theme_fonts(Ref<Font> &r_default_font, Ref<Font> &r_fallback_font) {
	ThemeDB *theme_db = ThemeDB::get_singleton();
	if (theme_db == nullptr) {
		r_default_font.unref();
		r_fallback_font.unref();
		return;
	}

	const Ref<Theme> default_theme = theme_db->get_default_theme();
	r_default_font = default_theme.is_valid() ? default_theme->get_default_font() : Ref<Font>();
	r_fallback_font = theme_db->get_fallback_font();
}

void spx_set_theme_fonts(const Ref<Font> &p_default_font, const Ref<Font> &p_fallback_font) {
	ThemeDB *theme_db = ThemeDB::get_singleton();
	ERR_FAIL_NULL(theme_db);

	const Ref<Theme> default_theme = theme_db->get_default_theme();
	if (default_theme.is_valid()) {
		default_theme->set_default_font(p_default_font);
	}
	theme_db->set_fallback_font(p_fallback_font);
}

void spx_set_project_theme_font(const Ref<Font> &p_font) {
	spx_set_theme_fonts(p_font, p_font);
}
