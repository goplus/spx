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
#include "spx_image_texture.h"
#include "spx_platform_mgr.h"
#include "spx_svg_utils.h"
#include "spx_theme_font.h"

void SpxResMgr::on_awake() {
	SpxBaseMgr::on_awake();
	if (!initial_theme_fonts_saved) {
		spx_get_theme_fonts(initial_theme_default_font, initial_theme_fallback_font);
		initial_theme_fonts_saved = true;
	}
	is_load_direct = true;
}

void SpxResMgr::on_reset(int reset_code) {
	svg_cache.clear();
	animation_clips.clear();
	display_fonts.clear();
	display_default_font.unref();
	if (initial_theme_fonts_saved) {
		spx_set_theme_fonts(initial_theme_default_font, initial_theme_fallback_font);
	}
	SpxSvgUtils::reset_font_registry();
}

bool SpxResMgr::is_dynamic_anim_mode() const {
	return !animation_clips.is_empty();
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

static Ref<AudioStream> _load_mp3(const Ref<FileAccess> &p_file) {
#ifdef MODULE_MINIMP3_ENABLED
	uint64_t len = p_file->get_length();

	Vector<uint8_t> data;
	data.resize(len);
	uint8_t *w = data.ptrw();

	p_file->get_buffer(w, len);

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

Ref<AudioStream> SpxResMgr::_load_audio_direct(const String &p_path) {
	String path = _to_engine_path(p_path);
	const Ref<AudioStream> *cached = cached_audio.getptr(path);
	if (cached != nullptr) {
		return *cached;
	}
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::READ);
	if (file.is_null()) {
		print_line("Failed to open audio file: " + path);
		return Ref<AudioStreamWAV>();
	}
	Ref<AudioStream> res;
	const String ext = path.get_extension().to_lower();
	if (ext == "mp3") {
		res = _load_mp3(file);
	} else if (ext == "wav") {
		res = AudioStreamWAV::load_from_file(path, Dictionary());
	} else {
		print_error("unknown audio extension " + ext + " path=" + path);
	}
	cached_audio.insert(path, res);
	return res;
}

namespace {

bool is_animation_number(const Variant &p_value) {
	return (p_value.get_type() == Variant::INT ||
				   p_value.get_type() == Variant::FLOAT) &&
			Math::is_finite(double(p_value));
}

bool is_positive_animation_integer(const Variant &p_value) {
	return is_animation_number(p_value) && double(p_value) > 0 &&
			double(p_value) <= INT32_MAX &&
			double(p_value) == Math::floor(double(p_value));
}

} // namespace

bool SpxResMgr::_parse_anim_json(const String &src, bool p_is_atlas,
		AnimPayload &out) {
	JSON json;
	ERR_FAIL_COND_V_MSG(json.parse(src) != OK, false,
			"Invalid animation JSON: " + json.get_error_message());
	ERR_FAIL_COND_V_MSG(json.get_data().get_type() != Variant::DICTIONARY, false,
			"Animation JSON must be an object.");
	Dictionary dict = json.get_data();
	ERR_FAIL_COND_V_MSG(!dict.has("frames") ||
					dict["frames"].get_type() != Variant::ARRAY,
			false, "Animation frames must be an array.");
	ERR_FAIL_COND_V_MSG(!dict.has("max_bitmap") ||
					!is_positive_animation_integer(dict["max_bitmap"]),
			false,
			"Animation max_bitmap must be a positive integer.");
	out.frames = dict["frames"];
	out.max_bitmap = dict["max_bitmap"];
	ERR_FAIL_COND_V_MSG(out.frames.is_empty(), false,
			"Animation must contain at least one frame.");
	if (p_is_atlas) {
		ERR_FAIL_COND_V_MSG(!dict.has("base_path") ||
						dict["base_path"].get_type() != Variant::STRING ||
						String(dict["base_path"]).is_empty(),
				false, "Animation atlas path is missing.");
		out.base_path = dict["base_path"];
	}

	bool has_svg = false;
	bool has_bitmap = false;
	for (int i = 0; i < out.frames.size(); i++) {
		ERR_FAIL_COND_V_MSG(out.frames[i].get_type() != Variant::DICTIONARY, false,
				"Animation frame must be an object.");
		Dictionary frame = out.frames[i];
		if (p_is_atlas) {
			for (const char *field : { "x", "y", "w", "h" }) {
				ERR_FAIL_COND_V_MSG(!frame.has(field) ||
								!is_animation_number(frame[field]),
						false, "Invalid animation atlas rectangle.");
			}
			ERR_FAIL_COND_V_MSG(double(frame["w"]) <= 0 || double(frame["h"]) <= 0,
					false, "Animation atlas size must be positive.");
		} else {
			ERR_FAIL_COND_V_MSG(!frame.has("path") ||
							frame["path"].get_type() != Variant::STRING ||
							String(frame["path"]).is_empty(),
					false, "Animation frame path is missing.");
			ERR_FAIL_COND_V_MSG(!frame.has("bitmap") ||
							!is_positive_animation_integer(frame["bitmap"]),
					false,
					"Animation bitmap must be a positive integer.");
			const bool svg = SpxSvgCache::is_svg_path(frame["path"]);
			has_svg = has_svg || svg;
			has_bitmap = has_bitmap || !svg;
		}
		if (frame.has("offset")) {
			ERR_FAIL_COND_V_MSG(frame["offset"].get_type() != Variant::ARRAY, false,
					"Animation offset must be an array.");
			Array offset = frame["offset"];
			ERR_FAIL_COND_V_MSG(
					offset.size() != 2 || !is_animation_number(offset[0]) ||
							!is_animation_number(offset[1]),
					false, "Animation offset must contain two finite numbers.");
		}
	}
	ERR_FAIL_COND_V_MSG(has_svg && has_bitmap, false,
			"Animation cannot mix SVG and bitmap frames.");
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

bool SpxResMgr::_build_normal_frames(const String &anim_key,
		const AnimPayload &payload,
		SpxAnimationClip &r_clip) {
	for (int i = 0; i < payload.frames.size(); i++) {
		Dictionary frame = payload.frames[i];
		String path = frame["path"];
		const double bitmap = frame["bitmap"];
		Ref<Texture2D> texture;
		if (SpxSvgCache::is_svg_path(path)) {
			const int scale =
					SpxSvgCache::raster_scale(float(payload.max_bitmap / bitmap));
			texture = load_svg_texture(path, scale);
			r_clip.svg_frame_scales.push_back(scale);
			r_clip.is_svg = true;
		} else {
			texture = load_texture_checked(path);
		}
		ERR_FAIL_COND_V_MSG(texture.is_null(), false,
				"Cannot load animation texture: " + path);
		r_clip.frames->add_frame(anim_key, texture);
		r_clip.offsets.push_back(_read_offset(frame) / bitmap);
	}
	return true;
}

bool SpxResMgr::_build_atlas_frames(const String &anim_key,
		const AnimPayload &payload,
		SpxAnimationClip &r_clip) {
	Ref<Texture2D> atlas = load_texture_checked(payload.base_path);
	ERR_FAIL_COND_V_MSG(atlas.is_null(), false,
			"Cannot load animation atlas: " + payload.base_path);
	for (int i = 0; i < payload.frames.size(); i++) {
		Dictionary frame = payload.frames[i];
		Ref<AtlasTexture> texture;
		texture.instantiate();
		texture->set_atlas(atlas);
		texture->set_region(Rect2(double(frame["x"]), double(frame["y"]),
				double(frame["w"]), double(frame["h"])));
		r_clip.frames->add_frame(anim_key, texture);
		r_clip.offsets.push_back(_read_offset(frame));
	}
	return true;
}

static Error _load_image(String path, Ref<Image> p_image) {
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
	return err;
}

Ref<Texture2D> SpxResMgr::load_texture_checked(const String &p_path,
		GdBool p_direct) {
	if ((!is_load_direct && !p_direct) || SpxSvgCache::is_svg_path(p_path)) {
		return load_texture(p_path, p_direct);
	}
	const String path = _to_engine_path(p_path);
	const Ref<Texture2D> *cached = cached_texture.getptr(path);
	if (cached != nullptr) {
		return *cached;
	}
	Ref<Image> image;
	image.instantiate();
	if (ImageLoader::load_image(path, image) != OK) {
		return Ref<Texture2D>();
	}
	Ref<Texture2D> texture = SpxImageTexture::create_from_image(image);
	cached_texture.insert(path, texture);
	return texture;
}

Ref<Texture2D> SpxResMgr::_load_texture_direct(const String &p_path) {
	String path = _to_engine_path(p_path);
	// data in tmp dir would not keep in cache
	if (cached_texture.has(path)) {
		return cached_texture[path];
	}

	Ref<Image> image;
	image.instantiate();

	const Error error = _load_image(path, image);

	Ref<ImageTexture> texture = SpxImageTexture::create_from_image(image);
	if (error == OK) {
		cached_texture.insert(path, texture);
	}
	return texture;
}
Ref<Texture2D> SpxResMgr::_reload_texture(String path) {
	if (SpxSvgCache::is_svg_path(path)) {
		return svg_cache.reload_image(_to_engine_path(path));
	}
	path = _to_engine_path(path);
	Ref<Image> image;
	image.instantiate();
	const Ref<Texture2D> *cached = cached_texture.getptr(path);
	if (ImageLoader::load_image(path, image) != OK) {
		return cached != nullptr ? *cached : Ref<Texture2D>();
	}
	if (cached != nullptr) {
		Ref<ImageTexture> texture = *cached;
		SpxImageTexture::replace_image(texture, image);
		return texture;
	}
	Ref<Texture2D> texture = SpxImageTexture::create_from_image(image);
	cached_texture.insert(path, texture);
	return texture;
}

void SpxResMgr::reload_texture(GdString path) {
	auto path_str = SpxStr(path);
	_reload_texture(path_str);
}

Ref<Texture2D> SpxResMgr::load_texture(String path, GdBool direct) {
	if (SpxSvgCache::is_svg_path(path)) {
		return load_svg_texture(path, 1);
	}

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
}

void SpxResMgr::update_caches(const Vector<String> &files) {
	for (auto &file : files) {
		auto path = _to_engine_path(file);
		cached_texture.erase(path);
		cached_audio.erase(path);
		svg_cache.invalidate_image(path);
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

bool SpxResMgr::has_animation(const String &p_key) const {
	return animation_clips.has(p_key);
}

Ref<SpriteFrames> SpxResMgr::get_animation_frames(const String &p_key, int p_raster_scale) {
	const SpxAnimationClip *clip = animation_clips.getptr(p_key);
	if (clip == nullptr) {
		return Ref<SpriteFrames>();
	}
	return clip->is_svg ? svg_cache.load_animation(p_key, clip->frames, clip->svg_frame_scales, p_raster_scale) : clip->frames;
}

Ref<ImageTexture> SpxResMgr::load_svg_texture(const String &p_path, int p_raster_scale) {
	return svg_cache.load_image(_to_engine_path(p_path), p_raster_scale);
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
	const String key =
			get_anim_key_name(SpxStr(p_sprite_type), SpxStr(p_anim_name));
	if (animation_clips.has(key)) {
		return;
	}
	ERR_FAIL_COND_MSG(fps < 0, "Animation FPS must not be negative.");
	AnimPayload payload;
	if (!_parse_anim_json(SpxStr(p_json_ctx), is_atlas, payload)) {
		return;
	}
	SpxAnimationClip clip;
	clip.frames.instantiate();
	clip.frames->remove_animation("default");
	clip.frames->add_animation(key);
	clip.frames->set_animation_speed(key, fps);
	if (!(is_atlas ? _build_atlas_frames(key, payload, clip)
				   : _build_normal_frames(key, payload, clip))) {
		ERR_PRINT("Cannot create complete animation: " + key);
		return;
	}
	animation_clips.insert(key, clip);
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
	if (!Thread::is_main_thread()) {
		return SpxReturnStr("Project fonts must be applied on the engine main thread.");
	}
	String error;
	ProjectFonts::Request request;
	if (!ProjectFonts::decode_request(default_font_path, font_paths, font_families, preferences, request, error) ||
			!ProjectFonts::validate_request(request, error)) {
		return SpxReturnStr(error);
	}
	ProjectFonts::Prepared prepared;
	if (!ProjectFonts::prepare(request, *this, prepared, error)) {
		return SpxReturnStr(error);
	}
	_commit_project_fonts(std::move(prepared));

	return SpxReturnStr(String());
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
	svg_cache.clear();
}

void SpxResMgr::set_default_font(GdString font_path) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "Project fonts must be applied on the engine main thread.");
	Vector<uint8_t> data;
	Ref<FontFile> font;
	String error;
	if (!ProjectFonts::prepare_font(SpxStr(font_path), *this, data, font, error)) {
		ERR_PRINT(error);
		return;
	}

	// Each incremental call publishes immediately. Use apply_project_fonts to
	// replace a complete configuration atomically.
	SpxSvgUtils::reset_font_registry();
	SpxSvgUtils::set_default_font(data.ptrw(), data.size());
	display_fonts.clear();
	display_default_font = font;
	display_fonts.insert("default", font);
	spx_set_project_theme_font(font);
	svg_cache.clear();
}

void SpxResMgr::register_font_face(GdString font_path, GdString family) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "Project fonts must be applied on the engine main thread.");
	const String name = SpxStr(family);
	const String key = ProjectFonts::fold_family(name);
	ERR_FAIL_COND_MSG(name.is_empty() || key == "default", "Font family must be nonempty and must not use the reserved name default.");
	Vector<uint8_t> data;
	Ref<FontFile> font;
	String error;
	if (!ProjectFonts::prepare_font(SpxStr(font_path), *this, data, font, error)) {
		ERR_PRINT(error);
		return;
	}

	SpxSvgUtils::add_font_face(name, data.ptrw(), data.size());
	display_fonts.insert(key, font);
	svg_cache.clear();
}

