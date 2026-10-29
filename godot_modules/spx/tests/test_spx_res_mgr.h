/**************************************************************************/
/*  test_spx_res_mgr.h                                                  */
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
#include "../spx_svg_cache.h"
#include "../spx_svg_utils.h"
#include "../spx_theme_font.h"
#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/io/json.h"
#include "core/io/resource_saver.h"
#include "scene/resources/audio_stream_wav.h"
#include "scene/resources/placeholder_textures.h"
#include "scene/theme/theme_db.h"
#include "spx_test_data.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"

#include <limits>
#include <thread>

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
	return TestSpx::take_string(p_res_mgr.apply_project_fonts(default_path.get_data(), p_paths, p_families, p_preferences));
}

static String apply_fonts(SpxResMgr &p_res_mgr, const String &p_default_path, GdStringArray &p_paths, GdStringArray &p_families, GdStringArray &p_preferences) {
	return apply_fonts_raw(p_res_mgr, p_default_path, p_paths.ptr(), p_families.ptr(), p_preferences.ptr());
}

static Ref<Image> render_project_text() {
	Ref<Image> image;
	image.instantiate();
	const String svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"160\" height=\"40\"><text x=\"2\" y=\"30\" font-family=\"Project, default\" font-size=\"24\">SPX fonts</text></svg>";
	REQUIRE(SpxImageLoaderSVG::rasterize(image, svg, 1, {}) == OK);
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

TEST_CASE("[SceneTree][SPX] SVG cache rejects invalid requests without evicting valid entries") {
	SvgFile svg;
	SpxSvgCache cache;
	const Ref<ImageTexture> texture = cache.load_texture(svg.path, 1);
	REQUIRE(texture.is_valid());
	Ref<SpriteFrames> source;
	source.instantiate();
	source->add_frame("default", texture, 2.5);
	const Ref<SpriteFrames> animation = cache.load_animation("default", source, { 1 }, 1);
	REQUIRE(animation.is_valid());
	CHECK(animation->get_frame_texture("default", 0) == texture);
	CHECK(animation->get_frame_duration("default", 0) == 2.5);
	ERR_PRINT_OFF;
	CHECK(cache.load_texture(svg.path, 0).is_null());
	CHECK(cache.load_texture("texture.png", 1).is_null());
	CHECK(cache.load_animation("default", source, { 1 }, 0).is_null());
	CHECK(cache.load_animation("default", Ref<SpriteFrames>(), { 1 }, 1).is_null());
	CHECK(cache.load_animation("missing", source, { 1 }, 1).is_null());
	CHECK(cache.load_animation("default", source, {}, 1).is_null());
	Ref<SpriteFrames> empty_frame;
	empty_frame.instantiate();
	empty_frame->add_frame("default", Ref<Texture2D>());
	CHECK(cache.load_animation("default", empty_frame, { 1 }, 2).is_null());
	ERR_PRINT_ON;

	bool texture_rejected = false;
	bool reload_rejected = false;
	bool animation_rejected = false;
	ERR_PRINT_OFF;
	std::thread worker([&]() {
		texture_rejected = cache.load_texture(svg.path, 1).is_null();
		reload_rejected = cache.reload_texture(svg.path).is_null();
		animation_rejected = cache.load_animation("default", source, { 1 }, 1).is_null();
		cache.invalidate_texture(svg.path);
		cache.clear();
	});
	worker.join();
	ERR_PRINT_ON;
	CHECK(texture_rejected);
	CHECK(reload_rejected);
	CHECK(animation_rejected);
	CHECK(cache.load_texture(svg.path, 1) == texture);
	CHECK(cache.load_animation("default", source, { 1 }, 1) == animation);
	CHECK(texture->get_size() == Vector2(8, 6));
}

TEST_CASE("[SceneTree][SPX] SVG reload retries missing files and preserves scales without a base texture") {
	SvgFile svg;
	SpxSvgCache cache;
	REQUIRE(DirAccess::remove_absolute(svg.path) == OK);
	ERR_PRINT_OFF;
	CHECK(cache.reload_texture(svg.path).is_null());
	ERR_PRINT_ON;
	svg.write(8, 6);
	const Ref<ImageTexture> enlarged = cache.load_texture(svg.path, 2);
	REQUIRE(enlarged.is_valid());
	CHECK(enlarged->get_size() == Vector2(16, 12));
	REQUIRE(DirAccess::remove_absolute(svg.path) == OK);
	ERR_PRINT_OFF;
	CHECK(cache.reload_texture(svg.path).is_null());
	ERR_PRINT_ON;
	CHECK(cache.load_texture(svg.path, 2) == enlarged);
	CHECK(enlarged->get_size() == Vector2(16, 12));
	svg.write(12, 10);
	const Ref<ImageTexture> base = cache.reload_texture(svg.path);
	REQUIRE(base.is_valid());
	CHECK(base->get_size() == Vector2(12, 10));
	CHECK(cache.load_texture(svg.path, 1) == base);
	CHECK(cache.load_texture(svg.path, 2) == enlarged);
	CHECK(enlarged->get_size() == Vector2(24, 20));
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
	resources.reload_texture(path.utf8().get_data());
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

TEST_CASE("[SceneTree][SPX] Bitmap reload preserves texture identity and retries failed reads") {
	TestSpx::TempDirectory files("spx_bitmap_reload");
	const String path = files.file("texture.png");
	const CharString path_utf8 = path.utf8();
	SpxResMgr resources;
	Ref<Image> image = Image::create_empty(8, 6, false, Image::FORMAT_RGBA8);
	image->fill(Color(0, 0, 0, 0));
	REQUIRE(image->save_png(path) == OK);
	resources.reload_texture(path_utf8.get_data());
	const Ref<Texture2D> texture = resources.load_texture_checked(path);
	REQUIRE(texture.is_valid());
	CHECK(texture->get_size() == Vector2(8, 6));
	CHECK(Rect2(resources.get_bound_from_alpha(path_utf8.get_data())) == Rect2());

	image = Image::create_empty(10, 12, false, Image::FORMAT_RGBA8);
	image->fill(Color(0, 0, 0, 0));
	image->set_pixel(2, 3, Color(0, 1, 0));
	image->set_pixel(6, 7, Color(0, 1, 0));
	REQUIRE(image->save_png(path) == OK);
	resources.reload_texture(path_utf8.get_data());
	CHECK(resources.load_texture_checked(path) == texture);
	CHECK(texture->get_size() == Vector2(10, 12));
	CHECK(texture->get_image()->get_pixel(2, 3) == Color(0, 1, 0));
	CHECK(Rect2(resources.get_bound_from_alpha(path_utf8.get_data())) == Rect2(2, 3, 5, 5));

	REQUIRE(DirAccess::remove_absolute(path) == OK);
	ERR_PRINT_OFF;
	resources.reload_texture(path_utf8.get_data());
	ERR_PRINT_ON;
	CHECK(resources.load_texture_checked(path) == texture);
	CHECK(texture->get_size() == Vector2(10, 12));
	CHECK(texture->get_image()->get_pixel(2, 3) == Color(0, 1, 0));

	image->fill(Color(1, 0, 0));
	REQUIRE(image->save_png(path) == OK);
	resources.update_caches(Vector<String>{ path });
	const Ref<Texture2D> replacement = resources.load_texture_checked(path);
	REQUIRE(replacement.is_valid());
	CHECK(replacement != texture);
	CHECK(replacement->get_image()->get_pixel(2, 3) == Color(1, 0, 0));
	CHECK(texture->get_image()->get_pixel(2, 3) == Color(0, 1, 0));
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
	CHECK(ProjectFonts::fold_family(String::utf8("9-AÉé")) == String::utf8("9-aÉé"));
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
	SUBCASE("rejects an empty face path") {
		request.faces.write[0].path = "";
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("Font path at index 0"));
	}
	SUBCASE("rejects an empty family") {
		request.faces.write[0].family = "";
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("Font family at index 0"));
	}
	SUBCASE("rejects empty preferences") {
		request.preferences.write[0] = "";
		CHECK_FALSE(ProjectFonts::validate_request(request, error));
		CHECK(error.contains("Font preference at index 0"));
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
	resources.on_awake();
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
	GdString null_value = nullptr;
	GdArrayInfo null_string = { 1, GD_ARRAY_TYPE_STRING, &null_value };
	CHECK(apply_fonts_raw(res_mgr, font_path, &null_string, empty.ptr(), empty.ptr()).contains("invalid string at index 0"));
	CHECK(TestSpx::take_string(res_mgr.apply_project_fonts(nullptr, empty.ptr(), empty.ptr(), empty.ptr())).contains("must be a string"));
	CHECK(apply_fonts_raw(res_mgr, "", empty.ptr(), empty.ptr(), empty.ptr()).contains("must not be empty"));
	String thread_error;
	std::thread worker([&]() {
		thread_error = TestSpx::take_string(res_mgr.apply_project_fonts("font.ttf", empty.ptr(), empty.ptr(), empty.ptr()));
	});
	worker.join();
	CHECK(thread_error.contains("main thread"));
	ERR_PRINT_OFF;
	std::thread setter([&]() {
		res_mgr.set_default_font("font.ttf");
		res_mgr.register_font_face("font.ttf", "Family");
		res_mgr.set_font_preferences(empty.ptr());
	});
	setter.join();
	res_mgr.register_font_face("font.ttf", "");
	ERR_PRINT_ON;
	missing_data.size = -1;
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, &missing_data, empty.ptr(), empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, empty.ptr(), &wrong_type, empty.ptr()).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, font_path, empty.ptr(), empty.ptr(), &wrong_type).is_empty());
	CHECK_FALSE(apply_fonts_raw(res_mgr, "missing_default.ttf", empty.ptr(), empty.ptr(), empty.ptr()).is_empty());
	GdStringArray reserved_family(Vector<String>{ "DEFAULT" });
	CHECK(apply_fonts(res_mgr, font_path, one_path, reserved_family, empty).contains("reserved name default"));
	GdStringArray repeated_paths(Vector<String>{ font_path, font_path });
	GdStringArray repeated_families(Vector<String>{ "Project", "PROJECT" });
	CHECK(apply_fonts(res_mgr, font_path, repeated_paths, repeated_families, empty).contains("duplicated after ASCII case folding"));
	GdStringArray unavailable_preferences(Vector<String>{ "Unavailable" });
	CHECK(apply_fonts(res_mgr, font_path, empty, empty, unavailable_preferences).contains("not an available font family"));

	CHECK(ThemeDB::get_singleton()->get_default_theme() == published_theme);
	CHECK(ThemeDB::get_singleton()->get_default_theme()->get_default_font() == published_font);
	CHECK(render_project_text()->get_data() == published_svg->get_data());
}

