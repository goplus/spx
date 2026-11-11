/**************************************************************************/
/*  test_spx_spine.h                                                      */
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
/* MERCHANTABILITY AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR  */
/* COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, */
/* WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT */
/* OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN  */
/* THE SOFTWARE.                                                          */
/**************************************************************************/

#ifndef TEST_SPX_SPINE_H
#define TEST_SPX_SPINE_H

#include "../spx_build_config.gen.h"
#include "../spx_sprite_mgr.h"
#include "../spx_sprite.h"
#include "spx_test_fixtures.h"

#ifndef SPX_SPINE_ENABLED
TEST_CASE("[SceneTree][SPX] Spine is disabled by default without changing sprite state") {
	TestSpx::EngineScene scene;
	CHECK_FALSE(SpxSpriteMgr::is_spine_supported());
	const GdObj id = scene.engine->get_sprite()->create_bare_sprite(Vector2());
	SpxSprite *sprite = scene.engine->get_sprite()->get_sprite(id);
	ERR_PRINT_OFF;
	CHECK_FALSE(scene.engine->get_sprite()->set_spine_skeleton(id, "missing.atlas", "missing.json", 0.1));
	ERR_PRINT_ON;
	CHECK_FALSE(sprite->is_spine_mode());
	CHECK_FALSE(scene.engine->get_sprite()->has_spine_animation(id, "walk"));
	CHECK(Rect2(scene.engine->get_sprite()->get_spine_bounds(id)) == Rect2());
	scene.engine->get_sprite()->clear_spine_skeleton(id);
	CHECK_FALSE(ClassDB::class_exists("SpineSprite"));
}
#endif

#ifdef SPX_SPINE_ENABLED

#include "../spx_res_mgr.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "../thirdparty/spine_godot/SpineAnimationState.h"
#include "../thirdparty/spine_godot/SpineSkeleton.h"
#include "../thirdparty/spine_godot/SpineSprite.h"
#include "../thirdparty/spine_godot/SpineTrackEntry.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/resources/material.h"
#include "scene/resources/shader.h"
#include "core/io/json.h"
#include "spx_test_fixtures.h"

