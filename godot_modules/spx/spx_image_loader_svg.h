/**************************************************************************/
/*  spx_image_loader_svg.h                                                */
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

#ifndef SPX_IMAGE_LOADER_SVG_H
#define SPX_IMAGE_LOADER_SVG_H

#include "core/io/image_loader.h"

class SpxImageLoaderSVG {
	static HashMap<Color, Color> forced_color_map;

	static void _replace_color_property(const HashMap<Color, Color> &p_color_map, const String &p_prefix, String &r_string);

public:
	static void set_forced_color_map(const HashMap<Color, Color> &p_color_map);

	// Optional output is the logical canvas size at p_raster_scale, before raster rounding.
	static Error rasterize(Ref<Image> p_image, String p_svg, float p_raster_scale, const HashMap<Color, Color> &p_color_map, Vector2 *r_logical_size = nullptr);

	static Error load_image(const String &p_path, Ref<Image> p_image, BitField<ImageFormatLoader::LoaderFlags> p_flags = ImageFormatLoader::FLAG_NONE, float p_raster_scale = 1.0f, Vector2 *r_logical_size = nullptr);
};

#endif // SPX_IMAGE_LOADER_SVG_H
