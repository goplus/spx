#include "svg_mgr.h"

#include "core/math/math_funcs.h"
#include "core/os/thread.h"

#include "spx.h"
#include "spx_engine.h"
#include "spx_image_loader_svg.h"
#include "spx_image_texture.h"
#include "spx_res_mgr.h"

SvgManager *SvgManager::get_singleton() {
	if (singleton == nullptr) {
		singleton = memnew(SvgManager);
	}
	return singleton;
}

void SvgManager::destroy_singleton() {
	if (singleton == nullptr) {
		return;
	}

	SvgManager *manager = singleton;
	singleton = nullptr;
	memdelete(manager);
}

SvgManager::SvgManager() = default;

SvgManager::~SvgManager() = default;

bool SvgManager::is_svg_file(const String &path) const {
	return path.to_lower().ends_with(".svg");
}

Ref<ImageTexture> SvgManager::get_svg_image(const String &image_path, int scale) {
	ERR_FAIL_COND_V_MSG(!Thread::is_main_thread(), Ref<ImageTexture>(), "SVG image caches may only be accessed on the main thread.");
	if (!is_svg_file(image_path)) {
		return Ref<ImageTexture>();
	}

	String path = resMgr->_to_engine_path(image_path);
	return _load_image(path, scale);
}

Ref<ImageTexture> SvgManager::reload_svg_image(const String &image_path) {
	ERR_FAIL_COND_V_MSG(
			!Thread::is_main_thread(), Ref<ImageTexture>(),
			"SVG image caches may only be accessed on the main thread.");
	const String path = resMgr->_to_engine_path(image_path);
	const String suffix = "@" + path;
	HashMap<int, Ref<Image>> prepared;
	prepared.insert(1, Ref<Image>());
	for (const KeyValue<String, Ref<ImageTexture>> &entry : svg_image_cache) {
		if (entry.key.ends_with(suffix)) {
			prepared.insert(entry.key.get_slice("@", 0).to_int(), Ref<Image>());
		}
	}
	for (KeyValue<int, Ref<Image>> &entry : prepared) {
		entry.value.instantiate();
		if (SpxImageLoaderSVG::load_image(
					path, entry.value, ImageFormatLoader::FLAG_NONE, entry.key) != OK) {
			const Ref<ImageTexture> *old =
					svg_image_cache.getptr(_make_image_key(path, 1));
			return old != nullptr ? *old : Ref<ImageTexture>();
		}
	}
	// Publish only after every cached scale is ready. Existing sprites and
	// animation frames keep their texture identity and see the same update.
	for (const KeyValue<int, Ref<Image>> &entry : prepared) {
		const String key = _make_image_key(path, entry.key);
		Ref<ImageTexture> *texture = svg_image_cache.getptr(key);
		if (texture != nullptr) {
			SpxImageTexture::replace_image(*texture, entry.value);
		} else {
			Ref<ImageTexture> created =
					SpxImageTexture::create_from_image(entry.value);
			created->set_path_cache(path);
			svg_image_cache.insert(key, created);
		}
	}
	return svg_image_cache[_make_image_key(path, 1)];
}

Ref<ImageTexture> SvgManager::get_svg_image(const String &image_path, float scale) {
	int int_scale = calculate_svg_scale(scale);
	return get_svg_image(image_path, int_scale);
}

Ref<SpriteFrames> SvgManager::get_svg_animation(const String &base_anim_key, int scale) {
	ERR_FAIL_COND_V_MSG(!Thread::is_main_thread(), Ref<SpriteFrames>(), "SVG animation caches may only be accessed on the main thread.");
	String key = _make_animation_key(base_anim_key, scale);

	if (svg_animation_cache.has(key)) {
		return svg_animation_cache[key];
	}

	return _load_animation(base_anim_key, scale);
}

void SvgManager::reset(bool p_clear_project_caches) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "SVG caches may only be reset on the main thread.");
	if (p_clear_project_caches) {
		svg_image_cache.clear();
	}
	svg_animation_cache.clear();
}

void SvgManager::update_caches(const Vector<String> &files) {
	ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "SVG caches may only be updated on the main thread.");
	bool invalidated_svg = false;
	for (const String &file : files) {
		const String path = resMgr->_to_engine_path(file);
		if (!is_svg_file(path)) {
			continue;
		}

		Vector<String> image_keys_to_erase;
		const String key_suffix = "@" + path;
		for (const KeyValue<String, Ref<ImageTexture>> &E : svg_image_cache) {
			if (E.key.ends_with(key_suffix)) {
				image_keys_to_erase.push_back(E.key);
			}
		}
		for (const String &key : image_keys_to_erase) {
			svg_image_cache.erase(key);
		}
		invalidated_svg = true;
	}

	if (invalidated_svg) {
		// Scaled animation frames may retain textures from any image scale.
		svg_animation_cache.clear();
	}
}