TEST_CASE("[SceneTree][SPX] Font preparation reads imported resources and reports unusable data") {
	TestSpx::TempDirectory files("spx_imported_fonts");
	SpxResMgr resources;
	Ref<FontFile> imported;
	imported.instantiate();
	const auto original_data = FileAccess::get_file_as_bytes(TestSpxData::get_path("fonts/noto_sans_clusters/NotoSans-Medium.ttf"));
	REQUIRE_FALSE(original_data.is_empty());
	imported->set_data(original_data);
	REQUIRE(ResourceSaver::save(imported, files.file("font.tres")) == OK);
	Vector<uint8_t> data;
	Ref<FontFile> display;
	String error;
	REQUIRE(ProjectFonts::prepare_font(files.file("font.tres"), resources, data, display, error));
	CHECK(data == original_data);
	CHECK(display.is_valid());
	CHECK_FALSE(display->is_allow_system_fallback());
	Ref<FontFile> empty;
	empty.instantiate();
	REQUIRE(ResourceSaver::save(empty, files.file("empty.tres")) == OK);
	files.write("empty.font", "");
	ERR_PRINT_OFF;
	CHECK_FALSE(ProjectFonts::prepare_font("", resources, data, display, error));
	CHECK(error.contains("must not be empty"));
	CHECK_FALSE(ProjectFonts::prepare_font(files.file("empty.tres"), resources, data, display, error));
	CHECK(error.contains("has no data"));
	CHECK_FALSE(ProjectFonts::prepare_font(files.file("empty.font"), resources, data, display, error));
	CHECK(error.contains("Font file is empty"));
	resources.register_font_face(files.file("missing.ttf").utf8().get_data(), "Missing");
	ERR_PRINT_ON;
	HashMap<String, Ref<FontFile>> fonts;
	fonts.insert("empty", Ref<FontFile>());
	auto fallback = ProjectFonts::build_display_font_chain(fonts, { "missing", "empty" });
	Ref<FontFile> fallback_file = fallback;
	REQUIRE(fallback_file.is_valid());
	CHECK_FALSE(fallback_file->is_allow_system_fallback());
}

