/**************************************************************************/
/*  test_spx_res_mgr.h                                                    */
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

#ifndef TEST_SPX_RES_MGR_H
#define TEST_SPX_RES_MGR_H

#include "../project_font_transaction.h"
#include "../spx_image_loader_svg.h"
#include "../spx_res_mgr.h"
#include "../spx_svg_utils.h"
#include "../spx_theme_font.h"
#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/io/json.h"
#include "scene/theme/theme_db.h"
#include "spx_test_data.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"

#include <limits>

namespace TestSpxResMgr {

struct ProjectFontStateReset {
	Ref<Font> previous_default_font;
	Ref<Font> previous_fallback_font;

	ProjectFontStateReset() {
		spx_get_theme_fonts(previous_default_font, previous_fallback_font);
		SpxSvgUtils::reset_font_registry();
	}

	~ProjectFontStateReset() {
		spx_set_theme_fonts(previous_default_font, previous_fallback_font);
		SpxSvgUtils::reset_font_registry();
	}
};

struct GdStringArray {
	Vector<CharString> storage;
	Vector<GdString> values;
	GdArrayInfo info = {};

	explicit GdStringArray(const Vector<String> &p_values) {
		storage.resize(p_values.size());
		values.resize(p_values.size());
		for (int i = 0; i < p_values.size(); i++) {
			storage.write[i] = p_values[i].utf8();
			values.write[i] = storage[i].get_data();
		}
		info.size = p_values.size();
		info.type = GD_ARRAY_TYPE_STRING;
		info.data = values.is_empty() ? nullptr : values.ptrw();
	}

	GdArray ptr() { return &info; }
};

static String apply_fonts_raw(SpxResMgr &p_res_mgr, const String &p_default_path, GdArray p_paths, GdArray p_families, GdArray p_preferences) {
	CharString default_path = p_default_path.utf8();
	GdString result = p_res_mgr.apply_project_fonts(default_path.get_data(), p_paths, p_families, p_preferences);
	String error = String::utf8(static_cast<const char *>(result));
	SpxAbi::free_return_cstr(result);
	return error;
}

static String apply_fonts(SpxResMgr &p_res_mgr, const String &p_default_path, GdStringArray &p_paths, GdStringArray &p_families, GdStringArray &p_preferences) {
	return apply_fonts_raw(p_res_mgr, p_default_path, p_paths.ptr(), p_families.ptr(), p_preferences.ptr());
}

static Ref<Image> render_project_text() {
	Ref<Image> image;
	image.instantiate();
	const String svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"160\" height=\"40\"><text x=\"2\" y=\"30\" font-family=\"Project, default\" font-size=\"24\">SPX fonts</text></svg>";
	REQUIRE(SpxImageLoaderSVG::create_image_from_string(image, svg, 1, false, {}) == OK);
	return image;
}

struct SvgFile {
	String path = TestUtils::get_temp_path("spx_resource_cache.svg");

	SvgFile() { write(8, 6); }
	~SvgFile() { DirAccess::remove_absolute(path); }

	void write(int p_width, int p_height) {
		Ref<FileAccess> file = FileAccess::open(path, FileAccess::WRITE);
		REQUIRE(file.is_valid());
		file->store_string(vformat("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\"><rect width=\"100%%\" height=\"100%%\" fill=\"red\"/></svg>", p_width, p_height));
	}