void SpxResMgr::set_font_preferences(GdArray preferences) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "Project fonts must be applied on the engine main thread.");
	Vector<String> values;
	String error;
	HashSet<String> families;
	for (const KeyValue<String, Ref<FontFile>> &entry : display_fonts) {
		families.insert(entry.key);
	}
	if (!ProjectFonts::strings_from_array(preferences, "Font preferences", values, error) ||
			!ProjectFonts::validate_preferences(values, families, error)) {
		ERR_PRINT(error);
		return;
	}
	Ref<Font> theme_font = ProjectFonts::build_display_font_chain(display_fonts, values);
	SpxSvgUtils::set_font_preferences(values);
	spx_set_project_theme_font(theme_font);
	svg_cache.clear();
}

bool SpxResMgr::is_svg_animation(const String &p_anim_key) const {
	const SpxAnimationClip *clip = animation_clips.getptr(p_anim_key);
	return clip != nullptr && clip->is_svg;
}

Vector2 SpxResMgr::get_animation_frame_offset(String anim_key, int frame_index) {
	const SpxAnimationClip *clip = animation_clips.getptr(anim_key);
	if (clip != nullptr && frame_index >= 0 &&
			frame_index < clip->offsets.size()) {
		return clip->offsets[frame_index];
	}
	return Vector2();
}
