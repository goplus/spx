/**************************************************************************/
/*  test_spx_svg.h                                                        */
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

#ifndef TEST_SPX_SVG_H
#define TEST_SPX_SVG_H

#include "core/io/file_access.h"
#include "core/io/image.h"
#include "../spx_image_loader_svg.h"
#include "../spx_svg_utils.h"
#include "../thirdparty/lunasvg/include/lunasvg.h"
#include "../thirdparty/lunasvg/source/embedded_cnfont.h"
#include "../thirdparty/lunasvg/source/graphics.h"
#include "../thirdparty/lunasvg/source/svgtextelement.h"
#include "spx_test_data.h"
#include "tests/test_macros.h"

#include <algorithm>
#include <cmath>
#include <thread>
#include <utility>
#include <vector>

namespace TestSpxSvg {

TEST_CASE("[SPX] SVG loading keeps unpremultiplied RGBA data") {
	Ref<Image> image = memnew(Image());
	const String svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"2\" height=\"3\"><rect width=\"2\" height=\"3\" fill=\"#ff000080\"/></svg>";
	const ScalableImageMemLoadFunc global_svg_loader = Image::_svg_scalable_mem_loader_func;

	CHECK(SpxImageLoaderSVG::create_image_from_string(image, svg, 2.0f, false, HashMap<Color, Color>()) == OK);
	CHECK(Image::_svg_scalable_mem_loader_func == global_svg_loader);
	CHECK(image->get_width() == 4);
	CHECK(image->get_height() == 6);

	const Color pixel = image->get_pixel(0, 0);
	CHECK(pixel.r > 0.95f);
	CHECK(pixel.g < 0.05f);
	CHECK(pixel.b < 0.05f);
	CHECK(pixel.a == doctest::Approx(0.5f).epsilon(0.05f));
}

TEST_CASE("[SPX] SVG loading rejects oversized rasterization") {
	Ref<Image> image = memnew(Image());
	const String svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"20000\" height=\"20000\"><rect width=\"20000\" height=\"20000\" fill=\"#000\"/></svg>";

	CHECK(SpxImageLoaderSVG::create_image_from_string(image, svg, 1.0f, false, HashMap<Color, Color>()) == ERR_INVALID_DATA);
}

struct LunaSVGTextConfigurationReset {
	~LunaSVGTextConfigurationReset() {
		lunasvg_set_grapheme_break_func(nullptr, nullptr);
		lunasvg_set_font_preferences(nullptr, 0);
		lunasvg::setShapingObserverFunction(nullptr, nullptr);
	}
};

struct SpxSvgUtilsFontRegistryReset {
	SpxSvgUtilsFontRegistryReset() {
		reset();
	}

