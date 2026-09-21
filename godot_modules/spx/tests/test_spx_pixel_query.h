/**************************************************************************/
/*  test_spx_pixel_query.h                                                    */
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

#ifndef TEST_SPX_PIXEL_QUERY_H
#define TEST_SPX_PIXEL_QUERY_H

#include "../spx_pixel_query.h"
#include "tests/test_macros.h"

namespace TestSpxPixelQuery {

TEST_CASE("[SPX] Pixel sampling keeps world-center origin at larger steps") {
	const Rect2i centers = SpxPixelQuery::pixel_centers(Rect2(-0.6, -0.6, 3.2, 3.2));
	Vector<Vector2> visited;
	CHECK_FALSE(SpxPixelQuery::any_pixel_center(centers, 2, [&](Vector2 point) {
		visited.push_back(point);
		return false;
	}));
	REQUIRE(visited.size() == 4);
	CHECK(visited[0] == Vector2(-0.5, -0.5));
	CHECK(visited[1] == Vector2(-0.5, 1.5));
	CHECK(visited[2] == Vector2(1.5, -0.5));
	CHECK(visited[3] == Vector2(1.5, 1.5));
}

TEST_CASE("[SPX] Pixel sampling applies flips after inverse transforms") {
	SpxPixelQuery::Snapshot snapshot;
	snapshot.image = Image::create_empty(2, 1, false, Image::FORMAT_RGBA8);
	snapshot.image->set_pixel(0, 0, Color(1, 0, 0, 1));
	snapshot.image->set_pixel(1, 0, Color(0, 1, 0, 1));
	snapshot.image_size = Vector2i(2, 1);
	snapshot.inverse_transform = Transform2D(0, Vector2(-10, 0));
	Color color;
	CHECK(SpxPixelQuery::sample(snapshot, Vector2(10.5, 0.5), color));
	CHECK(color == Color(1, 0, 0, 1));
	snapshot.flip_h = true;
	snapshot.collision_alpha_scale = 0.5;
	CHECK(SpxPixelQuery::sample_premultiplied(snapshot, Vector2(10.5, 0.5), color));
	CHECK(color == Color(0, 0.5, 0, 0.5));
	CHECK_FALSE(SpxPixelQuery::sample(snapshot, Vector2(12.5, 0.5), color));
}

} // namespace TestSpxPixelQuery

#endif // TEST_SPX_PIXEL_QUERY_H
