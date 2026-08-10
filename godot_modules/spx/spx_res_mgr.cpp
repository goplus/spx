/**************************************************************************/
/*  spx_platform_mgr.cpp                                                     */
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

#include "spx_res_mgr.h"

#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/io/image.h"
#include "core/io/image_loader.h"
#include "core/io/json.h"
#include "core/math/vector2.h"
#include "core/os/thread.h"
#include "modules/minimp3/audio_stream_mp3.h"
#include "modules/modules_enabled.gen.h"
#include "scene/2d/audio_stream_player_2d.h"
#include "scene/main/window.h"
#include "scene/resources/atlas_texture.h"
#include "scene/resources/audio_stream_wav.h"
#include "scene/resources/image_texture.h"
#include "scene/resources/sprite_frames.h"

#include "project_font_transaction.h"
#include "spx_engine.h"
#include "spx_image_loader_svg.h"
#include "spx_platform_mgr.h"
#include "spx_svg_utils.h"
#include "spx_theme_font.h"
#include "svg_mgr.h"

void SpxResMgr::on_awake() {
	SpxBaseMgr::on_awake();
	if (!initial_theme_fonts_saved) {
		spx_get_theme_fonts(initial_theme_default_font, initial_theme_fallback_font);
		initial_theme_fonts_saved = true;
	}
	is_load_direct = true;
	anim_frames.instantiate();
}

void SpxResMgr::on_reset(int reset_code) {
	animation_frame_offsets.clear();
	anim_frames->clear_all();
	display_fonts.clear();
	display_default_font.unref();
	if (initial_theme_fonts_saved) {
		spx_set_theme_fonts(initial_theme_default_font, initial_theme_fallback_font);
	}
	SpxSvgUtils::reset_font_registry();
}

bool SpxResMgr::is_dynamic_anim_mode() const {
	return is_dynamic_anim;
}

String SpxResMgr::_to_engine_path(const String &p_path) {
	String path = p_path;
	SpxEngine *engine = SpxEngine::get_singleton();
	SpxPlatformMgr *platform = engine != nullptr ? engine->get_platform() : nullptr;
	if (game_data_root != "res://" &&
			(platform == nullptr || !path.begins_with(platform->_get_persistant_data_dir()))) {
		if (path.begins_with("../")) {
			path = path.substr(3, -1);
		}
		path = game_data_root + "/" + path;
	}
	return path;
}

Ref<AudioStreamWAV> SpxResMgr::_load_wav(const String &path) {
	return AudioStreamWAV::load_from_file(path, Dictionary());
}

static Ref<AudioStream> _import_mp3(const String &p_path) {
#ifdef MODULE_MINIMP3_ENABLED
	Ref<FileAccess> f = FileAccess::open(p_path, FileAccess::READ);
	ERR_FAIL_COND_V(f.is_null(), Ref<AudioStreamMP3>());

	uint64_t len = f->get_length();

	Vector<uint8_t> data;
	data.resize(len);
	uint8_t *w = data.ptrw();

	f->get_buffer(w, len);

	Ref<AudioStreamMP3> mp3_stream;
	mp3_stream.instantiate();

	mp3_stream->set_data(data);
	ERR_FAIL_COND_V(!mp3_stream->get_data().size(), Ref<AudioStreamMP3>());
	return mp3_stream;
#else
	Ref<AudioStream> mp3_stream;
	return mp3_stream;
#endif
}

Ref<AudioStream> SpxResMgr::_load_mp3(const String &path) {
	return _import_mp3(path);
}

Ref<AudioStream> SpxResMgr::_load_audio_direct(const String &p_path) {
	String path = _to_engine_path(p_path);
	if (cached_audio.has(path)) {
		return cached_audio[path];
	}
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	if (file.is_null()) {
		print_line("Failed to open audio file: " + path);
		return Ref<AudioStreamWAV>();
	}
	Ref<AudioStream> res;
	const String ext = path.get_extension().to_lower();
	if (ext == "mp3") {
		res = _load_mp3(path);
	} else if (ext == "wav") {
		res = _load_wav(path);
	} else {
		print_error("unknown audio extension " + ext + " path=" + path);
	}
	cached_audio.insert(path, res);
	return res;
}

