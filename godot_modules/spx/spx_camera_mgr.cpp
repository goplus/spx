/**************************************************************************/
/*  spx_camera_mgr.cpp                                                    */
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

#include "spx_camera_mgr.h"

#include "scene/2d/camera_2d.h"
#include "scene/main/window.h"

#include "spx_coordinate.h"

void SpxCameraMgr::on_awake() {
	SpxBaseMgr::on_awake();
	camera = nullptr;
	auto nodes = get_root()->find_children("*", "Camera2D", true, false);
	for (int i = 0; i < nodes.size(); i++) {
		camera = Object::cast_to<Camera2D>(nodes[i]);
		if (camera != nullptr) {
			break;
		}
	}

	if (camera == nullptr) {
		camera = memnew(Camera2D);
		camera->set_name("SpxCamera2D");
		get_spx_root()->add_child(camera);
	}
}

void SpxCameraMgr::on_reset(int reset_code) {
	if (camera) {
		camera->set_position(Point2(0.0, 0.0));
	}
}

Vector2 SpxCameraMgr::get_global_mouse_position() {
	return camera->get_global_mouse_position();
}

void SpxCameraMgr::set_stretch_clear_color() {
	Viewport *vp = camera->get_viewport();
	vp->set_transparent_background(true);
	vp->set_use_hdr_2d(false);
	RenderingServer::get_singleton()->set_default_clear_color(Color(0, 0, 0, 0));
}

GdVec2 SpxCameraMgr::get_camera_position() {
	return godot_to_spx_vec2(camera->get_position());
}

void SpxCameraMgr::set_camera_position(GdVec2 position) {
	camera->set_position(spx_to_godot_vec2(position));
}

GdVec2 SpxCameraMgr::get_camera_zoom() {
	return camera->get_zoom();
}

void SpxCameraMgr::set_camera_zoom(GdVec2 size) {
	camera->set_zoom(size);
}

GdRect2 SpxCameraMgr::get_viewport_rect() {
	return camera->get_viewport_rect();
}

GdRect2 SpxCameraMgr::get_global_camera_rect() {
	Viewport *vp = camera->get_viewport();
	Transform2D screen_to_world = vp->get_canvas_transform().affine_inverse();

	Vector2 vp_size = vp->get_visible_rect().size;
	Vector2 tl = screen_to_world.xform(Vector2(0, 0));
	Vector2 br = screen_to_world.xform(vp_size);

	return Rect2(tl, br - tl);
}

GdRect2 SpxCameraMgr::get_stage_limits_rect() {
	if (!camera) {
		return Rect2();
	}

	real_t left = camera->get_limit(SIDE_LEFT);
	real_t top = camera->get_limit(SIDE_TOP);
	real_t right = camera->get_limit(SIDE_RIGHT);
	real_t bottom = camera->get_limit(SIDE_BOTTOM);

	return Rect2(left, top, right - left, bottom - top);
}

void SpxCameraMgr::set_camera_limit(GdInt side, GdInt limit) {
	camera->set_limit((Side)side, limit);
}

void SpxCameraMgr::set_camera_smoothing(GdBool enabled) {
	camera->set_position_smoothing_enabled(enabled);
}
