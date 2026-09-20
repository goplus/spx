/**************************************************************************/
/*  test_spx_sprite_render_util.h                                         */
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

#ifndef TEST_SPX_SPRITE_RENDER_UTIL_H
#define TEST_SPX_SPRITE_RENDER_UTIL_H

#include "../spx_camera_mgr.h"
#include "../spx_engine.h"
#include "../spx_res_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "../spx_sprite_render_util.h"
#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "core/io/json.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "scene/resources/image_texture.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"

namespace TestSpxSpriteRenderUtil {

TEST_CASE("[SPX] Render root preserves single-image render offset") {
	const Vector2 render_offset(12.0, -8.0);
	const Vector2 base_offset(1.0, 2.0);
	const Vector2 frame_offset(3.0, 4.0);
	const Vector2 render_scale(2.0, 0.5);

	const Vector2 anim_offset = spx_compute_anim_offset(true, base_offset, render_offset, frame_offset, render_scale);
	CHECK(render_offset + anim_offset == render_offset + base_offset + frame_offset * render_scale);
}

TEST_CASE("[SPX] Render root cancels costume offset for animation frames") {
	const Vector2 render_offset(12.0, -8.0);
	const Vector2 base_offset(1.0, 2.0);
	const Vector2 frame_offset(3.0, 4.0);
	const Vector2 render_scale(2.0, 0.5);

	const Vector2 anim_offset = spx_compute_anim_offset(false, base_offset, render_offset, frame_offset, render_scale);
	CHECK(render_offset + anim_offset == base_offset + frame_offset * render_scale);
}

TEST_CASE("[SceneTree][SPX] Animation frame UV rect is normalized from its atlas") {
	const Ref<Image> image = memnew(Image(200, 100, false, Image::FORMAT_RGBA8));
	const Ref<ImageTexture> texture = ImageTexture::create_from_image(image);
	Ref<AtlasTexture> atlas_frame;
	atlas_frame.instantiate();
	atlas_frame->set_atlas(texture);
	atlas_frame->set_region(Rect2(20, 10, 40, 30));

	Ref<SpriteFrames> frames;
	frames.instantiate();
	frames->add_frame("default", atlas_frame);

	const Rect2 uv_rect = spx_get_animation_frame_uv_rect(frames, "default", 0);
	CHECK(uv_rect.position.x == doctest::Approx(0.1));
	CHECK(uv_rect.position.y == doctest::Approx(0.1));
	CHECK(uv_rect.size.x == doctest::Approx(0.2));
	CHECK(uv_rect.size.y == doctest::Approx(0.3));
}

TEST_CASE("[SPX] Animation frame UV rect falls back for non-atlas frames") {
	Ref<SpriteFrames> frames;
	frames.instantiate();
	Ref<Texture2D> plain_texture;
	plain_texture.instantiate();
	frames->add_frame("default", plain_texture);

	const Rect2 default_uv(0, 0, 1, 1);
	CHECK(spx_get_animation_frame_uv_rect(frames, "missing", 0) == default_uv);
	CHECK(spx_get_animation_frame_uv_rect(frames, "default", 0) == default_uv);
}

// Only the managers used by visual operations are awakened. This exercises
// real SpxSprite entry points without starting the Go runtime or recorder.
struct VisualFixture {
	Node2D *root = nullptr;
	SpxResMgr *resources = nullptr;
	SpxSpriteMgr *sprites = nullptr;
	String png_path = TestUtils::get_temp_path("spx_visual.png");
	String svg_path = TestUtils::get_temp_path("spx_visual.svg");
	String retry_path = TestUtils::get_temp_path("spx_visual_retry.svg");

	VisualFixture() {
		SpxEngine::register_callbacks(nullptr);
		SpxEngine *engine = SpxEngine::get_singleton();
		root = memnew(Node2D);
		SceneTree::get_singleton()->get_root()->add_child(root);
		engine->set_root_node(SceneTree::get_singleton(), root);
		engine->get_camera()->on_awake();
		resources = engine->get_res();
		resources->on_awake();
		sprites = engine->get_sprite();
		sprites->on_awake();
		Ref<Image> image = Image::create_empty(8, 6, false, Image::FORMAT_RGBA8);
		image->fill(Color(1, 0, 0));
		CHECK(image->save_png(png_path) == OK);
		write_svg(svg_path);
		write_text(retry_path, "invalid SVG");
	}