TEST_CASE("[SceneTree][SPX] Resource loading mode resolves imported textures and reports missing resources") {
	TestSpx::TempDirectory files("spx_imported_resources");
	Ref<PlaceholderTexture2D> texture;
	texture.instantiate();
	texture->set_size(Vector2(7, 5));
	REQUIRE(ResourceSaver::save(texture, files.file("texture.tres")) == OK);
	SpxResMgr resources;
	resources.set_load_mode(false);
	auto loaded = resources.load_texture(files.file("texture.tres"));
	REQUIRE(loaded.is_valid());
	CHECK(loaded->get_size() == Vector2(7, 5));
	const CharString missing = files.file("missing.tres").utf8();
	ERR_PRINT_OFF;
	CHECK(resources.load_texture(missing.get_data()).is_null());
	CHECK(resources.load_audio(missing.get_data()).is_null());
	CHECK(Rect2(resources.get_bound_from_alpha(missing.get_data())) == Rect2(0, 0, 4, 4));
	CHECK(Vector2(resources.get_image_size(missing.get_data())) == Vector2(1, 1));
	CHECK(Vector2(resources.get_image_logical_size(missing.get_data())) == Vector2(1, 1));
	CHECK(resources.load_audio(missing.get_data(), true).is_null());
	ERR_PRINT_ON;
	auto image = Image::create_empty(4, 6, false, Image::FORMAT_RGBA8);
	REQUIRE(image->save_png(files.file("direct.png")) == OK);
	loaded = resources.load_texture(files.file("direct.png"), true);
	REQUIRE(loaded.is_valid());
	CHECK(loaded->get_size() == Vector2(4, 6));
}

