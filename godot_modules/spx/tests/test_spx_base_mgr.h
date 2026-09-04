/**************************************************************************/
/*  test_spx_base_mgr.h                                                   */
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

#ifndef TEST_SPX_BASE_MGR_H
#define TEST_SPX_BASE_MGR_H

#include "../spx_base_mgr.h"
#include "../web/godot_js_spx_util.h"
#include "tests/test_macros.h"

namespace TestSpxBaseMgr {

TEST_CASE("[SPX] ABI string arrays are safe when only partially populated") {
	GdArray array = SpxBaseMgr::create_array(GD_ARRAY_TYPE_STRING, 3);
	REQUIRE(array != nullptr);
	REQUIRE(array->data != nullptr);

	char **strings = static_cast<char **>(array->data);
	CHECK_EQ(strings[0], nullptr);
	CHECK_EQ(strings[1], nullptr);
	CHECK_EQ(strings[2], nullptr);

	GdString value = SpxBaseMgr::to_return_cstr("value");
	REQUIRE(value != nullptr);
	SpxBaseMgr::set_array<GdString>(array, 1, value);
	GdString *stored = SpxBaseMgr::get_array<GdString>(array, 1);
	REQUIRE(stored != nullptr);
	CHECK_EQ(*stored, value);
	SpxBaseMgr::free_array(array);
}

TEST_CASE("[SPX] ABI array access rejects type confusion and malformed storage") {
	CHECK(SpxBaseMgr::create_array(GD_ARRAY_TYPE_UNKNOWN, 0) == nullptr);
	CHECK(SpxBaseMgr::create_array(99, 0) == nullptr);

	GdArray floats = SpxBaseMgr::create_array(GD_ARRAY_TYPE_FLOAT, 2);
	REQUIRE(floats != nullptr);
	CHECK(SpxBaseMgr::get_array<float>(floats, 0) != nullptr);
	CHECK(SpxBaseMgr::get_array<int64_t>(floats, 0) == nullptr);
	CHECK(SpxBaseMgr::get_array<float>(floats, 2) == nullptr);
	SpxBaseMgr::free_array(floats);

	GdArrayInfo malformed = {};
	malformed.size = 1;
	malformed.type = GD_ARRAY_TYPE_FLOAT;
	malformed.data = nullptr;
	CHECK(SpxBaseMgr::get_array<float>(&malformed, 0) == nullptr);

	GdArray bytes = SpxBaseMgr::create_array(GD_ARRAY_TYPE_BYTE, 1);
	REQUIRE(bytes != nullptr);
	CHECK(SpxBaseMgr::get_array<uint8_t>(bytes, 0) != nullptr);
	CHECK(SpxBaseMgr::get_array<float>(bytes, 0) == nullptr);
	SpxBaseMgr::free_array(bytes);

	// GdInt and GdObj are the same 64-bit C ABI type, so either matching
	// 64-bit array tag must remain usable through the shared C++ typedef.
	GdArray objects = SpxBaseMgr::create_array(GD_ARRAY_TYPE_GDOBJ, 1);
	REQUIRE(objects != nullptr);
	CHECK(SpxBaseMgr::get_array<GdObj>(objects, 0) != nullptr);
	CHECK(SpxBaseMgr::get_array<GdInt>(objects, 0) != nullptr);
	SpxBaseMgr::free_array(objects);
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

} // namespace TestSpxBaseMgr

#endif // TEST_SPX_BASE_MGR_H