bool SpxResMgr::_parse_anim_json(const String &src, AnimPayload &out) {
	JSON json;
	Error error = json.parse(src);
	if (error != OK) {
		print_error("Failed to parse JSON: " + json.get_error_message());
		return false;
	}

	Dictionary dict = json.get_data();

	if (dict.has("base_path")) {
		out.base_path = dict["base_path"];
	}

	if (!dict.has("frames")) {
		print_error("JSON missing 'frames'");
		return false;
	}

	if (!dict.has("max_bitmap")) {
		print_error("JSON missing 'max_bitmap'");
		return false;
	}

	out.frames = dict["frames"];
	out.max_bitmap = dict["max_bitmap"];
	return true;
}

Vector2 SpxResMgr::_read_offset(const Dictionary &d) {
	if (!d.has("offset")) {
		return Vector2(0, 0);
	}

	Array off = d["offset"];
	if (off.size() < 2) {
		return Vector2(0, 0);
	}

	return Vector2(
			double(off[0]),
			double(off[1]));
}

void SpxResMgr::_build_normal_frames(
		const String &p_sprite_type,
		const String &anim_key,
		const AnimPayload &payload,
		Vector<Vector2> &out_offsets) {
	int svg_count = 0;
	for (int i = 0; i < payload.frames.size(); i++) {
		Dictionary f = payload.frames[i];

		String path = f["path"];
		int64_t bitmap = f["bitmap"];
		Vector2 offset = _read_offset(f) / float(bitmap);

		Ref<Texture2D> final_tex;
		if (svgMgr->is_svg_file(path)) {
			float scale = float(payload.max_bitmap) / float(bitmap);
			final_tex = svgMgr->get_svg_image(path, scale);
			svg_count++;
		} else {
			final_tex = load_texture(path);
		}

		if (!final_tex.is_valid()) {
			print_error("cannot load texture: " + path);
			continue;
		}

		anim_frames->add_frame(anim_key, final_tex);
		out_offsets.push_back(offset);
	}

	if (svg_count > 0 && svg_count != payload.frames.size()) {
		print_error(vformat(
				"[SpxResMgr::create_animation][ERR_SVG_FRAME_MISMATCH] "
				"Sprite='%s', Anim='%s', SVG_Count=%d, Frame_Count=%d — counts must match for SVG animations.",
				p_sprite_type,
				anim_key,
				svg_count,
				payload.frames.size()));
		return;
	}

	svgMgr->mark_svg_animation(anim_key, svg_count > 0);
}

void SpxResMgr::_build_atlas_frames(const String &anim_key, const AnimPayload &payload, Vector<Vector2> &out_offsets) {
	Ref<Texture2D> atlas = load_texture(payload.base_path);
	if (!atlas.is_valid()) {
		print_error("cannot load atlas: " + payload.base_path);
		return;
	}

	for (int i = 0; i < payload.frames.size(); i++) {
		Dictionary f = payload.frames[i];

		int64_t x = f["x"];
		int64_t y = f["y"];
		int64_t w = f["w"];
		int64_t h = f["h"];
		Vector2 offset = _read_offset(f);

		Ref<AtlasTexture> tex;
		tex.instantiate();
		tex->set_atlas(atlas);
		tex->set_region(Rect2(x, y, w, h));

		anim_frames->add_frame(anim_key, tex);
		out_offsets.push_back(offset);
	}
}