TEST_CASE("[SceneTree][SPX] Animation metadata rejects malformed schemas before publishing clips") {
	SpxResMgr resources;
	const char *invalid[] = {
		"{", "[]", "{}", R"({"frames":{},"max_bitmap":1})",
		R"({"frames":[]})", R"({"frames":[],"max_bitmap":1})",
		R"({"frames":[3],"max_bitmap":1})",
		R"({"frames":[{}],"max_bitmap":1})",
		R"({"frames":[{"path":3,"bitmap":1}],"max_bitmap":1})",
		R"({"frames":[{"path":"","bitmap":1}],"max_bitmap":1})",
		R"({"frames":[{"path":"frame.png"}],"max_bitmap":1})",
		R"({"frames":[{"path":"frame.png","bitmap":1,"offset":{}}],"max_bitmap":1})",
		R"({"frames":[{"path":"frame.png","bitmap":1,"offset":[]}],"max_bitmap":1})",
		R"({"frames":[{"path":"frame.png","bitmap":1,"offset":["x",0]}],"max_bitmap":1})",
		R"({"frames":[{"path":"frame.png","bitmap":1,"offset":[0,"y"]}],"max_bitmap":1})"
	};
	ERR_PRINT_OFF;
	for (const char *json : invalid) {
		resources.create_animation("Validation", "bad", json, 12, false);
		CHECK_FALSE(resources.has_animation("Validation::bad"));
	}
	for (const char *value : { "0", "-1", "0.5", "2147483648", "1e999", "\"text\"" }) {
		for (const char *field : { "max_bitmap", "bitmap" }) {
			String json = String("{\"frames\":[{\"path\":\"frame.png\",\"bitmap\":") + (String(field) == "bitmap" ? value : "1") + "}],\"max_bitmap\":" + (String(field) == "max_bitmap" ? value : "1") + "}";
			resources.create_animation("Validation", "bad", json.utf8().get_data(), 12, false);
			CHECK_FALSE(resources.has_animation("Validation::bad"));
		}
	}
	for (const char *atlas : { R"({"frames":[{"x":0,"y":0,"w":1,"h":1}],"max_bitmap":1,"base_path":3})",
				 R"({"frames":[{"x":0,"y":0,"w":1,"h":1}],"max_bitmap":1,"base_path":""})",
				 R"({"frames":[{"x":"bad","y":0,"w":1,"h":1}],"max_bitmap":1,"base_path":"missing.png"})",
				 R"({"frames":[{"x":0,"y":0,"w":0,"h":1}],"max_bitmap":1,"base_path":"missing.png"})",
				 R"({"frames":[{"x":0,"y":0,"w":1,"h":1}],"max_bitmap":1,"base_path":"missing.png"})" }) {
		resources.create_animation("Validation", "atlas", atlas, 12, true);
		CHECK_FALSE(resources.has_animation("Validation::atlas"));
	}
	resources.create_animation("Validation", "negative_fps", "{}", -1, false);
	CHECK_FALSE(resources.has_animation("Validation::negative_fps"));
	resources.create_animation("Validation", "missing_svg", R"({"frames":[{"path":"missing.svg","bitmap":1}],"max_bitmap":1})", 12, false);
	CHECK_FALSE(resources.has_animation("Validation::missing_svg"));
	ERR_PRINT_ON;
	CHECK(resources.get_animation_frame_offset("missing", -1) == Vector2());
}