	~SpxSvgUtilsFontRegistryReset() {
		reset();
		lunasvg_set_grapheme_break_func(nullptr, nullptr);
	}

private:
	void reset() {
		SpxSvgUtils::reset_font_registry();
		SpxSvgUtils::ensure_font_faces_registered();
	}
};

static void luna_set_strict_empty_font_preferences() {
	const char *empty_preferences = nullptr;
	lunasvg_set_font_preferences(&empty_preferences, 0);
}

static void luna_set_font_preferences(const char *family) {
	const char *preferences[] = { family };
	lunasvg_set_font_preferences(preferences, 1);
}

static void luna_set_font_preferences(const std::vector<const char *> &families) {
	lunasvg_set_font_preferences(families.data(), families.size());
}

TEST_CASE("[SPX] SpxSvgUtils preserves explicitly empty font preferences") {
	SpxSvgUtilsFontRegistryReset reset;
	Vector<String> preferences;
	SpxSvgUtils::set_font_preferences(preferences);
	SpxSvgUtils::ensure_font_faces_registered();

	CHECK(lunasvg::fontPreferencesConfigured());
}

static size_t codepoint_grapheme_breaks(const uint32_t *, size_t length, size_t *breaks, size_t capacity, void *) {
	const size_t count = MIN(length, capacity);
	for (size_t i = 0; i < count; i++) {
		breaks[i] = i + 1;
	}
	return count;
}

struct SVGShapingObservation {
	size_t start = 0;
	size_t end = 0;
	bool right_to_left = false;
	uint32_t script = 0;
	float width = 0;
	std::vector<uint32_t> glyph_indices;
	std::vector<size_t> clusters;
	std::vector<float> x_positions;
	std::vector<float> y_positions;
};

static void observe_svg_shaping(size_t start, size_t end, bool right_to_left,
		uint32_t script, const uint32_t *glyph_indices, const size_t *clusters,
		const float *x_positions, const float *y_positions, size_t glyph_count, float width, void *closure) {
	auto *observations = static_cast<std::vector<SVGShapingObservation> *>(closure);
	SVGShapingObservation observation;
	observation.start = start;
	observation.end = end;
	observation.right_to_left = right_to_left;
	observation.script = script;
	observation.width = width;
	for (size_t i = 0; i < glyph_count; ++i) {
		observation.glyph_indices.push_back(glyph_indices[i]);
		observation.clusters.push_back(clusters[i]);
		observation.x_positions.push_back(x_positions[i]);
		observation.y_positions.push_back(y_positions[i]);
	}
	observations->push_back(std::move(observation));
}

static bool svg_shaping_observation_is_supported(const SVGShapingObservation &observation) {
	return !observation.glyph_indices.empty() &&
			std::all_of(observation.glyph_indices.begin(), observation.glyph_indices.end(), [](uint32_t glyph_index) {
				return glyph_index != 0;
			});
}

static const SVGShapingObservation *find_supported_svg_shaping_observation(
		const std::vector<SVGShapingObservation> &observations, size_t start, size_t end, bool right_to_left) {
	for (const auto &observation : observations) {
		if (observation.start == start && observation.end == end && observation.right_to_left == right_to_left &&
				svg_shaping_observation_is_supported(observation)) {
			return &observation;
		}
	}
	return nullptr;
}

TEST_CASE("[SPX] LunaSVG clears font faces on the current thread") {
	bool added = false;
	int destroy_calls_before_clear = -1;
	int destroy_calls_after_clear = -1;
	std::thread worker([&]() {
		int destroy_calls = 0;
		auto destroy = +[](void *closure) {
			(*static_cast<int *>(closure))++;
		};
		added = lunasvg_add_font_face_from_data("reset probe", false, false,
				lunasvg::embedded_cnfont_data, lunasvg::embedded_cnfont_size, destroy, &destroy_calls);
		destroy_calls_before_clear = destroy_calls;
		lunasvg_clear_font_faces();
		destroy_calls_after_clear = destroy_calls;
	});
	worker.join();

	CHECK(added);
	CHECK(destroy_calls_before_clear == 0);
	CHECK(destroy_calls_after_clear == 1);
}

TEST_CASE("[SPX] SVG fallback keeps an extended grapheme cluster atomic") {
	LunaSVGTextConfigurationReset reset;
	struct GraphemeProbe {
		uint32_t text[8] = {};
		size_t length = 0;
		int calls = 0;
	};
	GraphemeProbe probe;
	auto callback = +[](const uint32_t *text, size_t length, size_t *breaks, size_t capacity, void *closure) -> size_t {
		GraphemeProbe *state = static_cast<GraphemeProbe *>(closure);
		state->calls++;
		state->length = length < 8 ? length : 8;
		for (size_t i = 0; i < state->length; i++) {
			state->text[i] = text[i];
		}
		if (capacity < 2) {
			return 0;
		}
		breaks[0] = 4;
		breaks[1] = length;
		return 2;
	};

	luna_set_strict_empty_font_preferences();
	lunasvg_set_grapheme_break_func(callback, &probe);
	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text x=\"0\" y=\"20\" font-size=\"20\">&#x2764;&#xFE0F;&#x200D;&#x1F525;A</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);

	CHECK(document != nullptr);
	lunasvg::Box bounds;
	if (document != nullptr) {
		bounds = document->boundingBox();
	}
	CHECK(probe.calls == 1);
	CHECK(probe.length == 5);
	CHECK(probe.text[0] == 0x2764);
	CHECK(probe.text[1] == 0xFE0F);
	CHECK(probe.text[2] == 0x200D);
	CHECK(probe.text[3] == 0x1F525);
	CHECK(probe.text[4] == 0x41);
	if (document != nullptr) {
		CHECK(bounds.w > 20.0f);
		CHECK(bounds.w < 25.0f);
	}
}

TEST_CASE("[SPX] SVG fallback segments common clusters without a callback") {
	LunaSVGTextConfigurationReset reset;
	luna_set_strict_empty_font_preferences();
	lunasvg_set_grapheme_break_func(nullptr, nullptr);
	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text x=\"0\" y=\"20\" font-size=\"20\">A&#x0301;B</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);

	CHECK(document != nullptr);
	if (document != nullptr) {
		const lunasvg::Box bounds = document->boundingBox();
		CHECK(bounds.w > 20.0f);
		CHECK(bounds.w < 25.0f);
	}
}

TEST_CASE("[SPX] LunaSVG trusts authoritative grapheme boundaries") {
	LunaSVGTextConfigurationReset reset;
	luna_set_strict_empty_font_preferences();
	auto callback = +[](const uint32_t *, size_t length, size_t *breaks, size_t capacity, void *) -> size_t {
		if (length != 3 || capacity < 2) {
			return 0;
		}
		// UAX #29 permits a break after a non-pictographic ZWJ sequence here.
		// The local emergency segmenter is deliberately not allowed to filter
		// an authoritative host result.
		breaks[0] = 2;
		breaks[1] = 3;
		return 2;
	};
	lunasvg_set_grapheme_break_func(callback, nullptr);
	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text x=\"0\" y=\"20\" font-size=\"20\">A&#x200D;B</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);
	REQUIRE(document != nullptr);
	CHECK(document->boundingBox().w > 20.0f);
}

TEST_CASE("[SPX] LunaSVG parses CSS font family names") {
	auto families = lunasvg::parseFontFamilyList(R"(Basic Chinese, Basic\ Chinese, \64 efault, serif, 'default', "Comma\2c Family")");
	REQUIRE(families.size() == 4);
	CHECK(families[0] == "Basic Chinese");
	CHECK(families[1] == "Basic Chinese");
	CHECK(families[2] == "default");
	CHECK(families[3] == "Comma,Family");
}

TEST_CASE("[SPX] LunaSVG renders embedded SVG glyphs for arbitrary family names") {
	LunaSVGTextConfigurationReset reset;
	const String font_path = TestSpxData::get_path("fonts/twitter_color_emoji/TwitterColorEmoji-SVGinOT.ttf");
	CHECK(lunasvg_add_font_face_from_file("Heart Emoji Test", false, false, font_path.utf8().get_data()));
	luna_set_font_preferences("Heart Emoji Test");

	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"64\" height=\"64\"><text x=\"4\" y=\"52\" font-size=\"48\">&#x2764;&#xFE0F;</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);
	REQUIRE(document != nullptr);

	auto bitmap = document->renderToBitmap(64, 64);
	REQUIRE(!bitmap.isNull());
	bitmap.convertToRGBA();
	int red_pixels = 0;
	for (int y = 0; y < bitmap.height(); y++) {
		const uint8_t *row = bitmap.data() + y * bitmap.stride();
		for (int x = 0; x < bitmap.width(); x++) {
			const uint8_t *pixel = row + x * 4;
			if (pixel[0] > 150 && pixel[1] < 100 && pixel[2] < 120 && pixel[3] > 100) {
				red_pixels++;
			}
		}
	}
	CHECK(red_pixels > 100);
}

TEST_CASE("[SPX] PlutoVG rejects out-of-range SVG-in-OT glyph data") {
	const String font_path = TestSpxData::get_path("fonts/twitter_color_emoji/TwitterColorEmoji-SVGinOT.ttf");
	plutovg_font_face_t *face = plutovg_font_face_load_from_file(font_path.utf8().get_data(), 0);
	REQUIRE(face != nullptr);
	const char *svg_data = nullptr;
	// Glyph 18 is outside every SVG document range in this subset. It must not
	// produce an unchecked pointer into unrelated font-table bytes.
	CHECK(plutovg_font_face_get_glyph_index_svg(face, 18, &svg_data) == 0);
	CHECK(svg_data == nullptr);
	plutovg_font_face_destroy(face);
}

TEST_CASE("[SPX] LunaSVG renders adjacent embedded SVG glyphs inside one shaped run") {
	LunaSVGTextConfigurationReset reset;
	const String font_path = TestSpxData::get_path("fonts/twitter_color_emoji/TwitterColorEmoji-SVGinOT.ttf");
	CHECK(lunasvg_add_font_face_from_file("Adjacent Emoji Test", false, false, font_path.utf8().get_data()));
	luna_set_font_preferences("Adjacent Emoji Test");

	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"128\" height=\"64\"><text x=\"4\" y=\"52\" font-size=\"48\">&#x2764;&#xFE0F;&#x2764;&#xFE0F;</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);
	REQUIRE(document != nullptr);
	CHECK(document->boundingBox().w > 80.0f);

	auto bitmap = document->renderToBitmap(128, 64);
	REQUIRE(!bitmap.isNull());
	bitmap.convertToRGBA();
	int red_pixels = 0;
	for (int y = 0; y < bitmap.height(); y++) {
		const uint8_t *row = bitmap.data() + y * bitmap.stride();
		for (int x = 0; x < bitmap.width(); x++) {
			const uint8_t *pixel = row + x * 4;
			if (pixel[0] > 150 && pixel[1] < 100 && pixel[2] < 120 && pixel[3] > 100) {
				red_pixels++;
			}
		}
	}
	CHECK(red_pixels > 200);
}

TEST_CASE("[SPX] LunaSVG shapes combining-mark clusters before fallback") {
	LunaSVGTextConfigurationReset reset;
	lunasvg_set_grapheme_break_func(codepoint_grapheme_breaks, nullptr);
	const String font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf");
	CHECK(lunasvg_add_font_face_from_file("Cluster Composition Test", false, false, font_path.utf8().get_data()));
	luna_set_font_preferences("Cluster Composition Test");

	const char decomposed_svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text x=\"0\" y=\"40\" font-size=\"32\">Cafe&#x301;</text></svg>";
	const char composed_svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text x=\"0\" y=\"40\" font-size=\"32\">Caf&#xE9;</text></svg>";
	auto decomposed = lunasvg::Document::loadFromData(decomposed_svg, sizeof(decomposed_svg) - 1);
	auto composed = lunasvg::Document::loadFromData(composed_svg, sizeof(composed_svg) - 1);
	REQUIRE(decomposed != nullptr);
	REQUIRE(composed != nullptr);

	const auto decomposed_bounds = decomposed->boundingBox();
	const auto composed_bounds = composed->boundingBox();
	CHECK(decomposed_bounds.x == doctest::Approx(composed_bounds.x));
	CHECK(decomposed_bounds.y == doctest::Approx(composed_bounds.y));
	CHECK(decomposed_bounds.w == doctest::Approx(composed_bounds.w));
	CHECK(decomposed_bounds.h == doctest::Approx(composed_bounds.h));
}

TEST_CASE("[SPX] LunaSVG shapes complete script runs with inferred direction") {
	LunaSVGTextConfigurationReset reset;
	lunasvg_set_grapheme_break_func(codepoint_grapheme_breaks, nullptr);
	auto add_font = [](const char *family, const char *path) {
		const String font_path = TestSpxData::get_path(path);
		return lunasvg_add_font_face_from_file(family, false, false, font_path.utf8().get_data());
	};
	CHECK(add_font("Latin Run Test", "fonts/noto_sans_clusters/NotoSans-Medium.ttf"));
	CHECK(add_font("Arabic Run Test", "fonts/noto_complex_scripts/NotoSansArabic-Subset.ttf"));
	CHECK(add_font("Hebrew Run Test", "fonts/noto_complex_scripts/NotoSansHebrew-Subset.ttf"));
	CHECK(add_font("Devanagari Run Test", "fonts/noto_complex_scripts/NotoSansDevanagari-Subset.ttf"));
	CHECK(add_font("Thai Run Test", "fonts/noto_complex_scripts/NotoSansThai-Subset.ttf"));

	std::vector<SVGShapingObservation> observations;
	lunasvg::setShapingObserverFunction(observe_svg_shaping, &observations);
	auto load = [&](const char *svg, size_t length) {
		observations.clear();
		auto document = lunasvg::Document::loadFromData(svg, length);
		REQUIRE(document != nullptr);
		CHECK(document->boundingBox().w > 0.0f);
	};

	luna_set_font_preferences("Arabic Run Test");
	const char arabic[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" font-size=\"32\">&#x633;&#x644;&#x627;&#x645;</text></svg>";
	load(arabic, sizeof(arabic) - 1);
	const auto *arabic_run = find_supported_svg_shaping_observation(observations, 0, 4, true);
	REQUIRE(arabic_run != nullptr);
	REQUIRE(arabic_run->clusters.size() == 3);
	CHECK(arabic_run->clusters.front() == 3);
	CHECK(arabic_run->clusters.back() == 0);

	luna_set_font_preferences(std::vector<const char *>{ "Latin Run Test", "Hebrew Run Test" });
	const char hebrew_mixed[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" font-size=\"32\">a &#x5E9;&#x5DC;&#x5D5;&#x5DD;</text></svg>";
	load(hebrew_mixed, sizeof(hebrew_mixed) - 1);
	CHECK(find_supported_svg_shaping_observation(observations, 0, 2, false) != nullptr);
	const auto *hebrew_run = find_supported_svg_shaping_observation(observations, 2, 6, true);
	REQUIRE(hebrew_run != nullptr);
	CHECK(hebrew_run->clusters.front() == 5);
	CHECK(hebrew_run->clusters.back() == 2);

	const char hebrew_rtl_mixed[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" direction=\"rtl\" font-size=\"32\">&#x5E9;&#x5DC;&#x5D5;&#x5DD; abc</text></svg>";
	load(hebrew_rtl_mixed, sizeof(hebrew_rtl_mixed) - 1);
	std::vector<std::pair<size_t, size_t>> visual_run_order;
	for (const auto &observation : observations) {
		if (!svg_shaping_observation_is_supported(observation)) {
			continue;
		}
		auto range = std::make_pair(observation.start, observation.end);
		if (range != std::make_pair<size_t, size_t>(5, 8) && range != std::make_pair<size_t, size_t>(0, 5)) {
			continue;
		}
		if (std::find(visual_run_order.begin(), visual_run_order.end(), range) == visual_run_order.end()) {
			visual_run_order.push_back(range);
		}
	}
	REQUIRE(visual_run_order.size() == 2);
	CHECK(visual_run_order[0].first == 5);
	CHECK(visual_run_order[0].second == 8);
	CHECK(visual_run_order[1].first == 0);
	CHECK(visual_run_order[1].second == 5);

	luna_set_font_preferences("Devanagari Run Test");
	const char devanagari[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" font-size=\"32\">&#x928;&#x92E;&#x938;&#x94D;&#x924;&#x947;</text></svg>";
	load(devanagari, sizeof(devanagari) - 1);
	const auto *devanagari_run = find_supported_svg_shaping_observation(observations, 0, 6, false);
	REQUIRE(devanagari_run != nullptr);
	CHECK(devanagari_run->glyph_indices.size() == 5);

	luna_set_font_preferences("Thai Run Test");
	// The second syllable stacks a tone mark above a vowel. It requires GPOS
	// context and has a non-zero vertical offset in Noto Sans Thai.
	const char thai[] = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" font-size=\"32\">&#xE40;&#xE01;&#xE49;&#xE32;&#xE01;&#xE34;&#xE48;</text></svg>";
	load(thai, sizeof(thai) - 1);
	const auto *thai_run = find_supported_svg_shaping_observation(observations, 0, 7, false);
	REQUIRE(thai_run != nullptr);
	CHECK(thai_run->glyph_indices.size() == 7);
	CHECK(std::any_of(thai_run->y_positions.begin(), thai_run->y_positions.end(), [](float position) {
		return std::abs(position) > 0.01f;
	}));
}

TEST_CASE("[SPX] LunaSVG preserves kerning across grapheme clusters") {
	LunaSVGTextConfigurationReset reset;
	lunasvg_set_grapheme_break_func(codepoint_grapheme_breaks, nullptr);
	const String font_path = TestSpxData::get_path("fonts/lunasvg_kerning.ttf");
	CHECK(lunasvg_add_font_face_from_file("Kerning Run Test", false, false, font_path.utf8().get_data()));
	luna_set_font_preferences("Kerning Run Test");

	std::vector<SVGShapingObservation> observations;
	lunasvg::setShapingObserverFunction(observe_svg_shaping, &observations);
	auto shape_width = [&](const char *text, size_t text_length) {
		observations.clear();
		std::string svg = "<svg xmlns=\"http://www.w3.org/2000/svg\"><text y=\"40\" font-size=\"32\">";
		svg.append(text, text_length);
		svg += "</text></svg>";
		auto document = lunasvg::Document::loadFromData(svg.data(), svg.size());
		if (document == nullptr) {
			return -1.0f;
		}
		document->boundingBox();
		const auto *run = find_supported_svg_shaping_observation(observations, 0, text_length, false);
		return run == nullptr ? -1.0f : run->width;
	};

	const auto pair_width = shape_width("AV", 2);
	const auto separate_width = shape_width("A", 1) + shape_width("V", 1);
	REQUIRE(pair_width >= 0.0f);
	REQUIRE(separate_width >= 0.0f);
	CHECK(pair_width < separate_width);
}

TEST_CASE("[SPX] LunaSVG renders an emoji ZWJ sequence as one shaped SVG glyph") {
	LunaSVGTextConfigurationReset text_reset;
	SpxSvgUtilsFontRegistryReset registry_reset;
	const String font_path = TestSpxData::get_path("fonts/twitter_color_emoji/TwitterColorEmoji-SVGinOT.ttf");
	const Vector<uint8_t> font_data = FileAccess::get_file_as_bytes(font_path);
	REQUIRE_FALSE(font_data.is_empty());
	SpxSvgUtils::add_font_face("Emoji Cluster Test", font_data.ptr(), font_data.size());
	SpxSvgUtils::set_font_preferences(Vector<String>{ "Emoji Cluster Test" });
	REQUIRE(SpxSvgUtils::ensure_font_faces_registered());
	std::vector<SVGShapingObservation> observations;
	lunasvg::setShapingObserverFunction(observe_svg_shaping, &observations);

	const char svg[] = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"64\" height=\"64\"><text x=\"4\" y=\"52\" font-size=\"48\">&#x1F469;&#x200D;&#x1F4BB;</text></svg>";
	auto document = lunasvg::Document::loadFromData(svg, sizeof(svg) - 1);
	REQUIRE(document != nullptr);
	const auto bounds = document->boundingBox();
	CHECK(bounds.w > 40.0f);
	CHECK(bounds.w < 55.0f);
	CHECK(std::any_of(observations.begin(), observations.end(), [](const SVGShapingObservation &observation) {
		return observation.start == 0 && observation.end == 3 && observation.glyph_indices == std::vector<uint32_t>{ 2795 } &&
				observation.clusters == std::vector<size_t>{ 0 };
	}));

	auto bitmap = document->renderToBitmap(64, 64);
	REQUIRE(!bitmap.isNull());
	bitmap.convertToRGBA();
	int colored_pixels = 0;
	for (int y = 0; y < bitmap.height(); y++) {
		const uint8_t *row = bitmap.data() + y * bitmap.stride();
		for (int x = 0; x < bitmap.width(); x++) {
			const uint8_t *pixel = row + x * 4;
			const int max_channel = MAX(pixel[0], MAX(pixel[1], pixel[2]));
			const int min_channel = MIN(pixel[0], MIN(pixel[1], pixel[2]));
			if (pixel[3] > 100 && max_channel - min_channel > 40) {
				colored_pixels++;
			}
		}
	}
	CHECK(colored_pixels > 100);
}

} // namespace TestSpxSvg

#endif // TEST_SPX_SVG_H