static void _load_image(String path, Ref<Image> p_image) {
	const bool is_svg = path.get_extension().nocasecmp_to("svg") == 0;
	Error err = is_svg ? SpxImageLoaderSVG::load_image(path, p_image) : ImageLoader::load_image(path, p_image);
	if (err != OK) {
		// Failed to load image , so give a pink image
		// pink color
		PackedByteArray data;
		for (int i = 0; i < 4 * 4; i++) {
			data.append(255); // R
			data.append(0); // G
			data.append(255); // B
			data.append(128); // A
		}
		p_image->set_data(4, 4, false, Image::FORMAT_RGBA8, data);
	}
}

Ref<Texture2D> SpxResMgr::_load_texture_direct(const String &p_path) {
	String path = _to_engine_path(p_path);
	// data in tmp dir would not keep in cache
	if (cached_texture.has(path)) {
		return cached_texture[path];
	}

	Ref<Image> image;
	image.instantiate();

	_load_image(path, image);

	Ref<ImageTexture> texture = ImageTexture::create_from_image(image);
	cached_texture.insert(path, texture);
	return texture;
}
Ref<Texture2D> SpxResMgr::_reload_texture(String path) {
	if (cached_texture.has(path)) {
		auto tex = (Ref<ImageTexture>)cached_texture[path];
		Ref<Image> image;
		image.instantiate();
		_load_image(path, image);
		tex->set_image(image);
		cached_texture.erase(path);
		cached_texture.insert(path, tex);
		return tex;
	} else {
		return _load_texture_direct(path);
	}
}

void SpxResMgr::reload_texture(GdString path) {
	auto path_str = SpxStr(path);
	_reload_texture(path_str);
}

Ref<Texture2D> SpxResMgr::load_texture(String path, GdBool direct) {
	// If SVG file, use SVG manager
	if (svgMgr->is_svg_file(path)) {
		return svgMgr->get_svg_image(path, 1); // Default 1x scale
	}

	// For non-SVG files, use original logic
	if (!is_load_direct && !direct) {
		Ref<Resource> res = ResourceLoader::load(path);
		if (res.is_null()) {
			print_line("load texture failed !", path);
			return Ref<Texture2D>();
		}
		return res;
	} else {
		return _load_texture_direct(path);
	}
}

void SpxResMgr::set_game_datas(String path, Vector<String> files) {
	print_line("SpxResMgr::set_game_datas", path);
	game_data_root = path;
	platformMgr->_set_persistant_data_dir(path);
	update_caches(files);
	svgMgr->update_caches(files);
}

void SpxResMgr::update_caches(const Vector<String> &files) {
	if (cached_texture.is_empty() && cached_audio.is_empty()) {
		return;
	}
	for (auto &file : files) {
		auto path = _to_engine_path(file);
		cached_texture.erase(path);
		cached_audio.erase(path);
	}
}

Ref<AudioStream> SpxResMgr::load_audio(String path, GdBool direct) {
	if (!is_load_direct && !direct) {
		Ref<Resource> res = ResourceLoader::load(path);
		if (res.is_null()) {
			print_line("load audio failed !", path);
			return Ref<AudioStream>();
		}
		return res;
	}
	return _load_audio_direct(path);
}

Ref<SpriteFrames> SpxResMgr::get_anim_frames(const String &anim_name) {
	return anim_frames;
}

String SpxResMgr::get_anim_key_name(const String &sprite_type_name, const String &anim_name) {
	return sprite_type_name + "::" + anim_name;
}

void SpxResMgr::create_animation(
		GdString p_sprite_type,
		GdString p_anim_name,
		GdString p_json_ctx,
		GdInt fps,
		GdBool is_atlas) {
	is_dynamic_anim = true;
	String sprite = SpxStr(p_sprite_type);
	String clip = SpxStr(p_anim_name);
	String ctx = SpxStr(p_json_ctx);

	String key = get_anim_key_name(sprite, clip);

	if (anim_frames->has_animation(key)) {
		return;
	}

	AnimPayload payload;
	if (!_parse_anim_json(ctx, payload)) {
		print_error("animation JSON parse failed");
		return;
	}

	anim_frames->add_animation(key);
	anim_frames->set_animation_speed(key, fps);

	Vector<Vector2> offsets;

	if (is_atlas) {
		_build_atlas_frames(key, payload, offsets);
	} else {
		_build_normal_frames(sprite, key, payload, offsets);
	}

	animation_frame_offsets[key] = offsets;
}

