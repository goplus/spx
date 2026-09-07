#ifndef SPX_IMAGE_TEXTURE_H
#define SPX_IMAGE_TEXTURE_H

#include "core/object/callable_method_pointer.h"
#include "scene/resources/image_texture.h"

// Keep uploaded pixels on the CPU for all Texture2D consumers, including
// collision queries, atlas extraction and tile copies. Ownership follows the
// texture; no global cache or resource metadata is needed.
class SpxImageTexture : public ImageTexture {
	mutable Ref<Image> cpu_image;

	void _invalidate_image() {
		cpu_image.unref();
	}

public:
	SpxImageTexture() {
		connect("changed", callable_mp(this, &SpxImageTexture::_invalidate_image));
	}

	static Ref<ImageTexture> create_from_image(const Ref<Image> &p_image) {
		ERR_FAIL_COND_V(p_image.is_null() || p_image->is_empty(), Ref<ImageTexture>());
		Ref<SpxImageTexture> texture;
		texture.instantiate();
		texture->set_image(p_image);
		texture->cpu_image = p_image->duplicate();
		return texture;
	}

	Ref<Image> get_image() const override {
		// Inherited set_image/update and resource reload emit changed. Read back
		// once after such an update, then retain that new snapshot as well.
		if (cpu_image.is_null()) {
			cpu_image = ImageTexture::get_image();
		}
		// Image::duplicate shares pixel storage until a caller writes to it.
		// Preserve get_image's independent, mutable result without copying pixels.
		return cpu_image.is_valid() ? Ref<Image>(cpu_image->duplicate()) : Ref<Image>();
	}
};

#endif // SPX_IMAGE_TEXTURE_H
