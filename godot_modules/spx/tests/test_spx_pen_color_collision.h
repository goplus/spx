/**************************************************************************/
/*  test_spx_pen_color_collision.h                                       */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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
/* SOFTWARE.                                                              */
/**************************************************************************/

#ifndef TEST_SPX_PEN_COLOR_COLLISION_H
#define TEST_SPX_PEN_COLOR_COLLISION_H

#include "../spx_engine.h"
#include "../spx_pen_mgr.h"
#include "../spx_pen_surface.h"
#include "../spx_sprite.h"
#include "../spx_sprite_mgr.h"
#include "core/io/image.h"
#include "scene/2d/physics/area_2d.h"
#include "scene/2d/physics/collision_shape_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "scene/resources/2d/circle_shape_2d.h"
#include "scene/resources/image_texture.h"
#include "scene/resources/sprite_frames.h"
#include "tests/test_macros.h"

class TestSpxPenCollisionInternalsAccessor {
public:
	static void install_image(SpxPenMgr *p_pen_mgr, const Ref<Image> &p_image) {
		if (p_pen_mgr->surface == nullptr) {
			p_pen_mgr->surface = memnew(SpxPenSurface);
			p_pen_mgr->surface->initialize(p_image->get_size());
		}
		p_pen_mgr->surface->collision_image = p_image;
		p_pen_mgr->surface->clear_requested = false;
		p_pen_mgr->surface->dirty = false;
	}

	static void draw_line(SpxPenMgr *p_pen_mgr, const Vector2 &p_from, const Vector2 &p_to, const Color &p_color) {
		p_pen_mgr->surface->draw_line(p_from, p_to, 1.0f, p_color, true);
	}

	static void remove_surface(SpxPenMgr *p_pen_mgr) {
		SpxPenSurface *surface = p_pen_mgr->surface;
		p_pen_mgr->surface = nullptr;
		if (surface != nullptr) {
			memdelete(surface);
		}
	}
};

namespace TestSpxPenColorCollision {

class SpriteMgrProbe : public SpxSpriteMgr {
public:
	void register_sprite(SpxSprite *p_sprite) { _register_sprite(p_sprite); }
};

struct EngineScope {
	SpxEngine *engine = nullptr;

	EngineScope() {
		SpxEngine::register_callbacks(nullptr);
		engine = SpxEngine::get_singleton();
	}

	~EngineScope() {
		if (engine != nullptr && SpxEngine::get_singleton() == engine) {
			TestSpxPenCollisionInternalsAccessor::remove_surface(engine->get_pen());
			SpxEngine::shutdown();
		}
	}
};

static Ref<Image> create_solid_image(const Size2i &p_size, const Color &p_color) {
	Ref<Image> image = memnew(Image(p_size.x, p_size.y, false, Image::FORMAT_RGBA8));
	image->fill(p_color);
	return image;
}

static SpxSprite *create_colored_sprite(SpriteMgrProbe &p_manager, GdObj p_id, const Color &p_color, bool p_backdrop = false) {
	SpxSprite *sprite = memnew(SpxSprite);
	sprite->set_gid(p_id);
	sprite->set_backdrop(p_backdrop);

	Node2D *render_root = memnew(Node2D);
	render_root->set_name("RenderRoot");
	sprite->add_child(render_root);

	AnimatedSprite2D *anim = memnew(AnimatedSprite2D);
	anim->set_name("Anim2D");
	render_root->add_child(anim);

	Area2D *area = memnew(Area2D);
	area->set_name("Area2D");
	sprite->add_child(area);

	CollisionShape2D *trigger = memnew(CollisionShape2D);
	trigger->set_name("Trigger2D");
	Ref<CircleShape2D> trigger_shape = memnew(CircleShape2D);
	trigger_shape->set_radius(1.0f);
	trigger->set_shape(trigger_shape);
	area->add_child(trigger);

	CollisionShape2D *collider = memnew(CollisionShape2D);
	collider->set_name("Collider2D");
	Ref<CircleShape2D> collider_shape = memnew(CircleShape2D);
	collider_shape->set_radius(1.0f);
	collider->set_shape(collider_shape);
	sprite->add_child(collider);

	SceneTree::get_singleton()->get_root()->add_child(sprite);
	sprite->on_start();

	Ref<SpriteFrames> frames = sprite->get_anim2d()->get_sprite_frames();
	frames->clear("default");
	frames->add_frame("default", ImageTexture::create_from_image(create_solid_image(Size2i(2, 2), p_color)));
	sprite->get_anim2d()->set_animation("default");
	p_manager.register_sprite(sprite);
	return sprite;
}

TEST_CASE("[SceneTree][SPX] Pen pixels participate in scene color collision at the rendered layer") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	EngineScope engine_scope;
	REQUIRE(engine_scope.engine != nullptr);

	const Color black(0, 0, 0, 1);
	const Color green(0, 1, 0, 1);
	const Color red(1, 0, 0, 1);
	TestSpxPenCollisionInternalsAccessor::install_image(engine_scope.engine->get_pen(), create_solid_image(Size2i(8, 8), black));

	SpriteMgrProbe manager;
	manager.set_pixel_collision_sampling_step(1);
	SpxSprite *subject = create_colored_sprite(manager, 1, Color(1, 1, 1, 1));

	CHECK(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	SpxSprite *backdrop = create_colored_sprite(manager, 2, green, true);
	backdrop->set_z_index(0);
	CHECK(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), green, 0.01f, 0.05f));

	SpxSprite *foreground = create_colored_sprite(manager, 3, red);
	foreground->set_z_index(0);
	CHECK(manager.check_collision_by_color(subject->get_gid(), red, 0.01f, 0.05f));
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	manager.destroy_sprite(subject->get_gid());
	manager.destroy_sprite(backdrop->get_gid());
	manager.destroy_sprite(foreground->get_gid());
}

TEST_CASE("[SceneTree][SPX] Pending pen clear hides the previous collision image") {
	REQUIRE_FALSE(SpxEngine::is_initialized());
	EngineScope engine_scope;
	REQUIRE(engine_scope.engine != nullptr);

	const Color black(0, 0, 0, 1);
	const Color red(1, 0, 0, 1);
	SpxPenMgr *pen_mgr = engine_scope.engine->get_pen();
	TestSpxPenCollisionInternalsAccessor::install_image(pen_mgr, create_solid_image(Size2i(8, 8), black));

	SpriteMgrProbe manager;
	manager.set_pixel_collision_sampling_step(1);
	SpxSprite *subject = create_colored_sprite(manager, 1, Color(1, 1, 1, 1));
	CHECK(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	pen_mgr->destroy_all_pens();
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	TestSpxPenCollisionInternalsAccessor::draw_line(pen_mgr, Vector2(-4, 0), Vector2(4, 0), red);
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	manager.destroy_sprite(subject->get_gid());
}

} // namespace TestSpxPenColorCollision

#endif // TEST_SPX_PEN_COLOR_COLLISION_H