void SpxResMgr::set_load_mode(GdBool is_direct_mode) {
	is_load_direct = is_direct_mode;
}

GdBool SpxResMgr::get_load_mode() {
	return is_load_direct;
}

GdRect2 SpxResMgr::get_bound_from_alpha(GdString path) {
	auto path_str = SpxStr(path);

	Ref<Texture2D> image = load_texture(path_str);
	if (image.is_null()) {
		print_line("Load texture failed ", path_str);
		return GdRect2(Vector2(0, 0), Size2(4, 4));
	}
	int width = image->get_width();
	int height = image->get_height();

	int min_x = width;
	int min_y = height;
	int max_x = 0;
	int max_y = 0;
	bool has_alpha = false;
	for (int y = 0; y < height; ++y) {
		for (int x = 0; x < width; ++x) {
			if (image->is_pixel_opaque(x, y)) { // Check if the pixel is not fully transparent
				has_alpha = true;
				if (x < min_x) {
					min_x = x;
				}
				if (y < min_y) {
					min_y = y;
				}
				if (x > max_x) {
					max_x = x;
				}
				if (y > max_y) {
					max_y = y;
				}
			}
		}
	}

	if (!has_alpha) {
		return Rect2();
	}

	return Rect2(Vector2(min_x, min_y), Vector2(max_x - min_x + 1, max_y - min_y + 1));
}

GdVec2 SpxResMgr::get_image_size(GdString path) {
	auto path_str = SpxStr(path);
	Ref<Texture2D> value = load_texture(path_str);
	if (value.is_valid()) {
		return value->get_size();
	}
	print_error("can not find a texture: " + path_str);
	return GdVec2(1, 1);
}

void SpxResMgr::free_str(GdString str_ptr) {
	free_return_cstr(str_ptr);
}
GdString SpxResMgr::read_all_text(GdString p_path) {
	auto path = SpxStr(p_path);
	path = _to_engine_path(path);
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	String value = "";
	if (file.is_null()) {
		print_line("Unable to open file.", path);
	} else {
		String file_content;
		while (!file->eof_reached()) {
			String line = file->get_line();
			file_content += line + "\n";
		}
		value = file_content;
		file->close();
	}
	return SpxReturnStr(value);
}

GdBool SpxResMgr::has_file(GdString p_path) {
	auto path = SpxStr(p_path);
	path = _to_engine_path(path);
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	return !file.is_null();
}

GdString SpxResMgr::list_directories(GdString p_path) {
	String path = SpxStr(p_path);
	path = _to_engine_path(path);
	PackedStringArray directories = DirAccess::get_directories_at(path);
	return SpxReturnStr(JSON::stringify(directories));
}

GdString SpxResMgr::apply_project_fonts(GdString default_font_path, GdArray font_paths, GdArray font_families, GdArray preferences) {
	auto fail = [](const String &p_error) -> GdString {
		return SpxReturnStr(p_error);
	};
	if (!Thread::is_main_thread()) {
		return fail("Project fonts must be applied on the engine main thread.");
	}
	String error;
	ProjectFonts::Request request;
	if (!ProjectFonts::decode_request(default_font_path, font_paths, font_families, preferences, request, error) ||
			!ProjectFonts::validate_request(request, error)) {
		return fail(error);
	}
	ProjectFonts::Prepared prepared;
	if (!ProjectFonts::prepare(request, *this, prepared, error)) {
		return fail(error);
	}
	_commit_project_fonts(std::move(prepared));

	return fail(String());
}