	void create_clip(SpxResMgr &p_resources) {
		Dictionary frame;
		frame["path"] = path;
		frame["bitmap"] = 2;
		Array frames;
		frames.push_back(frame);
		Dictionary clip;
		clip["frames"] = frames;
		clip["max_bitmap"] = 4;
		CharString json = JSON::stringify(clip).utf8();
		p_resources.create_animation("Cache", "walk", json.get_data(), 12, false);
	}
};

TEST_CASE("[SceneTree][SPX] SVG cache belongs to its resource manager") {
	ProjectFontStateReset reset;
	SvgFile svg;
	SpxResMgr first;
	const Ref<Texture2D> first_texture = first.load_texture(svg.path);
	REQUIRE(first_texture.is_valid());
	CHECK(first.load_texture(svg.path) == first_texture);
	CHECK(first.load_texture_checked(svg.path, true) == first_texture);
	first.set_load_mode(false);
	CHECK(first.load_texture_checked(svg.path) == first_texture);
	{
		SpxResMgr second;
		const Ref<Texture2D> second_texture = second.load_texture(svg.path);
		REQUIRE(second_texture.is_valid());
		CHECK(second_texture != first_texture);
		second.on_reset(0);
		CHECK(second.load_texture(svg.path) != second_texture);
	}
	CHECK(first.load_texture(svg.path) == first_texture);
	CHECK(first_texture->get_size() == Vector2(8, 6));
}

TEST_CASE("[SceneTree][SPX] SVG clip invalidation rebuilds every requested raster scale") {
	SvgFile svg;
	SpxResMgr resources;
	svg.create_clip(resources);
	const Ref<SpriteFrames> original = resources.get_animation_frames("Cache::walk");
	REQUIRE(original.is_valid());
	CHECK(original->get_frame_texture("Cache::walk", 0)->get_size() == Vector2(16, 12));
	const Ref<SpriteFrames> enlarged = resources.get_animation_frames("Cache::walk", 4);
	REQUIRE(enlarged.is_valid());
	CHECK(enlarged->get_frame_texture("Cache::walk", 0)->get_size() == Vector2(64, 48));

	svg.write(12, 10);
	resources.update_caches(Vector<String>{ svg.path });
	const Ref<SpriteFrames> replacement = resources.get_animation_frames("Cache::walk");
	REQUIRE(replacement.is_valid());
	CHECK(replacement != original);
	CHECK(replacement->get_frame_texture("Cache::walk", 0)->get_size() == Vector2(24, 20));
	const Ref<SpriteFrames> enlarged_replacement = resources.get_animation_frames("Cache::walk", 4);
	REQUIRE(enlarged_replacement.is_valid());
	CHECK(enlarged_replacement->get_frame_texture("Cache::walk", 0)->get_size() == Vector2(96, 80));
	CHECK(original->get_frame_texture("Cache::walk", 0)->get_size() == Vector2(16, 12));
}

TEST_CASE("[SceneTree][SPX] Direct texture loader preserves WebP frame size and alpha") {
	SpxResMgr res_mgr;
	res_mgr.set_load_mode(true);
	const String frame_path = TestUtils::get_data_path("images/icon.webp");

	const Ref<Texture2D> texture = res_mgr.load_texture(frame_path, true);
	REQUIRE(texture.is_valid());
	CHECK(texture->get_size() == Vector2(256, 256));

	const Ref<Image> image = texture->get_image();
	REQUIRE(image.is_valid());
	CHECK(image->get_size() == Vector2i(256, 256));
	CHECK(image->get_format() == Image::FORMAT_RGBA8);

	bool has_transparent_pixel = false;
	bool has_opaque_pixel = false;
	for (int y = 0; y < image->get_height() && (!has_transparent_pixel || !has_opaque_pixel); y++) {
		for (int x = 0; x < image->get_width(); x++) {
			const float alpha = image->get_pixel(x, y).a;
			has_transparent_pixel = has_transparent_pixel || alpha == 0.0f;
			has_opaque_pixel = has_opaque_pixel || alpha == 1.0f;
		}
	}
	CHECK(has_transparent_pixel);
	CHECK(has_opaque_pixel);

	const Ref<Texture2D> cached = res_mgr.load_texture(frame_path, true);
	CHECK(cached == texture);
	CHECK(res_mgr.load_texture_checked(frame_path, true) == texture);

	// Pixel consumers may modify their copy without changing the texture or
	// later collision/atlas queries sharing the same loaded resource.
	const Color original_pixel = image->get_pixel(0, 0);
	image->set_pixel(0, 0, Color(1, 0, 1, 1));
	CHECK(cached->get_image()->get_pixel(0, 0) == original_pixel);
}

TEST_CASE("[SceneTree][SPX] Direct bitmap failures preserve checked loading and retry") {
	SpxResMgr resources;
	const String path = TestUtils::get_temp_path("spx_bitmap_retry.png");
	if (FileAccess::exists(path)) {
		REQUIRE(DirAccess::remove_absolute(path) == OK);
	}

	ERR_PRINT_OFF
	const Ref<Texture2D> missing = resources.load_texture_checked(path);
	const Ref<Texture2D> placeholder = resources.load_texture(path);
	const Ref<Texture2D> still_missing = resources.load_texture_checked(path);
	ERR_PRINT_ON
	CHECK(missing.is_null());
	CHECK(still_missing.is_null());
	REQUIRE(placeholder.is_valid());
	CHECK(placeholder->get_size() == Vector2(4, 4));
	const Ref<Image> placeholder_image = placeholder->get_image();
	REQUIRE(placeholder_image.is_valid());
	CHECK(placeholder_image->get_pixel(0, 0).is_equal_approx(Color(1, 0, 1, 128.0f / 255.0f)));

	Ref<Image> image = Image::create_empty(8, 6, false, Image::FORMAT_RGBA8);
	image->fill(Color(0, 1, 0));
	REQUIRE(image->save_png(path) == OK);
	const Ref<Texture2D> loaded = resources.load_texture_checked(path);
	REQUIRE(loaded.is_valid());
	CHECK(loaded != placeholder);
	CHECK(loaded->get_size() == Vector2(8, 6));
	CHECK(resources.load_texture(path) == loaded);
	CHECK(resources.load_texture_checked(path) == loaded);
	CHECK(DirAccess::remove_absolute(path) == OK);
}

TEST_CASE("[SceneTree][SPX] Project theme font helper updates default and fallback fonts") {
	ProjectFontStateReset reset;
	Ref<FontFile> project_font;
	project_font.instantiate();

	spx_set_project_theme_font(project_font);
	const Ref<Theme> default_theme = ThemeDB::get_singleton()->get_default_theme();
	REQUIRE(default_theme.is_valid());
	CHECK(default_theme->get_default_font() == project_font);
	CHECK(ThemeDB::get_singleton()->get_fallback_font() == project_font);
}

TEST_CASE("[SPX] Project font request validation") {
	ProjectFonts::Request request;
	request.default_path = "res://default.ttf";
	request.faces.push_back({ "res://project.ttf", "Project", String() });
	request.preferences.push_back("project");
	request.preferences.push_back("default");
	String error;

	SUBCASE("canonicalizes family keys once") {
		CHECK(ProjectFonts::validate_request(request, error));
		CHECK(request.faces[0].family_key == "project");
	}

	SUBCASE("rejects the reserved default family") {
		request.faces.write[0].family = "DEFAULT";
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("reserved name default"));
	}