	~VisualFixture() {
		SpxEngine::shutdown();
		memdelete(root);
		DirAccess::remove_absolute(png_path);
		DirAccess::remove_absolute(svg_path);
		DirAccess::remove_absolute(retry_path);
	}

	static void write_text(const String &p_path, const String &p_text) {
		Ref<FileAccess> file = FileAccess::open(p_path, FileAccess::WRITE);
		REQUIRE(file.is_valid());
		file->store_string(p_text);
	}

	static void write_svg(const String &p_path) {
		write_text(
				p_path,
				"<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"8\" "
				"height=\"6\"><rect width=\"8\" height=\"6\" fill=\"red\"/></svg>");
	}

	void create_clip(const char *p_name, const String &p_first,
			const String &p_second, int p_bitmap = 1) {
		Array frames;
		for (const String &path : { p_first, p_second }) {
			Dictionary frame;
			frame["path"] = path;
			frame["bitmap"] = p_bitmap;
			Array offset;
			offset.push_back(4);
			offset.push_back(6);
			frame["offset"] = offset;
			frames.push_back(frame);
		}
		Dictionary payload;
		payload["frames"] = frames;
		payload["max_bitmap"] = 1;
		const CharString json = JSON::stringify(payload).utf8();
		resources->create_animation("ReviewSprite", p_name, json.get_data(), 10,
				false);
	}

	SpxSprite *create_sprite() {
		SpxSprite *sprite =
				sprites->get_sprite(sprites->create_bare_sprite(Vector2()));
		sprite->set_type_name("ReviewSprite");
		return sprite;
	}
};

TEST_CASE("[SceneTree][SPX] Animation players isolate loop metadata and share "
		  "textures") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	VisualFixture fixture;
	fixture.create_clip("walk", fixture.png_path, fixture.png_path);
	const Ref<SpriteFrames> shared =
			fixture.resources->get_anim_frames("ReviewSprite::walk");
	REQUIRE(shared.is_valid());
	SpxSprite *first = fixture.create_sprite();
	SpxSprite *second = fixture.create_sprite();
	first->play_anim("walk", 1, false, false);
	second->play_anim("walk", 1, true, false);
	const Ref<SpriteFrames> first_frames =
			first->get_anim2d()->get_sprite_frames();
	const Ref<SpriteFrames> second_frames =
			second->get_anim2d()->get_sprite_frames();
	CHECK(first_frames != second_frames);
	CHECK(first_frames != shared);
	CHECK_FALSE(first_frames->get_animation_loop("ReviewSprite::walk"));
	CHECK(second_frames->get_animation_loop("ReviewSprite::walk"));
	CHECK(shared->get_animation_loop("ReviewSprite::walk"));
	CHECK(first_frames->get_frame_texture("ReviewSprite::walk", 0) ==
			shared->get_frame_texture("ReviewSprite::walk", 0));
	CHECK(second_frames->get_frame_texture("ReviewSprite::walk", 0) ==
			shared->get_frame_texture("ReviewSprite::walk", 0));
	CHECK_EQ(first_frames->get_animation_names().size(), 1);
	first->get_anim2d()->set_frame_and_progress(1, 0.4);
	first->play_anim("walk", 2, false, false);
	CHECK(first->get_anim2d()->get_sprite_frames() == first_frames);
	CHECK_EQ(first->get_anim_frame(), 1);
	CHECK(first->get_anim2d()->get_frame_progress() == doctest::Approx(0.4));
	first->play_backwards_anim("walk");
	CHECK_FALSE(first_frames->get_animation_loop("ReviewSprite::walk"));
	CHECK(second_frames->get_animation_loop("ReviewSprite::walk"));
}

