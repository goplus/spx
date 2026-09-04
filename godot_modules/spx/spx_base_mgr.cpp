/**************************************************************************/
/*  spx_base_mgr.cpp                                                      */
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

#include "spx_base_mgr.h"
#include <limits>

#include <cstdlib>
#include <cstring>

#include "scene/2d/node_2d.h"
#include "scene/main/window.h"

#include "spx_engine.h"

#ifdef __EMSCRIPTEN__
extern "C" bool gdspx_register_array_info(GdArray array);
extern "C" bool gdspx_release_array_info(GdArray array);
extern "C" bool gdspx_validate_array_info(GdArray array);
#endif

namespace {

constexpr int32_t SPX_MAX_ARRAY_ELEMENTS = 16 * 1024 * 1024;
constexpr size_t SPX_MAX_ARRAY_BYTES = 256 * 1024 * 1024;

} // namespace

Node *SpxBaseMgr::create_owner_node() {
	return memnew(Node2D);
}

GdInt SpxBaseMgr::get_unique_id() {
	return SpxEngine::get_singleton()->get_unique_id();
}

void SpxBaseMgr::free_return_cstr(GdString str_ptr) {
	free((void *)str_ptr);
}

GdString SpxBaseMgr::to_return_cstr(const String &ret_val) {
	auto cstr = ret_val.utf8();
	char *result = (char *)malloc(cstr.size() + 1);
	if (result == nullptr) {
		return nullptr;
	}
	strcpy(result, cstr.get_data());
	return result;
}

Window *SpxBaseMgr::get_root() {
	return SpxEngine::get_singleton()->get_root();
}

Node *SpxBaseMgr::get_spx_root() {
	return SpxEngine::get_singleton()->get_spx_root();
}

SceneTree *SpxBaseMgr::get_tree() {
	return SpxEngine::get_singleton()->get_tree();
}

void SpxBaseMgr::on_awake() {
	owner = create_owner_node();
	if (owner == nullptr) {
		return;
	}
	owner->set_name(get_class_name());
	get_spx_root()->add_child(owner);
}

void SpxBaseMgr::on_start() {
}

void SpxBaseMgr::on_update(float delta) {
}

void SpxBaseMgr::on_fixed_update(float delta) {
}

void SpxBaseMgr::on_destroy() {
	if (owner != nullptr) {
		owner->queue_free();
		owner = nullptr;
	}
}

void SpxBaseMgr::on_reset(int reset_code) {
}

void SpxBaseMgr::on_exit(int exit_code) {
}

void SpxBaseMgr::on_pause() {
	// Default implementation - override in derived classes if needed
}

void SpxBaseMgr::on_resume() {
	// Default implementation - override in derived classes if needed
}

GdArray SpxBaseMgr::create_array(int32_t type, int32_t size) {
	if (size < 0 || size > SPX_MAX_ARRAY_ELEMENTS) {
		return nullptr;
	}

	constexpr auto get_element_size = [](int32_t array_type) -> size_t {
		switch (array_type) {
			case GD_ARRAY_TYPE_INT64:
				return sizeof(int64_t);
			case GD_ARRAY_TYPE_FLOAT:
				return sizeof(float);
			case GD_ARRAY_TYPE_BOOL:
				return sizeof(uint8_t);
			case GD_ARRAY_TYPE_STRING:
				return sizeof(char *);
			case GD_ARRAY_TYPE_BYTE:
				return sizeof(uint8_t);
			case GD_ARRAY_TYPE_GDOBJ:
				return sizeof(GdObj);
			default:
				return 0;
		}
	};
	const size_t element_size = get_element_size(type);
	if (element_size == 0 || static_cast<size_t>(size) > std::numeric_limits<size_t>::max() / element_size ||
			static_cast<size_t>(size) * element_size > SPX_MAX_ARRAY_BYTES) {
		return nullptr;
	}

	GdArray array = (GdArray)malloc(sizeof(GdArrayInfo));
	if (!array) {
		return nullptr;
	}

	array->size = size;
	array->type = type;

	if (size == 0) {
		array->data = nullptr;
#ifdef __EMSCRIPTEN__
		if (!gdspx_register_array_info(array)) {
			free(array);
			return nullptr;
		}
#endif
		return array;
	}

	// String arrays are released slot-by-slot. Zero initialization guarantees
	// partially populated arrays never attempt to free indeterminate pointers;
	// calloc also rejects a size multiplication overflow.
	array->data = calloc(static_cast<size_t>(size), element_size);
	if (!array->data) {
		free(array);
		return nullptr;
	}

#ifdef __EMSCRIPTEN__
	if (!gdspx_register_array_info(array)) {
		free(array->data);
		free(array);
		return nullptr;
	}
#endif

	return array;
}

void SpxBaseMgr::free_array(GdArray array) {
	if (!array) {
		return;
	}
#ifdef __EMSCRIPTEN__
	// Web allocations are owned by the private metadata registry. Unknown
	// pointers fail closed instead of supplying mutable free parameters.
	gdspx_release_array_info(array);
#else
	// Special handling for string arrays - need to free each string
	if (array->type == GD_ARRAY_TYPE_STRING && array->data && array->size > 0 &&
			array->size <= SPX_MAX_ARRAY_ELEMENTS) {
		char **strings = (char **)array->data;
		for (int64_t i = 0; i < array->size; i++) {
			if (strings[i]) {
				free(strings[i]);
			}
		}
	}

	if (array->data) {
		free(array->data);
	}
	free(array);
#endif
}

void *SpxBaseMgr::_get_array(GdArray array, int64_t index, int type_size, int32_t expected_type) {
#ifdef __EMSCRIPTEN__
	if (!gdspx_validate_array_info(array)) {
		return nullptr;
	}
#endif
	if (!array || index < 0 || array->size <= 0 || array->size > SPX_MAX_ARRAY_ELEMENTS || index >= array->size ||
			type_size <= 0 || array->data == nullptr) {
		return nullptr;
	}

	const bool compatible_type =
			array->type == expected_type ||
			(expected_type == GD_ARRAY_TYPE_INT64 && array->type == GD_ARRAY_TYPE_GDOBJ) ||
			(expected_type == GD_ARRAY_TYPE_BOOL && array->type == GD_ARRAY_TYPE_BYTE);
	if (!compatible_type) {
		return nullptr;
	}

	size_t element_size = 0;
	switch (array->type) {
		case GD_ARRAY_TYPE_INT64:
		case GD_ARRAY_TYPE_GDOBJ:
			element_size = sizeof(int64_t);
			break;
		case GD_ARRAY_TYPE_FLOAT:
			element_size = sizeof(float);
			break;
		case GD_ARRAY_TYPE_BOOL:
		case GD_ARRAY_TYPE_BYTE:
			element_size = sizeof(uint8_t);
			break;
		case GD_ARRAY_TYPE_STRING:
			element_size = sizeof(char *);
			break;
		default:
			return nullptr;
	}
	if (element_size != static_cast<size_t>(type_size) ||
			static_cast<uint64_t>(index) >
					std::numeric_limits<size_t>::max() / element_size) {
		return nullptr;
	}

	const size_t offset = static_cast<size_t>(index) * element_size;
	return static_cast<char *>(array->data) + offset;
}