	SUBCASE("rejects duplicate folded families") {
		request.faces.push_back({ "res://duplicate.ttf", "PROJECT", String() });
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("duplicated after ASCII case folding"));
	}

	SUBCASE("rejects unavailable preferences") {
		request.preferences.write[0] = "Missing";
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("not an available font family"));
	}
}

TEST_CASE("[SceneTree][SPX] Project font transaction accepts typed empty arrays") {
	ProjectFontStateReset reset;
	SpxResMgr res_mgr;
	const String font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf");
	GdStringArray empty_paths(Vector<String>{});
	GdStringArray empty_families(Vector<String>{});
	GdStringArray empty_preferences(Vector<String>{});

	CHECK(apply_fonts(res_mgr, font_path, empty_paths, empty_families, empty_preferences).is_empty());
	REQUIRE(ThemeDB::get_singleton()->get_default_theme().is_valid());
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font().is_valid());
}

TEST_CASE("[SceneTree][SPX] Invalid project font preserves the published transaction") {
	ProjectFontStateReset reset;
	SpxResMgr res_mgr;
	const String font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf");
	GdStringArray project_paths(Vector<String>{ font_path });
	GdStringArray project_families(Vector<String>{ "Project" });
	GdStringArray project_preferences(Vector<String>{ "Project", "default" });
	REQUIRE(apply_fonts(res_mgr, font_path, project_paths, project_families, project_preferences).is_empty());

	const Ref<Image> published_svg = render_project_text();
	REQUIRE(published_svg->get_used_rect().has_area());
	const Ref<Theme> published_theme = ThemeDB::get_singleton()->get_default_theme();
	const Ref<Font> published_font = published_theme->get_default_font();
	GdStringArray bad_paths(Vector<String>{ TestUtils::get_data_path("images/icon.png") });
	GdStringArray bad_families(Vector<String>{ "Broken" });
	GdStringArray bad_preferences(Vector<String>{ "Broken" });

	ERR_PRINT_OFF
	const String error = apply_fonts(res_mgr, font_path, bad_paths, bad_families, bad_preferences);
	ERR_PRINT_ON
	CHECK_FALSE(error.is_empty());
	CHECK(render_project_text()->get_data() == published_svg->get_data());
	CHECK(ThemeDB::get_singleton()->get_default_theme() == published_theme);
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() == published_font);
}