TEST_CASE("[SceneTree][SPX] Animation creation publishes complete clips and "
		  "retries failures") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	VisualFixture fixture;
	ERR_PRINT_OFF
	fixture.create_clip("retry", fixture.svg_path, fixture.retry_path);
	ERR_PRINT_ON
	CHECK(fixture.resources->get_anim_frames("ReviewSprite::retry").is_null());
	CHECK_FALSE(fixture.resources->is_dynamic_anim_mode());
	CHECK_FALSE(fixture.resources->is_svg_animation("ReviewSprite::retry"));
	VisualFixture::write_svg(fixture.retry_path);
	fixture.create_clip("retry", fixture.svg_path, fixture.retry_path);
	const Ref<SpriteFrames> frames =
			fixture.resources->get_anim_frames("ReviewSprite::retry");
	REQUIRE(frames.is_valid());
	CHECK_EQ(frames->get_frame_count("ReviewSprite::retry"), 2);
	CHECK(fixture.resources->is_svg_animation("ReviewSprite::retry"));
	CHECK(fixture.resources->get_animation_frame_offset("ReviewSprite::retry",
				  1) == Vector2(4, 6));
	CHECK(fixture.resources->get_anim_frames("ReviewSprite::missing").is_null());

	ERR_PRINT_OFF
	fixture.create_clip("mixed", fixture.svg_path, fixture.png_path);
	fixture.create_clip("zero", fixture.png_path, fixture.png_path, 0);
	ERR_PRINT_ON
	CHECK(fixture.resources->get_anim_frames("ReviewSprite::mixed").is_null());
	CHECK(fixture.resources->get_anim_frames("ReviewSprite::zero").is_null());
	fixture.create_clip("mixed", fixture.png_path, fixture.png_path);
	CHECK(fixture.resources->get_anim_frames("ReviewSprite::mixed").is_valid());
}

TEST_CASE("[SceneTree][SPX] Visual switches commit raster scale for forward "
		  "and backward playback") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	VisualFixture fixture;
	fixture.create_clip("svg", fixture.svg_path, fixture.svg_path);
	fixture.create_clip("bitmap", fixture.png_path, fixture.png_path);
	SpxSprite *sprite = fixture.create_sprite();
	const CharString png_path = fixture.png_path.utf8();
	sprite->set_texture(png_path.get_data());
	sprite->set_render_scale(Vector2(2, 2));
	sprite->play_anim("svg", 1.75, false, false);
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(1, 1));
	CHECK(sprite->get_anim2d()
					->get_sprite_frames()
					->get_frame_texture("ReviewSprite::svg", 0)
					->get_size() == Vector2(16, 12));
	sprite->play_backwards_anim("bitmap");
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(2, 2));
	CHECK_EQ(sprite->get_anim_frame(), 1);
	CHECK(sprite->get_anim_playing_speed() == doctest::Approx(-1));
	sprite->play_backwards_anim("svg");
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(1, 1));
	CHECK_EQ(sprite->get_anim_frame(), 1);
	sprite->set_anim("bitmap");
	CHECK_EQ(sprite->get_anim_frame(), 1);
	CHECK(sprite->get_anim_playing_speed() == doctest::Approx(-1));
	sprite->set_anim("svg");

	sprite->get_anim2d()->set_frame_and_progress(1, 0.4);
	sprite->pause_anim();
	sprite->set_anim_speed_scale(0);
	sprite->set_render_scale(Vector2(4, 4));
	CHECK_FALSE(sprite->is_playing_anim());
	CHECK_EQ(sprite->get_anim_frame(), 1);
	CHECK(sprite->get_anim2d()->get_frame_progress() == doctest::Approx(0.4));
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(1, 1));
	CHECK(sprite->get_anim2d()
					->get_sprite_frames()
					->get_frame_texture("ReviewSprite::svg", 0)
					->get_size() == Vector2(32, 24));
}