void SpxResMgr::_commit_project_fonts(ProjectFonts::Prepared &&p_prepared) {
	// Preparation performs every fallible operation. Publish the complete
	// generation to all consumers before invalidating previously rendered SVGs.
	Vector<SpxSvgProjectFontFace> svg_faces;
	svg_faces.resize(p_prepared.faces.size());
	for (int i = 0; i < p_prepared.faces.size(); i++) {
		svg_faces.write[i].family = p_prepared.faces[i].spec.family;
		svg_faces.write[i].data = p_prepared.faces[i].data;
	}
	SpxSvgUtils::apply_font_registry(p_prepared.default_data, svg_faces, p_prepared.preferences);
	display_fonts = std::move(p_prepared.display_fonts);
	display_default_font = std::move(p_prepared.default_font);
	spx_set_project_theme_font(p_prepared.theme_font);
	SvgManager::get_singleton()->reset(true);
}

void SpxResMgr::set_default_font(GdString font_path) {
	String path = SpxStr(font_path);
	if (path.is_empty()) {
		ERR_PRINT("Can not open empty font path.");
		return;
	}
	Vector<uint8_t> font_data;
	String engine_path = _to_engine_path(path);
	if (!ProjectFonts::load_font_data(path, engine_path, font_data)) {
		return;
	}
	Ref<FontFile> font = ProjectFonts::create_display_font(font_data);
	if (font.is_null() || font->get_face_count() <= 0) {
		ERR_PRINT("Default font file is not a supported font: " + path);
		return;
	}
	if (!SpxSvgUtils::is_font_data_valid(font_data)) {
		ERR_PRINT("Default font file is not supported by LunaSVG: " + path);
		return;
	}

	// update svg
	// Setting the default font begins a complete project font transaction.
	// Drop any faces left by an earlier bootstrap before registering this one.
	SpxSvgUtils::reset_font_registry();
	SpxSvgUtils::set_default_font(font_data.ptrw(), (int)font_data.size());

	// Start a new project font configuration. Named faces are registered after
	// this call and set_font_preferences commits the ordered fallback chain.
	display_fonts.clear();
	display_default_font = font;
	display_fonts.insert("default", display_default_font);
	spx_set_project_theme_font(display_default_font);
}

void SpxResMgr::register_font_face(GdString font_path, GdString family) {
	String path = SpxStr(font_path);
	if (path.is_empty()) {
		ERR_PRINT("Can not open empty font path.");
		return;
	}

	String svg_family = SpxStr(family);
	if (svg_family.is_empty()) {
		ERR_PRINT("Can not register empty SVG font family.");
		return;
	}

	Vector<uint8_t> font_data;
	String engine_path = _to_engine_path(path);
	if (!ProjectFonts::load_font_data(path, engine_path, font_data)) {
		return;
	}
	Ref<FontFile> font = ProjectFonts::create_display_font(font_data);
	if (font.is_null() || font->get_face_count() <= 0) {
		ERR_PRINT("Project font file is not a supported font: " + path);
		return;
	}
	if (!SpxSvgUtils::is_font_data_valid(font_data)) {
		ERR_PRINT("Project font file is not supported by LunaSVG: " + path);
		return;
	}

	SpxSvgUtils::add_font_face(svg_family, font_data.ptrw(), (int)font_data.size());
	display_fonts.insert(ProjectFonts::fold_family(svg_family), font);
}

void SpxResMgr::set_font_preferences(GdArray preferences) {
	Vector<String> values = ProjectFonts::preferences_from_array(preferences);
	SpxSvgUtils::set_font_preferences(values);
	spx_set_project_theme_font(ProjectFonts::build_display_font_chain(display_fonts, values));
}

Vector2 SpxResMgr::get_animation_frame_offset(String anim_key, int frame_index) {
	if (animation_frame_offsets.has(anim_key)) {
		const Vector<Vector2> &offsets = animation_frame_offsets[anim_key];
		if (frame_index >= 0 && frame_index < offsets.size()) {
			return offsets[frame_index];
		}
	}
	return Vector2(0, 0);
}