TEST_CASE("[SceneTree][SPX] Resource shutdown restores the host theme fonts") {
	ProjectFontStateReset restore_fonts;
	SpxResMgr resources;
	resources.on_awake();
	const Ref<Image> original_svg = render_project_text();
	const CharString font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf").utf8();
	resources.set_default_font(font_path.get_data());
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() != restore_fonts.previous_default_font);
	resources.on_destroy();
	CHECK(render_project_text()->get_data() == original_svg->get_data());
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() == restore_fonts.previous_default_font);
	CHECK(ThemeDB::get_singleton()->get_fallback_font() == restore_fonts.previous_fallback_font);
}

TEST_CASE("[SceneTree][SPX] Incremental font updates validate before publishing and invalidate SVGs") {
	ProjectFontStateReset restore_fonts;
	SpxResMgr resources;
	SvgFile svg;
	const CharString font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf").utf8();
	auto image = resources.load_svg_texture(svg.path, 1);
	resources.set_default_font(font_path.get_data());
	auto after_default = resources.load_svg_texture(svg.path, 1);
	CHECK(after_default != image);
	resources.register_font_face(font_path.get_data(), "Project");
	auto after_face = resources.load_svg_texture(svg.path, 1);
	CHECK(after_face != after_default);
	GdStringArray preferences(Vector<String>{ "Project", "default" });
	resources.set_font_preferences(preferences.ptr());
	auto after_preferences = resources.load_svg_texture(svg.path, 1);
	CHECK(after_preferences != after_face);

	const Ref<Image> published_svg = render_project_text();
	REQUIRE(published_svg->get_used_rect().has_area());
	const Ref<Font> published = ThemeDB::get_singleton()->get_default_theme()->get_default_font();
	GdStringArray unknown(Vector<String>{ "Missing" });
	GdStringArray duplicate(Vector<String>{ "Project", "PROJECT" });
	GdArrayInfo malformed = {};
	malformed.type = GD_ARRAY_TYPE_FLOAT;
	ERR_PRINT_OFF
	resources.set_font_preferences(&malformed);
	resources.set_font_preferences(unknown.ptr());
	resources.set_font_preferences(duplicate.ptr());
	resources.register_font_face(font_path.get_data(), "DEFAULT");
	resources.set_default_font("missing-font.ttf");
	ERR_PRINT_ON
	CHECK(render_project_text()->get_data() == published_svg->get_data());
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() == published);
	CHECK(resources.load_svg_texture(svg.path, 1) == after_preferences);
}

TEST_CASE("[SceneTree][SPX] Malformed project font arrays are rejected atomically") {
	ProjectFontStateReset reset;
	SpxResMgr res_mgr;
	const String font_path = TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf");
	GdStringArray empty(Vector<String>{});
	GdStringArray preferences(Vector<String>{ "default" });
	REQUIRE(apply_fonts(res_mgr, font_path, empty, empty, preferences).is_empty());
	const Ref<Image> published_svg = render_project_text();
	REQUIRE(published_svg->get_used_rect().has_area());
	const Ref<Font> published_font = ThemeDB::get_singleton()->get_default_theme()->get_default_font();
	const Ref<Theme> published_theme = ThemeDB::get_singleton()->get_default_theme();

	GdArrayInfo wrong_type = {};
	wrong_type.type = GD_ARRAY_TYPE_FLOAT;
	GdArrayInfo missing_data = {};
	missing_data.size = 1;
	missing_data.type = GD_ARRAY_TYPE_STRING;
	GdString oversized_value = "unused";
	GdArrayInfo oversized = {};
	oversized.size = std::numeric_limits<int32_t>::max();
	oversized.type = GD_ARRAY_TYPE_STRING;
	oversized.data = &oversized_value;
	GdStringArray one_path(Vector<String>{ font_path });

	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, nullptr, empty.ptr(), empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, &wrong_type, empty.ptr(), empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, &missing_data, empty.ptr(), empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, &oversized, empty.ptr(), empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, one_path.ptr(), empty.ptr(), empty.ptr()).is_empty());

	CHECK(ThemeDB::get_singleton()->get_default_theme() == published_theme);
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() == published_font);
	CHECK(render_project_text()->get_data() == published_svg->get_data());
}

} // namespace TestSpxResMgr

#endif // TEST_SPX_RES_MGR_H
