/**************************************************************************/
/*  spx_image_loader_svg.cpp                                              */
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

#include "spx_image_loader_svg.h"

#include "core/io/file_access.h"
#include "spx_svg_utils.h"

#include <lunasvg.h>

#include <cstring>

HashMap<Color, Color> SpxImageLoaderSVG::forced_color_map = HashMap<Color, Color>();

void SpxImageLoaderSVG::set_forced_color_map(const HashMap<Color, Color> &p_color_map) {
	forced_color_map = p_color_map;
}

void SpxImageLoaderSVG::_replace_color_property(const HashMap<Color, Color> &p_color_map, const String &p_prefix, String &r_string) {
	// Replace colors in the SVG based on what is passed in `p_color_map`.
	// Used to change the colors of editor icons based on the used theme.
	// The strings being replaced are typically of the form:
	//   fill="#5abbef"
	// But can also be 3-letter codes, include alpha, be "none" or a named color
	// string ("blue"). So we convert to Godot Color to compare with `p_color_map`.

	const int prefix_len = p_prefix.length();
	int pos = r_string.find(p_prefix);
	while (pos != -1) {
		pos += prefix_len; // Skip prefix.
		int end_pos = r_string.find_char('"', pos);
		ERR_FAIL_COND_MSG(end_pos == -1, vformat("Malformed SVG string after property \"%s\".", p_prefix));
		const String color_code = r_string.substr(pos, end_pos - pos);
		if (color_code != "none" && !color_code.begins_with("url(")) {
			const Color color = Color(color_code); // Handles both HTML codes and named colors.
			if (p_color_map.has(color)) {
				r_string = r_string.left(pos) + "#" + p_color_map[color].to_html(false) + r_string.substr(end_pos);
			}
		}
		// Search for other occurrences.
		pos = r_string.find(p_prefix, pos);
	}
}

