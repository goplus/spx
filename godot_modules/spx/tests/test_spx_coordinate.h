/**************************************************************************/
/*  test_spx_coordinate.h                                                 */
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

#ifndef TEST_SPX_COORDINATE_H
#define TEST_SPX_COORDINATE_H

#include "../spx_coordinate.h"
#include "tests/test_macros.h"

namespace TestSpxCoordinate {

TEST_CASE("[SPX] Coordinate conversion reflects only the Y axis") {
	CHECK(spx_to_godot_vec2(Vector2(12.5, 7.25)) == Vector2(12.5, -7.25));
	CHECK(godot_to_spx_vec2(Vector2(-3.5, 9.0)) == Vector2(-3.5, -9.0));
}

TEST_CASE("[SPX] Coordinate conversion round trips") {
	const Vector2 position(-123.25, 456.75);
	CHECK(godot_to_spx_vec2(spx_to_godot_vec2(position)) == position);
}

} // namespace TestSpxCoordinate

#endif // TEST_SPX_COORDINATE_H