#ifdef MODULE_MINIMP3_ENABLED
TEST_CASE("[SceneTree][SPX] Direct MP3 audio loads playable data and invalidates its cache") {
	SpxResMgr resources;
	const String path = TestSpxData::get_path("audio/tone.mp3");
	const Ref<AudioStream> loaded = resources.load_audio(path, true);
	REQUIRE(loaded.is_valid());
	CHECK(loaded->get_length() > 0);
	CHECK(loaded->instantiate_playback().is_valid());
	CHECK(resources.load_audio(path, true) == loaded);
	resources.update_caches(Vector<String>{ path });
	const Ref<AudioStream> replacement = resources.load_audio(path, true);
	REQUIRE(replacement.is_valid());
	CHECK(replacement != loaded);
	CHECK(replacement->get_length() == loaded->get_length());
}
#endif

TEST_CASE("[SceneTree][SPX] Resource manager resolves project paths and invalidates direct audio caches") {
	TestSpx::TempDirectory files("spx_resource_paths");
	files.write("text.txt", "alpha\nbeta");
	REQUIRE(DirAccess::make_dir_recursive_absolute(files.file("assets")) == OK);
	TestSpx::EngineScene scene;
	Ref<AudioStreamWAV> wave;
	wave.instantiate();
	wave->set_format(AudioStreamWAV::FORMAT_16_BITS);
	wave->set_mix_rate(8000);
	PackedByteArray pcm;
	pcm.resize(160);
	pcm.fill(0);
	wave->set_data(pcm);
	REQUIRE(wave->save_to_wav(files.file("tone.wav")) == OK);
	auto *resources = scene.engine->get_res();
	resources->set_game_datas(files.path, Vector<String>());
	resources->set_load_mode(true);
	CHECK(resources->get_load_mode());
	CHECK(resources->has_file("text.txt"));
	CHECK(resources->has_file("../text.txt"));
	CHECK_FALSE(resources->has_file("missing.txt"));
	CHECK(TestSpx::take_string(resources->read_all_text("text.txt")) == "alpha\nbeta\n");
	CHECK(TestSpx::take_string(resources->read_all_text("missing.txt")).is_empty());
	Variant parsed = JSON::parse_string(TestSpx::take_string(resources->list_directories(".")));
	Array dirs = parsed;
	REQUIRE(dirs.size() == 1);
	CHECK(String(dirs[0]) == "assets");
	Ref<AudioStreamWAV> loaded = resources->load_audio("tone.wav");
	REQUIRE(loaded.is_valid());
	CHECK(loaded->get_mix_rate() == 8000);
	CHECK(loaded->get_data() == pcm);
	CHECK(resources->load_audio("tone.wav") == loaded);
	resources->update_caches(Vector<String>({ "tone.wav" }));
	CHECK(resources->load_audio("tone.wav") != loaded);
	CHECK(resources->load_audio("missing.wav").is_null());
	files.write("invalid.mp3", "invalid");
	files.write("unsupported.bin", "bytes");
	ERR_PRINT_OFF;
	CHECK(resources->load_audio("invalid.mp3").is_null());
	CHECK(resources->load_audio("unsupported.bin").is_null());
	ERR_PRINT_ON;
}