Error SpxImageLoaderSVG::rasterize(Ref<Image> p_image, String p_svg, float p_raster_scale, const HashMap<Color, Color> &p_color_map, Vector2 *r_logical_size) {
	if (!p_color_map.is_empty()) {
		_replace_color_property(p_color_map, "stop-color=\"", p_svg);
		_replace_color_property(p_color_map, "fill=\"", p_svg);
		_replace_color_property(p_color_map, "stroke=\"", p_svg);
	}

	ERR_FAIL_COND_V_MSG(Math::is_zero_approx(p_raster_scale), ERR_INVALID_PARAMETER, "SpxImageLoaderSVG: Can't load SVG with a scale of 0.");
	ERR_FAIL_COND_V_MSG(!Math::is_finite(p_raster_scale) || p_raster_scale < 0.0f, ERR_INVALID_PARAMETER, "SpxImageLoaderSVG: Scale must be finite and positive.");

	ERR_FAIL_COND_V_MSG(!SpxSvgUtils::ensure_font_faces_registered(), ERR_CANT_CREATE,
			"SpxImageLoaderSVG: Failed to install the complete project font snapshot.");

	const CharString bytes = p_svg.utf8();
	auto document = lunasvg::Document::loadFromData(bytes.get_data(), bytes.length());
	if (document == nullptr) {
		return ERR_INVALID_DATA;
	}

	const double width = document->width();
	const double height = document->height();
	ERR_FAIL_COND_V_MSG(!Math::is_finite(width) || !Math::is_finite(height) || width < 0 || height < 0, ERR_INVALID_DATA, "SpxImageLoaderSVG: Invalid SVG dimensions.");
	// Treat Scratch's zero-sized costumes as transparent images.
	if (width == 0 || height == 0) {
		PackedByteArray transparent_pixel;
		transparent_pixel.resize(4);
		transparent_pixel.fill(0);
		p_image->set_data(1, 1, false, Image::FORMAT_RGBA8, transparent_pixel);
		if (r_logical_size != nullptr) {
			*r_logical_size = Vector2(1, 1);
		}
		return OK;
	}

	const double scaled_width = width * p_raster_scale;
	const double scaled_height = height * p_raster_scale;
	ERR_FAIL_COND_V_MSG(scaled_width > Image::MAX_WIDTH || scaled_height > Image::MAX_HEIGHT, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG dimensions are too large.");

	const uint32_t requested_width = Math::ceil(scaled_width);
	const uint32_t requested_height = Math::ceil(scaled_height);
	ERR_FAIL_COND_V_MSG(requested_width == 0 || requested_height == 0, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG dimensions became empty after scaling.");

	const uint64_t requested_pixel_count = (uint64_t)requested_width * requested_height;
	ERR_FAIL_COND_V_MSG(requested_pixel_count > Image::MAX_PIXELS, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG rasterized image is too large.");

	auto bitmap = document->renderToBitmap(requested_width, requested_height, 0x00000000);
	ERR_FAIL_COND_V_MSG(bitmap.isNull(), ERR_INVALID_DATA, "SpxImageLoaderSVG: Failed to rasterize SVG.");
	bitmap.convertToRGBA();

	const int bitmap_width = bitmap.width();
	const int bitmap_height = bitmap.height();
	ERR_FAIL_COND_V_MSG(bitmap_width <= 0 || bitmap_height <= 0, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG rasterization returned an empty bitmap.");

	const uint64_t bitmap_pixel_count = (uint64_t)bitmap_width * bitmap_height;
	ERR_FAIL_COND_V_MSG(bitmap_pixel_count > Image::MAX_PIXELS, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG rasterized image is too large.");

	Vector<uint8_t> result;
	result.resize((int64_t)bitmap_pixel_count * 4);

	const uint8_t *buffer = bitmap.data();
	ERR_FAIL_COND_V_MSG(buffer == nullptr, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG rasterization returned no pixel data.");

	const int stride = bitmap.stride();
	const uint64_t row_bytes = (uint64_t)bitmap_width * 4;
	ERR_FAIL_COND_V_MSG(stride < (int)row_bytes, ERR_INVALID_DATA, "SpxImageLoaderSVG: SVG rasterized bitmap stride is invalid.");

	uint8_t *dst = result.ptrw();
	for (int y = 0; y < bitmap_height; y++) {
		memcpy(dst + (row_bytes * y), buffer + ((uint64_t)stride * y), row_bytes);
	}

	p_image->set_data(bitmap_width, bitmap_height, false, Image::FORMAT_RGBA8, result);
	if (r_logical_size != nullptr) {
		*r_logical_size = Vector2(scaled_width, scaled_height);
	}

	return OK;
}

Error SpxImageLoaderSVG::load_image(const String &p_path, Ref<Image> p_image, BitField<ImageFormatLoader::LoaderFlags> p_flags, float p_raster_scale, Vector2 *r_logical_size) {
	Ref<FileAccess> file = FileAccess::open(p_path, FileAccess::READ);
	ERR_FAIL_COND_V_MSG(file.is_null(), ERR_CANT_OPEN, "SpxImageLoaderSVG: Cannot open SVG file: " + p_path);
	Vector<uint8_t> buffer;
	buffer.resize(file->get_length());
	file->get_buffer(buffer.ptrw(), buffer.size());

	String svg;
	Error err = svg.parse_utf8((const char *)buffer.ptr(), buffer.size());
	if (err != OK) {
		return err;
	}

	const HashMap<Color, Color> empty_color_map;
	const HashMap<Color, Color> &color_map = p_flags & ImageFormatLoader::FLAG_CONVERT_COLORS ? forced_color_map : empty_color_map;
	err = rasterize(p_image, svg, p_raster_scale, color_map, r_logical_size);
	if (err != OK) {
		return err;
	}
	if (p_image->is_empty()) {
		return ERR_INVALID_DATA;
	}

	if (p_flags & ImageFormatLoader::FLAG_FORCE_LINEAR) {
		p_image->srgb_to_linear();
	}
	return OK;
}