int SvgManager::calculate_svg_scale(Vector2 required_scale) {
	float scale = MAX(Math::abs(required_scale.x), Math::abs(required_scale.y));
	return calculate_svg_scale(scale);
}

int SvgManager::calculate_svg_scale(float required_scale) {
	float scale = Math::abs(required_scale);
	if (scale <= 1.0f) {
		return 1;
	}

	// Match Scratch's SVG MIP rule, but clamp to the largest SVG raster scale
	// we allow to cache. Larger render scales stay pinned at this ceiling.
	const int max_svg_scale = 1024;
	int target_scale = 1;
	while ((float)target_scale < scale && target_scale < max_svg_scale) {
		target_scale <<= 1;
	}
	return target_scale;
}

String SvgManager::_make_image_key(const String &path, int scale) {
	return String::num(scale) + "@" + path;
}

String SvgManager::_make_animation_key(const String &name, int scale) {
	return String::num(scale) + "@" + name;
}

Ref<SpriteFrames> SvgManager::_load_animation(const String &anim_name, int scale) {
	String key = _make_animation_key(anim_name, scale);

	if (svg_animation_cache.has(key)) {
		return svg_animation_cache[key];
	}

	Ref<SpriteFrames> frames;

	if (is_svg_file(anim_name)) {
		print_line("_load_animation: end with .svg anim_name: " + anim_name, "scale: " + String::num(scale));
		return frames;
	}

	SpxEngine *engine = SpxEngine::get_singleton();
	auto res_mgr = engine != nullptr ? engine->get_res() : nullptr;
	if (!res_mgr) {
		print_error("[SvgManager] Cannot access SpxResMgr");
		return Ref<SpriteFrames>();
	}

	auto existing_frames = res_mgr->get_anim_frames(anim_name);
	if (!existing_frames.is_valid() ||
			!existing_frames->has_animation(anim_name)) {
		print_error("[SvgManager] Animation not found: " + anim_name);
		return Ref<SpriteFrames>();
	}

	// If scale is 1, it's already created during res_mgr initialization, so
	// return the cached one directly.
	if (scale == 1) {
		svg_animation_cache[key] = existing_frames;
		return existing_frames;
	}

	if (!existing_frames->has_animation(anim_name)) {
		print_error("[SvgManager] Animation key not found: " + anim_name);
		return Ref<SpriteFrames>();
	}

	// Create new SpriteFrames, replacing SVG textures with scaled versions
	Ref<SpriteFrames> new_frames;
	new_frames.instantiate();
	new_frames->remove_animation("default");
	new_frames->add_animation(anim_name);

	new_frames->set_animation_loop(anim_name, existing_frames->get_animation_loop(anim_name));
	new_frames->set_animation_speed(anim_name, existing_frames->get_animation_speed(anim_name));

	int frame_count = existing_frames->get_frame_count(anim_name);
	for (int i = 0; i < frame_count; i++) {
		auto original_texture = existing_frames->get_frame_texture(anim_name, i);
		float duration = existing_frames->get_frame_duration(anim_name, i);

		ERR_FAIL_COND_V_MSG(original_texture.is_null(), Ref<SpriteFrames>(),
				"SVG animation frame texture is missing.");
		String texture_path = original_texture->get_path(); // engine path
		if (is_svg_file(texture_path)) {
			Ref<ImageTexture> scaled_texture = _load_image(texture_path, scale * res_mgr->get_animation_svg_frame_scale(anim_name, i));
			if (scaled_texture.is_null()) {
				return Ref<SpriteFrames>();
			}
			new_frames->add_frame(anim_name, scaled_texture, duration);
		} else {
			new_frames->add_frame(anim_name, original_texture, duration);
		}
	}

	svg_animation_cache[key] = new_frames;

	return new_frames;
}

Ref<ImageTexture> SvgManager::_load_image(const String &path /*engine path*/, int scale) {
	String key = _make_image_key(path, scale);

	if (svg_image_cache.has(key)) {
		return svg_image_cache[key];
	}
	Ref<Image> image;
	image.instantiate();
	Error err = SpxImageLoaderSVG::load_image(path, image, ImageFormatLoader::FLAG_NONE, (float)scale);
	if (err == OK) {
		Ref<ImageTexture> texture = SpxImageTexture::create_from_image(image);
		texture->set_path_cache(path); // cache raw path, not engine path

		svg_image_cache[key] = texture;
		return texture;
	}

	auto msg = "[SvgManager] Failed to load SVG image: " + path + " at scale " + String::num(scale);
	if (Spx::is_debug_mode()) {
		print_error(msg);
	} else {
		print_line(msg);
	}

	return Ref<ImageTexture>();
}
