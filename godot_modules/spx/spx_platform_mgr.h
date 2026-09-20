/**************************************************************************/
/*  spx_platform_mgr.h                                                       */
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

#ifndef SPX_PLATFORM_MGR_H
#define SPX_PLATFORM_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_manager.h"

class SpxPlatformMgr : public SpxManager {
	String persistent_data_dir = "res://";
	bool window_size_uses_content_scale = false;

public:

	void on_awake() override;
	void on_reset(int reset_code) override;
	void _set_persistent_data_dir(String path);
	String _get_persistent_data_dir();

public:
	//Expose as few interfaces as possible to prevent misuse.
	SPX_BIND void set_stretch_mode(GdBool enable);
	SPX_BIND void set_stretch_aspect(GdBool is_keep);
	SPX_BIND void set_stretch_content_scale(GdInt width, GdInt height);

	SPX_BIND void set_window_position(GdVec2 pos);
	SPX_BIND GdVec2 get_window_position();
	SPX_BIND void set_window_size(GdInt width, GdInt height, GdBool with_content_scale);
	SPX_BIND GdVec2 get_window_size();
	SPX_BIND void set_window_title(GdString title);
	SPX_BIND GdString get_window_title();
	SPX_BIND void set_window_fullscreen(GdBool enable);
	SPX_BIND GdBool is_window_fullscreen();
	SPX_BIND void set_debug_mode(GdBool enable);
	SPX_BIND GdBool is_debug_mode();
	SPX_BIND GdBool is_main_thread();

	SPX_BIND GdFloat get_time_scale();
	SPX_BIND void set_time_scale(GdFloat time_scale);

	SPX_BIND GdInt get_max_fps();
	SPX_BIND void set_max_fps(GdInt fps);

	SPX_BIND GdString get_persistent_data_dir();
	SPX_BIND void set_persistent_data_dir(GdString path);
	SPX_BIND GdBool is_in_persistent_data_dir(GdString path);
};

#endif // SPX_PLATFORM_MGR_H