TEST_CASE("[SceneTree][SPX] Failed visual preparation preserves source and "
		  "actual raster scale") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	VisualFixture fixture;
	fixture.create_clip("svg", fixture.svg_path, fixture.svg_path);
	SpxSprite *sprite = fixture.create_sprite();
	sprite->play_anim("svg", 1, false, false);
	const Ref<SpriteFrames> original = sprite->get_anim2d()->get_sprite_frames();
	sprite->get_anim2d()->set_frame_and_progress(1, 0.4);
	const CharString broken_path = fixture.retry_path.utf8();
	const CharString broken_bitmap = fixture.png_path.utf8();
	VisualFixture::write_text(fixture.png_path, "invalid PNG");
	ERR_PRINT_OFF
	sprite->set_texture(broken_bitmap.get_data());
	sprite->set_texture(broken_path.get_data());
	sprite->play_anim("missing", 1, true, false);
	sprite->play_backwards_anim("missing");
	ERR_PRINT_ON
	CHECK(sprite->get_anim2d()->get_sprite_frames() == original);
	CHECK(sprite->get_anim2d()->get_animation() ==
			StringName("ReviewSprite::svg"));
	CHECK_EQ(sprite->get_anim_frame(), 1);
	CHECK(sprite->is_playing_anim());
	CHECK_FALSE(original->get_animation_loop("ReviewSprite::svg"));

	// The cached 1x source stays valid while the requested 2x source fails.
	VisualFixture::write_text(fixture.svg_path, "invalid SVG");
	ERR_PRINT_OFF
	sprite->set_render_scale(Vector2(2, 2));
	ERR_PRINT_ON
	CHECK(sprite->get_anim2d()->get_sprite_frames() == original);
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(2, 2));
	VisualFixture::write_svg(fixture.svg_path);
	sprite->set_render_scale(Vector2(2, 2));
	CHECK(sprite->get_anim2d()->get_sprite_frames() != original);
	CHECK(sprite->get_anim2d()->get_scale() == Vector2(1, 1));
	CHECK_EQ(sprite->get_anim_frame(), 1);
	CHECK(sprite->get_anim2d()->get_frame_progress() == doctest::Approx(0.4));
	CHECK_FALSE(sprite->get_anim2d()->get_sprite_frames()->get_animation_loop(
			"ReviewSprite::svg"));
}

TEST_CASE("[SceneTree][SPX] Texture reload normalizes paths and preserves "
		  "existing references on failure") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	VisualFixture fixture;
	fixture.resources->set_game_datas(fixture.png_path.get_base_dir(),
			Vector<String>());
	const Ref<Texture2D> texture =
			fixture.resources->load_texture("spx_visual.png");
	REQUIRE(texture.is_valid());
	Ref<Image> replacement =
			Image::create_empty(12, 10, false, Image::FORMAT_RGBA8);
	replacement->fill(Color(0, 0, 1));
	REQUIRE(replacement->save_png(fixture.png_path) == OK);
	fixture.resources->reload_texture("spx_visual.png");
	CHECK(texture == fixture.resources->load_texture("spx_visual.png"));
	CHECK(texture->get_size() == Vector2(12, 10));
	VisualFixture::write_text(fixture.png_path, "invalid PNG");
	ERR_PRINT_OFF
	fixture.resources->reload_texture("spx_visual.png");
	ERR_PRINT_ON
	CHECK(texture->get_size() == Vector2(12, 10));
	CHECK(texture->get_image()->get_pixel(0, 0) == Color(0, 0, 1));

	SpxSprite *sprite = fixture.create_sprite();
	sprite->set_render_scale(Vector2(2, 2));
	sprite->set_texture("spx_visual.svg");
	const Ref<Texture2D> svg_texture =
			sprite->get_anim2d()->get_sprite_frames()->get_frame_texture("default",
					0);
	REQUIRE(svg_texture.is_valid());
	VisualFixture::write_text(
			fixture.svg_path,
			"<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"12\" "
			"height=\"10\"><rect width=\"12\" height=\"10\" fill=\"blue\"/></svg>");
	fixture.resources->reload_texture("spx_visual.svg");
	CHECK(svg_texture ==
			sprite->get_anim2d()->get_sprite_frames()->get_frame_texture("default",
					0));
	CHECK(svg_texture->get_size() == Vector2(24, 20));
	VisualFixture::write_text(fixture.svg_path, "invalid SVG");
	ERR_PRINT_OFF
	fixture.resources->reload_texture("spx_visual.svg");
	ERR_PRINT_ON
	CHECK(svg_texture->get_size() == Vector2(24, 20));
}

} // namespace TestSpxSpriteRenderUtil

#endif // TEST_SPX_SPRITE_RENDER_UTIL_H
