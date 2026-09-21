#ifndef SPX_SVG_CACHE_H
#define SPX_SVG_CACHE_H

#include "core/math/vector2.h"
#include "core/templates/hash_map.h"
#include "scene/resources/image_texture.h"
#include "scene/resources/sprite_frames.h"

// Owned by SpxResMgr. Paths entering this cache are already engine paths.
class SpxSvgCache {
	HashMap<String, HashMap<int, Ref<ImageTexture>>> images;
	HashMap<String, HashMap<int, Ref<SpriteFrames>>> animations;

public:
	static bool is_svg_path(const String &p_path);
	static int raster_scale(Vector2 p_required_scale);
	static int raster_scale(float p_required_scale);

	Ref<ImageTexture> load_image(const String &p_path, int p_scale);
	Ref<ImageTexture> reload_image(const String &p_path);
	Ref<SpriteFrames> load_animation(const String &p_key, const Ref<SpriteFrames> &p_source, const Vector<int> &p_frame_scales, int p_scale);
	void invalidate_image(const String &p_path);
	void clear();
};

#endif // SPX_SVG_CACHE_H