namespace TestSpxSpine {

static int looped = 0;
static int finished = 0;

struct Assets : TestSpx::TempDirectory {
	Assets() :
			TempDirectory("spx_spine") {
		write_png("quad.png", Vector2i(16, 8), Color(1, 1, 1));
		write("quad.atlas", "scale: 1\nquad.png\nsize: 16,8\nformat: RGBA8888\nfilter: Linear,Linear\nrepeat: none\nquad\n  bounds: 0,0,16,8\n");
		write("quad.json", R"({"skeleton":{"spine":"4.2.99"},"bones":[{"name":"root"}],"slots":[{"name":"slot","bone":"root","attachment":"quad"}],"skins":[{"name":"default","attachments":{"slot":{"quad":{"type":"region","width":16,"height":8}}}}],"animations":{"move":{"bones":{"root":{"translate":[{"x":0,"y":0},{"time":1,"x":4,"y":0}]}}}}})");
	}
};

static SpineSprite *get_child(SpxSprite *p_sprite) {
	return Object::cast_to<SpineSprite>(p_sprite->get_node_or_null(NodePath("RenderRoot/Spine")));
}

TEST_CASE("[SceneTree][SPX] Spine playback resumes, loops and keeps its pose when resized") {
	TestSpx::EngineScene scene(TestSpx::EngineScene::FULL_RUNTIME);
	looped = finished = 0;
	scene.engine->get_callbacks()->func_on_sprite_animation_looped = [](GdObj) { ++looped; };
	scene.engine->get_callbacks()->func_on_sprite_animation_finished = [](GdObj) { ++finished; };
	Assets files;
	SpxSpriteMgr *manager = scene.engine->get_sprite();
	GdObj id = manager->create_bare_sprite(Vector2());
	SpxSprite *sprite = manager->get_sprite(id);
	REQUIRE(sprite->set_spine_skeleton(files.file("quad.atlas").utf8().get_data(), files.file("quad.json").utf8().get_data(), 0.1f));
	CHECK(SpxSpriteMgr::is_spine_supported());
	SpineSprite *child = get_child(sprite);
	REQUIRE(child != nullptr);
	child->set_update_mode(SpineConstant::UpdateMode_Manual);
	CHECK_FALSE(sprite->get_anim2d()->is_visible());
	CHECK(sprite->get_spine_bounds().size == Vector2(16, 8));
	Ref<SpineAtlasResource> relative_atlas;
	relative_atlas.instantiate();
	Ref<DirAccess> cwd = DirAccess::open(".");
	REQUIRE(cwd.is_valid());
	const String relative_path = cwd->get_current_dir().path_to_file(files.file("quad.atlas"));
	CHECK(relative_atlas->load_from_atlas_file(relative_path) == OK);

	CHECK(manager->has_spine_animation(id, "move"));
	CHECK_FALSE(manager->has_spine_animation(id, "missing"));
	Ref<Shader> shader;
	shader.instantiate();
	shader->set_code("shader_type canvas_item; uniform vec4 color = vec4(1.0); uniform float alpha_amount = 0.0;");
	Ref<ShaderMaterial> material;
	material.instantiate();
	material->set_shader(shader);
	sprite->get_anim2d()->set_material(material);
	sprite->set_color(Color(0.5, 0.25, 1, 0.8));
	sprite->set_material_params("alpha_amount", 0.5);
	CHECK(child->get_modulate().r == doctest::Approx(0.5));
	CHECK(child->get_modulate().a == doctest::Approx(0.4));
	sprite->set_color(Color(1, 1, 1, 0.6));
	CHECK(child->get_modulate().a == doctest::Approx(0.3));
	sprite->set_material_params("alpha_amount", 0);
	CHECK(child->get_modulate().a == doctest::Approx(0.6));

	sprite->play_anim("move", 1, true, false);
	child->update_skeleton(1.25f);
	CHECK(looped == 1);
	CHECK(finished == 0);
	CHECK(sprite->is_playing_anim());
	CHECK(TestSpx::take_string(sprite->get_anim()) == "move");
	CHECK(TestSpx::take_string(sprite->get_current_anim_name()) == "move");
	Ref<SpineTrackEntry> entry = child->get_animation_state()->get_current(0);
	const float time = entry->get_track_time();
	sprite->pause_anim();
	CHECK_FALSE(sprite->is_playing_anim());
	child->update_skeleton(0.25f);
	CHECK(entry->get_track_time() == doctest::Approx(time));
	sprite->play_anim("move", 2, true, false);
	child->update_skeleton(0.1f);
	CHECK(sprite->is_playing_anim());
	CHECK(entry->get_track_time() > time);
	const float before_resize = entry->get_track_time();
	const Rect2 pose = child->get_skeleton()->get_bounds();
	sprite->set_render_scale(Vector2(2, 3));
	CHECK(child->get_scale() == Vector2(2, 3));
	CHECK(entry->get_track_time() == doctest::Approx(before_resize));
	CHECK(child->get_skeleton()->get_bounds() == pose);
	sprite->set_anim_flip_h(true);
	CHECK(child->get_scale() == Vector2(-2, 3));
	sprite->stop_anim();
	CHECK_FALSE(sprite->is_playing_anim());
	sprite->play_anim("move", 1, false, false);
	child->update_skeleton(1.1f);
	CHECK(finished == 1);
	CHECK_FALSE(sprite->is_playing_anim());
	sprite->hide();
	sprite->play_anim("move", 1, false, false);
	child->update_skeleton(1.1f);
	CHECK(finished == 2);
	CHECK_FALSE(sprite->is_playing_anim());
	sprite->play_anim("move", 1, true, false);
	child->update_skeleton(1.1f);
	CHECK(looped == 2);
	CHECK(sprite->is_playing_anim());
	sprite->show();
	sprite->clear_spine_skeleton();
	CHECK_FALSE(sprite->is_spine_mode());
	CHECK(sprite->get_anim2d()->is_visible());
	CHECK(sprite->get_spine_bounds() == Rect2());
	SceneTree::get_singleton()->process(0);
}

TEST_CASE("[SceneTree][SPX] Spine speed survives pause, skeleton replacement and frame playback") {
	TestSpx::EngineScene scene;
	Assets files;
	const CharString atlas_path = files.file("quad.atlas").utf8();
	const CharString skeleton_path = files.file("quad.json").utf8();
	SpxSpriteMgr *manager = scene.engine->get_sprite();
	for (float speed : { 2.5f, -1.5f, 0.0f }) {
		CAPTURE(speed);
		const GdObj id = manager->create_bare_sprite(Vector2());
		SpxSprite *sprite = manager->get_sprite(id);
		AnimatedSprite2D *animated = sprite->get_anim2d();
		Ref<SpriteFrames> frames = animated->get_sprite_frames();
		frames->add_animation("frames");
		frames->add_frame("frames", scene.engine->get_res()->load_texture(files.file("quad.png")));
		manager->set_anim_speed_scale(id, 0.5f);
		REQUIRE(sprite->set_spine_skeleton(atlas_path.get_data(), skeleton_path.get_data(), 0.1f));
		CHECK(manager->get_anim_speed_scale(id) == doctest::Approx(0.5f));
		SpineSprite *child = get_child(sprite);
		REQUIRE(child != nullptr);
		child->set_update_mode(SpineConstant::UpdateMode_Manual);

		manager->play_anim(id, "move", -2.0f, true, false);
		manager->set_anim_speed_scale(id, speed);
		Ref<SpineTrackEntry> entry = child->get_animation_state()->get_current(0);
		REQUIRE(entry.is_valid());
		CHECK(manager->get_anim_speed_scale(id) == doctest::Approx(speed));
		CHECK(animated->get_speed_scale() == doctest::Approx(speed));
		CHECK(manager->get_anim_playing_speed(id) == doctest::Approx(-2.0f * speed));
		CHECK(child->get_time_scale() == doctest::Approx(Math::abs(speed)));
		CHECK(entry->get_reverse() == (speed > 0.0f));
		child->update_skeleton(0.1f);
		CHECK(entry->get_track_time() == doctest::Approx(0.2f * Math::abs(speed)));
		CHECK(manager->is_playing_anim(id));

		manager->pause_anim(id);
		const float paused_time = entry->get_track_time();
		manager->set_anim_speed_scale(id, -speed);
		CHECK_FALSE(manager->is_playing_anim(id));
		CHECK(manager->get_anim_playing_speed(id) == 0.0f);
		CHECK(child->get_time_scale() == 0.0f);
		child->update_skeleton(0.1f);
		CHECK(entry->get_track_time() == doctest::Approx(paused_time));
		manager->set_anim_speed_scale(id, speed);
		manager->play_anim(id, "move", 2.0f, true, false);
		CHECK(entry->get_track_time() == doctest::Approx(paused_time));
		CHECK(entry->get_reverse() == (speed < 0.0f));
		CHECK(manager->get_anim_playing_speed(id) == doctest::Approx(2.0f * speed));

		REQUIRE(sprite->set_spine_skeleton(atlas_path.get_data(), skeleton_path.get_data(), 0.3f));
		SpineSprite *replacement = get_child(sprite);
		REQUIRE(replacement != nullptr);
		CHECK(replacement != child);
		replacement->set_update_mode(SpineConstant::UpdateMode_Manual);
		CHECK_FALSE(manager->is_playing_anim(id));
		CHECK(manager->get_anim_speed_scale(id) == doctest::Approx(speed));
		manager->play_anim(id, "move", 2.0f, true, false);
		CHECK(replacement->get_time_scale() == doctest::Approx(Math::abs(speed)));
		CHECK(manager->get_anim_playing_speed(id) == doctest::Approx(2.0f * speed));

		manager->clear_spine_skeleton(id);
		CHECK_FALSE(sprite->is_spine_mode());
		CHECK(animated->is_visible());
		CHECK(manager->get_anim_speed_scale(id) == doctest::Approx(speed));
		manager->play_anim(id, "frames", 2.0f, true, false);
		CHECK(manager->is_playing_anim(id));
		CHECK(manager->get_anim_playing_speed(id) == doctest::Approx(2.0f * speed));
	}
	SceneTree::get_singleton()->process(0);
}

TEST_CASE("[SceneTree][SPX] Spine loads transactionally and keeps per-sprite mix independent") {
	TestSpx::EngineScene scene;
	Assets files;
	SpxResMgr *resources = scene.engine->get_res();
	Ref<SpineSkeletonDataResource> a = resources->load_spine_data(files.file("quad.atlas"), files.file("quad.json"), 0.1f);
	Ref<SpineSkeletonDataResource> b = resources->load_spine_data(files.file("quad.atlas"), files.file("quad.json"), 0.7f);
	REQUIRE(a.is_valid());
	REQUIRE(b.is_valid());
	CHECK(a->get_atlas_res() == b->get_atlas_res());
	CHECK(a->get_skeleton_file_res() == b->get_skeleton_file_res());
	CHECK(a->get_default_mix() == doctest::Approx(0.1f));
	CHECK(b->get_default_mix() == doctest::Approx(0.7f));
	resources->update_caches({ files.file("quad.png") });
	Ref<SpineSkeletonDataResource> fresh = resources->load_spine_data(files.file("quad.atlas"), files.file("quad.json"), 0.1f);
	REQUIRE(fresh.is_valid());
	CHECK(fresh->get_atlas_res() != a->get_atlas_res());
	GdObj id = scene.engine->get_sprite()->create_bare_sprite(Vector2());
	SpxSprite *sprite = scene.engine->get_sprite()->get_sprite(id);
	REQUIRE(sprite->set_spine_skeleton(files.file("quad.atlas").utf8().get_data(), files.file("quad.json").utf8().get_data(), 0.1f));
	SpineSprite *child = get_child(sprite);
	ERR_PRINT_OFF;
	CHECK_FALSE(sprite->set_spine_skeleton(files.file("missing.atlas").utf8().get_data(), files.file("quad.json").utf8().get_data(), 0.1f));
	ERR_PRINT_ON;
	CHECK(get_child(sprite) == child);
	CHECK(sprite->is_spine_mode());
}

TEST_CASE("[SceneTree][SPX] Spine clones own their skeleton and can replace it repeatedly") {
	TestSpx::EngineScene scene;
	Assets files;
	SpxSpriteMgr *manager = scene.engine->get_sprite();
	const GdObj original_id = manager->create_bare_sprite(Vector2());
	SpxSprite *original = manager->get_sprite(original_id);
	REQUIRE(original->set_spine_skeleton(files.file("quad.atlas").utf8().get_data(), files.file("quad.json").utf8().get_data(), 0.1f));
	original->set_anim_speed_scale(2.5f);
	original->play_anim("move", 1, true, false);
	SpineSprite *original_child = get_child(original);
	REQUIRE(original_child != nullptr);
	original_child->set_update_mode(SpineConstant::UpdateMode_Manual);
	original_child->update_skeleton(0.1f);
	const GdObj clone_id = manager->clone_sprite(original_id);
	SpxSprite *clone = manager->get_sprite(clone_id);
	REQUIRE(clone != nullptr);
	CHECK(clone->get_anim_speed_scale() == doctest::Approx(2.5f));
	for (int i = 0; i < 2; ++i) {
		REQUIRE(clone->set_spine_skeleton(files.file("quad.atlas").utf8().get_data(), files.file("quad.json").utf8().get_data(), 0.3f));
		SpineSprite *child = get_child(clone);
		REQUIRE(child != nullptr);
		CHECK(child != original_child);
		CHECK(clone->get_anim_speed_scale() == doctest::Approx(i == 0 ? 2.5f : -2.0f));
		clone->set_anim_speed_scale(-2.0f);
		CHECK(original->get_anim_speed_scale() == doctest::Approx(2.5f));
		CHECK(original_child->get_time_scale() == doctest::Approx(2.5f));
		child->set_update_mode(SpineConstant::UpdateMode_Manual);
		clone->play_anim("move", 1, false, false);
		child->update_skeleton(1.1f);
		CHECK_FALSE(clone->is_playing_anim());
		CHECK(original->is_playing_anim());
		CHECK(original_child->get_animation_state()->get_current(0)->get_track_time() == doctest::Approx(0.25f));
	}
	SceneTree::get_singleton()->process(0);
}

TEST_CASE("[SPX] Spine atlas rejects missing and escaping textures") {
	Assets files;
	Ref<SpineAtlasResource> atlas;
	atlas.instantiate();
	for (const String &page : { String("../quad.png"), String("sub/../quad.png"), String("..\\quad.png"), String("missing.png") }) {
		files.write("bad.atlas", page + "\nsize: 16,8\nformat: RGBA8888\nfilter: Linear,Linear\nrepeat: none\nquad\n  bounds: 0,0,16,8\n");
		ERR_PRINT_OFF;
		CHECK(atlas->load_from_atlas_file(files.file("bad.atlas")) != OK);
		ERR_PRINT_ON;
	}
}


TEST_CASE("[SPX] Spine imported atlas validates metadata and propagates failures") {
	TestSpx::TempDirectory files("spx_spine_import");
	Ref<SpineAtlasResource> atlas;
	atlas.instantiate();
	Dictionary content;
	content["source_path"] = "res://spx_spine_import/quad.atlas";
	content["atlas_data"] = "";
	content["normal_texture_prefix"] = "n";
	content["specular_texture_prefix"] = "s";
	files.write("quad.spatlas", JSON::stringify(content));
	CHECK(atlas->load_from_file(files.file("quad.spatlas"), "res://spx_spine_import/quad.atlas") == OK);
	CHECK(atlas->load_from_file(files.file("quad.spatlas"), "res://other/quad.atlas") == ERR_INVALID_DATA);
	SpineAtlasResourceFormatLoader loader;
	Error error = OK;
	CHECK(loader.load(files.file("quad.spatlas"), "res://other/quad.ATLAS", &error, false, nullptr, ResourceFormatLoader::CACHE_MODE_IGNORE).is_null());
	CHECK(error == ERR_INVALID_DATA);
	for (const String &source : { String("/tmp/quad.atlas"), String("res://../quad.atlas"), String("res://sub/../quad.atlas"), String("res://sub\\quad.atlas"), String("file://quad.atlas") }) {
		content["source_path"] = source;
		files.write("quad.spatlas", JSON::stringify(content));
		CHECK(atlas->load_from_file(files.file("quad.spatlas")) == ERR_INVALID_DATA);
	}
	content["source_path"] = "res://spx_spine_import/quad.atlas";
	content["atlas_data"] = "../outside.png\nsize: 16,8\n\n";
	files.write("quad.spatlas", JSON::stringify(content));
	CHECK(atlas->load_from_file(files.file("quad.spatlas")) != OK);
	files.write("quad.spatlas", "[]");
	error = OK;
	CHECK(loader.load(files.file("quad.spatlas"), "res://spx_spine_import/quad.atlas", &error, false, nullptr, ResourceFormatLoader::CACHE_MODE_IGNORE).is_null());
	CHECK(error == ERR_INVALID_DATA);
}

} // namespace TestSpxSpine
#endif // SPX_SPINE_ENABLED
#endif