TEST_CASE("[SceneTree][SPX] Atlas animation metadata selects exact frame regions and rejects invalid rectangles") {
	TestSpx::TempDirectory files("spx_atlas_animation");
	files.write_png("atlas.png", Vector2i(16, 8), Color(1, 0, 0));
	TestSpx::EngineScene scene;
	auto *resources = scene.engine->get_res();
	Dictionary first;
	first["x"] = 0;
	first["y"] = 0;
	first["w"] = 8;
	first["h"] = 8;
	Dictionary second = first.duplicate();
	second["x"] = 8;
	Array offset;
	offset.push_back(2);
	offset.push_back(3);
	second["offset"] = offset;
	Dictionary payload;
	payload["base_path"] = files.file("atlas.png");
	payload["max_bitmap"] = 1;
	Array entries;
	entries.push_back(first);
	entries.push_back(second);
	payload["frames"] = entries;
	auto json = JSON::stringify(payload).utf8();
	resources->create_animation("Dragon", "walk", json.get_data(), 12, true);
	auto frames = resources->get_animation_frames("Dragon::walk");
	REQUIRE(frames.is_valid());
	CHECK(frames->get_frame_count("Dragon::walk") == 2);
	CHECK(frames->get_animation_speed("Dragon::walk") == 12);
	CHECK(resources->get_animation_frame_offset("Dragon::walk", -1) == Vector2());
	CHECK(resources->get_animation_frame_offset("Dragon::walk", 2) == Vector2());
	Ref<AtlasTexture> atlas = frames->get_frame_texture("Dragon::walk", 1);
	REQUIRE(atlas.is_valid());
	CHECK(atlas->get_region() == Rect2(8, 0, 8, 8));
	CHECK(atlas->get_atlas()->get_size() == Vector2(16, 8));
	resources->create_animation("Dragon", "walk", json.get_data(), 30, true);
	CHECK(resources->get_animation_frames("Dragon::walk") == frames);
	CHECK(frames->get_animation_speed("Dragon::walk") == 12);
	for (int invalid = 0; invalid < 3; ++invalid) {
		Dictionary bad = payload.duplicate(true);
		if (invalid == 0) {
			bad.erase("base_path");
		}
		if (invalid == 1) {
			Array entries = bad["frames"];
			Dictionary entry = entries[0];
			entry.erase("w");
		}
		if (invalid == 2) {
			Array entries = bad["frames"];
			Dictionary entry = entries[0];
			entry["h"] = 0;
		}
		auto text = JSON::stringify(bad).utf8();
		ERR_PRINT_OFF;
		resources->create_animation("Dragon", "invalid", text.get_data(), 12, true);
		ERR_PRINT_ON;
		CHECK_FALSE(resources->has_animation("Dragon::invalid"));
	}
}

} // namespace TestSpxResMgr

#endif // TEST_SPX_RES_MGR_H
