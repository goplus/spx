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

namespace {

constexpr int tiled_svg_image_threshold = 64;
constexpr double tiled_svg_overlap = 2.0;

static int count_svg_image_elements(const String &p_svg) {
	int count = 0;
	int cursor = 0;
	while ((cursor = p_svg.find("<image", cursor)) >= 0) {
		const int after = cursor + 6;
		if (after >= p_svg.length() || p_svg[after] == ' ' || p_svg[after] == '\t' || p_svg[after] == '\n' || p_svg[after] == '\r' || p_svg[after] == '/' || p_svg[after] == '>') {
			count++;
		}
		cursor = after;
	}
	return count;
}

static String adjust_svg_image_attribute(const String &p_tag, const char *p_name, double p_delta) {
	const String name = String(" ") + p_name + "=";
	const int attr = p_tag.find(name);
	if (attr < 0) {
		return p_tag;
	}
	const int value_start = attr + name.length();
	if (value_start >= p_tag.length() || (p_tag[value_start] != '\"' && p_tag[value_start] != '\'')) {
		return p_tag;
	}
	const char quote = p_tag[value_start];
	const int value_end = p_tag.find_char(quote, value_start + 1);
	if (value_end < 0) {
		return p_tag;
	}
	const String value = p_tag.substr(value_start + 1, value_end - value_start - 1);
	const double number = value.to_float();
	if (!Math::is_finite(number) || (p_delta > 0.0 && number <= 0.0)) {
		return p_tag;
	}
	const String adjusted = String::num_real(number + p_delta);
	return p_tag.substr(0, value_start + 1) + adjusted + p_tag.substr(value_end);
}

static String expand_tiled_svg_images(const String &p_svg) {
	if (count_svg_image_elements(p_svg) < tiled_svg_image_threshold) {
		return p_svg;
	}

	String result;
	int cursor = 0;
	while (true) {
		const int image_start = p_svg.find("<image", cursor);
		if (image_start < 0) {
			result += p_svg.substr(cursor);
			break;
		}
		const int after = image_start + 6;
		if (after >= p_svg.length() || !(p_svg[after] == ' ' || p_svg[after] == '\t' || p_svg[after] == '\n' || p_svg[after] == '\r' || p_svg[after] == '/' || p_svg[after] == '>')) {
			result += p_svg.substr(cursor, after - cursor);
			cursor = after;
			continue;
		}
		const int tag_end = p_svg.find_char('>', after);
		if (tag_end < 0) {
			result += p_svg.substr(cursor);
			break;
		}
		result += p_svg.substr(cursor, image_start - cursor);
		String tag = p_svg.substr(image_start, tag_end - image_start + 1);
		if (tag.find("transform=") >= 0 && tag.find("scale") >= 0) {
			const double half_overlap = tiled_svg_overlap * 0.5;
			tag = adjust_svg_image_attribute(tag, "x", -half_overlap);
			tag = adjust_svg_image_attribute(tag, "y", -half_overlap);
			tag = adjust_svg_image_attribute(tag, "width", tiled_svg_overlap);
			tag = adjust_svg_image_attribute(tag, "height", tiled_svg_overlap);
		}
		result += tag;
		cursor = tag_end + 1;
	}
	return result;
}

} // namespace

namespace TestSpxSvg {

String expand_tiled_svg_images_for_test(const String &p_svg) {
	return expand_tiled_svg_images(p_svg);
}

} // namespace TestSpxSvg

Error SpxImageLoaderSVG::rasterize(Ref<Image> p_image, const String &p_svg, float p_raster_scale, Vector2 *r_logical_size) {
	ERR_FAIL_COND_V_MSG(Math::is_zero_approx(p_raster_scale), ERR_INVALID_PARAMETER, "SpxImageLoaderSVG: Can't load SVG with a scale of 0.");
	ERR_FAIL_COND_V_MSG(!Math::is_finite(p_raster_scale) || p_raster_scale < 0.0f, ERR_INVALID_PARAMETER, "SpxImageLoaderSVG: Scale must be finite and positive.");

	ERR_FAIL_COND_V_MSG(!SpxSvgUtils::ensure_font_faces_registered(), ERR_CANT_CREATE,
			"SpxImageLoaderSVG: Failed to install the complete project font snapshot.");

	const String raster_svg = expand_tiled_svg_images(p_svg);
	const CharString bytes = raster_svg.utf8();
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

Error SpxImageLoaderSVG::load_image(const String &p_path, Ref<Image> p_image, float p_raster_scale, Vector2 *r_logical_size) {
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

	err = rasterize(p_image, svg, p_raster_scale, r_logical_size);
	if (err != OK) {
		return err;
	}
	if (p_image->is_empty()) {
		return ERR_INVALID_DATA;
	}

	return OK;
}
