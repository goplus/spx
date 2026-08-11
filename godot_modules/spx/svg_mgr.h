#ifndef SVG_MANAGER_H
#define SVG_MANAGER_H

#include "core/math/vector2.h"
#include "core/templates/hash_map.h"
#include "scene/resources/image_texture.h"
#include "scene/resources/sprite_frames.h"

class SvgManager {
public:
	static SvgManager *get_singleton();
	static void destroy_singleton();

	SvgManager();
	~SvgManager();

private:
	// New simplified data structure
	HashMap<String, Ref<ImageTexture>> svg_image_cache; // "scale@image_path" -> ImageTexture
	HashMap<String, Ref<SpriteFrames>> svg_animation_cache; // "scale@animation_name" -> SpriteFrames
	HashMap<String, Vector2> svg_image_raw_size_cache;

	HashMap<String, bool> is_svg_animation_registry;
	static inline SvgManager *singleton = nullptr;

public:
	bool is_svg_file(const String &path) const;
	bool is_svg_animation(const String &base_anim_key);
	void mark_svg_animation(const String &base_anim_key, bool is_svg_animation);

	Ref<SpriteFrames> get_svg_animation(const String &base_anim_key, int scale);
	Ref<ImageTexture> get_svg_image(const String &image_path, int scale);
	Ref<ImageTexture> get_svg_image(const String &image_path, float scale);

	int calculate_svg_scale(Vector2 required_scale);
	int calculate_svg_scale(float required_scale);

	void reset(bool p_clear_project_caches = true);
	void update_caches(const Vector<String> &files);

private:
	String _make_image_key(const String &path, int scale); // "scale@path"
	String _make_animation_key(const String &name, int scale); // "scale@name"

	Ref<ImageTexture> _load_image(const String &path, int scale);
	Ref<SpriteFrames> _load_animation(const String &base_anim_key, int scale);
};

#endif // SVG_MANAGER_H
