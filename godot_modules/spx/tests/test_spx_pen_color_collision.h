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

namespace TestSpxPenColorCollision {

class SpriteMgrProbe : public SpxSpriteMgr {
private:
	Ref<Image> pen_image;
	SpxPenSurface *pen_surface = nullptr;

protected:
	Ref<Image> _get_pen_collision_image() const override {
		return pen_surface != nullptr ? pen_surface->get_image() : pen_image;
	}

public:
	void register_sprite(SpxSprite *p_sprite) { _register_sprite(p_sprite); }
	void set_pen_image(const Ref<Image> &p_image) { pen_image = p_image; }
	void set_pen_surface(SpxPenSurface *p_surface) { pen_surface = p_surface; }
};

class PenSurfaceProbe : public SpxPenSurface {
	GDCLASS(PenSurfaceProbe, SpxPenSurface);

private:
	Ref<Image> readback_image;
	mutable int readback_count = 0;

protected:
	Ref<Image> _read_collision_image() const override {
		readback_count++;
		return readback_image;
	}

public:
	void set_readback_image(const Ref<Image> &p_image) { readback_image = p_image; }
	int get_readback_count() const { return readback_count; }
};

struct EngineScope {
	SpxEngine *engine = nullptr;

	EngineScope() {
		SpxEngine::register_callbacks(nullptr);
		engine = SpxEngine::get_singleton();
	}

	~EngineScope() {
		if (engine != nullptr && SpxEngine::get_singleton() == engine) {
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

	SpriteMgrProbe manager;
	manager.set_pixel_collision_sampling_step(1);
	manager.set_pen_image(create_solid_image(Size2i(8, 8), black));
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
	PenSurfaceProbe *pen_surface = memnew(PenSurfaceProbe);
	SceneTree::get_singleton()->get_root()->add_child(pen_surface);
	pen_surface->initialize(Size2i(8, 8));
	Ref<Image> black_pen_image = create_solid_image(Size2i(8, 8), black);
	pen_surface->set_readback_image(black_pen_image);
	pen_surface->draw_line(Vector2(-4, 0), Vector2(4, 0), 1.0f, black, true);
	pen_surface->flush();

	SpriteMgrProbe manager;
	manager.set_pixel_collision_sampling_step(1);
	manager.set_pen_surface(pen_surface);
	SpxSprite *subject = create_colored_sprite(manager, 1, Color(1, 1, 1, 1));
	CHECK(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));
	CHECK_EQ(pen_surface->get_readback_count(), 1);
	CHECK(pen_surface->get_image() == black_pen_image);
	CHECK_EQ(pen_surface->get_readback_count(), 1);

	pen_surface->clear();
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));

	pen_surface->draw_line(Vector2(-4, 0), Vector2(4, 0), 1.0f, red, true);
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));
	CHECK_EQ(pen_surface->get_readback_count(), 1);

	pen_surface->set_readback_image(create_solid_image(Size2i(8, 8), red));
	pen_surface->flush();
	CHECK_FALSE(manager.check_collision_by_color(subject->get_gid(), black, 0.01f, 0.05f));
	CHECK(manager.check_collision_by_color(subject->get_gid(), red, 0.01f, 0.05f));
	CHECK_EQ(pen_surface->get_readback_count(), 2);

	manager.destroy_sprite(subject->get_gid());
	pen_surface->queue_free();
}

} // namespace TestSpxPenColorCollision

#endif // TEST_SPX_PEN_COLOR_COLLISION_H
