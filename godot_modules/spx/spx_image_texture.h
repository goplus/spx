#ifndef SPX_IMAGE_TEXTURE_H
#define SPX_IMAGE_TEXTURE_H

#include "core/object/callable_method_pointer.h"
#include "scene/resources/bit_map.h"
#include "scene/resources/image_texture.h"

#include <cstdint>

// Keep uploaded pixels on the CPU while the texture lives.
class SpxImageTexture : public ImageTexture {
	mutable Ref<Image> cpu_image;
	mutable Ref<BitMap> cpu_alpha_cache;
	bool uploading_image = false;
	uint64_t content_version = 0;

	void _invalidate_image() {
		content_version++;
		if (!uploading_image) {
			cpu_image.unref();
		}
		cpu_alpha_cache.unref();
	}

	void _replace_image(const Ref<Image> &p_image) {
		cpu_image = p_image->duplicate();
		uploading_image = true;
		ImageTexture::set_image(p_image);
		uploading_image = false;
	}

public:
	uint64_t image_version() const { return content_version; }

	SpxImageTexture() {
		connect("changed", callable_mp(this, &SpxImageTexture::_invalidate_image));
	}

	static Ref<ImageTexture> create_from_image(const Ref<Image> &p_image) {
		ERR_FAIL_COND_V(p_image.is_null() || p_image->is_empty(), Ref<ImageTexture>());
		Ref<SpxImageTexture> texture;
		texture.instantiate();
		texture->_replace_image(p_image);
		return texture;
	}

	static void replace_image(const Ref<ImageTexture> &p_texture, const Ref<Image> &p_image) {
		ERR_FAIL_COND(p_texture.is_null() || p_image.is_null() || p_image->is_empty());
		// No GDCLASS: RTTI safely distinguishes this private subclass.
		if (SpxImageTexture *texture = dynamic_cast<SpxImageTexture *>(p_texture.ptr())) {
			texture->_replace_image(p_image);
		} else {
			p_texture->set_image(p_image);
		}
	}

	bool is_pixel_opaque(int p_x, int p_y) const override {
		if (cpu_alpha_cache.is_null()) {
			Ref<Image> image = get_image();
			if (image.is_null()) {
				return true;
			}
			if (image->is_compressed()) {
				image->decompress();
			}
			cpu_alpha_cache.instantiate();
			cpu_alpha_cache->create_from_image_alpha(image);
		}
		const Size2i size = cpu_alpha_cache->get_size();
		if (size.x <= 0 || size.y <= 0 || get_width() <= 0 || get_height() <= 0) {
			return true;
		}
		return cpu_alpha_cache->get_bit(CLAMP(p_x * size.x / get_width(), 0, size.x - 1), CLAMP(p_y * size.y / get_height(), 0, size.y - 1));
	}

	Ref<Image> get_image() const override {
		// Fetch inherited mutations; SPX uploads already retain CPU pixels.
		if (cpu_image.is_null()) {
			cpu_image = ImageTexture::get_image();
		}
		// Copy-on-write keeps returned images independent.
		return cpu_image.is_valid() ? Ref<Image>(cpu_image->duplicate()) : Ref<Image>();
	}
};

#endif // SPX_IMAGE_TEXTURE_H
