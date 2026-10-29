/**************************************************************************/
/*  test_spx_manager.h                                                   */
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

#ifndef TEST_SPX_ABI_H
#define TEST_SPX_ABI_H

#include "../spx_abi.h"
#include "../web/godot_js_spx_util.h"
#include "core/string/print_string.h"
#include "tests/test_macros.h"

namespace TestSpxAbi {

TEST_CASE("[SPX] ABI string arrays are safe when only partially populated") {
	GdArray array = SpxAbi::create_array(GD_ARRAY_TYPE_STRING, 3);
	REQUIRE(array != nullptr);
	REQUIRE(array->data != nullptr);

	char **strings = static_cast<char **>(array->data);
	CHECK_EQ(strings[0], nullptr);
	CHECK_EQ(strings[1], nullptr);
	CHECK_EQ(strings[2], nullptr);

	GdString value = SpxAbi::to_return_cstr("value");
	REQUIRE(value != nullptr);
	SpxAbi::set_array<GdString>(array, 1, value);
	GdString *stored = SpxAbi::get_array<GdString>(array, 1);
	REQUIRE(stored != nullptr);
	CHECK_EQ(*stored, value);
	SpxAbi::free_array(array);
}

TEST_CASE("[SPX] ABI array access rejects type confusion and malformed storage") {
	CHECK(SpxAbi::create_array(GD_ARRAY_TYPE_BYTE, -1) == nullptr);
	CHECK(SpxAbi::create_array(GD_ARRAY_TYPE_BYTE, INT32_MAX) == nullptr);
	CHECK(SpxAbi::create_array(GD_ARRAY_TYPE_UNKNOWN, 0) == nullptr);
	CHECK(SpxAbi::create_array(99, 0) == nullptr);

	GdArray floats = SpxAbi::create_array(GD_ARRAY_TYPE_FLOAT, 2);
	REQUIRE(floats != nullptr);
	REQUIRE(SpxAbi::get_array<float>(floats, 0) != nullptr);
	SpxAbi::set_array<float>(floats, 0, 1.5f);
	SpxAbi::set_array<float>(floats, -1, 9.0f);
	SpxAbi::set_array<float>(floats, 2, 9.0f);
	CHECK(*SpxAbi::get_array<float>(floats, 0) == 1.5f);
	CHECK(SpxAbi::get_array<double>(floats, 0) == nullptr);
	CHECK(SpxAbi::get_array<int64_t>(floats, 0) == nullptr);
	CHECK(SpxAbi::get_array<float>(floats, 2) == nullptr);
	SpxAbi::free_array(floats);

	GdArrayInfo malformed = {};
	malformed.size = 1;
	malformed.type = GD_ARRAY_TYPE_FLOAT;
	malformed.data = nullptr;
	CHECK(SpxAbi::get_array<float>(&malformed, 0) == nullptr);

	GdArray bytes = SpxAbi::create_array(GD_ARRAY_TYPE_BYTE, 1);
	REQUIRE(bytes != nullptr);
	CHECK(SpxAbi::get_array<uint8_t>(bytes, 0) != nullptr);
	CHECK(SpxAbi::get_array<float>(bytes, 0) == nullptr);
	SpxAbi::free_array(bytes);

	// GdInt and GdObj are the same 64-bit C ABI type, so either matching
	// 64-bit array tag must remain usable through the shared C++ typedef.
	GdArray objects = SpxAbi::create_array(GD_ARRAY_TYPE_GDOBJ, 1);
	REQUIRE(objects != nullptr);
	CHECK(SpxAbi::get_array<GdObj>(objects, 0) != nullptr);
	CHECK(SpxAbi::get_array<GdInt>(objects, 0) != nullptr);
	SpxAbi::free_array(objects);
	SpxAbi::free_array(nullptr);
}

TEST_CASE("[SPX] Preallocated object pools keep ownership across reuse and expansion") {
	ObjectPool<int> pool(2);
	int *first = pool.acquire();
	int *second = pool.acquire();
	int *third = pool.acquire();
	REQUIRE(first != nullptr);
	REQUIRE(second != nullptr);
	REQUIRE(third != nullptr);
	CHECK(first != second);
	CHECK(second != third);
	CHECK(pool.is_active(first));
	CHECK(pool.is_active(second));
	CHECK(pool.is_active(third));
	CHECK_FALSE(pool.is_active(nullptr));
	pool.release(nullptr);
	pool.release(second);
	CHECK_FALSE(pool.is_active(second));
	CHECK(pool.acquire() == second);
	CHECK(pool.is_active(second));
}

TEST_CASE("[SPX] ABI empty arrays and invalid access preserve borrowed storage") {
	for (int type : { GD_ARRAY_TYPE_STRING, GD_ARRAY_TYPE_INT64, GD_ARRAY_TYPE_GDOBJ, GD_ARRAY_TYPE_FLOAT, GD_ARRAY_TYPE_BOOL, GD_ARRAY_TYPE_BYTE }) {
		GdArray empty = SpxAbi::create_array(type, 0);
		REQUIRE(empty != nullptr);
		CHECK(empty->size == 0);
		CHECK(empty->type == type);
		CHECK(empty->data == nullptr);
		CHECK(SpxAbi::get_array<GdString>(empty, 0) == nullptr);
		SpxAbi::free_array(empty);
	}

	GdInt values[] = { 17, 29 };
	GdArrayInfo borrowed = { 2, GD_ARRAY_TYPE_INT64, values };
	CHECK(SpxAbi::get_array<GdInt>(nullptr, 0) == nullptr);
	SpxAbi::set_array<GdInt>(nullptr, 0, 99);
	CHECK(SpxAbi::get_array<GdInt>(&borrowed, -1) == nullptr);
	CHECK(SpxAbi::get_array<GdInt>(&borrowed, INT64_MAX) == nullptr);
	CHECK(SpxAbi::_get_array(&borrowed, 0, 0, GD_ARRAY_TYPE_INT64) == nullptr);
	CHECK(SpxAbi::_get_array(&borrowed, 0, -1, GD_ARRAY_TYPE_INT64) == nullptr);
	SpxAbi::set_array<float>(&borrowed, 0, 99.0f);
	SpxAbi::set_array<uint32_t>(&borrowed, 0, 99);
	SpxAbi::set_array<GdString>(&borrowed, 0, "not an integer");
	for (int size : { -1, 0, INT32_MAX }) {
		borrowed.size = size;
		CHECK(SpxAbi::get_array<GdInt>(&borrowed, 0) == nullptr);
		SpxAbi::set_array<GdInt>(&borrowed, 0, 99);
	}
	borrowed.size = 2;
	borrowed.type = GD_ARRAY_TYPE_UNKNOWN;
	CHECK(SpxAbi::get_array<uint32_t>(&borrowed, 0) == nullptr);
	borrowed.type = GD_ARRAY_TYPE_INT64;
	CHECK(values[0] == 17);
	CHECK(values[1] == 29);
	SpxAbi::set_array<GdInt>(&borrowed, 1, 31);
	CHECK(values[0] == 17);
	CHECK(values[1] == 31);
	CHECK(SpxAbi::get_array<const GdInt>(&borrowed, 1) == &values[1]);
}

TEST_CASE("[SPX] ObjectPool rejects foreign and duplicate pointers") {
	ObjectPool<int> pool(0);
	int *owned = pool.acquire();
	REQUIRE(owned != nullptr);
	int *foreign = new int(0);

	pool.release(owned);
	pool.release(foreign);
	int *reacquired = pool.acquire();
	CHECK(reacquired == owned);
	pool.release(reacquired);
	pool.release(reacquired);

	if (reacquired != foreign) {
		delete foreign;
	}
}

} // namespace TestSpxAbi

#endif // TEST_SPX_ABI_H
