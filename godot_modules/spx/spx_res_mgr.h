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

#ifndef SPX_RES_MGR_H
#define SPX_RES_MGR_H

#include "gdextension_spx_ext.h"
#include "scene/resources/font.h"
#include "scene/resources/sprite_frames.h"
#include "servers/audio/audio_stream.h"
#include "spx_base_mgr.h"
#include "spx_svg_cache.h"

class Texture2D;

namespace ProjectFonts {
struct Prepared;
}

struct AnimPayload {
	String base_path;
	Array frames;
	int64_t max_bitmap = 1;
};

struct SpxAnimationClip {
	Ref<SpriteFrames> frames;
	Vector<Vector2> offsets;
	Vector<int> svg_frame_scales;
	bool is_svg = false;
};

class SpxResMgr : public SpxBaseMgr {
	SPXCLASS(SpxResMgr, SpxBaseMgr)

public:
	virtual ~SpxResMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor

private:
	HashMap<String, Ref<Texture2D>> cached_texture;
	HashMap<String, Ref<AudioStream>> cached_audio;
	HashMap<String, Ref<FontFile>> display_fonts;
	Ref<FontFile> display_default_font;
	Ref<Font> initial_theme_default_font;
	Ref<Font> initial_theme_fallback_font;
	bool initial_theme_fonts_saved = false;
	bool is_load_direct = true;
	String game_data_root = "res://";
	HashMap<String, SpxAnimationClip> animation_clips;
	SpxSvgCache svg_cache;

private:
	Ref<Texture2D> _load_texture_direct(const String &p_path);
	Ref<AudioStream> _load_audio_direct(const String &p_path);

	bool _parse_anim_json(const String &src, bool p_is_atlas, AnimPayload &out);
	Vector2 _read_offset(const Dictionary &d);
	bool _build_normal_frames(const String &anim_key, const AnimPayload &payload,
			SpxAnimationClip &r_clip);
	bool _build_atlas_frames(const String &anim_key, const AnimPayload &payload,
			SpxAnimationClip &r_clip);
	void _commit_project_fonts(ProjectFonts::Prepared &&p_prepared);

public:
	void on_awake() override;
	void on_reset(int reset_code) override;
	Ref<Texture2D> load_texture(String path, GdBool direct = false);
	Ref<Texture2D> load_texture_checked(const String &p_path,
			GdBool p_direct = false);
	Ref<AudioStream> load_audio(String path, GdBool direct = false);
	Ref<Texture2D> _reload_texture(String path);
	void set_game_datas(String path, Vector<String> files);
	void update_caches(const Vector<String> &files);
	bool has_animation(const String &p_key) const;
	Ref<SpriteFrames> get_animation_frames(const String &p_key, int p_raster_scale = 1);
	Ref<ImageTexture> load_svg_texture(const String &p_path, int p_raster_scale);
	String get_anim_key_name(const String &sprite_type_name, const String &anim_name);
	bool is_dynamic_anim_mode() const;
	bool is_svg_animation(const String &p_anim_key) const;
	Vector2 get_animation_frame_offset(String anim_key, int frame_index);
	String _to_engine_path(const String &p_path);

public:
	SPX_BIND void create_animation(GdString p_sprite_type, GdString p_anim_name, GdString p_json_ctx, GdInt fps, GdBool is_atlas);
	SPX_BIND void set_load_mode(GdBool is_direct_mode);
	SPX_BIND GdBool get_load_mode();
	SPX_BIND GdRect2 get_bound_from_alpha(GdString p_path);
	SPX_BIND GdVec2 get_image_size(GdString p_path);
	SPX_BIND GdString read_all_text(GdString p_path);
	SPX_BIND GdBool has_file(GdString p_path);
	SPX_BIND GdString list_directories(GdString p_path);
	SPX_BIND void reload_texture(GdString path);
	// Atomically applies a complete project font configuration. Returns an
	// allocated empty string on success, or an allocated diagnostic on failure.
	SPX_BIND GdString apply_project_fonts(GdString default_font_path, GdArray font_paths, GdArray font_families, GdArray preferences);
	SPX_BIND void set_default_font(GdString font_path);
	SPX_BIND void register_font_face(GdString font_path, GdString family);
	SPX_BIND void set_font_preferences(GdArray preferences);
};

#endif // SPX_RES_MGR_H
